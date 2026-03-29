package cmd

import (
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "Manage database users",
}

var usersListCmd = &cobra.Command{
	Use:   "list <service-id>",
	Short: "List database users for a service",
	Args:  cobra.ExactArgs(1),
	RunE:  runUsersList,
}

var usersRevealPasswordCmd = &cobra.Command{
	Use:   "reveal-password <service-id> <username>",
	Short: "Reveal the password for a database user",
	Args:  cobra.ExactArgs(2),
	RunE:  runUsersRevealPassword,
}

func init() {
	usersCmd.AddCommand(usersListCmd)
	usersCmd.AddCommand(usersRevealPasswordCmd)
}

func runUsersList(cmd *cobra.Command, args []string) error {
	client := newClient()

	svc, err := resolveService(client, args[0])
	if err != nil {
		return err
	}

	result, err := client.ListUsers(svc.ID)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(result)
	}

	if len(result.Users) == 0 {
		fmt.Printf("No users found for service %q.\n", svc.Name)
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader([]string{"USERNAME", "CREATED AT"})
	table.SetBorder(false)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("  ")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetTablePadding("  ")
	table.SetNoWhiteSpace(true)

	for _, u := range result.Users {
		table.Append([]string{
			u.Username,
			u.CreatedAt.Format("2006-01-02 15:04:05"),
		})
	}
	table.Render()
	return nil
}

func runUsersRevealPassword(cmd *cobra.Command, args []string) error {
	client := newClient()

	svc, err := resolveService(client, args[0])
	if err != nil {
		return err
	}

	username := args[1]
	result, err := client.RevealPassword(svc.ID, username)
	if err != nil {
		return err
	}

	if jsonOut {
		return printJSON(result)
	}

	fmt.Printf("Username:          %s\n", result.Username)
	fmt.Printf("Password:          %s\n", result.Password)
	fmt.Printf("Host:              %s\n", result.Host)
	fmt.Printf("Port:              %d\n", result.Port)
	fmt.Printf("Database:          %s\n", result.Database)
	if result.ConnectionString != "" {
		fmt.Printf("Connection String: %s\n", result.ConnectionString)
	}
	return nil
}
