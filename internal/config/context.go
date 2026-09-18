package config

import (
	"context"
	"errors"
)

type contextKey struct{}

// WithContext returns a context carrying cfg.
func WithContext(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, contextKey{}, cfg)
}

// FromContext returns the config stored by WithContext.
func FromContext(ctx context.Context) (*Config, error) {
	cfg, ok := ctx.Value(contextKey{}).(*Config)
	if !ok || cfg == nil {
		return nil, errors.New("config not found in context")
	}

	return cfg, nil
}
