package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	foundrydb "github.com/anorph/foundrydb-sdk-go/foundrydb"
)

func buildConnStringMux(svc foundrydb.Service, creds foundrydb.RevealPasswordResponse) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/managed-services/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/reveal-password") {
			json.NewEncoder(w).Encode(creds)
			return
		}
		json.NewEncoder(w).Encode(svc)
	})
	return mux
}

func sampleCreds() foundrydb.RevealPasswordResponse {
	return foundrydb.RevealPasswordResponse{
		Username: "app_user",
		Password: "s3cr3t",
		Host:     "my-pg.db.foundrydb.com",
		Port:     5432,
		Database: "defaultdb",
	}
}

func TestRunConnectionString_URLFormat_PostgreSQL(t *testing.T) {
	svc := sampleService()
	creds := sampleCreds()

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "postgresql://") {
		t.Errorf("expected postgresql:// scheme, got: %q", out)
	}
	if !strings.Contains(out, "app_user") {
		t.Errorf("expected username in URL, got: %q", out)
	}
	if !strings.Contains(out, "my-pg.db.foundrydb.com") {
		t.Errorf("expected host in URL, got: %q", out)
	}
}

func TestRunConnectionString_URLFormat_MySQL(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.MySQL
	creds := sampleCreds()
	creds.Port = 3306

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "mysql://") {
		t.Errorf("expected mysql:// scheme, got: %q", out)
	}
}

func TestRunConnectionString_URLFormat_MongoDB(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.MongoDB
	creds := sampleCreds()
	creds.Port = 27017

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "mongodb://") {
		t.Errorf("expected mongodb:// scheme, got: %q", out)
	}
	if !strings.Contains(out, "tls=true") {
		t.Errorf("expected TLS parameter, got: %q", out)
	}
}

func TestRunConnectionString_URLFormat_Valkey(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.Valkey
	creds := sampleCreds()
	creds.Port = 6380

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "rediss://") {
		t.Errorf("expected rediss:// scheme, got: %q", out)
	}
}

func TestRunConnectionString_URLFormat_Kafka(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.Kafka
	creds := sampleCreds()
	creds.Port = 9093

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "kafka://") {
		t.Errorf("expected kafka:// scheme, got: %q", out)
	}
}

func TestRunConnectionString_URLFormat_UnknownType(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.MSSQL
	creds := sampleCreds()
	creds.Port = 1433

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "mssql://") {
		t.Errorf("expected mssql:// scheme, got: %q", out)
	}
}

func TestRunConnectionString_PSQLFormat(t *testing.T) {
	svc := sampleService()
	creds := sampleCreds()

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "psql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "PGPASSWORD") {
		t.Errorf("expected PGPASSWORD in psql output, got: %q", out)
	}
	if !strings.Contains(out, "psql") {
		t.Errorf("expected psql command in output, got: %q", out)
	}
}

func TestRunConnectionString_PSQLFormat_WrongType(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.MySQL
	creds := sampleCreds()

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	_, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "psql")
	if err == nil {
		t.Fatal("expected error for psql format with non-postgresql service")
	}
	if !strings.Contains(err.Error(), "only valid for PostgreSQL") {
		t.Errorf("expected type error, got: %v", err)
	}
}

func TestRunConnectionString_MySQLFormat(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.MySQL
	creds := sampleCreds()
	creds.Port = 3306

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "mysql")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "mysql") {
		t.Errorf("expected mysql command in output, got: %q", out)
	}
	if !strings.Contains(out, "--ssl-mode=REQUIRED") {
		t.Errorf("expected SSL mode in output, got: %q", out)
	}
}

func TestRunConnectionString_MySQLFormat_WrongType(t *testing.T) {
	svc := sampleService()
	creds := sampleCreds()

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	_, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "mysql")
	if err == nil {
		t.Fatal("expected error for mysql format with non-mysql service")
	}
}

func TestRunConnectionString_MongoshFormat(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.MongoDB
	creds := sampleCreds()
	creds.Port = 27017

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "mongosh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "mongosh") {
		t.Errorf("expected mongosh command in output, got: %q", out)
	}
	if !strings.Contains(out, "mongodb://") {
		t.Errorf("expected mongodb:// URI, got: %q", out)
	}
}

func TestRunConnectionString_MongoshFormat_WrongType(t *testing.T) {
	svc := sampleService()
	creds := sampleCreds()

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	_, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "mongosh")
	if err == nil {
		t.Fatal("expected error for mongosh format with non-mongodb service")
	}
}

func TestRunConnectionString_RedisCLIFormat(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.Valkey
	creds := sampleCreds()
	creds.Port = 6380

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "redis-cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "redis-cli") {
		t.Errorf("expected redis-cli command in output, got: %q", out)
	}
	if !strings.Contains(out, "--tls") {
		t.Errorf("expected --tls flag in output, got: %q", out)
	}
}

func TestRunConnectionString_RedisCLIFormat_WrongType(t *testing.T) {
	svc := sampleService()
	creds := sampleCreds()

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	_, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "redis-cli")
	if err == nil {
		t.Fatal("expected error for redis-cli format with non-valkey service")
	}
}

func TestRunConnectionString_EnvFormat_PostgreSQL(t *testing.T) {
	svc := sampleService()
	creds := sampleCreds()

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "export PGHOST") {
		t.Errorf("expected PGHOST in env output, got: %q", out)
	}
	if !strings.Contains(out, "export PGPASSWORD") {
		t.Errorf("expected PGPASSWORD in env output, got: %q", out)
	}
	if !strings.Contains(out, "export PGSSLMODE") {
		t.Errorf("expected PGSSLMODE in env output, got: %q", out)
	}
}

func TestRunConnectionString_EnvFormat_MySQL(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.MySQL
	creds := sampleCreds()
	creds.Port = 3306

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "export MYSQL_HOST") {
		t.Errorf("expected MYSQL_HOST in env output, got: %q", out)
	}
}

func TestRunConnectionString_EnvFormat_MongoDB(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.MongoDB
	creds := sampleCreds()
	creds.Port = 27017

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "export MONGODB_URI") {
		t.Errorf("expected MONGODB_URI in env output, got: %q", out)
	}
}

func TestRunConnectionString_EnvFormat_Valkey(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.Valkey
	creds := sampleCreds()
	creds.Port = 6380

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "export REDIS_HOST") {
		t.Errorf("expected REDIS_HOST in env output, got: %q", out)
	}
	if !strings.Contains(out, "export REDIS_TLS") {
		t.Errorf("expected REDIS_TLS in env output, got: %q", out)
	}
}

func TestRunConnectionString_EnvFormat_Kafka(t *testing.T) {
	svc := sampleService()
	svc.DatabaseType = foundrydb.Kafka
	creds := sampleCreds()
	creds.Port = 9093

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "env")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "export DATABASE_URL") {
		t.Errorf("expected DATABASE_URL in env output, got: %q", out)
	}
}

func TestRunConnectionString_InvalidFormat(t *testing.T) {
	svc := sampleService()
	creds := sampleCreds()

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	_, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "invalid")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("expected 'unknown format' error, got: %v", err)
	}
}

func TestRunConnectionString_HostFromDNS(t *testing.T) {
	// When creds.Host is empty, host should fall back to service DNS records
	svc := sampleService()
	svc.DNSRecords = []foundrydb.DNSRecord{
		{FullDomain: "my-pg.db.foundrydb.com", RecordType: "A", Value: "1.2.3.4"},
	}
	creds := sampleCreds()
	creds.Host = "" // force DNS fallback
	creds.Port = 0

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "my-pg.db.foundrydb.com") {
		t.Errorf("expected DNS-derived host in URL, got: %q", out)
	}
}

func TestRunConnectionString_DatabaseFromCreds(t *testing.T) {
	// When --database is defaultdb and creds.Database is set, use creds.Database
	svc := sampleService()
	creds := sampleCreds()
	creds.Database = "mydb"

	_, cleanup := setupTestServer(t, buildConnStringMux(svc, creds))
	defer cleanup()

	out, err := executeCommand(t, "connection-string", svc.ID, "--user", "app_user", "--format", "url")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "mydb") {
		t.Errorf("expected creds database name in URL, got: %q", out)
	}
}

// -- shellEscape --------------------------------------------------------------

func TestShellEscape(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "'simple'"},
		{"with'quote", "'with'\\''quote'"},
		{"no-special", "'no-special'"},
		{"it's", "'it'\\''s'"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := shellEscape(tc.input)
			if got != tc.expected {
				t.Errorf("shellEscape(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

// -- buildURL -----------------------------------------------------------------

func TestBuildURL(t *testing.T) {
	tests := []struct {
		dbType   string
		expected string
	}{
		{"postgresql", "postgresql://"},
		{"mysql", "mysql://"},
		{"mongodb", "mongodb://"},
		{"valkey", "rediss://"},
		{"kafka", "kafka://"},
		{"mssql", "mssql://"},
	}

	for _, tc := range tests {
		t.Run(tc.dbType, func(t *testing.T) {
			got := buildURL(tc.dbType, "host.example.com", 1234, "user", "pass", "db")
			if !strings.HasPrefix(got, tc.expected) {
				t.Errorf("buildURL(%q) = %q, want prefix %q", tc.dbType, got, tc.expected)
			}
		})
	}
}
