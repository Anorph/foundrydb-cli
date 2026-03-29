package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication credentials",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Save API credentials to ~/.fdb/config.toml",
	RunE:  runAuthLogin,
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove saved credentials",
	RunE:  runAuthLogout,
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE:  runAuthStatus,
}

func init() {
	authLoginCmd.Flags().StringP("api-url", "u", "", "API base URL")
	authLoginCmd.Flags().StringP("username", "n", "", "Username")
	authLoginCmd.Flags().StringP("password", "p", "", "Password (will prompt if not provided)")

	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authLogoutCmd)
	authCmd.AddCommand(authStatusCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	loginURL, _ := cmd.Flags().GetString("api-url")
	loginUser, _ := cmd.Flags().GetString("username")
	loginPass, _ := cmd.Flags().GetString("password")

	if loginURL == "" {
		loginURL = viper.GetString("api_url")
		fmt.Printf("API URL [%s]: ", loginURL)
		var input string
		fmt.Scanln(&input)
		if input != "" {
			loginURL = input
		}
	}

	if loginUser == "" {
		loginUser = viper.GetString("username")
		fmt.Printf("Username [%s]: ", loginUser)
		var input string
		fmt.Scanln(&input)
		if input != "" {
			loginUser = input
		}
	}

	if loginPass == "" {
		fmt.Print("Password: ")
		passBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		fmt.Println()
		loginPass = string(passBytes)
	}

	// Verify credentials by calling the API
	client := newClient()
	client.BaseURL = loginURL
	client.Username = loginUser
	client.Password = loginPass

	_, err := client.ListServices()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Save credentials
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	content := fmt.Sprintf("api_url = %q\nusername = %q\npassword = %q\n", loginURL, loginUser, loginPass)
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}

	fmt.Printf("Credentials saved to %s\n", configPath)
	fmt.Printf("Logged in as %s @ %s\n", loginUser, loginURL)
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("No credentials saved.")
		return nil
	}

	if err := os.Remove(configPath); err != nil {
		return fmt.Errorf("remove config file: %w", err)
	}

	fmt.Println("Logged out. Credentials removed.")
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	currentURL := viper.GetString("api_url")
	currentUser := viper.GetString("username")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("Not logged in (no config file found).")
		fmt.Printf("Config path: %s\n", configPath)
		return nil
	}

	// Test current credentials
	client := newClient()
	_, apiErr := client.ListServices()

	fmt.Printf("Config file: %s\n", configPath)
	fmt.Printf("API URL:     %s\n", currentURL)
	fmt.Printf("Username:    %s\n", currentUser)

	if apiErr != nil {
		fmt.Printf("Status:      INVALID (credentials rejected by API)\n")
		fmt.Printf("Error:       %s\n", apiErr)
	} else {
		fmt.Printf("Status:      OK\n")
	}

	return nil
}
