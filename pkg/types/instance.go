// Package types defines common data types used throughout the application.
package types

import (
	"time"


"github.com/docker/docker/api/types/container"
)

// PostgreSQLInstance represents a PostgreSQL database instance.
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

// CreateInstanceOptions holds options for creating a new PostgreSQL instance.
type CreateInstanceOptions struct {
	// Version specifies the PostgreSQL version to use (default: "17").
	Version string `json:"version,omitempty"`

	// Database specifies the database name (default: "postgres").
	Database string `json:"database,omitempty"`

	// Username specifies the PostgreSQL username (default: "postgres").
	Username string `json:"username,omitempty"`

	// Password specifies the PostgreSQL password (auto-generated if empty).
	Password string `json:"password,omitempty"`
}

// Container is an alias for Docker container type to avoid importing Docker types everywhere.
type Container = container.Summary

// ContainerListOptions is an alias for Docker container list options.
type ContainerListOptions = container.ListOptions
