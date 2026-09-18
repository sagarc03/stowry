package cli

import (
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/lmittmann/tint"
)

// setupLogging installs the process-wide logger and routes the standard log
// package through it.
func setupLogging(level string) {
	logger := slog.New(tint.NewTextHandler(os.Stdout, &tint.Options{
		Level:      parseLevel(level),
		AddSource:  true,
		TimeFormat: "15:04:05.000",
	}))
	slog.SetDefault(logger)

	log.SetFlags(0)
	log.SetOutput(slog.NewLogLogger(logger.Handler(), slog.LevelInfo).Writer())
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
