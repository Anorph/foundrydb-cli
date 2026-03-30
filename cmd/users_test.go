package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/anorph/foundrydb-cli/internal/api"
)

func sampleUsers() []api.DatabaseUser {
	return []api.DatabaseUser{
		{Username: "app_user", CreatedAt: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)},
		{Username: "readonly", CreatedAt: time.Date(2025, 1, 2, 8, 0, 0, 0, time.UTC)},
	}
}

func TestRunUsersList_WithUsers(t *testing.T) {
	svc := sampleService()
	users := sampleUsers()

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/database-users") {
			json.NewEncoder(w).Encode(api.UserListResponse{Users: users})
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "users", "list", svc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "app_user") {
		t.Errorf("expected username in output, got: %q", out)
	}
	if !strings.Contains(out, "readonly") {
		t.Errorf("expected username in output, got: %q", out)
	}
	if !strings.Contains(out, "2025-01-01") {
		t.Errorf("expected created_at in output, got: %q", out)
	}
}

func TestRunUsersList_Empty(t *testing.T) {
	svc := sampleService()
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/database-users") {
			json.NewEncoder(w).Encode(api.UserListResponse{Users: []api.DatabaseUser{}})
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "users", "list", svc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No users found") {
		t.Errorf("expected 'No users found', got: %q", out)
	}
}

func TestRunUsersList_JSONOut(t *testing.T) {
	svc := sampleService()
	users := sampleUsers()

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/database-users") {
			json.NewEncoder(w).Encode(api.UserListResponse{Users: users})
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	jsonOut = true
	defer func() { jsonOut = false }()

	out, err := executeCommand(t, "users", "list", svc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"users"`) {
		t.Errorf("expected JSON output, got: %q", out)
	}
}

func TestRunUsersRevealPassword_Success(t *testing.T) {
	svc := sampleService()
	revealResp := api.RevealPasswordResponse{
		Username:         "app_user",
		Password:         "s3cr3t",
		Host:             "my-pg.db.foundrydb.com",
		Port:             5432,
		Database:         "defaultdb",
		ConnectionString: "postgresql://app_user:s3cr3t@my-pg.db.foundrydb.com:5432/defaultdb",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reveal-password") {
			json.NewEncoder(w).Encode(revealResp)
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "users", "reveal-password", svc.ID, "app_user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "app_user") {
		t.Errorf("expected username in output, got: %q", out)
	}
	if !strings.Contains(out, "s3cr3t") {
		t.Errorf("expected password in output, got: %q", out)
	}
	if !strings.Contains(out, "my-pg.db.foundrydb.com") {
		t.Errorf("expected host in output, got: %q", out)
	}
	if !strings.Contains(out, "5432") {
		t.Errorf("expected port in output, got: %q", out)
	}
	if !strings.Contains(out, "Connection String") {
		t.Errorf("expected connection string in output, got: %q", out)
	}
}

func TestRunUsersRevealPassword_NoConnectionString(t *testing.T) {
	svc := sampleService()
	revealResp := api.RevealPasswordResponse{
		Username: "app_user",
		Password: "s3cr3t",
		Host:     "my-pg.db.foundrydb.com",
		Port:     5432,
		Database: "defaultdb",
		// ConnectionString intentionally empty
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reveal-password") {
			json.NewEncoder(w).Encode(revealResp)
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "users", "reveal-password", svc.ID, "app_user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(out, "Connection String") {
		t.Errorf("did not expect connection string when empty, got: %q", out)
	}
}

func TestRunUsersRevealPassword_JSONOut(t *testing.T) {
	svc := sampleService()
	revealResp := api.RevealPasswordResponse{
		Username: "app_user",
		Password: "s3cr3t",
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reveal-password") {
			json.NewEncoder(w).Encode(revealResp)
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	jsonOut = true
	defer func() { jsonOut = false }()

	out, err := executeCommand(t, "users", "reveal-password", svc.ID, "app_user")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"username"`) {
		t.Errorf("expected JSON output, got: %q", out)
	}
}

func TestRunUsersRevealPassword_APIError(t *testing.T) {
	svc := sampleService()
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reveal-password") {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "users", "reveal-password", svc.ID, "app_user")
	if err == nil {
		t.Fatal("expected error from API")
	}
}
