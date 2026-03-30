package api

import "time"

// Service represents a managed database service
type Service struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	DatabaseType string    `json:"database_type"`
	Version      string    `json:"version"`
	Status       string    `json:"status"`
	PlanName     string    `json:"plan_name"`
	StorageSizeGB int      `json:"storage_size_gb"`
	StorageTier  string    `json:"storage_tier"`
	Zone         string    `json:"zone"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	DNSRecords   []DNSRecord `json:"dns_records"`
	Nodes        []Node    `json:"nodes"`
}

// DNSRecord represents a DNS record for a service
type DNSRecord struct {
	FullDomain string `json:"full_domain"`
	Port       int    `json:"port"`
	Type       string `json:"type"`
}

// Node represents a database node
type Node struct {
	ID   string `json:"id"`
	Role string `json:"role"`
	IP   string `json:"ip"`
}

// ServiceListResponse is the response from GET /managed-services/
type ServiceListResponse struct {
	Services []Service `json:"services"`
	Total    int       `json:"total"`
	Limit    int       `json:"limit"`
	Offset   int       `json:"offset"`
}

// CreateServiceRequest is the request body for POST /managed-services/
type CreateServiceRequest struct {
	Name          string   `json:"name"`
	DatabaseType  string   `json:"database_type"`
	Version       string   `json:"version"`
	PlanName      string   `json:"plan_name"`
	Zone          string   `json:"zone"`
	StorageSizeGB *int     `json:"storage_size_gb"`
	StorageTier   string   `json:"storage_tier"`
	AllowedCIDRs  []string `json:"allowed_cidrs,omitempty"`
}

// DatabaseUser represents a database user
type DatabaseUser struct {
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

// UserListResponse is the response from GET /managed-services/{id}/database-users
type UserListResponse struct {
	Users []DatabaseUser `json:"users"`
}

// RevealPasswordResponse is the response from POST .../reveal-password
type RevealPasswordResponse struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Database         string `json:"database"`
	ConnectionString string `json:"connection_string"`
}

// Backup represents a backup record
type Backup struct {
	ID        string    `json:"id"`
	Type      string    `json:"backup_type"`
	Status    string    `json:"status"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

// BackupListResponse is the response from GET /managed-services/{id}/backups
type BackupListResponse struct {
	Backups []Backup `json:"backups"`
}

// TriggerBackupResponse is the response from POST /managed-services/{id}/backups
type TriggerBackupResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// MetricsData holds the core metrics values returned inside the "metrics" key
type MetricsData struct {
	CPUUsagePercent        float64 `json:"cpu_usage_percent"`
	MemoryUsagePercent     float64 `json:"memory_usage_percent"`
	DiskUsagePercent       float64 `json:"disk_usage_percent"`
	DatabaseConnectionsActive int  `json:"database_connections_active"`
	DatabaseQueriesPerSecond  float64 `json:"database_queries_per_second"`
	DatabaseCacheHitRatio     float64 `json:"database_cache_hit_ratio"`
	DatabaseReplicationLagSeconds float64 `json:"database_replication_lag_seconds"`
}

// MetricsResponse is the full response from GET /managed-services/{id}/metrics/current
type MetricsResponse struct {
	ServiceID   string      `json:"service_id"`
	DatabaseType string     `json:"database_type"`
	Timestamp   string      `json:"timestamp"`
	Metrics     MetricsData `json:"metrics"`
}

// RequestLogsResponse is the response from POST /managed-services/{id}/logs
type RequestLogsResponse struct {
	TaskID string `json:"task_id"`
}

// LogsResponse is the response from GET /managed-services/{id}/logs?task_id=X
type LogsResponse struct {
	Status string `json:"status"`
	Logs   string `json:"logs"`
}

// Organization represents an organization the user belongs to
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Role string `json:"role"`
}

// OrganizationListResponse is the response from GET /organizations/
type OrganizationListResponse struct {
	Organizations []Organization `json:"organizations"`
}
