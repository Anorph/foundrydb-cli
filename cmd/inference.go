package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// inferenceModelAdapter is one version of a customer LoRA fine-tuned adapter in
// the serving registry of a managed inference service. Promoting it downloads
// the weights onto the base-model GPU, verifies their hash, and hot-loads them
// into vLLM; once active the service answers to it as
// foundrydb_managed/<served_model_name> on the OpenAI-compatible endpoint.
type inferenceModelAdapter struct {
	ID                 string  `json:"id"`
	OrganizationID     string  `json:"organization_id"`
	InferenceServiceID *string `json:"inference_service_id"`
	BaseModelID        string  `json:"base_model_id"`
	ServedModelName    string  `json:"served_model_name"`
	Version            int     `json:"version"`
	FilesBucket        string  `json:"files_bucket"`
	FilesKeyPrefix     string  `json:"files_key_prefix"`
	AdapterSHA256      string  `json:"adapter_sha256"`
	SizeBytes          int64   `json:"size_bytes"`
	BaseModelLicense   string  `json:"base_model_license,omitempty"`
	Status             string  `json:"status"`
	CreatedAt          string  `json:"created_at"`
	PromotedAt         *string `json:"promoted_at,omitempty"`
}

type inferenceAdapterEnvelope struct {
	Adapter inferenceModelAdapter `json:"adapter"`
}

type inferenceAdapterListEnvelope struct {
	Adapters []inferenceModelAdapter `json:"adapters"`
}

var inferenceCmd = &cobra.Command{
	Use:   "inference",
	Short: "Manage inference services",
}

var inferenceAdaptersCmd = &cobra.Command{
	Use:   "adapters",
	Short: "Manage LoRA fine-tuned adapters for an inference service",
}

var inferenceAdaptersRegisterCmd = &cobra.Command{
	Use:   "register",
	Short: "Register an uploaded LoRA adapter version",
	Long: "Record an uploaded LoRA fine-tuned adapter version in the serving registry, making it promotable.\n\n" +
		"Call it after uploading the adapter artifact (adapter_model.safetensors and adapter_config.json) to\n" +
		"the organization's Files bucket. The version enters the registry with status \"uploaded\" and is not\n" +
		"bound to any service until it is promoted.",
	RunE: runInferenceAdaptersRegister,
}

var inferenceAdaptersListCmd = &cobra.Command{
	Use:   "list <service-id>",
	Short: "List the LoRA adapter versions for an inference service",
	Long: "List the LoRA fine-tuned adapter versions relevant to the service, newest first: the versions bound\n" +
		"to it (the active version plus its superseded history) together with the organization's uploaded,\n" +
		"not-yet-promoted versions trained on this service's base model, so a freshly registered adapter can\n" +
		"be promoted from here.",
	Args: cobra.ExactArgs(1),
	RunE: runInferenceAdaptersList,
}

var inferenceAdaptersPromoteCmd = &cobra.Command{
	Use:   "promote <service-id> <adapter-id>",
	Short: "Promote a LoRA adapter version onto the serving GPU",
	Long: "Promote a LoRA fine-tuned adapter version onto the service's serving GPU: the platform downloads the\n" +
		"weights from Files, verifies their hash, and hot-loads them into vLLM with no restart. The promoted\n" +
		"version becomes active and any previously active version is marked superseded. Rollback is the same\n" +
		"command on a prior (superseded) version.",
	Args: cobra.ExactArgs(2),
	RunE: runInferenceAdaptersPromote,
}

func init() {
	inferenceAdaptersRegisterCmd.Flags().String("base-model", "", "Base model the adapter was trained against (required)")
	inferenceAdaptersRegisterCmd.Flags().String("served-model-name", "", "Customer-facing served model name; letters, digits, '.', '_', '-' only, max 128 (required)")
	inferenceAdaptersRegisterCmd.Flags().Int("version", 0, "Monotonic version per (organization, served model name), at least 1 (required)")
	inferenceAdaptersRegisterCmd.Flags().String("files-bucket", "", "Files bucket holding the adapter artifact (required)")
	inferenceAdaptersRegisterCmd.Flags().String("files-key-prefix", "", "Files key prefix holding adapter_model.safetensors and adapter_config.json (required)")
	inferenceAdaptersRegisterCmd.Flags().String("sha256", "", "64-character lowercase hex sha256 of adapter_model.safetensors (required)")
	inferenceAdaptersRegisterCmd.Flags().Int64("size-bytes", 0, "Size of the adapter artifact in bytes (required)")
	inferenceAdaptersRegisterCmd.Flags().String("base-model-license", "", "Base-model license that travels with the weights (optional)")

	inferenceAdaptersCmd.AddCommand(inferenceAdaptersRegisterCmd)
	inferenceAdaptersCmd.AddCommand(inferenceAdaptersListCmd)
	inferenceAdaptersCmd.AddCommand(inferenceAdaptersPromoteCmd)
	inferenceCmd.AddCommand(inferenceAdaptersCmd)
}

// inferenceRequest issues an authenticated request to the inference-services
// API and returns the raw response body. It resolves the API URL, basic-auth
// credentials, and active organization from flags and config the same way the
// SDK client does, and is used directly because the pinned SDK release does not
// yet expose the adapter registry operations.
func inferenceRequest(method, path string, body []byte) ([]byte, error) {
	apiBaseURL := viper.GetString("api_url")
	if apiURL != "" {
		apiBaseURL = apiURL
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	httpReq, err := http.NewRequest(method, apiBaseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set("Accept", "application/json")

	user := viper.GetString("username")
	pass := viper.GetString("password")
	if username != "" {
		user = username
	}
	if password != "" {
		pass = password
	}
	httpReq.SetBasicAuth(user, pass)

	org := viper.GetString("org")
	if orgID != "" {
		org = orgID
	}
	if org != "" {
		httpReq.Header.Set("X-Active-Org-ID", org)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(respBody))
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, msg)
	}
	return respBody, nil
}

func runInferenceAdaptersRegister(cmd *cobra.Command, args []string) error {
	baseModel, _ := cmd.Flags().GetString("base-model")
	servedModelName, _ := cmd.Flags().GetString("served-model-name")
	version, _ := cmd.Flags().GetInt("version")
	filesBucket, _ := cmd.Flags().GetString("files-bucket")
	filesKeyPrefix, _ := cmd.Flags().GetString("files-key-prefix")
	sha256, _ := cmd.Flags().GetString("sha256")
	sizeBytes, _ := cmd.Flags().GetInt64("size-bytes")
	baseModelLicense, _ := cmd.Flags().GetString("base-model-license")

	missing := []string{}
	if baseModel == "" {
		missing = append(missing, "--base-model")
	}
	if servedModelName == "" {
		missing = append(missing, "--served-model-name")
	}
	if version < 1 {
		missing = append(missing, "--version (>= 1)")
	}
	if filesBucket == "" {
		missing = append(missing, "--files-bucket")
	}
	if filesKeyPrefix == "" {
		missing = append(missing, "--files-key-prefix")
	}
	if sha256 == "" {
		missing = append(missing, "--sha256")
	}
	if sizeBytes < 0 {
		missing = append(missing, "--size-bytes (>= 0)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing or invalid required flags: %s", strings.Join(missing, ", "))
	}

	payload := map[string]interface{}{
		"base_model_id":     baseModel,
		"served_model_name": servedModelName,
		"version":           version,
		"files_bucket":      filesBucket,
		"files_key_prefix":  filesKeyPrefix,
		"adapter_sha256":    sha256,
		"size_bytes":        sizeBytes,
	}
	if baseModelLicense != "" {
		payload["base_model_license"] = baseModelLicense
	}
	// The active organization is resolved server-side from the request (basic
	// auth plus the X-Active-Org-ID header set by inferenceRequest).
	if org := activeOrg(); org != "" {
		payload["organization_id"] = org
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	respBody, err := inferenceRequest(http.MethodPost, "/inference-services/adapters", body)
	if err != nil {
		return err
	}

	var env inferenceAdapterEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if jsonOut {
		return printJSON(env.Adapter)
	}
	fmt.Printf("Registered adapter %s (%s v%d), status %s.\n",
		env.Adapter.ID, env.Adapter.ServedModelName, env.Adapter.Version, env.Adapter.Status)
	return nil
}

func runInferenceAdaptersList(cmd *cobra.Command, args []string) error {
	serviceID := args[0]

	respBody, err := inferenceRequest(http.MethodGet, "/inference-services/"+serviceID+"/adapters", nil)
	if err != nil {
		return err
	}

	var env inferenceAdapterListEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if jsonOut {
		return printJSON(env.Adapters)
	}

	if len(env.Adapters) == 0 {
		fmt.Println("No adapters found.")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "SERVED MODEL", "VERSION", "STATUS", "BASE MODEL", "PROMOTED AT"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("  ")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	for _, a := range env.Adapters {
		shortID := a.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		promoted := "-"
		if a.PromotedAt != nil && *a.PromotedAt != "" {
			promoted = *a.PromotedAt
		}
		table.Append([]string{
			shortID,
			a.ServedModelName,
			fmt.Sprintf("%d", a.Version),
			a.Status,
			a.BaseModelID,
			promoted,
		})
	}
	table.Render()
	return nil
}

func runInferenceAdaptersPromote(cmd *cobra.Command, args []string) error {
	serviceID := args[0]
	adapterID := args[1]

	respBody, err := inferenceRequest(http.MethodPost,
		"/inference-services/"+serviceID+"/adapters/"+adapterID+"/promote", nil)
	if err != nil {
		return err
	}

	var env inferenceAdapterEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if jsonOut {
		return printJSON(env.Adapter)
	}
	fmt.Printf("Promoted adapter %s (%s v%d) — now %s.\n",
		env.Adapter.ID, env.Adapter.ServedModelName, env.Adapter.Version, env.Adapter.Status)
	return nil
}

// activeOrg resolves the effective organization id from the --org flag or
// config, matching how newClient scopes requests.
func activeOrg() string {
	org := viper.GetString("org")
	if orgID != "" {
		org = orgID
	}
	return org
}
