package main

import (
	"os"

	"github.com/sagarc03/stowry/clientcli"
	"github.com/spf13/cobra"
)

var (
	version = "dev"

	cfgFile    string
	profile    string
	endpoint   string
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

Configuration:
  Profiles are configured in ~/.stowry/config.yaml
  Use 'stowry-cli configure' to manage profiles

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
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (env: STOWRY_CONFIG, default: ~/.stowry/config.yaml)")
	rootCmd.PersistentFlags().StringVarP(&profile, "profile", "p", "", "use named profile (env: STOWRY_PROFILE)")
	rootCmd.PersistentFlags().StringVarP(&endpoint, "endpoint", "e", "", "endpoint URL override (env: STOWRY_ENDPOINT)")
	rootCmd.PersistentFlags().StringVarP(&accessKey, "access-key", "a", "", "access key override (env: STOWRY_ACCESS_KEY)")
	rootCmd.PersistentFlags().StringVarP(&secretKey, "secret-key", "k", "", "secret key override (env: STOWRY_SECRET_KEY)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "suppress non-essential output")

	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(configureCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// buildConfig resolves configuration from profiles, env vars, and flags.
// Precedence (highest to lowest):
// 1. CLI flags (--endpoint, --access-key, --secret-key)
// 2. Environment variables (STOWRY_ENDPOINT, STOWRY_ACCESS_KEY, STOWRY_SECRET_KEY)
// 3. Selected profile (--profile or STOWRY_PROFILE)
// 4. Default profile from config file
func buildConfig() (*clientcli.Config, error) {
	var configs []*clientcli.Config

	// 1. Load profile from config file
	// Priority: --config flag > STOWRY_CONFIG env > default path
	configPath := cfgFile
	if configPath == "" {
		configPath = clientcli.ConfigPathFromEnv()
	}
	if configPath == "" {
		configPath = clientcli.DefaultConfigPath()
	}

	// Determine which profile to use
	profileName := profile
	if profileName == "" {
		profileName = clientcli.ProfileFromEnv()
	}

	if configPath != "" {
		configFile, loadErr := clientcli.LoadConfigFile(configPath)
		if loadErr == nil {
			// Get profile (by name or default)
			p, profileErr := configFile.GetProfile(profileName)
			if profileErr != nil {
				// Only error if user explicitly requested a profile
				if profileName != "" {
					return nil, profileErr
				}
				// No profiles configured, that's ok - continue with env/flags
			} else {
				configs = append(configs, clientcli.ConfigFromProfile(p))
			}
		} else if cfgFile != "" {
			// Only error if user explicitly specified a config file
			return nil, loadErr
		}
		// Ignore file not found for default config path
	}

	// 2. Load from environment variables
	envCfg := clientcli.ConfigFromEnv()
	configs = append(configs, envCfg)

	// 3. Load from flags
	flagCfg := &clientcli.Config{
		Endpoint:  endpoint,
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

// getConfigPath returns the config file path to use.
// Priority: --config flag > STOWRY_CONFIG env > default path
func getConfigPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	if envPath := clientcli.ConfigPathFromEnv(); envPath != "" {
		return envPath
	}
	return clientcli.DefaultConfigPath()
}
