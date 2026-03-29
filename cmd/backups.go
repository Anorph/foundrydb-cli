package cmd

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var backupsCmd = &cobra.Command{
	Use:   "backups",
	Short: "Manage database backups",
}

var backupsListCmd = &cobra.Command{
	Use:   "list <service-id>",
	Short: "List backups for a service",
	Args:  cobra.ExactArgs(1),
	RunE:  runBackupsList,
}

var backupsTriggerCmd = &cobra.Command{
	Use:   "trigger <service-id>",
	Short: "Trigger a manual backup",
	Args:  cobra.ExactArgs(1),
	RunE:  runBackupsTrigger,
}

func init() {
	backupsCmd.AddCommand(backupsListCmd)
	backupsCmd.AddCommand(backupsTriggerCmd)
}

func runBackupsList(cmd *cobra.Command, args []string) error {
	client := newClient()

	svc, err := resolveService(client, args[0])
	if err != nil {
		return err
	}

	result, err := client.ListBackups(svc.ID)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(result)
	}

	if len(result.Backups) == 0 {
		fmt.Printf("No backups found for service %q.\n", svc.Name)
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "TYPE", "STATUS", "SIZE", "CREATED AT"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("  ")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	for _, b := range result.Backups {
		shortID := b.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		sizeStr := "-"
		if b.SizeBytes > 0 {
			sizeStr = formatBytes(b.SizeBytes)
		}
		table.Append([]string{
			shortID,
			b.Type,
			b.Status,
			sizeStr,
			b.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	table.Render()
	fmt.Printf("\nTotal: %d backups\n", len(result.Backups))
	return nil
}

func runBackupsTrigger(cmd *cobra.Command, args []string) error {
	client := newClient()

	svc, err := resolveService(client, args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Triggering backup for service %q...\n", svc.Name)

	result, err := client.TriggerBackup(svc.ID)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(result)
	}

	fmt.Printf("Backup triggered successfully.\n")
	fmt.Printf("  Backup ID: %s\n", result.ID)
	fmt.Printf("  Status:    %s\n", result.Status)
	fmt.Printf("\nUse 'fdb backups list %s' to monitor progress.\n", svc.ID)
	return nil
}
