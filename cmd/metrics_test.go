package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestRunMetrics_Success(t *testing.T) {
	svc := sampleService()
	metricsResp := metricsResponse{
		ServiceID:    svc.ID,
		DatabaseType: "postgresql",
		Timestamp:    "2025-01-01T12:00:00Z",
		Metrics: metricsData{
			CPUUsagePercent:           42.5,
			MemoryUsagePercent:        60.0,
			DiskUsagePercent:          15.3,
			DatabaseConnectionsActive: 7,
			DatabaseQueriesPerSecond:  100.5,
			DatabaseCacheHitRatio:     98.2,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/metrics/current") {
			json.NewEncoder(w).Encode(metricsResp)
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "metrics", svc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "42.5") {
		t.Errorf("expected CPU usage in output, got: %q", out)
	}
	if !strings.Contains(out, "60.0") {
		t.Errorf("expected memory usage in output, got: %q", out)
	}
	if !strings.Contains(out, "15.3") {
		t.Errorf("expected disk usage in output, got: %q", out)
	}
	if !strings.Contains(out, "7") {
		t.Errorf("expected active connections in output, got: %q", out)
	}
	if !strings.Contains(out, "100.5") {
		t.Errorf("expected queries/sec in output, got: %q", out)
	}
	if !strings.Contains(out, svc.Name) {
		t.Errorf("expected service name in output, got: %q", out)
	}
}

func TestRunMetrics_JSONOut(t *testing.T) {
	svc := sampleService()
	metricsResp := metricsResponse{
		ServiceID: svc.ID,
		Metrics:   metricsData{CPUUsagePercent: 25.0},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/metrics/current") {
			json.NewEncoder(w).Encode(metricsResp)
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	jsonOut = true
	defer func() { jsonOut = false }()

	out, err := executeCommand(t, "metrics", svc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"metrics"`) {
		t.Errorf("expected JSON output, got: %q", out)
	}
}

func TestRunMetrics_APIError(t *testing.T) {
	svc := sampleService()
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/metrics/current") {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "metrics", svc.ID)
	if err == nil {
		t.Fatal("expected error from metrics API")
	}
}

func TestRunMetrics_ServiceNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	mux.HandleFunc("/managed-services", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "metrics", "nonexistent-service")
	if err == nil {
		t.Fatal("expected error when service not found")
	}
}
