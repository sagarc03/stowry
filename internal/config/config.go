// Package config loads the server configuration from a YAML file, the
// environment and CLI flags. Every setting is reachable from any of the three:
// server.error_document in YAML is STOWRY_SERVER_ERROR_DOCUMENT in the
// environment.
package config

import (
	"time"

	"github.com/sagarc03/stowry/internal/database"
	"github.com/sagarc03/stowry/internal/keybackend"
	"github.com/sagarc03/stowry/internal/middleware"
	"github.com/sagarc03/stowry/sign"
	"github.com/sagarc03/stowry/types"
)

// Config describes the config file. The methods below build what the packages
// that consume it take.
type Config struct {
	Server   Server   `mapstructure:"server"`
	Service  Service  `mapstructure:"service"`
	Database Database `mapstructure:"database"`
	Storage  Storage  `mapstructure:"storage"`
	Auth     Auth     `mapstructure:"auth"`
	CORS     CORS     `mapstructure:"cors"`
	Log      Log      `mapstructure:"log"`
}

type Server struct {
	Port int              `mapstructure:"port" validate:"min=1,max=65535"`
	Mode types.ServerMode `mapstructure:"mode" validate:"required,oneof=store static spa"`
	// MaxUploadSize caps a PUT body in bytes. Zero means no limit.
	MaxUploadSize int64 `mapstructure:"max_upload_size" validate:"min=0"`
	// ErrorDocument is the object served for a 404 in static and SPA modes.
	ErrorDocument string `mapstructure:"error_document"`
}

type Service struct {
	CleanupTimeout time.Duration `mapstructure:"cleanup_timeout" validate:"required"`
}

type Database struct {
	Type   string `mapstructure:"type" validate:"required,oneof=sqlite postgres"`
	DSN    string `mapstructure:"dsn" validate:"required"`
	Tables Tables `mapstructure:"tables"`
}

type Tables struct {
	MetaData string `mapstructure:"meta_data" validate:"required,table_name"`
}

type Storage struct {
	Path string `mapstructure:"path" validate:"required"`
}

type Auth struct {
	Read  Access `mapstructure:"read" validate:"required,oneof=public private"`
	Write Access `mapstructure:"write" validate:"required,oneof=public private"`
	AWS   AWS    `mapstructure:"aws"`
	Keys  Keys   `mapstructure:"keys"`
}

// Access says whether a request must be signed.
type Access string

const (
	AccessPublic  Access = "public"
	AccessPrivate Access = "private"
)

func (a Access) Private() bool { return a == AccessPrivate }

type AWS struct {
	Region  string `mapstructure:"region" validate:"required"`
	Service string `mapstructure:"service" validate:"required"`
}

type Keys struct {
	// Inline requires both halves: a half-written pair is an operator mistake.
	Inline []KeyPair `mapstructure:"inline" validate:"dive"`
	// File is the path to a JSON array of key pairs.
	File string `mapstructure:"file"`
}

type KeyPair struct {
	AccessKey string `mapstructure:"access_key" validate:"required"`
	SecretKey string `mapstructure:"secret_key" validate:"required"`
}

// CORS is applied only when AllowedOrigins is set, so serving any origin is
// spelled out as ["*"] rather than left implicit.
type CORS struct {
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	ExposedHeaders   []string `mapstructure:"exposed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	// MaxAge is how long a browser may cache a preflight, in seconds.
	MaxAge int `mapstructure:"max_age" validate:"min=0"`
}

type Log struct {
	Level string `mapstructure:"level" validate:"required,oneof=debug info warn error"`
}

// Defaults is the configuration before any file, environment variable or flag
// is applied.
func Defaults() Config {
	return Config{
		Server: Server{
			Port: 5708,
			Mode: types.ModeStore,
		},
		Service: Service{
			CleanupTimeout: 30 * time.Second,
		},
		Database: Database{
			Type:   "sqlite",
			DSN:    "stowry.db",
			Tables: Tables{MetaData: "stowry_metadata"},
		},
		Storage: Storage{
			Path: "./data",
		},
		Auth: Auth{
			Read:  AccessPublic,
			Write: AccessPublic,
			AWS:   AWS{Region: "us-east-1", Service: "s3"},
		},
		Log: Log{
			Level: "info",
		},
	}
}

// DatabaseConfig builds the metadata backend's configuration.
func (c *Config) DatabaseConfig() database.Config {
	return database.Config{
		Type:   c.Database.Type,
		DSN:    c.Database.DSN,
		Tables: types.Tables{MetaData: c.Database.Tables.MetaData},
	}
}

// KeysConfig builds the access key store's configuration.
func (c *Config) KeysConfig() keybackend.KeysConfig {
	inline := make([]keybackend.KeyPair, 0, len(c.Auth.Keys.Inline))
	for _, p := range c.Auth.Keys.Inline {
		inline = append(inline, keybackend.KeyPair{AccessKey: p.AccessKey, SecretKey: p.SecretKey})
	}

	return keybackend.KeysConfig{Inline: inline, File: c.Auth.Keys.File}
}

// AWSConfig builds the signature verifier's configuration.
func (c *Config) AWSConfig() sign.AWSConfig {
	return sign.AWSConfig{Region: c.Auth.AWS.Region, Service: c.Auth.AWS.Service}
}

// CORSConfig builds the policy and reports whether one was configured.
func (c *Config) CORSConfig() (middleware.CORSConfig, bool) {
	if len(c.CORS.AllowedOrigins) == 0 {
		return middleware.CORSConfig{}, false
	}

	return middleware.CORSConfig{
		AllowedOrigins:   c.CORS.AllowedOrigins,
		AllowedMethods:   c.CORS.AllowedMethods,
		AllowedHeaders:   c.CORS.AllowedHeaders,
		ExposedHeaders:   c.CORS.ExposedHeaders,
		AllowCredentials: c.CORS.AllowCredentials,
		MaxAge:           c.CORS.MaxAge,
	}, true
}
