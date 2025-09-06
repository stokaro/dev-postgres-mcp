// Package types defines common interfaces used throughout the application.
package types

import (
	"context"
)

// DatabaseManager defines the interface for managing database instances.
type DatabaseManager interface {
	// CreateInstance creates a new database instance.
	CreateInstance(ctx context.Context, opts CreateInstanceOptions) (*DatabaseInstance, error)

	// ListInstances returns all database instances of this type.
	ListInstances(ctx context.Context) ([]*DatabaseInstance, error)

	// GetInstance returns a specific database instance by ID.
	GetInstance(ctx context.Context, id string) (*DatabaseInstance, error)

	// DropInstance removes a database instance.
	DropInstance(ctx context.Context, id string) error

	// HealthCheck performs a health check on a database instance.
	HealthCheck(ctx context.Context, id string) (*HealthCheckResult, error)

	// Cleanup removes all instances managed by this manager.
	Cleanup(ctx context.Context) error

	// GetInstanceCount returns the number of instances managed by this manager.
	GetInstanceCount() int

	// GetDatabaseType returns the database type this manager handles.
	GetDatabaseType() DatabaseType
}

// HealthCheckResult represents the result of a health check.
type HealthCheckResult struct {
	// Status indicates the health status.
	Status HealthStatus `json:"status"`

	// Message provides additional information about the health status.
	Message string `json:"message"`

	// Duration is how long the health check took.
	Duration string `json:"duration"`

	// Timestamp is when the health check was performed.
	Timestamp string `json:"timestamp"`
}

// HealthStatus represents the health status of a database instance.
type HealthStatus string

const (
	// HealthStatusHealthy indicates the instance is healthy.
	HealthStatusHealthy HealthStatus = "healthy"

	// HealthStatusUnhealthy indicates the instance is unhealthy.
	HealthStatusUnhealthy HealthStatus = "unhealthy"

	// HealthStatusStarting indicates the instance is starting up.
	HealthStatusStarting HealthStatus = "starting"

	// HealthStatusUnknown indicates the health status is unknown.
	HealthStatusUnknown HealthStatus = "unknown"
)

// String returns the string representation of the health status.
func (hs HealthStatus) String() string {
	return string(hs)
}
