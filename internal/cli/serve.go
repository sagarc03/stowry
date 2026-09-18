package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry/internal/config"
	"github.com/sagarc03/stowry/internal/database"
	"github.com/sagarc03/stowry/internal/handler"
	"github.com/sagarc03/stowry/internal/keybackend"
	"github.com/sagarc03/stowry/internal/middleware"
	"github.com/sagarc03/stowry/internal/service"
	"github.com/sagarc03/stowry/sign"
)

const (
	readTimeout     = 30 * time.Second
	writeTimeout    = 30 * time.Second
	idleTimeout     = 120 * time.Second
	shutdownTimeout = 30 * time.Second
)

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the HTTP server",
		RunE:  runServe,
	}

	cmd.Flags().Int("port", 5708, "HTTP server port")
	cmd.Flags().String("mode", "store", "server mode (store, static, spa)")

	return cmd
}

func runServe(cmd *cobra.Command, _ []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	// Checked here rather than in config.Load so the offline commands stay
	// usable on a config the server rejects, and so the warning it can emit
	// goes through the configured logger.
	if err := cfg.ValidateForServe(); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseConfig())
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	// An in-memory database is created empty by this process, so nothing could
	// have migrated it. A database on disk is migrated deliberately, by init.
	if cfg.Database.DSN == config.MemoryPath {
		if err := db.Migrate(ctx); err != nil {
			return fmt.Errorf("migrate database: %w", err)
		}
	}

	if err := db.Validate(ctx); err != nil {
		return fmt.Errorf("validate database schema: %w", err)
	}

	slog.Info("connected to database", "type", cfg.Database.Type)

	storage, err := openStorage(cfg.Storage.Path)
	if err != nil {
		return err
	}

	verifier, err := newVerifier(cfg)
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	handler.Register(&handler.Opts{
		Mode:          cfg.Server.Mode,
		ErrorDocument: cfg.Server.ErrorDocument,
		MaxUploadSize: cfg.Server.MaxUploadSize,
		Mux:           mux,
		Svc:           service.New(db, storage),
		Logger:        slog.Default(),
		ReadVerifier:  verifierWhen(cfg.Auth.Read.Private(), verifier),
		WriteVerifier: verifierWhen(cfg.Auth.Write.Private(), verifier),
		Middleware:    serverMiddleware(cfg),
	})

	return listenAndServe(ctx, cancel, cfg.Server.Port, mux)
}

// openStorage returns the object filesystem for path, which is a directory or
// config.MemoryPath.
//
// 0o700 is owner-only. A Kubernetes deployment that needs shared access should
// set fsGroup in securityContext and pre-create the directory with 0o750.
func openStorage(path string) (afero.Fs, error) {
	if path == config.MemoryPath {
		return afero.NewMemMapFs(), nil
	}

	if err := os.MkdirAll(path, 0o700); err != nil {
		return nil, fmt.Errorf("create storage directory: %w", err)
	}

	return afero.NewBasePathFs(afero.NewOsFs(), path), nil
}

func newVerifier(cfg *config.Config) (*sign.SignatureVerifier, error) {
	store, err := keybackend.NewSecretStore(cfg.KeysConfig())
	if err != nil {
		return nil, fmt.Errorf("create secret store: %w", err)
	}

	return sign.NewSignatureVerifier(sign.AuthConfig{AWS: cfg.AWSConfig()}, store), nil
}

// verifierWhen returns verifier only when the route it guards is private. A nil
// RequestVerifier leaves the route public, and a typed nil would not.
func verifierWhen(private bool, verifier *sign.SignatureVerifier) middleware.RequestVerifier {
	if !private {
		return nil
	}

	return verifier
}

// serverMiddleware is the chain every route runs through, outermost first.
// CORS comes before authentication because browsers omit credentials from a
// preflight, which a signature check would then reject.
func serverMiddleware(cfg *config.Config) []func(http.Handler) http.Handler {
	chain := []func(http.Handler) http.Handler{
		middleware.WithRequestID,
		func(next http.Handler) http.Handler {
			return middleware.WithLogging(slog.Default(), next)
		},
	}

	if cors, enabled := cfg.CORSConfig(); enabled {
		chain = append(chain, middleware.WithCORS(cors))
	}

	return chain
}

func listenAndServe(ctx context.Context, cancel context.CancelFunc, port int, mux *http.ServeMux) error {
	addr := fmt.Sprintf(":%d", port)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		IdleTimeout:  idleTimeout,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-sigCh:
		case <-ctx.Done():
			return
		}

		slog.Info("shutting down")

		shutdownCtx, stop := context.WithTimeout(context.Background(), shutdownTimeout)
		defer stop()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown", "error", err)
		}

		cancel()
	}()

	slog.Info("starting server", "addr", addr)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}

	return nil
}
