package main

import (
	"context"
	"errors"

	"github.com/sagarc03/stowry/config"
)

// configKey is the context key for storing the loaded configuration.
type configKey struct{}

// withConfig returns a new context with the config stored.
func withConfig(ctx context.Context, cfg *config.Config) context.Context {
	return context.WithValue(ctx, configKey{}, cfg)
}

// configFromContext retrieves the config from context.
// Returns an error if config is not found.
func configFromContext(ctx context.Context) (*config.Config, error) {
	cfg, ok := ctx.Value(configKey{}).(*config.Config)
	if !ok || cfg == nil {
		return nil, errors.New("config not found in context")
	}
	return cfg, nil
}
