package cmd

import (
	"testing"

	"github.com/spf13/viper"
)

func TestNewClient_FromViper(t *testing.T) {
	// Reset flag overrides so viper values are used
	apiURL = ""
	username = ""
	password = ""
	orgID = ""

	viper.Set("api_url", "http://example.com")
	viper.Set("username", "alice")
	viper.Set("password", "secret")
	viper.Set("org", "my-org")
	defer viper.Reset()

	client := newClient()

	if client.BaseURL != "http://example.com" {
		t.Errorf("expected BaseURL 'http://example.com', got %q", client.BaseURL)
	}
	if client.Username != "alice" {
		t.Errorf("expected Username 'alice', got %q", client.Username)
	}
	if client.Password != "secret" {
		t.Errorf("expected Password 'secret', got %q", client.Password)
	}
	if client.OrgID != "my-org" {
		t.Errorf("expected OrgID 'my-org', got %q", client.OrgID)
	}
	if client.HTTPClient == nil {
		t.Error("expected HTTPClient to be non-nil")
	}
}

func TestNewClient_FlagOverridesViper(t *testing.T) {
	// Set viper values first
	viper.Set("api_url", "http://viper-url.com")
	viper.Set("username", "viper-user")
	viper.Set("password", "viper-pass")
	viper.Set("org", "viper-org")
	defer viper.Reset()

	// Flag overrides
	apiURL = "http://flag-url.com"
	username = "flag-user"
	password = "flag-pass"
	orgID = "flag-org"
	defer func() {
		apiURL = ""
		username = ""
		password = ""
		orgID = ""
	}()

	client := newClient()

	if client.BaseURL != "http://flag-url.com" {
		t.Errorf("expected flag URL to override viper, got %q", client.BaseURL)
	}
	if client.Username != "flag-user" {
		t.Errorf("expected flag username to override viper, got %q", client.Username)
	}
	if client.Password != "flag-pass" {
		t.Errorf("expected flag password to override viper, got %q", client.Password)
	}
	if client.OrgID != "flag-org" {
		t.Errorf("expected flag org to override viper, got %q", client.OrgID)
	}
}

func TestNewClient_Defaults(t *testing.T) {
	// With nothing set, defaults from initConfig should apply.
	// Reset everything.
	apiURL = ""
	username = ""
	password = ""
	orgID = ""
	viper.Reset()
	// Apply defaults as initConfig does
	viper.SetDefault("api_url", "https://api.foundrydb.com")
	viper.SetDefault("username", "admin")

	client := newClient()

	if client.BaseURL != "https://api.foundrydb.com" {
		t.Errorf("expected default BaseURL, got %q", client.BaseURL)
	}
	if client.Username != "admin" {
		t.Errorf("expected default username 'admin', got %q", client.Username)
	}
	if client.OrgID != "" {
		t.Errorf("expected empty OrgID by default, got %q", client.OrgID)
	}
}

func TestGetConfigPath(t *testing.T) {
	path, err := getConfigPath()
	if err != nil {
		t.Fatalf("getConfigPath returned error: %v", err)
	}
	if path == "" {
		t.Error("expected non-empty config path")
	}
	if len(path) < 5 || path[len(path)-5:] != ".toml" {
		t.Errorf("expected .toml extension, got %q", path)
	}
}
