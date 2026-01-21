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

	"github.com/sagarc03/stowry"
	"github.com/sagarc03/stowry/config"
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

	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	cfg, err := config.FromContext(cmd.Context())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(cmd.Context())
	defer cancel()

	if err = cfg.Database.Tables.Validate(); err != nil {
		return fmt.Errorf("invalid database config: %w", err)
	}

	db, err := database.Connect(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer func() { _ = db.Close() }()

	if err = db.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	if err = db.Validate(ctx); err != nil {
		return fmt.Errorf("validate database schema: %w", err)
	}

	repo := db.GetRepo()
	slog.Info("connected to database", "type", cfg.Database.Type)

	err = os.MkdirAll(cfg.Storage.Path, 0o750)
	if err != nil {
		return fmt.Errorf("create storage directory: %w", err)
	}

	root, err := os.OpenRoot(cfg.Storage.Path)
	if err != nil {
		return fmt.Errorf("open storage root: %w", err)
	}
	defer func() { _ = root.Close() }()

	storage := filesystem.NewFileStorage(root)

	mode, err := stowry.ParseServerMode(cfg.Server.Mode)
	if err != nil {
		return fmt.Errorf("parse server mode: %w", err)
	}

	service, err := stowry.NewStowryService(repo, storage, mode)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}

	store, err := keybackend.NewSecretStore(cfg.Auth.Keys)
	if err != nil {
		return fmt.Errorf("create secret store: %w", err)
	}

	authCfg := stowry.AuthConfig{
		AWS: cfg.Auth.AWS,
	}
	verifier := stowry.NewSignatureVerifier(authCfg, store)

	var readVerifier, writeVerifier stowryhttp.RequestVerifier
	if cfg.Auth.Read != "public" {
		readVerifier = verifier
	}
	if cfg.Auth.Write != "public" {
		writeVerifier = verifier
	}

	handlerConfig := stowryhttp.HandlerConfig{
		Mode:          mode,
		ReadVerifier:  readVerifier,
		WriteVerifier: writeVerifier,
		CORS:          cfg.CORS,
	}

	handler := stowryhttp.NewHandler(&handlerConfig, service)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)

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
