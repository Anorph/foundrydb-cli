package cmd

import (
	"context"
	"fmt"
	"os"

	foundrydb "github.com/anorph/foundrydb-sdk-go/foundrydb"
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

	ctx := context.Background()
	backups, err := client.ListBackups(ctx, svc.ID)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(backups)
	}

	if len(backups) == 0 {
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

	for _, b := range backups {
		shortID := b.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		sizeStr := "-"
		if b.SizeBytes != nil && *b.SizeBytes > 0 {
			sizeStr = formatBytes(*b.SizeBytes)
		}
		table.Append([]string{
			shortID,
			string(b.BackupType),
			string(b.Status),
			sizeStr,
			b.CreatedAt,
		})
	}
	table.Render()
	fmt.Printf("\nTotal: %d backups\n", len(backups))
	return nil
}

func runBackupsTrigger(cmd *cobra.Command, args []string) error {
	client := newClient()

	svc, err := resolveService(client, args[0])
	if err != nil {
		return err
	}

	fmt.Printf("Triggering backup for service %q...\n", svc.Name)

	ctx := context.Background()
	backup, err := client.TriggerBackup(ctx, svc.ID, foundrydb.CreateBackupRequest{})
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(backup)
	}

	fmt.Printf("Backup triggered successfully.\n")
	fmt.Printf("  Backup ID: %s\n", backup.ID)
	fmt.Printf("  Status:    %s\n", backup.Status)
	fmt.Printf("\nUse 'fdb backups list %s' to monitor progress.\n", svc.ID)
	return nil
}
