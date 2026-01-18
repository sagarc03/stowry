package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/filesystem"
	stowryhttp "github.com/sagarc03/stowry/http"
	"github.com/sagarc03/stowry/keybackend"
	"github.com/sagarc03/stowry/postgres"
	"github.com/sagarc03/stowry/sqlite"

	_ "modernc.org/sqlite"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the HTTP server",
	Long:  `Start the Stowry HTTP server.`,
	RunE:  runServe,
}

func init() {
	serveCmd.Flags().Int("port", 5708, "HTTP server port")
	serveCmd.Flags().String("mode", "store", "server mode (store, static, spa)")

	_ = viper.BindPFlag("server.port", serveCmd.Flags().Lookup("port"))
	_ = viper.BindPFlag("server.mode", serveCmd.Flags().Lookup("mode"))

	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	repo, closeDB, err := initDB(ctx)
	if err != nil {
		return fmt.Errorf("init db: %w", err)
	}
	defer closeDB()

	storagePath := viper.GetString("storage.path")
	err = os.MkdirAll(storagePath, 0o750)
	if err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}

	root, err := os.OpenRoot(storagePath)
	if err != nil {
		return fmt.Errorf("open storage root: %w", err)
	}
	defer func() { _ = root.Close() }()

	storage := filesystem.NewFileStorage(root)

	modeStr := viper.GetString("server.mode")
	mode, err := stowry.ParseServerMode(modeStr)
	if err != nil {
		return fmt.Errorf("parse server mode: %w", err)
	}

	service, err := stowry.NewStowryService(repo, storage, mode)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	store := keybackend.NewMapSecretStore(getAccessKeys())
	verifier := stowry.NewSignatureVerifier(
		viper.GetString("auth.region"),
		viper.GetString("auth.service"),
		store,
	)

	var readVerifier, writeVerifier stowryhttp.RequestVerifier
	if !viper.GetBool("access.public_read") {
		readVerifier = verifier
	}
	if !viper.GetBool("access.public_write") {
		writeVerifier = verifier
	}

	handlerConfig := stowryhttp.HandlerConfig{
		Mode:          mode,
		ReadVerifier:  readVerifier,
		WriteVerifier: writeVerifier,
		CORS:          getCORSConfig(),
	}

	handler := stowryhttp.NewHandler(&handlerConfig, service)

	port := viper.GetInt("server.port")
	addr := fmt.Sprintf(":%d", port)

	server := &http.Server{
		Addr:         addr,
		Handler:      handler.Router(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		slog.Info("shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("server shutdown error", "err", err)
		}
		cancel()
	}()

	slog.Info("starting server", "addr", addr, "mode", mode)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func initDB(ctx context.Context) (stowry.MetaDataRepo, func(), error) {
	dbType := viper.GetString("database.type")
	dsn := viper.GetString("database.dsn")
	tableName := viper.GetString("database.table")

	tables := stowry.Tables{MetaData: tableName}

	switch dbType {
	case "sqlite":
		return initSQLite(ctx, dsn, tables)
	case "postgres":
		return initPostgres(ctx, dsn, tables)
	default:
		return nil, nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

func initSQLite(ctx context.Context, dsn string, tables stowry.Tables) (stowry.MetaDataRepo, func(), error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("open sqlite: %w", err)
	}

	err = db.PingContext(ctx)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("ping sqlite: %w", err)
	}

	err = sqlite.Migrate(ctx, db, tables)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("migrate sqlite: %w", err)
	}

	err = sqlite.ValidateSchema(ctx, db, tables)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("validate sqlite schema: %w", err)
	}

	repo, err := sqlite.NewRepo(db, tables)
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("create sqlite repo: %w", err)
	}

	cleanup := func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Warn("error closing sqlite", "err", closeErr)
		}
	}

	slog.Info("connected to sqlite", "dsn", dsn)
	return repo, cleanup, nil
}

func initPostgres(ctx context.Context, dsn string, tables stowry.Tables) (stowry.MetaDataRepo, func(), error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("connect postgres: %w", err)
	}

	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("ping postgres: %w", err)
	}

	err = postgres.Migrate(ctx, pool, tables)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("migrate postgres: %w", err)
	}

	err = postgres.ValidateSchema(ctx, pool, tables)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("validate postgres schema: %w", err)
	}

	repo, err := postgres.NewRepo(pool, tables)
	if err != nil {
		pool.Close()
		return nil, nil, fmt.Errorf("create postgres repo: %w", err)
	}

	slog.Info("connected to postgres")
	return repo, pool.Close, nil
}

func getAccessKeys() map[string]string {
	keys := make(map[string]string)

	authKeys := viper.Get("auth.keys")
	if authKeys == nil {
		return keys
	}

	keyList, ok := authKeys.([]any)
	if !ok {
		return keys
	}

	for _, k := range keyList {
		keyMap, ok := k.(map[string]any)
		if !ok {
			continue
		}

		accessKey, _ := keyMap["access_key"].(string)
		secretKey, _ := keyMap["secret_key"].(string)

		if accessKey != "" && secretKey != "" {
			keys[accessKey] = secretKey
		}
	}

	return keys
}

func getCORSConfig() stowryhttp.CORSConfig {
	return stowryhttp.CORSConfig{
		Enabled:          viper.GetBool("cors.enabled"),
		AllowedOrigins:   viper.GetStringSlice("cors.allowed_origins"),
		AllowedMethods:   viper.GetStringSlice("cors.allowed_methods"),
		AllowedHeaders:   viper.GetStringSlice("cors.allowed_headers"),
		ExposedHeaders:   viper.GetStringSlice("cors.exposed_headers"),
		AllowCredentials: viper.GetBool("cors.allow_credentials"),
		MaxAge:           viper.GetInt("cors.max_age"),
	}
}
