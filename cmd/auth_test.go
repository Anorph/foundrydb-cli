package cmd

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	foundrydb "github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/spf13/viper"
)

func TestRunAuthLogout_NoConfig(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	cfgFile = ""

	out, err := executeCommand(t, "auth", "logout")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No credentials saved") {
		t.Errorf("expected 'No credentials saved', got: %q", out)
	}
}

func TestRunAuthLogout_WithConfig(t *testing.T) {
	dir := t.TempDir()
	fdbDir := filepath.Join(dir, ".fdb")
	if err := os.MkdirAll(fdbDir, 0700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(fdbDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(`api_url = "http://example.com"`), 0600); err != nil {
		t.Fatal(err)
	}

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	cfgFile = ""

	out, err := executeCommand(t, "auth", "logout")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Logged out") {
		t.Errorf("expected 'Logged out', got: %q", out)
	}
	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		t.Errorf("expected config file to be removed after logout")
	}
}

func TestRunAuthStatus_NoConfig(t *testing.T) {
	dir := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	cfgFile = ""

	viper.Reset()
	viper.SetDefault("api_url", "https://api.foundrydb.com")
	viper.SetDefault("username", "admin")

	apiURL = ""
	username = ""
	password = ""
	orgID = ""

	out, err := executeCommand(t, "auth", "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Not logged in") {
		t.Errorf("expected 'Not logged in', got: %q", out)
	}
}

func TestRunAuthStatus_WithValidCredentials(t *testing.T) {
	svc := sampleService()
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(listServicesResponse{Services: []foundrydb.Service{svc}})
	})
	srv, cleanup := setupTestServer(t, mux)
	defer cleanup()

	dir := t.TempDir()
	fdbDir := filepath.Join(dir, ".fdb")
	os.MkdirAll(fdbDir, 0700)
	configContent := `api_url = "` + srv.URL + `"` + "\nusername = \"test\"\npassword = \"test\"\n"
	os.WriteFile(filepath.Join(fdbDir, "config.toml"), []byte(configContent), 0600)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	viper.Reset()
	viper.Set("api_url", srv.URL)
	viper.Set("username", "test")
	viper.Set("password", "test")

	apiURL = ""
	username = ""
	password = ""
	orgID = ""

	out, err := executeCommand(t, "auth", "status")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "API URL") {
		t.Errorf("expected API URL in output, got: %q", out)
	}
}

func TestRunAuthLogin_WithFlags(t *testing.T) {
	svc := sampleService()
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(listServicesResponse{Services: []foundrydb.Service{svc}})
	})
	srv, cleanup := setupTestServer(t, mux)
	defer cleanup()

	dir := t.TempDir()
	fdbDir := filepath.Join(dir, ".fdb")
	os.MkdirAll(fdbDir, 0700)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", dir)
	defer os.Setenv("HOME", oldHome)

	viper.Reset()

	out, err := executeCommand(t, "auth", "login",
		"--api-url", srv.URL,
		"--username", "test",
		"--password", "testpass",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Logged in as") {
		t.Errorf("expected 'Logged in as' in output, got: %q", out)
	}
	configPath := filepath.Join(dir, ".fdb", "config.toml")
	data, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Errorf("expected config file to be created, got error: %v", readErr)
	} else if !strings.Contains(string(data), "testpass") {
		t.Errorf("expected password in config file, got: %s", data)
	}
}

func TestRunAuthLogin_InvalidCredentials(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	srv, cleanup := setupTestServer(t, mux)
	defer cleanup()

	viper.Reset()

	_, err := executeCommand(t, "auth", "login",
		"--api-url", srv.URL,
		"--username", "bad",
		"--password", "wrong",
	)
	if err == nil {
		t.Fatal("expected authentication error")
	}
	if !strings.Contains(err.Error(), "authentication failed") {
		t.Errorf("expected 'authentication failed' error, got: %v", err)
	}
}
