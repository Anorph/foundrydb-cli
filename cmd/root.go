package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anorph/foundrydb-cli/internal/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile  string
	apiURL   string
	username string
	password string
	jsonOut  bool
)

// rootCmd is the base command
var rootCmd = &cobra.Command{
	Use:     "fdb",
	Short:   "fdb - CLI for FoundryDB managed database platform",
	Version: "0.1.0",
	Long: `fdb is a command-line interface for managing databases on the FoundryDB platform.

It allows you to create, inspect, and manage PostgreSQL, MySQL, MongoDB,
Valkey, and Kafka services through a simple CLI.`,
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.fdb/config.toml)")
	rootCmd.PersistentFlags().StringVar(&apiURL, "api-url", "", "FoundryDB API base URL (default: https://api.foundrydb.com)")
	rootCmd.PersistentFlags().StringVar(&username, "username", "", "API username (default: admin)")
	rootCmd.PersistentFlags().StringVar(&password, "password", "", "API password")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output raw JSON instead of formatted tables")

	viper.BindPFlag("api_url", rootCmd.PersistentFlags().Lookup("api-url"))
	viper.BindPFlag("username", rootCmd.PersistentFlags().Lookup("username"))
	viper.BindPFlag("password", rootCmd.PersistentFlags().Lookup("password"))

	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(servicesCmd)
	rootCmd.AddCommand(usersCmd)
	rootCmd.AddCommand(backupsCmd)
	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(connectionStringCmd)
	rootCmd.AddCommand(logsCmd)
	rootCmd.AddCommand(metricsCmd)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			configDir := filepath.Join(home, ".fdb")
			viper.AddConfigPath(configDir)
			viper.SetConfigName("config")
			viper.SetConfigType("toml")
		}
	}

	viper.SetEnvPrefix("FDB")
	viper.AutomaticEnv()

	// Defaults
	viper.SetDefault("api_url", "https://api.foundrydb.com")
	viper.SetDefault("username", "admin")

	viper.ReadInConfig()
}

// newClient creates an API client from current config/flags
func newClient() *api.Client {
	url := viper.GetString("api_url")
	user := viper.GetString("username")
	pass := viper.GetString("password")

	// Flag overrides
	if apiURL != "" {
		url = apiURL
	}
	if username != "" {
		user = username
	}
	if password != "" {
		pass = password
	}

	return api.NewClient(url, user, pass)
}

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory: %w", err)
	}
	return filepath.Join(home, ".fdb", "config.toml"), nil
}
