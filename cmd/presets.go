package cmd

import (
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

// Preset describes a service configuration preset returned by the API.
type Preset struct {
	Name           string `json:"name"`
	DisplayName    string `json:"display_name"`
	DatabaseType   string `json:"database_type"`
	Description    string `json:"description"`
	DefaultVersion string `json:"default_version"`
	DefaultPlan    string `json:"default_plan"`
	DefaultStorage int    `json:"default_storage_gb"`
	AgentFramework string `json:"agent_framework,omitempty"`
	AgentPurpose   string `json:"agent_purpose,omitempty"`
}

// ListPresetsResponse is the envelope returned by GET /presets.
type ListPresetsResponse struct {
	Presets []Preset `json:"presets"`
}

var presetsCmd = &cobra.Command{
	Use:   "presets",
	Short: "Manage service presets for AI agent workloads",
}

var presetsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available service presets",
	RunE:  runPresetsList,
}

func init() {
	presetsCmd.AddCommand(presetsListCmd)
}

func runPresetsList(cmd *cobra.Command, args []string) error {
	presets, err := fetchPresets()
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(presets)
	}

	if len(presets) == 0 {
		fmt.Println("No presets available.")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"NAME", "DISPLAY NAME", "DATABASE", "VERSION", "PLAN", "STORAGE", "DESCRIPTION"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("  ")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	for _, p := range presets {
		storageStr := ""
		if p.DefaultStorage > 0 {
			storageStr = fmt.Sprintf("%d GB", p.DefaultStorage)
		}
		desc := p.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		table.Append([]string{
			p.Name,
			p.DisplayName,
			p.DatabaseType,
			p.DefaultVersion,
			p.DefaultPlan,
			storageStr,
			desc,
		})
	}
	table.Render()
	fmt.Printf("\nTotal: %d presets\n", len(presets))
	fmt.Println("\nUsage: fdb services create --preset <name> --name <service-name>")
	return nil
}

// fetchPresets retrieves the list of available presets from the API.
func fetchPresets() ([]Preset, error) {
	apiBaseURL := viper.GetString("api_url")
	if apiURL != "" {
		apiBaseURL = apiURL
	}
	apiBaseURL = strings.TrimRight(apiBaseURL, "/")

	req, err := http.NewRequest(http.MethodGet, apiBaseURL+"/presets", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	user := viper.GetString("username")
	pass := viper.GetString("password")
	if username != "" {
		user = username
	}
	if password != "" {
		pass = password
	}
	req.SetBasicAuth(user, pass)

	org := viper.GetString("org")
	if orgID != "" {
		org = orgID
	}
	if org != "" {
		req.Header.Set("X-Active-Org-ID", org)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request GET /presets: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(body))
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, msg)
	}

	var result ListPresetsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Presets, nil
}
