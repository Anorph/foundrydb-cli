package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/anorph/foundrydb-cli/internal/api"
)

func TestRunOrgList_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.OrganizationListResponse{Organizations: []api.Organization{}})
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "org", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No organizations found") {
		t.Errorf("expected 'No organizations found', got: %q", out)
	}
}

func TestRunOrgList_WithOrgs(t *testing.T) {
	orgs := []api.Organization{
		{ID: "org-abc123def456", Name: "Acme Corp", Slug: "acme", Role: "owner"},
		{ID: "org-xyz789", Name: "Sideproject", Slug: "side", Role: "member"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.OrganizationListResponse{
			Organizations: orgs,
		})
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "org", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Acme Corp") {
		t.Errorf("expected org name in output, got: %q", out)
	}
	if !strings.Contains(out, "acme") {
		t.Errorf("expected org slug in output, got: %q", out)
	}
	if !strings.Contains(out, "owner") {
		t.Errorf("expected role in output, got: %q", out)
	}
	if !strings.Contains(out, "Total: 2") {
		t.Errorf("expected total count in output, got: %q", out)
	}
}

func TestRunOrgList_CountMatchesOrgs(t *testing.T) {
	// The total is always len(Organizations) since there is no TotalCount field
	orgs := []api.Organization{
		{ID: "org-abc123", Name: "Acme", Slug: "acme", Role: "owner"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.OrganizationListResponse{
			Organizations: orgs,
		})
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "org", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Total: 1") {
		t.Errorf("expected total count 1, got: %q", out)
	}
}

func TestRunOrgList_ShortID(t *testing.T) {
	// IDs longer than 8 chars should be truncated in the table
	org := api.Organization{
		ID:   "org-abcdefghijklmnop",
		Name: "Long ID Org",
		Slug: "long",
		Role: "admin",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.OrganizationListResponse{
			Organizations: []api.Organization{org},
		})
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "org", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The full ID should NOT appear; only the first 8 chars
	if strings.Contains(out, "org-abcdefghijklmnop") {
		t.Errorf("expected truncated ID, but found full ID in output: %q", out)
	}
	if !strings.Contains(out, "org-abcd") {
		t.Errorf("expected truncated ID prefix in output, got: %q", out)
	}
}

func TestRunOrgList_ShortIDUnder8(t *testing.T) {
	// IDs shorter than or equal to 8 chars should not be truncated
	org := api.Organization{
		ID:   "org-abc",
		Name: "Short ID Org",
		Slug: "short",
		Role: "owner",
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.OrganizationListResponse{
			Organizations: []api.Organization{org},
		})
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "org", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "org-abc") {
		t.Errorf("expected full short ID in output, got: %q", out)
	}
}

func TestRunOrgList_JSONOut(t *testing.T) {
	orgs := []api.Organization{
		{ID: "org-abc123", Name: "Acme", Slug: "acme", Role: "owner"},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(api.OrganizationListResponse{
			Organizations: orgs,
		})
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	jsonOut = true
	defer func() { jsonOut = false }()

	out, err := executeCommand(t, "org", "list")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"organizations"`) {
		t.Errorf("expected JSON output, got: %q", out)
	}
}

func TestRunOrgList_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/organizations/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "org", "list")
	if err == nil {
		t.Fatal("expected error but got nil")
	}
}
