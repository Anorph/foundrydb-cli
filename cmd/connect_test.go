package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/anorph/foundrydb-cli/internal/api"
)

func TestDefaultPort(t *testing.T) {
	tests := []struct {
		dbType   string
		expected int
	}{
		{"postgresql", 5432},
		{"mysql", 3306},
		{"mongodb", 27017},
		{"valkey", 6380},
		{"kafka", 9093},
		{"unknown", 5432}, // falls through to default
	}

	for _, tc := range tests {
		t.Run(tc.dbType, func(t *testing.T) {
			got := defaultPort(tc.dbType)
			if got != tc.expected {
				t.Errorf("defaultPort(%q) = %d, want %d", tc.dbType, got, tc.expected)
			}
		})
	}
}

func TestGetHostPort_FromDNS(t *testing.T) {
	svc := &api.Service{
		Name: "my-pg",
		DNSRecords: []api.DNSRecord{
			{FullDomain: "my-pg.db.foundrydb.com", Port: 5432},
		},
	}
	host, port, err := getHostPort(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "my-pg.db.foundrydb.com" {
		t.Errorf("expected DNS domain, got %q", host)
	}
	if port != 5432 {
		t.Errorf("expected port 5432, got %d", port)
	}
}

func TestGetHostPort_FromNodes(t *testing.T) {
	svc := &api.Service{
		Name:         "my-pg",
		DatabaseType: "postgresql",
		Nodes: []api.Node{
			{ID: "node-abc", Role: "primary", IP: "10.0.0.5"},
		},
	}
	host, port, err := getHostPort(svc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "10.0.0.5" {
		t.Errorf("expected node IP, got %q", host)
	}
	if port != 5432 {
		t.Errorf("expected default postgresql port 5432, got %d", port)
	}
}

func TestGetHostPort_NoRecordsNoNodes(t *testing.T) {
	svc := &api.Service{
		Name:       "my-pg",
		DNSRecords: nil,
		Nodes:      nil,
	}
	_, _, err := getHostPort(svc)
	if err == nil {
		t.Fatal("expected error when no DNS records or nodes")
	}
	if !strings.Contains(err.Error(), "no DNS records") {
		t.Errorf("expected 'no DNS records' error, got: %v", err)
	}
}

func TestLaunchShell_KafkaUnsupported(t *testing.T) {
	err := launchShell("kafka", "host", 9093, "user", "pass", "db")
	if err == nil {
		t.Fatal("expected error for kafka shell")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("expected 'not supported' error, got: %v", err)
	}
}

func TestLaunchShell_UnknownType(t *testing.T) {
	err := launchShell("mssql", "host", 1433, "user", "pass", "db")
	if err == nil {
		t.Fatal("expected error for unsupported database type")
	}
	if !strings.Contains(err.Error(), "unsupported database type") {
		t.Errorf("expected 'unsupported database type' error, got: %v", err)
	}
}

func TestRunConnect_ServiceNotRunning(t *testing.T) {
	svc := sampleService()
	svc.Status = "provisioning"

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "connect", svc.ID)
	if err == nil {
		t.Fatal("expected error when service is not running")
	}
	if !strings.Contains(err.Error(), "not running") {
		t.Errorf("expected 'not running' error, got: %v", err)
	}
}

func TestRunConnect_NoHostOrNodes(t *testing.T) {
	svc := sampleService()
	// No DNS records and no nodes - should get a host error
	svc.DNSRecords = nil
	svc.Nodes = nil

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "connect", svc.ID)
	if err == nil {
		t.Fatal("expected error when service has no DNS records or nodes")
	}
	if !strings.Contains(err.Error(), "no DNS records") {
		t.Errorf("expected 'no DNS records' error, got: %v", err)
	}
}

func TestRunConnect_ServiceNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "connect", "nonexistent")
	if err == nil {
		t.Fatal("expected error when service not found")
	}
}
