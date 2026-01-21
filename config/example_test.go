package config_test

import (
	"context"
	"fmt"
	"log"

	"github.com/sagarc03/stowry/config"
)

func ExampleLoad() {
	// Load with defaults only (no config file)
	cfg, err := config.Load(nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Port: %d, Mode: %s\n", cfg.Server.Port, cfg.Server.Mode)
	// Output: Port: 5708, Mode: store
}

func ExampleWithContext() {
	cfg, _ := config.Load(nil, nil)

	// Store config in context
	ctx := config.WithContext(context.Background(), cfg)

	// Retrieve later (e.g., in a subcommand)
	retrieved, err := config.FromContext(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Retrieved port: %d\n", retrieved.Server.Port)
	// Output: Retrieved port: 5708
}
