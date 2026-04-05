package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	foundrydb "github.com/anorph/foundrydb-sdk-go/foundrydb"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Manage database services",
}

var servicesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all managed services",
	RunE:  runServicesList,
}

var servicesGetCmd = &cobra.Command{
	Use:   "get <id-or-name>",
	Short: "Get details of a service",
	Args:  cobra.ExactArgs(1),
	RunE:  runServicesGet,
}

var servicesCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new managed service",
	RunE:  runServicesCreate,
}

var servicesDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete a managed service",
	Args:  cobra.ExactArgs(1),
	RunE:  runServicesDelete,
}

func init() {
	servicesCreateCmd.Flags().String("name", "", "Service name (required)")
	servicesCreateCmd.Flags().String("type", "", "Database type: postgresql, mysql, mongodb, valkey, kafka, opensearch, mssql (required)")
	servicesCreateCmd.Flags().String("version", "", "Database version (required). Supported versions: postgresql=14/15/16/17/18, mysql=8.4, mongodb=6.0/7.0/8.0, valkey=7.2/8.0/8.1/9.0, kafka=3.6/3.7/3.8/3.9/4.0, opensearch=2, mssql=4.8")
	servicesCreateCmd.Flags().String("plan", "tier-2", "Compute plan (e.g. tier-2)")
	servicesCreateCmd.Flags().String("zone", "se-sto1", "Cloud zone (default: se-sto1)")
	servicesCreateCmd.Flags().Int("storage-size", 50, "Storage size in GB")
	servicesCreateCmd.Flags().String("storage-tier", "maxiops", "Storage tier: standard or maxiops")
	servicesCreateCmd.Flags().StringSlice("allowed-cidrs", []string{}, "Allowed CIDR ranges")
	servicesCreateCmd.Flags().String("preset", "", "Service preset for AI agent workloads (e.g. ai-agent-pg, ai-agent-valkey)")
	servicesCreateCmd.Flags().Int("ttl-hours", 0, "Auto-delete service after N hours (1-720, for ephemeral workloads)")
	servicesCreateCmd.Flags().Bool("ephemeral", false, "Mark service as ephemeral (auto-cleanup enabled)")
	servicesCreateCmd.Flags().String("agent-framework", "", "AI agent framework: langchain, crewai, autogen, claude")
	servicesCreateCmd.Flags().String("agent-purpose", "", "AI agent purpose: conversation_history, session_cache")

	servicesDeleteCmd.Flags().Bool("confirm", false, "Skip confirmation prompt")

	servicesCmd.AddCommand(servicesListCmd)
	servicesCmd.AddCommand(servicesGetCmd)
	servicesCmd.AddCommand(servicesCreateCmd)
	servicesCmd.AddCommand(servicesDeleteCmd)
}

func runServicesList(cmd *cobra.Command, args []string) error {
	client := newClient()
	ctx := context.Background()
	services, err := client.ListServices(ctx)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(services)
	}

	if len(services) == 0 {
		fmt.Println("No services found.")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "NAME", "TYPE", "VERSION", "STATUS", "PLAN", "ZONE"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("  ")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	for _, svc := range services {
		shortID := svc.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		table.Append([]string{
			shortID,
			svc.Name,
			string(svc.DatabaseType),
			svc.Version,
			formatStatus(string(svc.Status)),
			svc.PlanName,
			svc.Zone,
		})
	}
	table.Render()
	fmt.Printf("\nTotal: %d services\n", len(services))
	return nil
}

func runServicesGet(cmd *cobra.Command, args []string) error {
	client := newClient()

	// Try to resolve by name if the arg is not a UUID
	svc, err := resolveService(client, args[0])
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(svc)
	}

	fmt.Printf("ID:           %s\n", svc.ID)
	fmt.Printf("Name:         %s\n", svc.Name)
	fmt.Printf("Database:     %s %s\n", string(svc.DatabaseType), svc.Version)
	fmt.Printf("Status:       %s\n", formatStatus(string(svc.Status)))
	fmt.Printf("Plan:         %s\n", svc.PlanName)
	fmt.Printf("Storage:      %d GB (%s)\n", svc.StorageSizeGB, string(svc.StorageTier))
	fmt.Printf("Zone:         %s\n", svc.Zone)
	fmt.Printf("Created:      %s\n", svc.CreatedAt)
	fmt.Printf("Updated:      %s\n", svc.UpdatedAt)

	if len(svc.DNSRecords) > 0 {
		fmt.Printf("\nDNS Records:\n")
		for _, rec := range svc.DNSRecords {
			if rec.RecordType != "" && rec.Value != "" {
				fmt.Printf("  %s (%s -> %s)\n", rec.FullDomain, rec.RecordType, rec.Value)
			} else {
				fmt.Printf("  %s\n", rec.FullDomain)
			}
		}
	}

	return nil
}

// createServiceRequestExtended extends the SDK's CreateServiceRequest with
// AI agent preset fields. These fields are serialized as part of the JSON body
// sent to the API.
type createServiceRequestExtended struct {
	foundrydb.CreateServiceRequest

	Preset         string `json:"preset,omitempty"`
	TTLHours       *int   `json:"ttl_hours,omitempty"`
	Ephemeral      *bool  `json:"ephemeral,omitempty"`
	AgentFramework string `json:"agent_framework,omitempty"`
	AgentPurpose   string `json:"agent_purpose,omitempty"`
}

func runServicesCreate(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	dbType, _ := cmd.Flags().GetString("type")
	version, _ := cmd.Flags().GetString("version")
	plan, _ := cmd.Flags().GetString("plan")
	zone, _ := cmd.Flags().GetString("zone")
	storageSize, _ := cmd.Flags().GetInt("storage-size")
	storageTier, _ := cmd.Flags().GetString("storage-tier")
	allowedCIDRs, _ := cmd.Flags().GetStringSlice("allowed-cidrs")
	preset, _ := cmd.Flags().GetString("preset")
	ttlHours, _ := cmd.Flags().GetInt("ttl-hours")
	ephemeral, _ := cmd.Flags().GetBool("ephemeral")
	agentFramework, _ := cmd.Flags().GetString("agent-framework")
	agentPurpose, _ := cmd.Flags().GetString("agent-purpose")

	// Prompt for missing required fields
	if name == "" {
		fmt.Print("Service name: ")
		fmt.Scanln(&name)
	}
	if name == "" {
		return fmt.Errorf("service name is required")
	}

	if dbType == "" {
		fmt.Print("Database type (postgresql/mysql/mongodb/valkey/kafka/opensearch/mssql): ")
		fmt.Scanln(&dbType)
	}
	if dbType == "" {
		return fmt.Errorf("database type is required")
	}

	validTypes := map[string]bool{
		"postgresql": true, "mysql": true, "mongodb": true,
		"valkey": true, "kafka": true, "opensearch": true, "mssql": true,
	}
	if !validTypes[dbType] {
		return fmt.Errorf("invalid database type %q, must be one of: postgresql, mysql, mongodb, valkey, kafka, opensearch, mssql", dbType)
	}

	if version == "" {
		defaultVersions := map[string]string{
			"postgresql": "17", "mysql": "8.4", "mongodb": "7.0",
			"valkey": "8.1", "kafka": "3.9", "opensearch": "2", "mssql": "4.8",
		}
		fmt.Printf("Database version [%s]: ", defaultVersions[dbType])
		fmt.Scanln(&version)
		if version == "" {
			version = defaultVersions[dbType]
		}
	}

	// Validate AI agent flags
	if agentFramework != "" {
		validFrameworks := map[string]bool{
			"langchain": true, "crewai": true, "autogen": true, "claude": true,
		}
		if !validFrameworks[agentFramework] {
			return fmt.Errorf("invalid agent framework %q, must be one of: langchain, crewai, autogen, claude", agentFramework)
		}
	}

	if agentPurpose != "" {
		validPurposes := map[string]bool{
			"conversation_history": true, "session_cache": true,
		}
		if !validPurposes[agentPurpose] {
			return fmt.Errorf("invalid agent purpose %q, must be one of: conversation_history, session_cache", agentPurpose)
		}
	}

	if ttlHours != 0 && (ttlHours < 1 || ttlHours > 720) {
		return fmt.Errorf("ttl-hours must be between 1 and 720, got %d", ttlHours)
	}

	baseReq := foundrydb.CreateServiceRequest{
		Name:          name,
		DatabaseType:  foundrydb.DatabaseType(dbType),
		Version:       version,
		PlanName:      plan,
		Zone:          zone,
		StorageSizeGB: &storageSize,
		StorageTier:   storageTier,
	}

	if len(allowedCIDRs) > 0 {
		baseReq.AllowedCIDRs = allowedCIDRs
	}

	hasPresetFields := preset != "" || ttlHours != 0 || ephemeral || agentFramework != "" || agentPurpose != ""

	statusParts := []string{
		fmt.Sprintf("%s %s", dbType, version),
		fmt.Sprintf("plan=%s", plan),
		fmt.Sprintf("zone=%s", zone),
		fmt.Sprintf("storage=%dGB %s", storageSize, storageTier),
	}
	if preset != "" {
		statusParts = append(statusParts, fmt.Sprintf("preset=%s", preset))
	}
	if ephemeral {
		statusParts = append(statusParts, "ephemeral=true")
	}
	if ttlHours != 0 {
		statusParts = append(statusParts, fmt.Sprintf("ttl=%dh", ttlHours))
	}

	fmt.Printf("Creating service %q (%s)...\n", name, strings.Join(statusParts, ", "))

	var svc *foundrydb.Service
	var err error

	if hasPresetFields {
		// Use extended request with AI agent fields via raw HTTP call,
		// since the SDK's CreateServiceRequest does not include these fields yet.
		extReq := createServiceRequestExtended{
			CreateServiceRequest: baseReq,
			Preset:               preset,
			AgentFramework:       agentFramework,
			AgentPurpose:         agentPurpose,
		}
		if ttlHours != 0 {
			extReq.TTLHours = &ttlHours
		}
		if ephemeral {
			extReq.Ephemeral = &ephemeral
		}
		svc, err = createServiceRaw(extReq)
	} else {
		client := newClient()
		ctx := context.Background()
		svc, err = client.CreateService(ctx, baseReq)
	}

	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(svc)
	}

	fmt.Printf("Service created successfully.\n")
	fmt.Printf("  ID:     %s\n", svc.ID)
	fmt.Printf("  Name:   %s\n", svc.Name)
	fmt.Printf("  Status: %s\n", svc.Status)
	if preset != "" {
		fmt.Printf("  Preset: %s\n", preset)
	}
	if ephemeral {
		fmt.Printf("  Ephemeral: yes\n")
	}
	if ttlHours != 0 {
		fmt.Printf("  TTL:    %d hours\n", ttlHours)
	}
	fmt.Printf("\nUse 'fdb services get %s' to monitor provisioning progress.\n", svc.ID)
	return nil
}

// createServiceRaw sends a service creation request with extended fields directly
// via HTTP, bypassing the SDK client which does not yet support preset fields.
func createServiceRaw(req createServiceRequestExtended) (*foundrydb.Service, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	apiBaseURL := viper.GetString("api_url")
	if apiURL != "" {
		apiBaseURL = apiURL
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")

	httpReq, err := http.NewRequest(http.MethodPost, apiBaseURL+"/managed-services", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
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
		return nil, fmt.Errorf("request POST /managed-services: %w", err)
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

	var svc foundrydb.Service
	if err := json.Unmarshal(respBody, &svc); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &svc, nil
}

func runServicesDelete(cmd *cobra.Command, args []string) error {
	confirmed, _ := cmd.Flags().GetBool("confirm")
	serviceID := args[0]

	client := newClient()

	// Resolve service to show name in confirmation
	svc, err := resolveService(client, serviceID)
	if err != nil {
		return err
	}

	if !confirmed {
		fmt.Printf("This will permanently delete service %q (ID: %s).\n", svc.Name, svc.ID)
		fmt.Print("Type the service name to confirm: ")
		var input string
		fmt.Scanln(&input)
		if input != svc.Name {
			return fmt.Errorf("confirmation failed: expected %q, got %q", svc.Name, input)
		}
	}

	fmt.Printf("Deleting service %q...\n", svc.Name)
	ctx := context.Background()
	if err := client.DeleteService(ctx, svc.ID); err != nil {
		return err
	}

	fmt.Printf("Service %q has been deleted.\n", svc.Name)
	return nil
}

// resolveService finds a service by ID or name
func resolveService(client *foundrydb.Client, idOrName string) (*foundrydb.Service, error) {
	ctx := context.Background()

	// Try direct ID lookup first
	svc, err := client.GetService(ctx, idOrName)
	if err == nil && svc != nil {
		return svc, nil
	}

	// If that failed, search by name in the list
	services, listErr := client.ListServices(ctx)
	if listErr != nil {
		return nil, fmt.Errorf("service not found by ID (%s) and could not list services: %w", err, listErr)
	}

	var matches []foundrydb.Service
	for _, s := range services {
		if s.Name == idOrName {
			matches = append(matches, s)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no service found with ID or name %q", idOrName)
	}
	if len(matches) > 1 {
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = m.ID
		}
		return nil, fmt.Errorf("multiple services named %q found, use an ID instead: %s", idOrName, strings.Join(ids, ", "))
	}

	return &matches[0], nil
}

func formatStatus(status string) string {
	// The API returns PascalCase statuses like "Running", "Pending", "ProvisioningVM", etc.
	// Pass them through as-is so they display accurately.
	return status
}

func printJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

// formatBytes formats bytes into a human-readable string
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return strconv.FormatFloat(float64(bytes)/GB, 'f', 1, 64) + " GB"
	case bytes >= MB:
		return strconv.FormatFloat(float64(bytes)/MB, 'f', 1, 64) + " MB"
	case bytes >= KB:
		return strconv.FormatFloat(float64(bytes)/KB, 'f', 1, 64) + " KB"
	default:
		return strconv.FormatInt(bytes, 10) + " B"
	}
}
