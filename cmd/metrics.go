package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// metricsData holds the core metrics values returned inside the "metrics" key
type metricsData struct {
	CPUUsagePercent               float64 `json:"cpu_usage_percent"`
	MemoryUsagePercent            float64 `json:"memory_usage_percent"`
	DiskUsagePercent              float64 `json:"disk_usage_percent"`
	DatabaseConnectionsActive     int     `json:"database_connections_active"`
	DatabaseQueriesPerSecond      float64 `json:"database_queries_per_second"`
	DatabaseCacheHitRatio         float64 `json:"database_cache_hit_ratio"`
	DatabaseReplicationLagSeconds float64 `json:"database_replication_lag_seconds"`
}

// metricsResponse is the full response from GET /managed-services/{id}/metrics/current
type metricsResponse struct {
	ServiceID    string      `json:"service_id"`
	DatabaseType string      `json:"database_type"`
	Timestamp    string      `json:"timestamp"`
	Metrics      metricsData `json:"metrics"`
}

var metricsCmd = &cobra.Command{
	Use:   "metrics <service-id>",
	Short: "Show current metrics for a service",
	Args:  cobra.ExactArgs(1),
	RunE:  runMetrics,
}

func runMetrics(cmd *cobra.Command, args []string) error {
	client := newClient()

	svc, err := resolveService(client, args[0])
	if err != nil {
		return err
	}

	metricsResp, err := doGetMetrics(svc.ID)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(metricsResp)
	}

	m := metricsResp.Metrics
	fmt.Printf("Metrics for service %q\n\n", svc.Name)
	fmt.Printf("  CPU Usage:          %.1f%%\n", m.CPUUsagePercent)
	fmt.Printf("  Memory Usage:       %.1f%%\n", m.MemoryUsagePercent)
	fmt.Printf("  Disk Usage:         %.1f%%\n", m.DiskUsagePercent)
	fmt.Printf("  Active Connections: %d\n", m.DatabaseConnectionsActive)
	fmt.Printf("  Queries/sec:        %.1f\n", m.DatabaseQueriesPerSecond)
	fmt.Printf("  Cache Hit Ratio:    %.1f%%\n", m.DatabaseCacheHitRatio)

	return nil
}

// doGetMetrics sends GET /managed-services/{id}/metrics/current and returns the response.
func doGetMetrics(serviceID string) (*metricsResponse, error) {
	baseURL := viper.GetString("api_url")
	user := viper.GetString("username")
	pass := viper.GetString("password")
	org := viper.GetString("org")
	if apiURL != "" {
		baseURL = apiURL
	}
	if username != "" {
		user = username
	}
	if password != "" {
		pass = password
	}
	if orgID != "" {
		org = orgID
	}

	path := fmt.Sprintf("%s/managed-services/%s/metrics/current", baseURL, serviceID)
	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(user, pass)
	req.Header.Set("Accept", "application/json")
	if org != "" {
		req.Header.Set("X-Active-Org-ID", org)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(data))
	}
	var result metricsResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}
