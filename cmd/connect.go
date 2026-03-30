package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/anorph/foundrydb-cli/internal/api"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect <service-name-or-id>",
	Short: "Open an interactive database shell for a service",
	Long: `Opens an interactive database shell using the appropriate client:
  postgresql -> psql
  mysql      -> mysql
  mongodb    -> mongosh
  valkey     -> redis-cli
  kafka      -> kafka-console-consumer (requires kafkacat)`,
	Args: cobra.ExactArgs(1),
	RunE: runConnect,
}

func init() {
	connectCmd.Flags().StringP("user", "u", "", "Database username (uses first available user if not specified)")
	connectCmd.Flags().StringP("database", "d", "defaultdb", "Database name to connect to")
}

func runConnect(cmd *cobra.Command, args []string) error {
	client := newClient()

	svc, err := resolveService(client, args[0])
	if err != nil {
		return err
	}

	if !strings.EqualFold(svc.Status, "running") {
		return fmt.Errorf("service %q is not running (status: %s)", svc.Name, svc.Status)
	}

	// Find host and port from DNS records
	host, port, err := getHostPort(svc)
	if err != nil {
		return err
	}

	// Determine user
	connectUser, _ := cmd.Flags().GetString("user")
	connectDB, _ := cmd.Flags().GetString("database")

	if connectUser == "" {
		users, usersErr := client.ListUsers(svc.ID)
		if usersErr != nil || len(users.Users) == 0 {
			return fmt.Errorf("could not determine a user to connect with; use --user to specify one")
		}
		connectUser = users.Users[0].Username
	}

	// Reveal the password
	creds, err := client.RevealPassword(svc.ID, connectUser)
	if err != nil {
		return fmt.Errorf("reveal password for user %q: %w", connectUser, err)
	}

	fmt.Printf("Connecting to %s service %q as %s...\n\n", svc.DatabaseType, svc.Name, connectUser)

	return launchShell(svc.DatabaseType, host, port, connectUser, creds.Password, connectDB)
}

func getHostPort(svc *api.Service) (string, int, error) {
	if len(svc.DNSRecords) > 0 {
		return svc.DNSRecords[0].FullDomain, svc.DNSRecords[0].Port, nil
	}
	if len(svc.Nodes) > 0 {
		return svc.Nodes[0].IP, defaultPort(svc.DatabaseType), nil
	}
	return "", 0, fmt.Errorf("service %q has no DNS records or nodes", svc.Name)
}

func defaultPort(dbType string) int {
	switch dbType {
	case "postgresql":
		return 5432
	case "mysql":
		return 3306
	case "mongodb":
		return 27017
	case "valkey":
		return 6380
	case "kafka":
		return 9093
	default:
		return 5432
	}
}

func launchShell(dbType, host string, port int, user, password, database string) error {
	switch dbType {
	case "postgresql":
		return launchPsql(host, port, user, password, database)
	case "mysql":
		return launchMySQL(host, port, user, password, database)
	case "mongodb":
		return launchMongosh(host, port, user, password, database)
	case "valkey":
		return launchRedisCLI(host, port, user, password)
	case "kafka":
		return fmt.Errorf("interactive Kafka shell not supported; use kafka-console-consumer directly")
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}
}

func launchPsql(host string, port int, user, password, database string) error {
	binary, err := exec.LookPath("psql")
	if err != nil {
		return fmt.Errorf("psql not found in PATH; install PostgreSQL client tools")
	}

	cmd := exec.Command(binary,
		fmt.Sprintf("host=%s user=%s dbname=%s port=%d sslmode=require", host, user, database, port),
	)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", password))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func launchMySQL(host string, port int, user, password, database string) error {
	binary, err := exec.LookPath("mysql")
	if err != nil {
		return fmt.Errorf("mysql client not found in PATH; install MySQL client tools")
	}

	cmd := exec.Command(binary,
		"-h", host,
		"-P", fmt.Sprintf("%d", port),
		"-u", user,
		fmt.Sprintf("-p%s", password),
		"--ssl-mode=REQUIRED",
		database,
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func launchMongosh(host string, port int, user, password, database string) error {
	binary, err := exec.LookPath("mongosh")
	if err != nil {
		// Fall back to mongo
		binary, err = exec.LookPath("mongo")
		if err != nil {
			return fmt.Errorf("mongosh (or mongo) not found in PATH; install MongoDB Shell")
		}
	}

	connStr := fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?tls=true&tlsAllowInvalidCertificates=true",
		user, password, host, port, database)
	cmd := exec.Command(binary, connStr)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func launchRedisCLI(host string, port int, user, password string) error {
	binary, err := exec.LookPath("redis-cli")
	if err != nil {
		return fmt.Errorf("redis-cli not found in PATH; install Redis client tools")
	}

	cmd := exec.Command(binary,
		"-h", host,
		"-p", fmt.Sprintf("%d", port),
		"--tls",
		"--user", user,
		"--pass", password,
		"--insecure",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
