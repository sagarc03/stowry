package main

import (
	"os"

	"github.com/sagarc03/stowry/clientcli"
	"github.com/spf13/cobra"
)

var (
	version = "dev"

	cfgFile    string
	server     string
	accessKey  string
	secretKey  string
	jsonOutput bool
	quiet      bool
)

var rootCmd = &cobra.Command{
	Use:     "stowry-cli",
	Version: version,
	Short:   "Client for Stowry object storage",
	Long: `Stowry CLI - Client for Stowry object storage server

Commands work differently depending on server mode:
  - upload:   Works in all modes (store, static, spa)
  - download: Works in all modes (behavior varies by mode)
  - delete:   Works in all modes (store, static, spa)
  - list:     Only works in store mode

Download behavior by server mode:
  - store:  Returns file or 404
  - static: Returns file, tries path/index.html, or 404
  - spa:    Returns file or falls back to /index.html`,
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default: ~/.stowry/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&server, "server", "s", "", "server URL (default: http://localhost:5708, env: STOWRY_SERVER)")
	rootCmd.PersistentFlags().StringVarP(&accessKey, "access-key", "a", "", "access key (env: STOWRY_ACCESS_KEY)")
	rootCmd.PersistentFlags().StringVarP(&secretKey, "secret-key", "k", "", "secret key (env: STOWRY_SECRET_KEY)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")

	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(listCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// buildConfig merges config from file, env vars, and flags (flags take precedence).
func buildConfig() (*clientcli.Config, error) {
	var configs []*clientcli.Config

	// 1. Load from config file
	configPath := cfgFile
	if configPath == "" {
		configPath = clientcli.DefaultConfigPath()
	}

	if configPath != "" {
		fileCfg, err := clientcli.LoadConfigFromFile(configPath)
		if err != nil {
			// Only error if user explicitly specified a config file
			if cfgFile != "" {
				return nil, err
			}
			// Ignore file not found for default config path
			// Other errors are also ignored since this is a default path
		} else {
			configs = append(configs, fileCfg)
		}
	}

	// 2. Load from environment variables
	envCfg := clientcli.ConfigFromEnv()
	configs = append(configs, envCfg)

	// 3. Load from flags
	flagCfg := &clientcli.Config{
		Server:    server,
		AccessKey: accessKey,
		SecretKey: secretKey,
	}
	configs = append(configs, flagCfg)

	// Merge all configs
	return clientcli.MergeConfig(configs...), nil
}

// getFormatter returns the appropriate formatter based on flags.
func getFormatter() clientcli.Formatter {
	return clientcli.NewFormatter(jsonOutput, quiet)
}

// getClient creates and returns a configured client.
func getClient() (*clientcli.Client, error) {
	cfg, err := buildConfig()
	if err != nil {
		return nil, err
	}

	return clientcli.New(cfg)
}
