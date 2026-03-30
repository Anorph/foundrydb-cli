package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/anorph/foundrydb-cli/internal/api"
)

func TestRunLogs_Success(t *testing.T) {
	svc := sampleService()
	taskResp := api.RequestLogsResponse{TaskID: "task-abc123"}
	logsResp := api.LogsResponse{Status: "completed", Logs: "line1\nline2\nline3"}

	pollCount := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/logs"):
			json.NewEncoder(w).Encode(taskResp)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/logs") && strings.Contains(r.URL.RawQuery, "task_id"):
			pollCount++
			json.NewEncoder(w).Encode(logsResp)
		default:
			json.NewEncoder(w).Encode(svc)
		}
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "logs", svc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "line1") {
		t.Errorf("expected log lines in output, got: %q", out)
	}
	if pollCount == 0 {
		t.Error("expected at least one poll for log results")
	}
}

func TestRunLogs_JSONOut(t *testing.T) {
	svc := sampleService()
	taskResp := api.RequestLogsResponse{TaskID: "task-abc123"}
	logsResp := api.LogsResponse{Status: "done", Logs: "line1\nline2"}

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/logs"):
			json.NewEncoder(w).Encode(taskResp)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/logs"):
			json.NewEncoder(w).Encode(logsResp)
		default:
			json.NewEncoder(w).Encode(svc)
		}
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	jsonOut = true
	defer func() { jsonOut = false }()

	out, err := executeCommand(t, "logs", svc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, `"logs"`) {
		t.Errorf("expected JSON output, got: %q", out)
	}
}

func TestRunLogs_SuccessStatus(t *testing.T) {
	// Test that "success" status (not just "completed"/"done") is accepted
	svc := sampleService()
	taskResp := api.RequestLogsResponse{TaskID: "task-xyz"}
	logsResp := api.LogsResponse{Status: "success", Logs: "success log output"}

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/logs"):
			json.NewEncoder(w).Encode(taskResp)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/logs"):
			json.NewEncoder(w).Encode(logsResp)
		default:
			json.NewEncoder(w).Encode(svc)
		}
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "logs", svc.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "success log output") {
		t.Errorf("expected log output, got: %q", out)
	}
}

func TestRunLogs_Failed(t *testing.T) {
	svc := sampleService()
	taskResp := api.RequestLogsResponse{TaskID: "task-fail"}
	logsResp := api.LogsResponse{Status: "failed"}

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/logs"):
			json.NewEncoder(w).Encode(taskResp)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/logs"):
			json.NewEncoder(w).Encode(logsResp)
		default:
			json.NewEncoder(w).Encode(svc)
		}
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "logs", svc.ID)
	if err == nil {
		t.Fatal("expected error when log retrieval fails")
	}
	if !strings.Contains(err.Error(), "log retrieval failed") {
		t.Errorf("expected 'log retrieval failed' error, got: %v", err)
	}
}

func TestRunLogs_RequestError(t *testing.T) {
	svc := sampleService()
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/logs") {
			http.Error(w, "server error", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "logs", svc.ID)
	if err == nil {
		t.Fatal("expected error from log request")
	}
}

func TestRunLogs_PollError(t *testing.T) {
	svc := sampleService()
	taskResp := api.RequestLogsResponse{TaskID: "task-poll-error"}

	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/logs"):
			json.NewEncoder(w).Encode(taskResp)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/logs"):
			http.Error(w, "server error", http.StatusInternalServerError)
		default:
			json.NewEncoder(w).Encode(svc)
		}
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "logs", svc.ID)
	if err == nil {
		t.Fatal("expected error from poll")
	}
	if !strings.Contains(err.Error(), "poll logs") {
		t.Errorf("expected 'poll logs' error, got: %v", err)
	}
}

func TestRunLogs_WithLinesFlag(t *testing.T) {
	svc := sampleService()
	taskResp := api.RequestLogsResponse{TaskID: "task-lines"}
	logsResp := api.LogsResponse{Status: "completed", Logs: "line1"}

	var capturedURL string
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/logs"):
			capturedURL = r.URL.String()
			json.NewEncoder(w).Encode(taskResp)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/logs"):
			json.NewEncoder(w).Encode(logsResp)
		default:
			json.NewEncoder(w).Encode(svc)
		}
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "logs", svc.ID, "--lines", "200")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(capturedURL, "lines=200") {
		t.Errorf("expected lines=200 in request URL, got: %q", capturedURL)
	}
}
