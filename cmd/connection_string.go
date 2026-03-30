package cmd

import (
	"context"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

var connectionStringCmd = &cobra.Command{
	Use:   "connection-string <service-id>",
	Short: "Get connection string for a service",
	Long: `Returns a connection string in the requested format:
  url      - Generic URL (e.g. postgresql://user:pass@host:port/db)
  env      - Shell environment variable exports
  psql     - psql command-line arguments
  mysql    - mysql command-line arguments
  mongosh  - mongosh connection URI
  redis-cli - redis-cli command-line arguments`,
	Args: cobra.ExactArgs(1),
	RunE: runConnectionString,
}

func init() {
	connectionStringCmd.Flags().StringP("user", "u", "", "Database username (required)")
	connectionStringCmd.Flags().String("format", "url", "Output format: url, env, psql, mysql, mongosh, redis-cli")
	connectionStringCmd.Flags().StringP("database", "d", "defaultdb", "Database name")
	connectionStringCmd.MarkFlagRequired("user")
}

func runConnectionString(cmd *cobra.Command, args []string) error {
	client := newClient()

	svc, err := resolveService(client, args[0])
	if err != nil {
		return err
	}

	connectUser, _ := cmd.Flags().GetString("user")
	format, _ := cmd.Flags().GetString("format")
	database, _ := cmd.Flags().GetString("database")

	ctx := context.Background()
	creds, err := client.RevealPassword(ctx, svc.ID, connectUser)
	if err != nil {
		return fmt.Errorf("reveal password for user %q: %w", connectUser, err)
	}

	host := creds.Host
	port := int(creds.Port)
	if host == "" {
		h, p, hErr := getHostPort(svc)
		if hErr != nil {
			return hErr
		}
		host = h
		port = p
	}
	if database == "defaultdb" && creds.Database != "" {
		database = creds.Database
	}

	dbType := string(svc.DatabaseType)

	switch format {
	case "url":
		fmt.Println(buildURL(dbType, host, port, connectUser, creds.Password, database))

	case "env":
		printEnvFormat(dbType, host, port, connectUser, creds.Password, database)

	case "psql":
		if dbType != "postgresql" {
			return fmt.Errorf("psql format is only valid for PostgreSQL services")
		}
		fmt.Printf("PGPASSWORD=%s psql \"host=%s port=%d user=%s dbname=%s sslmode=require\"\n",
			shellEscape(creds.Password), host, port, connectUser, database)

	case "mysql":
		if dbType != "mysql" {
			return fmt.Errorf("mysql format is only valid for MySQL services")
		}
		fmt.Printf("mysql -h %s -P %d -u %s -p'%s' --ssl-mode=REQUIRED %s\n",
			host, port, connectUser, creds.Password, database)

	case "mongosh":
		if dbType != "mongodb" {
			return fmt.Errorf("mongosh format is only valid for MongoDB services")
		}
		fmt.Printf("mongosh \"mongodb://%s:%s@%s:%d/%s?tls=true&tlsAllowInvalidCertificates=true\"\n",
			connectUser, url.QueryEscape(creds.Password), host, port, database)

	case "redis-cli":
		if dbType != "valkey" {
			return fmt.Errorf("redis-cli format is only valid for Valkey services")
		}
		fmt.Printf("redis-cli -h %s -p %d --tls --user %s --pass '%s' --insecure\n",
			host, port, connectUser, creds.Password)

	default:
		return fmt.Errorf("unknown format %q; valid formats: url, env, psql, mysql, mongosh, redis-cli", format)
	}

	return nil
}

func buildURL(dbType, host string, port int, user, password, database string) string {
	scheme := map[string]string{
		"postgresql": "postgresql",
		"mysql":      "mysql",
		"mongodb":    "mongodb",
		"valkey":     "rediss",
		"kafka":      "kafka",
	}[dbType]
	if scheme == "" {
		scheme = dbType
	}

	encodedPass := url.QueryEscape(password)

	switch dbType {
	case "valkey":
		return fmt.Sprintf("%s://%s:%s@%s:%d", scheme, user, encodedPass, host, port)
	case "mongodb":
		return fmt.Sprintf("%s://%s:%s@%s:%d/%s?tls=true&tlsAllowInvalidCertificates=true",
			scheme, user, encodedPass, host, port, database)
	default:
		return fmt.Sprintf("%s://%s:%s@%s:%d/%s?sslmode=require",
			scheme, user, encodedPass, host, port, database)
	}
}

func printEnvFormat(dbType, host string, port int, user, password, database string) {
	switch dbType {
	case "postgresql":
		fmt.Printf("export PGHOST=%s\n", host)
		fmt.Printf("export PGPORT=%d\n", port)
		fmt.Printf("export PGUSER=%s\n", user)
		fmt.Printf("export PGPASSWORD=%s\n", password)
		fmt.Printf("export PGDATABASE=%s\n", database)
		fmt.Printf("export PGSSLMODE=require\n")
	case "mysql":
		fmt.Printf("export MYSQL_HOST=%s\n", host)
		fmt.Printf("export MYSQL_PORT=%d\n", port)
		fmt.Printf("export MYSQL_USER=%s\n", user)
		fmt.Printf("export MYSQL_PWD=%s\n", password)
		fmt.Printf("export MYSQL_DATABASE=%s\n", database)
	case "mongodb":
		connStr := fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?tls=true&tlsAllowInvalidCertificates=true",
			user, url.QueryEscape(password), host, port, database)
		fmt.Printf("export MONGODB_URI=%s\n", connStr)
	case "valkey":
		fmt.Printf("export REDIS_HOST=%s\n", host)
		fmt.Printf("export REDIS_PORT=%d\n", port)
		fmt.Printf("export REDIS_USER=%s\n", user)
		fmt.Printf("export REDIS_PASSWORD=%s\n", password)
		fmt.Printf("export REDIS_TLS=true\n")
	default:
		connStr := buildURL(dbType, host, port, user, password, database)
		fmt.Printf("export DATABASE_URL=%s\n", connStr)
	}
}

// shellEscape escapes a string for use in shell single-quote context
func shellEscape(s string) string {
	// Wrap in single quotes, escaping existing single quotes
	result := ""
	for _, c := range s {
		if c == '\'' {
			result += "'\\''"
		} else {
			result += string(c)
		}
	}
	return "'" + result + "'"
}
