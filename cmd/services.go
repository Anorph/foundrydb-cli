package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/anorph/foundrydb-cli/internal/api"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
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

	servicesDeleteCmd.Flags().Bool("confirm", false, "Skip confirmation prompt")

	servicesCmd.AddCommand(servicesListCmd)
	servicesCmd.AddCommand(servicesGetCmd)
	servicesCmd.AddCommand(servicesCreateCmd)
	servicesCmd.AddCommand(servicesDeleteCmd)
}

func runServicesList(cmd *cobra.Command, args []string) error {
	client := newClient()
	result, err := client.ListServices()
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(result)
	}

	if len(result.Services) == 0 {
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

	for _, svc := range result.Services {
		shortID := svc.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		table.Append([]string{
			shortID,
			svc.Name,
			svc.DatabaseType,
			svc.Version,
			formatStatus(svc.Status),
			svc.PlanName,
			svc.Zone,
		})
	}
	table.Render()
	fmt.Printf("\nTotal: %d services\n", result.TotalCount)
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
	fmt.Printf("Database:     %s %s\n", svc.DatabaseType, svc.Version)
	fmt.Printf("Status:       %s\n", formatStatus(svc.Status))
	fmt.Printf("Plan:         %s\n", svc.PlanName)
	fmt.Printf("Storage:      %d GB (%s)\n", svc.StorageSizeGB, svc.StorageTier)
	fmt.Printf("Zone:         %s\n", svc.Zone)
	fmt.Printf("Created:      %s\n", svc.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Updated:      %s\n", svc.UpdatedAt.Format("2006-01-02 15:04:05"))

	if len(svc.DNSRecords) > 0 {
		fmt.Printf("\nDNS Records:\n")
		for _, rec := range svc.DNSRecords {
			fmt.Printf("  %s:%d (%s)\n", rec.FullDomain, rec.Port, rec.Type)
		}
	}

	if len(svc.Nodes) > 0 {
		fmt.Printf("\nNodes:\n")
		for _, node := range svc.Nodes {
			fmt.Printf("  %s  role=%-10s  ip=%s\n", node.ID[:8], node.Role, node.IP)
		}
	}

	return nil
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

	req := api.CreateServiceRequest{
		Name:          name,
		DatabaseType:  dbType,
		Version:       version,
		PlanName:      plan,
		Zone:          zone,
		StorageSizeGB: &storageSize,
		StorageTier:   storageTier,
	}

	if len(allowedCIDRs) > 0 {
		req.AllowedCIDRs = allowedCIDRs
	}

	fmt.Printf("Creating service %q (%s %s, plan=%s, zone=%s, storage=%dGB %s)...\n",
		name, dbType, version, plan, zone, storageSize, storageTier)

	client := newClient()
	svc, err := client.CreateService(req)
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
	fmt.Printf("\nUse 'fdb services get %s' to monitor provisioning progress.\n", svc.ID)
	return nil
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
	if err := client.DeleteService(svc.ID); err != nil {
		return err
	}

	fmt.Printf("Service %q has been deleted.\n", svc.Name)
	return nil
}

// resolveService finds a service by ID or name
func resolveService(client *api.Client, idOrName string) (*api.Service, error) {
	// Try direct ID lookup first
	svc, err := client.GetService(idOrName)
	if err == nil {
		return svc, nil
	}

	// If that failed, search by name in the list
	list, listErr := client.ListServices()
	if listErr != nil {
		return nil, fmt.Errorf("service not found by ID (%s) and could not list services: %w", err, listErr)
	}

	var matches []api.Service
	for _, s := range list.Services {
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
	switch status {
	case "running":
		return "running"
	case "provisioning", "pending":
		return "provisioning"
	case "stopped":
		return "stopped"
	case "error", "failed":
		return "error"
	case "deleting":
		return "deleting"
	default:
		return status
	}
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
