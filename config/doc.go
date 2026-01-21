// Package config provides configuration loading and validation for Stowry.
//
// The package handles YAML configuration files, environment variables, and CLI flags
// with automatic merging and validation using go-playground/validator.
//
// # Configuration Precedence
//
// Values are loaded in this order (later sources override earlier ones):
//
//  1. Default values
//  2. Configuration file(s) - multiple files merged left-to-right
//  3. Environment variables (STOWRY_ prefix)
//  4. CLI flags
//
// # Usage
//
//	cfg, err := config.Load([]string{"config.yaml"}, cmd.Flags())
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Store in context for subcommands
//	ctx = config.WithContext(ctx, cfg)
//
//	// Retrieve later
//	cfg, err = config.FromContext(ctx)
//
// # Environment Variables
//
// All config keys map to environment variables with STOWRY_ prefix:
//   - server.port → STOWRY_SERVER_PORT
//   - database.type → STOWRY_DATABASE_TYPE
//   - auth.read → STOWRY_AUTH_READ
//
// # Configuration Structure
//
// The Config struct contains:
//   - Server: port, mode (store/static/spa), and max_upload_size
//   - Service: cleanup_timeout for background operations
//   - Database: type, DSN, and table names
//   - Storage: file storage path
//   - Auth: access control (read/write), AWS settings, and keys
//   - CORS: cross-origin resource sharing settings
//   - Log: logging level
//
// # Validation
//
// Configuration is validated using struct tags:
//   - Port must be 1-65535
//   - Mode must be store, static, or spa
//   - Auth read/write must be public or private
//   - Log level must be debug, info, warn, or error
package config
