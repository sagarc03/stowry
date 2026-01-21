package main

import (
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/lmittmann/tint"

	"github.com/sagarc03/stowry/config"
)

func setupLogging(cfg *config.Config) {
	level := parseLevel(cfg.Log.Level)

	h := tint.NewHandler(os.Stdout, &tint.Options{
		Level:      level,
		AddSource:  true,
		TimeFormat: "15:04:05.000",
	})

	logger := slog.New(h)
	slog.SetDefault(logger)

	log.SetFlags(0)
	log.SetOutput(
		slog.NewLogLogger(
			slog.Default().Handler(),
			slog.LevelInfo,
		).Writer(),
	)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
