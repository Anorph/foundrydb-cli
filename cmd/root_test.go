package cmd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	foundrydb "github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/spf13/viper"
)

func TestNewClient_FromViper(t *testing.T) {
	// Reset flag overrides so viper values are used
	apiURL = ""
	username = ""
	password = ""
	orgID = ""

	svc := sampleService()
	mux := http.NewServeMux()
	var capturedAuth string
	var capturedOrgID string
	mux.HandleFunc("/managed-services", func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedOrgID = r.Header.Get("X-Active-Org-ID")
		json.NewEncoder(w).Encode(foundrydb.ListServicesResponse{Services: []foundrydb.Service{svc}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	viper.Set("api_url", srv.URL)
	viper.Set("username", "alice")
	viper.Set("password", "secret")
	viper.Set("org", "my-org")
	defer viper.Reset()

	client := newClient()
	if client == nil {
		t.Fatal("expected non-nil client")
	}

	// Verify credentials are used by making a call
	_, err := client.ListServices(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Basic auth should be set
	if capturedAuth == "" {
		t.Error("expected Authorization header to be sent")
	}
	if capturedOrgID != "my-org" {
		t.Errorf("expected OrgID 'my-org', got %q", capturedOrgID)
	}
}

func TestNewClient_FlagOverridesViper(t *testing.T) {
	svc := sampleService()
	mux := http.NewServeMux()
	var capturedOrgID string
	mux.HandleFunc("/managed-services", func(w http.ResponseWriter, r *http.Request) {
		capturedOrgID = r.Header.Get("X-Active-Org-ID")
		json.NewEncoder(w).Encode(foundrydb.ListServicesResponse{Services: []foundrydb.Service{svc}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Set viper values first
	viper.Set("api_url", "http://viper-url.com")
	viper.Set("username", "viper-user")
	viper.Set("password", "viper-pass")
	viper.Set("org", "viper-org")
	defer viper.Reset()

	// Flag overrides
	apiURL = srv.URL
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
	_, err := client.ListServices(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if capturedOrgID != "flag-org" {
		t.Errorf("expected flag org 'flag-org' to override viper, got %q", capturedOrgID)
	}
}

func TestNewClient_Defaults(t *testing.T) {
	// With nothing set, newClient() should return a non-nil client.
	apiURL = ""
	username = ""
	password = ""
	orgID = ""
	viper.Reset()
	viper.SetDefault("api_url", "https://api.foundrydb.com")
	viper.SetDefault("username", "admin")

	client := newClient()
	if client == nil {
		t.Error("expected non-nil client with defaults")
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
