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

	metrics, err := client.GetMetrics(svc.ID)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(metrics)
	}

	fmt.Printf("Metrics for service %q\n\n", svc.Name)
	fmt.Printf("  CPU Usage:          %.1f%%\n", metrics.CPUUsagePercent)
	fmt.Printf("  Memory Usage:       %.1f%%\n", metrics.MemoryUsagePercent)
	fmt.Printf("  Disk Usage:         %.1f%%\n", metrics.DiskUsagePercent)
	fmt.Printf("  Active Connections: %d\n", metrics.ActiveConnections)
	fmt.Printf("  Reads/sec:          %.1f\n", metrics.ReadsPerSecond)
	fmt.Printf("  Writes/sec:         %.1f\n", metrics.WritesPerSecond)

	return nil
}
