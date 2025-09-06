// Package types defines common data types used throughout the application.
package types

import (
	"time"

	"github.com/docker/docker/api/types/container"
)

// DatabaseType represents the type of database.
type DatabaseType string

const (
	// DatabaseTypePostgreSQL represents PostgreSQL database.
	DatabaseTypePostgreSQL DatabaseType = "postgresql"
	// DatabaseTypeMySQL represents MySQL database.
	DatabaseTypeMySQL DatabaseType = "mysql"
	// DatabaseTypeMariaDB represents MariaDB database.
	DatabaseTypeMariaDB DatabaseType = "mariadb"
)

// String returns the string representation of the database type.
func (dt DatabaseType) String() string {
	return string(dt)
}

// IsValid checks if the database type is valid.
func (dt DatabaseType) IsValid() bool {
	switch dt {
	case DatabaseTypePostgreSQL, DatabaseTypeMySQL, DatabaseTypeMariaDB:
		return true
	default:
		return false
	}
}

// DefaultPort returns the default port for the database type.
func (dt DatabaseType) DefaultPort() int {
	switch dt {
	case DatabaseTypePostgreSQL:
		return 5432
	case DatabaseTypeMySQL:
		return 3306
	case DatabaseTypeMariaDB:
		return 3306
	default:
		return 0
	}
}

// DefaultVersion returns the default version for the database type.
func (dt DatabaseType) DefaultVersion() string {
	switch dt {
	case DatabaseTypePostgreSQL:
		return "17"
	case DatabaseTypeMySQL:
		return "8.0"
	case DatabaseTypeMariaDB:
		return "11"
	default:
		return ""
	}
}

// DefaultDatabase returns the default database name for the database type.
func (dt DatabaseType) DefaultDatabase() string {
	switch dt {
	case DatabaseTypePostgreSQL:
		return "postgres"
	case DatabaseTypeMySQL, DatabaseTypeMariaDB:
		return "mysql"
	default:
		return ""
	}
}

// DefaultUsername returns the default username for the database type.
func (dt DatabaseType) DefaultUsername() string {
	switch dt {
	case DatabaseTypePostgreSQL:
		return "postgres"
	case DatabaseTypeMySQL, DatabaseTypeMariaDB:
		return "root"
	default:
		return ""
	}
}

// DatabaseInstance represents a generic database instance.
type DatabaseInstance struct {
	// ID is the unique identifier for this instance.
	ID string `json:"id"`

	// Type is the database type (postgresql, mysql, mariadb).
	Type DatabaseType `json:"type"`

	// ContainerID is the Docker container ID.
	ContainerID string `json:"container_id"`

	// Port is the host port where the database is accessible.
	Port int `json:"port"`

	// Database is the name of the database.
	Database string `json:"database"`

	// Username is the database username.
	Username string `json:"username"`

	// Password is the database password.
	Password string `json:"password"`

	// Version is the database version (e.g., "17", "8.0", "11").
	Version string `json:"version"`

	// DSN is the complete Data Source Name for connecting to the database.
	DSN string `json:"dsn"`

	// CreatedAt is the timestamp when the instance was created.
	CreatedAt time.Time `json:"created_at"`

	// Status represents the current status of the instance.
	// Possible values: "starting", "running", "stopped", "unhealthy", "unknown"
	Status string `json:"status"`
}

// PostgreSQLInstance represents a PostgreSQL database instance.
// Deprecated: Use DatabaseInstance instead.
type PostgreSQLInstance struct {
	// ID is the unique identifier for this instance.
	ID string `json:"id"`

	// ContainerID is the Docker container ID.
	ContainerID string `json:"container_id"`

	// Port is the host port where PostgreSQL is accessible.
	Port int `json:"port"`

	// Database is the name of the PostgreSQL database.
	Database string `json:"database"`

	// Username is the PostgreSQL username.
	Username string `json:"username"`

	// Password is the PostgreSQL password.
	Password string `json:"password"`

	// Version is the PostgreSQL version (e.g., "17", "16").
	Version string `json:"version"`

	// DSN is the complete Data Source Name for connecting to the database.
	DSN string `json:"dsn"`

	// CreatedAt is the timestamp when the instance was created.
	CreatedAt time.Time `json:"created_at"`

	// Status represents the current status of the instance.
	// Possible values: "starting", "running", "stopped", "unhealthy", "unknown"
	Status string `json:"status"`
}

// ToGeneric converts a PostgreSQLInstance to a generic DatabaseInstance.
func (p *PostgreSQLInstance) ToGeneric() *DatabaseInstance {
	return &DatabaseInstance{
		ID:          p.ID,
		Type:        DatabaseTypePostgreSQL,
		ContainerID: p.ContainerID,
		Port:        p.Port,
		Database:    p.Database,
		Username:    p.Username,
		Password:    p.Password,
		Version:     p.Version,
		DSN:         p.DSN,
		CreatedAt:   p.CreatedAt,
		Status:      p.Status,
	}
}

// CreateInstanceOptions holds options for creating a new database instance.
type CreateInstanceOptions struct {
	// Type specifies the database type (postgresql, mysql, mariadb).
	Type DatabaseType `json:"type,omitempty"`

	// Version specifies the database version to use (defaults vary by type).
	Version string `json:"version,omitempty"`

	// Database specifies the database name (defaults vary by type).
	Database string `json:"database,omitempty"`

	// Username specifies the database username (defaults vary by type).
	Username string `json:"username,omitempty"`

	// Password specifies the database password (auto-generated if empty).
	Password string `json:"password,omitempty"`
}

// Container is an alias for Docker container type to avoid importing Docker types everywhere.
type Container = container.Summary

// ContainerListOptions is an alias for Docker container list options.
type ContainerListOptions = container.ListOptions
