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

	"github.com/spf13/cobra"

	"github.com/sagarc03/stowry/internal/config"
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
		Use:     "serve",
		GroupID: groupServer,
		Short:   "Start the HTTP server",
		Long: `Start the HTTP server.

Store mode routes uploads, deletes and listing; static and spa serve reads only,
and reject auth.read private at startup because a browser cannot sign a request.

The default config keeps both the database and the objects in memory, so with no
configuration nothing survives the process. A storage directory that does not
exist is created, owner-only.

An in-memory database is migrated on startup since nothing else could have. A
database on disk is not: use --migrate here, or run 'stowry migrate' first.`,
		RunE: runServe,
	}

	d := config.Defaults()
	cmd.Flags().Int("port", 0, usage("port", "HTTP server port", d.Server.Port))
	cmd.Flags().String("mode", "", usage("mode", "server mode: store, static or spa", d.Server.Mode))
	cmd.Flags().BoolP("populate", "p", false,
		usage("populate", "record the files already in the storage directory before serving", d.Storage.Populate))
	cmd.Flags().BoolP("migrate", "m", false,
		usage("migrate", "create the metadata schema before serving", d.Database.Migrate))

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

	// An in-memory database is created empty by this process, so nothing could
	// have migrated it. A database on disk is migrated only when asked.
	migrate := cfg.Database.DSN == config.MemoryPath || cfg.Database.Migrate

	db, err := openDatabase(ctx, cfg, migrate)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	if err := db.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
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

	svc := service.New(db, storage)

	if cfg.Storage.Populate {
		entries, err := svc.Populate(ctx)
		if err != nil {
			return fmt.Errorf("populate storage: %w", err)
		}

		slog.Info("populated storage", "files", len(entries), "path", cfg.Storage.Path)
	}

	mux := http.NewServeMux()
	handler.Register(&handler.Opts{
		Mode:          cfg.Server.Mode,
		ErrorDocument: cfg.Server.ErrorDocument,
		MaxUploadSize: cfg.Server.MaxUploadSize,
		Mux:           mux,
		Svc:           svc,
		Logger:        slog.Default(),
		ReadVerifier:  verifierWhen(cfg.Auth.Read.Private(), verifier),
		WriteVerifier: verifierWhen(cfg.Auth.Write.Private(), verifier),
	})

	return listenAndServe(ctx, cancel, cfg.Server.Port, withServerMiddleware(cfg, mux))
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

// withServerMiddleware wraps the mux rather than its routes, so a CORS
// preflight and a 404 are covered too; the mux answers both before any route
// runs. It also puts CORS ahead of authentication, which it must be: browsers
// omit credentials from a preflight.
func withServerMiddleware(cfg *config.Config, mux *http.ServeMux) http.Handler {
	var h http.Handler = mux

	if cors, enabled := cfg.CORSConfig(); enabled {
		h = middleware.WithCORS(cors)(h)
	}

	h = middleware.WithLogging(slog.Default(), h)

	return middleware.WithRequestID(h)
}

func listenAndServe(ctx context.Context, cancel context.CancelFunc, port int, h http.Handler) error {
	addr := fmt.Sprintf(":%d", port)

	server := &http.Server{
		Addr:         addr,
		Handler:      h,
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
