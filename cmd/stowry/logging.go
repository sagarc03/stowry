package main

import (
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/lmittmann/tint"
	"github.com/spf13/viper"
)

func setupLogging() {
	env := viper.GetString("env")

	isProd := env == "prod" || env == "production"

	levelStr := viper.GetString("log.level")
	if levelStr == "" {
		if isProd {
			levelStr = "info"
		} else {
			levelStr = "debug"
		}
	}
	level := parseLevel(levelStr)

	var h slog.Handler
	if isProd {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level:     level,
			AddSource: false,
			ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					a.Key = "ts"
					return slog.String("ts", a.Value.Time().UTC().Format(time.RFC3339Nano))
				}
				return a
			},
		})
	} else {
		h = tint.NewHandler(os.Stdout, &tint.Options{
			Level:      level,
			AddSource:  true,
			TimeFormat: "15:04:05.000",
		})
	}

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
