package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/database"
	"github.com/sagarc03/stowry/filesystem"
	stowryhttp "github.com/sagarc03/stowry/http"
	"github.com/sagarc03/stowry/keybackend"
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

	dbCfg := database.Config{
		Type:   viper.GetString("database.type"),
		DSN:    viper.GetString("database.dsn"),
		Tables: stowry.Tables{MetaData: viper.GetString("database.table")},
	}

	if err := dbCfg.Tables.Validate(); err != nil {
		return fmt.Errorf("invalid database config: %w", err)
	}

	db, err := database.Connect(ctx, dbCfg)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err = db.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	if viper.GetBool("database.auto_migrate") {
		if err = db.Migrate(ctx); err != nil {
			return fmt.Errorf("migrate database: %w", err)
		}
		slog.Info("database migration complete")
	}

	if err = db.Validate(ctx); err != nil {
		return fmt.Errorf("validate database schema: %w", err)
	}

	repo := db.GetRepo()
	slog.Info("connected to database", "type", dbCfg.Type)

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
	authCfg := stowry.AuthConfig{
		Region:  viper.GetString("auth.region"),
		Service: viper.GetString("auth.service"),
	}
	verifier := stowry.NewSignatureVerifier(authCfg, store)

	var readVerifier, writeVerifier stowryhttp.RequestVerifier
	if !viper.GetBool("access.public_read") {
		readVerifier = verifier
	}
	if !viper.GetBool("access.public_write") {
		writeVerifier = verifier
	}

	corsConfig := stowryhttp.CORSConfig{
		Enabled:          viper.GetBool("cors.enabled"),
		AllowedOrigins:   viper.GetStringSlice("cors.allowed_origins"),
		AllowedMethods:   viper.GetStringSlice("cors.allowed_methods"),
		AllowedHeaders:   viper.GetStringSlice("cors.allowed_headers"),
		ExposedHeaders:   viper.GetStringSlice("cors.exposed_headers"),
		AllowCredentials: viper.GetBool("cors.allow_credentials"),
		MaxAge:           viper.GetInt("cors.max_age"),
	}

	handlerConfig := stowryhttp.HandlerConfig{
		Mode:          mode,
		ReadVerifier:  readVerifier,
		WriteVerifier: writeVerifier,
		CORS:          corsConfig,
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
