package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var orgCmd = &cobra.Command{
	Use:   "org",
	Short: "Manage organizations",
}

var orgListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all organizations the authenticated user belongs to",
	RunE:  runOrgList,
}

func init() {
	orgCmd.AddCommand(orgListCmd)
}

func runOrgList(cmd *cobra.Command, args []string) error {
	client := newClient()
	ctx := context.Background()
	orgs, err := client.ListOrganizations(ctx)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(orgs)
	}

	if len(orgs) == 0 {
		fmt.Println("No organizations found.")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"ID", "NAME", "SLUG", "ROLE"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("  ")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	for _, org := range orgs {
		shortID := org.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		table.Append([]string{
			shortID,
			org.Name,
			org.Slug,
			org.Role,
		})
	}
	table.Render()
	fmt.Printf("\nTotal: %d organization(s)\n", len(orgs))
	return nil
}
