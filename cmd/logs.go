package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs <service-id>",
	Short: "Retrieve logs from a service",
	Args:  cobra.ExactArgs(1),
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().IntP("lines", "n", 100, "Number of log lines to retrieve")
}

func runLogs(cmd *cobra.Command, args []string) error {
	client := newClient()
	lines, _ := cmd.Flags().GetInt("lines")

	svc, err := resolveService(client, args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Requesting logs for service %q (last %d lines)...\n", svc.Name, lines)

	taskResp, err := client.RequestLogs(svc.ID, lines)
	if err != nil {
		return fmt.Errorf("request logs: %w", err)
	}

	// Poll for log results with a timeout of 60 seconds
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		logsResp, pollErr := client.PollLogs(svc.ID, taskResp.TaskID)
		if pollErr != nil {
			return fmt.Errorf("poll logs: %w", pollErr)
		}

		switch logsResp.Status {
		case "completed", "done", "success":
			if jsonOut {
				return printJSON(logsResp)
			}
			fmt.Println(logsResp.Logs)
			return nil
		case "failed", "error":
			return fmt.Errorf("log retrieval failed")
		default:
			// Still pending - wait before retrying
			time.Sleep(2 * time.Second)
		}
	}

	return fmt.Errorf("timed out waiting for log retrieval (task_id: %s)", taskResp.TaskID)
}
