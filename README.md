# fdb - FoundryDB CLI

A command-line interface for the [FoundryDB](https://foundrydb.com) managed database platform. Manage PostgreSQL, MySQL, MongoDB, Valkey, and Kafka services from your terminal.

## Installation

### Download pre-built binary

Download the latest release from the [releases page](https://github.com/anorph/foundrydb-cli/releases).

```bash
# macOS (Apple Silicon)
curl -L https://github.com/anorph/foundrydb-cli/releases/latest/download/fdb-darwin-arm64 -o /usr/local/bin/fdb
chmod +x /usr/local/bin/fdb

# macOS (Intel)
curl -L https://github.com/anorph/foundrydb-cli/releases/latest/download/fdb-darwin-amd64 -o /usr/local/bin/fdb
chmod +x /usr/local/bin/fdb

# Linux (amd64)
curl -L https://github.com/anorph/foundrydb-cli/releases/latest/download/fdb-linux-amd64 -o /usr/local/bin/fdb
chmod +x /usr/local/bin/fdb
```

### Build from source

Requires Go 1.24+.

```bash
git clone https://github.com/anorph/foundrydb-cli.git
cd foundrydb-cli
CGO_ENABLED=0 go build -o fdb ./cmd/fdb/
sudo mv fdb /usr/local/bin/
```

## Configuration

### Login (recommended)

```bash
fdb auth login
```

This prompts for your API URL, username, and password, verifies the credentials, and saves them to `~/.fdb/config.toml`.

### Config file

Credentials are stored at `~/.fdb/config.toml`:

```toml
api_url = "https://api.foundrydb.com"
username = "admin"
password = "your-password"
```

The file is created with `0600` permissions (owner read/write only).

### Environment variables

All config values can be set via environment variables with the `FDB_` prefix:

```bash
export FDB_API_URL=https://api.foundrydb.com
export FDB_USERNAME=admin
export FDB_PASSWORD=your-password
```

### Global flags

These flags work with every command:

```
--api-url string    API base URL (default: https://api.foundrydb.com)
--username string   Username (default: admin)
--password string   Password
--json              Output raw JSON instead of formatted tables
--config string     Config file path (default: ~/.fdb/config.toml)
```

## Commands

### Authentication

```bash
# Save credentials
fdb auth login

# Save with flags (non-interactive)
fdb auth login --api-url https://api.foundrydb.com --username admin --password secret

# Check authentication status
fdb auth status

# Remove saved credentials
fdb auth logout
```

### Services

```bash
# List all services
fdb services list

# List services as JSON
fdb services list --json

# Get service details (by ID or name)
fdb services get my-postgres
fdb services get 8f3a2c1d-...

# Create a service (interactive prompts for missing fields)
fdb services create

# Create a service with flags (non-interactive)
fdb services create \
  --name my-postgres \
  --type postgresql \
  --version 17 \
  --plan tier-2 \
  --zone se-sto1 \
  --storage-size 50 \
  --storage-tier maxiops

# Create with allowed CIDRs
fdb services create \
  --name my-postgres \
  --type postgresql \
  --version 17 \
  --allowed-cidrs "1.2.3.4/32,10.0.0.0/8"

# Delete a service (prompts for name confirmation)
fdb services delete <service-id>

# Delete without confirmation prompt
fdb services delete <service-id> --confirm
```

Supported database types: `postgresql`, `mysql`, `mongodb`, `valkey`, `kafka`

### Connect (interactive shell)

Opens a native database shell using locally installed client tools.

```bash
# Connect to a service (auto-selects first user)
fdb connect my-postgres

# Connect as a specific user
fdb connect my-postgres --user app_user

# Connect to a specific database
fdb connect my-postgres --user app_user --database myapp
```

Required local tools by database type:

| Database   | Required tool |
|------------|---------------|
| PostgreSQL | `psql`        |
| MySQL      | `mysql`       |
| MongoDB    | `mongosh`     |
| Valkey     | `redis-cli`   |

### Connection Strings

```bash
# URL format (default)
fdb connection-string <service-id> --user app_user

# Shell environment variables
fdb connection-string <service-id> --user app_user --format env

# psql command
fdb connection-string <service-id> --user app_user --format psql

# mysql command
fdb connection-string <service-id> --user app_user --format mysql

# mongosh URI
fdb connection-string <service-id> --user app_user --format mongosh

# redis-cli command
fdb connection-string <service-id> --user app_user --format redis-cli

# Specify database name
fdb connection-string <service-id> --user app_user --database myapp --format env
```

### Database Users

```bash
# List users for a service
fdb users list <service-id>
fdb users list my-postgres

# Reveal password for a user
fdb users reveal-password <service-id> <username>
fdb users reveal-password my-postgres app_user
```

### Backups

```bash
# List backups for a service
fdb backups list <service-id>
fdb backups list my-postgres

# Trigger a manual backup
fdb backups trigger <service-id>
fdb backups trigger my-postgres
```

### Logs

```bash
# Get last 100 lines of logs (default)
fdb logs <service-id>

# Get last 500 lines
fdb logs <service-id> --lines 500

# Output as JSON
fdb logs <service-id> --json
```

### Metrics

```bash
# Show current metrics
fdb metrics <service-id>
fdb metrics my-postgres

# Output as JSON
fdb metrics my-postgres --json
```

## Examples

### Full workflow: create and connect

```bash
# 1. Login
fdb auth login

# 2. Create a PostgreSQL service
fdb services create \
  --name dev-pg \
  --type postgresql \
  --version 17 \
  --plan tier-2 \
  --zone se-sto1 \
  --storage-size 50 \
  --storage-tier maxiops

# 3. Wait for it to be running
fdb services get dev-pg

# 4. List available users
fdb users list dev-pg

# 5. Connect interactively
fdb connect dev-pg --user app_user

# 6. Or get connection string for your app
fdb connection-string dev-pg --user app_user --format env
```

### Backup workflow

```bash
# Trigger a backup
fdb backups trigger my-postgres

# List all backups
fdb backups list my-postgres

# Output as JSON for scripting
fdb backups list my-postgres --json | jq '.backups[] | select(.status == "completed")'
```

### Scripting with JSON output

```bash
# Get all running services
fdb services list --json | jq '.services[] | select(.status == "running") | .name'

# Get service ID by name
SERVICE_ID=$(fdb services list --json | jq -r '.services[] | select(.name == "my-postgres") | .id')

# Reveal password and export as env var
eval "$(fdb connection-string "$SERVICE_ID" --user app_user --format env)"
echo "Connected to $PGHOST"
```

## License

Apache 2.0. See [LICENSE](LICENSE).
