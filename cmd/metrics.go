package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

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

	metricsResp, err := client.GetMetrics(svc.ID)
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
