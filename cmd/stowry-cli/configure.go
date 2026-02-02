package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/sagarc03/stowry/clientcli"
	"github.com/spf13/cobra"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Manage server profiles",
	Long: `Manage server profiles in the configuration file.

Profiles allow you to save connection settings for multiple Stowry servers
and easily switch between them using --profile or STOWRY_PROFILE.

Configuration is stored in ~/.stowry/config.yaml`,
}

var configureListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured profiles",
	Long: `List all profiles configured in the config file.

The default profile is marked with an asterisk (*).`,
	RunE: runConfigureList,
}

var configureAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new profile",
	Long: `Add a new profile interactively.

You will be prompted for:
  - Endpoint URL
  - Access key
  - Secret key
  - Whether to set as default

The endpoint connection will be tested before saving (use --skip-test to skip).

Use 'configure update' to modify an existing profile.`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigureAdd,
}

var configureUpdateCmd = &cobra.Command{
	Use:   "update <name>",
	Short: "Update an existing profile",
	Long: `Update an existing profile interactively.

Current values are shown as defaults. Press Enter to keep the current value.

You will be prompted for:
  - Endpoint URL
  - Access key
  - Secret key
  - Whether to set as default

The endpoint connection will be tested before saving (use --skip-test to skip).

Use 'configure add' to create a new profile.`,
	Args: cobra.ExactArgs(1),
	RunE: runConfigureUpdate,
}

var configureRemoveCmd = &cobra.Command{
	Use:     "remove <name>",
	Aliases: []string{"rm"},
	Short:   "Remove a profile",
	Args:    cobra.ExactArgs(1),
	RunE:    runConfigureRemove,
}

var configureSetDefaultCmd = &cobra.Command{
	Use:   "set-default <name>",
	Short: "Set the default profile",
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigureSetDefault,
}

var configureShowCmd = &cobra.Command{
	Use:   "show [name]",
	Short: "Show profile details",
	Long: `Show details for a profile.

If no name is provided, shows the default profile.
Secrets are hidden by default; use --show-secrets to reveal them.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConfigureShow,
}

var (
	showSecrets bool
	skipTest    bool
)

func init() {
	configureCmd.AddCommand(configureListCmd)
	configureCmd.AddCommand(configureAddCmd)
	configureCmd.AddCommand(configureUpdateCmd)
	configureCmd.AddCommand(configureRemoveCmd)
	configureCmd.AddCommand(configureSetDefaultCmd)
	configureCmd.AddCommand(configureShowCmd)

	configureShowCmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "show secret values")
	configureListCmd.Flags().BoolVar(&showSecrets, "show-secrets", false, "show secret values")
	configureAddCmd.Flags().BoolVar(&skipTest, "skip-test", false, "skip connection test")
	configureUpdateCmd.Flags().BoolVar(&skipTest, "skip-test", false, "skip connection test")
}

func runConfigureList(_ *cobra.Command, _ []string) error {
	configPath := getConfigPath()

	cfg, err := clientcli.LoadConfigFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Println("No profiles configured.")
			fmt.Println("Run 'stowry-cli configure add <name>' to create one.")
			return nil
		}
		return fmt.Errorf("load config: %w", err)
	}

	if len(cfg.Profiles) == 0 {
		fmt.Println("No profiles configured.")
		fmt.Println("Run 'stowry-cli configure add <name>' to create one.")
		return nil
	}

	// Find default profile name
	defaultName := ""
	for _, p := range cfg.Profiles {
		if p.Default {
			defaultName = p.Name
			break
		}
	}
	if defaultName == "" && len(cfg.Profiles) > 0 {
		defaultName = cfg.Profiles[0].Name
	}

	formatter := getFormatter()
	return formatter.FormatProfileList(os.Stdout, cfg.Profiles, defaultName, showSecrets)
}

func runConfigureAdd(_ *cobra.Command, args []string) error {
	name := args[0]
	configPath := getConfigPath()

	// Load existing config or create new
	cfg, err := clientcli.LoadConfigFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			cfg = &clientcli.ConfigFile{}
		} else {
			return fmt.Errorf("load config: %w", err)
		}
	}

	// Check if profile already exists
	if existingProfile, _ := cfg.GetProfile(name); existingProfile != nil {
		fmt.Printf("Profile '%s' already exists. Use 'stowry-cli configure update %s' to modify it.\n", name, name)
		return nil
	}

	// Prompt for endpoint URL
	endpointPrompt := promptui.Prompt{
		Label:    "Endpoint URL",
		Default:  clientcli.DefaultEndpoint,
		Validate: validateEndpointURL,
	}
	endpointURL, err := endpointPrompt.Run()
	if err != nil {
		return handlePromptError(err)
	}

	// Prompt for access key
	accessKeyPrompt := promptui.Prompt{
		Label: "Access Key",
	}
	accessKeyVal, err := accessKeyPrompt.Run()
	if err != nil {
		return handlePromptError(err)
	}

	// Prompt for secret key
	secretKeyPrompt := promptui.Prompt{
		Label: "Secret Key",
		Mask:  '*',
	}
	secretKeyVal, err := secretKeyPrompt.Run()
	if err != nil {
		return handlePromptError(err)
	}

	// Prompt for default
	setAsDefault := false
	if len(cfg.Profiles) == 0 {
		setAsDefault = true // First profile is always default
	} else {
		defaultPrompt := promptui.Prompt{
			Label:     "Set as default profile",
			IsConfirm: true,
		}
		if _, promptErr := defaultPrompt.Run(); promptErr == nil {
			setAsDefault = true
		}
	}

	// Test connection (unless skipped)
	if !skipTest {
		fmt.Print("Testing connection... ")
		if connErr := testServerConnection(endpointURL); connErr != nil {
			fmt.Println("FAILED")
			fmt.Printf("Warning: Could not connect to server: %v\n", connErr)

			continuePrompt := promptui.Prompt{
				Label:     "Save profile anyway",
				IsConfirm: true,
			}
			if _, promptErr := continuePrompt.Run(); promptErr != nil {
				fmt.Println("Cancelled.")
				return nil //nolint:nilerr // User cancelled, not an error
			}
		} else {
			fmt.Println("OK")
		}
	}

	// Create profile
	newProfile := clientcli.Profile{
		Name:      name,
		Endpoint:  strings.TrimSuffix(endpointURL, "/"),
		AccessKey: accessKeyVal,
		SecretKey: secretKeyVal,
		Default:   setAsDefault,
	}

	// If setting as default, clear default from others
	if setAsDefault {
		for i := range cfg.Profiles {
			cfg.Profiles[i].Default = false
		}
	}

	// Add profile
	if err := cfg.AddProfile(newProfile); err != nil {
		return fmt.Errorf("add profile: %w", err)
	}

	// Save config
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Profile '%s' added.\n", name)
	if setAsDefault {
		fmt.Printf("Set as default profile.\n")
	}

	return nil
}

func runConfigureRemove(_ *cobra.Command, args []string) error {
	name := args[0]
	configPath := getConfigPath()

	cfg, err := clientcli.LoadConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// Check if profile exists
	if _, err = cfg.GetProfile(name); err != nil {
		return err
	}

	// Confirm removal
	prompt := promptui.Prompt{
		Label:     fmt.Sprintf("Remove profile '%s'", name),
		IsConfirm: true,
	}
	if _, promptErr := prompt.Run(); promptErr != nil {
		fmt.Println("Cancelled.")
		return nil //nolint:nilerr // User cancelled, not an error
	}

	if err := cfg.RemoveProfile(name); err != nil {
		return fmt.Errorf("remove profile: %w", err)
	}

	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Profile '%s' removed.\n", name)
	return nil
}

func runConfigureSetDefault(_ *cobra.Command, args []string) error {
	name := args[0]
	configPath := getConfigPath()

	cfg, err := clientcli.LoadConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	if err := cfg.SetDefault(name); err != nil {
		return err
	}

	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Default profile set to '%s'.\n", name)
	return nil
}

func runConfigureShow(_ *cobra.Command, args []string) error {
	configPath := getConfigPath()

	cfg, err := clientcli.LoadConfigFile(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	name := ""
	if len(args) > 0 {
		name = args[0]
	}

	p, err := cfg.GetProfile(name)
	if err != nil {
		return err
	}

	// Check if this is the default profile
	isDefault := p.Default
	if !isDefault && name == "" {
		isDefault = true // If we got here with empty name, it's the default
	}

	formatter := getFormatter()
	return formatter.FormatProfileShow(os.Stdout, *p, isDefault, showSecrets)
}

// testServerConnection tests if the server is reachable.
// It sends a GET request to the root path and considers any HTTP response as success.
func testServerConnection(endpointURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Any HTTP response means the server is reachable
	// Even 401/403/404 are fine - server is up
	return nil
}

// handlePromptError handles promptui errors.
func handlePromptError(err error) error {
	if errors.Is(err, promptui.ErrInterrupt) {
		fmt.Println("\nCancelled.")
		os.Exit(0)
	}
	if errors.Is(err, promptui.ErrAbort) {
		fmt.Println("Cancelled.")
		return nil
	}
	return err
}

// validateEndpointURL validates an endpoint URL.
func validateEndpointURL(input string) error {
	if input == "" {
		return errors.New("endpoint URL is required")
	}
	parsedURL, parseErr := url.Parse(input)
	if parseErr != nil {
		return fmt.Errorf("invalid URL: %w", parseErr)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.New("URL must start with http:// or https://")
	}
	return nil
}

// maskSecretForPrompt masks a secret for display in prompts.
func maskSecretForPrompt(secret string) string {
	if secret == "" {
		return "(not set)"
	}
	if len(secret) <= 8 {
		return "********"
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}

func runConfigureUpdate(_ *cobra.Command, args []string) error {
	name := args[0]
	configPath := getConfigPath()

	// Load existing config
	cfg, err := clientcli.LoadConfigFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("Profile '%s' not found. Use 'stowry-cli configure add %s' to create it.\n", name, name)
			return nil
		}
		return fmt.Errorf("load config: %w", err)
	}

	// Get existing profile
	existingProfile, err := cfg.GetProfile(name)
	if err != nil {
		if errors.Is(err, clientcli.ErrProfileNotFound) {
			fmt.Printf("Profile '%s' not found. Use 'stowry-cli configure add %s' to create it.\n", name, name)
			return nil
		}
		return err
	}

	// Prompt for endpoint URL (show current value as default)
	endpointPrompt := promptui.Prompt{
		Label:    "Endpoint URL",
		Default:  existingProfile.Endpoint,
		Validate: validateEndpointURL,
	}
	endpointURL, err := endpointPrompt.Run()
	if err != nil {
		return handlePromptError(err)
	}

	// Prompt for access key (show current value as default)
	accessKeyPrompt := promptui.Prompt{
		Label:   "Access Key",
		Default: existingProfile.AccessKey,
	}
	accessKeyVal, err := accessKeyPrompt.Run()
	if err != nil {
		return handlePromptError(err)
	}

	// Prompt for secret key (show masked current value, empty keeps current)
	secretKeyPrompt := promptui.Prompt{
		Label: fmt.Sprintf("Secret Key [%s]", maskSecretForPrompt(existingProfile.SecretKey)),
		Mask:  '*',
	}
	secretKeyVal, err := secretKeyPrompt.Run()
	if err != nil {
		return handlePromptError(err)
	}
	// Keep existing secret if user didn't enter a new one
	if secretKeyVal == "" {
		secretKeyVal = existingProfile.SecretKey
	}

	// Prompt for default (only if not already default)
	setAsDefault := existingProfile.Default
	if !existingProfile.Default {
		defaultPrompt := promptui.Prompt{
			Label:     "Set as default profile",
			IsConfirm: true,
		}
		if _, promptErr := defaultPrompt.Run(); promptErr == nil {
			setAsDefault = true
		}
	}

	// Test connection (unless skipped)
	if !skipTest {
		fmt.Print("Testing connection... ")
		if connErr := testServerConnection(endpointURL); connErr != nil {
			fmt.Println("FAILED")
			fmt.Printf("Warning: Could not connect to server: %v\n", connErr)

			continuePrompt := promptui.Prompt{
				Label:     "Save profile anyway",
				IsConfirm: true,
			}
			if _, promptErr := continuePrompt.Run(); promptErr != nil {
				fmt.Println("Cancelled.")
				return nil //nolint:nilerr // User cancelled, not an error
			}
		} else {
			fmt.Println("OK")
		}
	}

	// Update profile
	updatedProfile := clientcli.Profile{
		Name:      name,
		Endpoint:  strings.TrimSuffix(endpointURL, "/"),
		AccessKey: accessKeyVal,
		SecretKey: secretKeyVal,
		Default:   setAsDefault,
	}

	// If setting as default, clear default from others
	if setAsDefault && !existingProfile.Default {
		for i := range cfg.Profiles {
			cfg.Profiles[i].Default = false
		}
	}

	// Update profile
	if err := cfg.UpdateProfile(updatedProfile); err != nil {
		return fmt.Errorf("update profile: %w", err)
	}

	// Save config
	if err := cfg.Save(configPath); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	fmt.Printf("Profile '%s' updated.\n", name)
	if setAsDefault && !existingProfile.Default {
		fmt.Printf("Set as default profile.\n")
	}

	return nil
}
