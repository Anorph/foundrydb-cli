package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func sampleAdapter() inferenceModelAdapter {
	svc := "svc-abc"
	promoted := "2026-07-17T02:00:00Z"
	return inferenceModelAdapter{
		ID:                 "adp12345-0000-0000-0000-000000000000",
		OrganizationID:     "org-1",
		InferenceServiceID: &svc,
		BaseModelID:        "mistral-small",
		ServedModelName:    "support-bot",
		Version:            2,
		FilesBucket:        "org-1-adapters",
		FilesKeyPrefix:     "support-bot/v2",
		AdapterSHA256:      strings.Repeat("a", 64),
		SizeBytes:          104857600,
		BaseModelLicense:   "apache-2.0",
		Status:             "active",
		CreatedAt:          "2026-07-17T00:00:00Z",
		PromotedAt:         &promoted,
	}
}

// resetRegisterFlags clears any flag values persisted on the shared cobra
// command by a previous Execute, so each register test starts from defaults.
func resetRegisterFlags() {
	f := inferenceAdaptersRegisterCmd.Flags()
	for _, name := range []string{"base-model", "served-model-name", "files-bucket", "files-key-prefix", "sha256", "base-model-license"} {
		_ = f.Set(name, "")
	}
	_ = f.Set("version", "0")
	_ = f.Set("size-bytes", "0")
}

func TestRunInferenceAdaptersRegister_Success(t *testing.T) {
	resetRegisterFlags()
	uploaded := sampleAdapter()
	uploaded.InferenceServiceID = nil
	uploaded.Status = "uploaded"
	uploaded.PromotedAt = nil

	var gotBody map[string]interface{}
	mux := http.NewServeMux()
	mux.HandleFunc("/inference-services/adapters", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		json.NewEncoder(w).Encode(inferenceAdapterEnvelope{Adapter: uploaded})
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "inference", "adapters", "register",
		"--base-model", "mistral-small",
		"--served-model-name", "support-bot",
		"--version", "2",
		"--files-bucket", "org-1-adapters",
		"--files-key-prefix", "support-bot/v2",
		"--sha256", strings.Repeat("a", 64),
		"--size-bytes", "104857600",
		"--base-model-license", "apache-2.0",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotBody["base_model_id"] != "mistral-small" {
		t.Errorf("expected base_model_id in body, got %v", gotBody["base_model_id"])
	}
	if gotBody["served_model_name"] != "support-bot" {
		t.Errorf("expected served_model_name in body, got %v", gotBody["served_model_name"])
	}
	if gotBody["adapter_sha256"] != strings.Repeat("a", 64) {
		t.Errorf("expected adapter_sha256 in body")
	}
	if !strings.Contains(out, "uploaded") {
		t.Errorf("expected status uploaded in output, got: %q", out)
	}
}

func TestRunInferenceAdaptersRegister_MissingFlags(t *testing.T) {
	resetRegisterFlags()
	mux := http.NewServeMux()
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "inference", "adapters", "register",
		"--served-model-name", "support-bot")
	if err == nil {
		t.Fatal("expected an error for missing required flags")
	}
	if !strings.Contains(err.Error(), "missing or invalid required flags") {
		t.Errorf("expected validation error, got: %v", err)
	}
}

func TestRunInferenceAdaptersList_Success(t *testing.T) {
	uploaded := sampleAdapter()
	uploaded.ID = "adp99999-0000-0000-0000-000000000000"
	uploaded.InferenceServiceID = nil
	uploaded.Version = 3
	uploaded.Status = "uploaded"
	uploaded.PromotedAt = nil
	active := sampleAdapter()

	mux := http.NewServeMux()
	mux.HandleFunc("/inference-services/svc-abc/adapters", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		json.NewEncoder(w).Encode(inferenceAdapterListEnvelope{
			Adapters: []inferenceModelAdapter{uploaded, active},
		})
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "inference", "adapters", "list", "svc-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "support-bot") {
		t.Errorf("expected served model in output, got: %q", out)
	}
	if !strings.Contains(out, "uploaded") || !strings.Contains(out, "active") {
		t.Errorf("expected both statuses in output, got: %q", out)
	}
}

func TestRunInferenceAdaptersList_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/inference-services/svc-abc/adapters", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(inferenceAdapterListEnvelope{Adapters: []inferenceModelAdapter{}})
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "inference", "adapters", "list", "svc-abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "No adapters found") {
		t.Errorf("expected empty message, got: %q", out)
	}
}

func TestRunInferenceAdaptersPromote_Success(t *testing.T) {
	active := sampleAdapter()

	mux := http.NewServeMux()
	mux.HandleFunc("/inference-services/svc-abc/adapters/adp-1/promote", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		json.NewEncoder(w).Encode(inferenceAdapterEnvelope{Adapter: active})
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	out, err := executeCommand(t, "inference", "adapters", "promote", "svc-abc", "adp-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Promoted") || !strings.Contains(out, "active") {
		t.Errorf("expected promotion confirmation, got: %q", out)
	}
}

func TestRunInferenceAdaptersPromote_APIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/inference-services/svc-abc/adapters/adp-1/promote", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"base model mismatch"}`, http.StatusBadRequest)
	})
	_, cleanup := setupTestServer(t, mux)
	defer cleanup()

	_, err := executeCommand(t, "inference", "adapters", "promote", "svc-abc", "adp-1")
	if err == nil {
		t.Fatal("expected an error on HTTP 400")
	}
	if !strings.Contains(err.Error(), "HTTP 400") {
		t.Errorf("expected HTTP 400 error, got: %v", err)
	}
}
