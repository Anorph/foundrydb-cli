package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// requestLogsResponse is the response from POST /managed-services/{id}/logs
type requestLogsResponse struct {
	TaskID string `json:"task_id"`
}

// logsResponse is the response from GET /managed-services/{id}/logs?task_id=X
type logsResponse struct {
	Status string `json:"status"`
	Logs   string `json:"logs"`
}

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

	taskResp, err := doRequestLogs(svc.ID, lines)
	if err != nil {
		return fmt.Errorf("request logs: %w", err)
	}

	// Poll for log results with a timeout of 60 seconds
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		logsResp, pollErr := doPollLogs(svc.ID, taskResp.TaskID)
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

// doRequestLogs sends POST /managed-services/{id}/logs?lines=N and returns the task ID.
func doRequestLogs(serviceID string, lines int) (*requestLogsResponse, error) {
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

	path := fmt.Sprintf("%s/managed-services/%s/logs?lines=%d", baseURL, serviceID, lines)
	req, err := http.NewRequest(http.MethodPost, path, nil)
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
	var result requestLogsResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}

// doPollLogs sends GET /managed-services/{id}/logs?task_id=X and returns the log result.
func doPollLogs(serviceID, taskID string) (*logsResponse, error) {
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

	path := fmt.Sprintf("%s/managed-services/%s/logs?task_id=%s", baseURL, serviceID, taskID)
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
	var result logsResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &result, nil
}
