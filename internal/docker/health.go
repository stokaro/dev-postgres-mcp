package docker

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// HealthStatus represents the health status of a container or service.
type HealthStatus string

const (
	// HealthStatusHealthy indicates the service is healthy and ready.
	HealthStatusHealthy HealthStatus = "healthy"
	// HealthStatusUnhealthy indicates the service is not working properly.
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	// HealthStatusStarting indicates the service is starting up.
	HealthStatusStarting HealthStatus = "starting"
	// HealthStatusStopped indicates the service is stopped.
	HealthStatusStopped HealthStatus = "stopped"
	// HealthStatusUnknown indicates the health status cannot be determined.
	HealthStatusUnknown HealthStatus = "unknown"
)

// HealthCheck represents a health check result.
type HealthCheck struct {
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message"`
	Timestamp time.Time    `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

// HealthChecker provides health checking functionality for PostgreSQL containers.
type HealthChecker struct {
	client *Client
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker(client *Client) *HealthChecker {
	return &HealthChecker{
		client: client,
	}
}

// CheckContainerHealth checks the health of a container using Docker's health check.
func (hc *HealthChecker) CheckContainerHealth(ctx context.Context, containerID string) (*HealthCheck, error) {
	start := time.Now()

	inspect, err := hc.client.InspectContainer(ctx, containerID)
	if err != nil {
		return &HealthCheck{
			Status:    HealthStatusUnknown,
			Message:   fmt.Sprintf("Failed to inspect container: %v", err),
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		}, nil
	}

	if !inspect.State.Running {
		return &HealthCheck{
			Status:    HealthStatusStopped,
			Message:   "Container is not running",
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		}, nil
	}

	// Check Docker health status
	if inspect.State.Health != nil {
		var status HealthStatus
		var message string

		switch inspect.State.Health.Status {
		case "healthy":
			status = HealthStatusHealthy
			message = "Container health check passed"
		case "unhealthy":
			status = HealthStatusUnhealthy
			message = "Container health check failed"
			if len(inspect.State.Health.Log) > 0 {
				lastLog := inspect.State.Health.Log[len(inspect.State.Health.Log)-1]
				message = fmt.Sprintf("Container health check failed: %s", lastLog.Output)
			}
		case "starting":
			status = HealthStatusStarting
			message = "Container is starting up"
		default:
			status = HealthStatusUnknown
			message = fmt.Sprintf("Unknown health status: %s", inspect.State.Health.Status)
		}

		return &HealthCheck{
			Status:    status,
			Message:   message,
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		}, nil
	}

	// If no health check is configured, assume healthy if running
	return &HealthCheck{
		Status:    HealthStatusHealthy,
		Message:   "Container is running (no health check configured)",
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}, nil
}

// CheckPostgreSQLConnection checks if PostgreSQL is accessible by attempting a connection.
func (hc *HealthChecker) CheckPostgreSQLConnection(ctx context.Context, dsn string) (*HealthCheck, error) {
	start := time.Now()

	slog.Debug("Checking PostgreSQL connection", "dsn", dsn)

	// Create a context with timeout for the connection attempt
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return &HealthCheck{
			Status:    HealthStatusUnhealthy,
			Message:   fmt.Sprintf("Failed to open database connection: %v", err),
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		}, nil
	}
	defer db.Close()

	// Set connection timeout
	db.SetConnMaxLifetime(5 * time.Second)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)

	// Attempt to ping the database
	if err := db.PingContext(connCtx); err != nil {
		return &HealthCheck{
			Status:    HealthStatusUnhealthy,
			Message:   fmt.Sprintf("Failed to ping database: %v", err),
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		}, nil
	}

	// Try a simple query to ensure the database is fully operational
	var result int
	err = db.QueryRowContext(connCtx, "SELECT 1").Scan(&result)
	if err != nil {
		return &HealthCheck{
			Status:    HealthStatusUnhealthy,
			Message:   fmt.Sprintf("Failed to execute test query: %v", err),
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		}, nil
	}

	if result != 1 {
		return &HealthCheck{
			Status:    HealthStatusUnhealthy,
			Message:   "Test query returned unexpected result",
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		}, nil
	}

	return &HealthCheck{
		Status:    HealthStatusHealthy,
		Message:   "PostgreSQL connection successful",
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}, nil
}

// CheckPostgreSQLInstance performs a comprehensive health check on a PostgreSQL instance.
func (hc *HealthChecker) CheckPostgreSQLInstance(ctx context.Context, containerID, dsn string) (*HealthCheck, error) {
	// First check container health
	containerHealth, err := hc.CheckContainerHealth(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to check container health: %w", err)
	}

	// If container is not healthy, return that status
	if containerHealth.Status != HealthStatusHealthy {
		return containerHealth, nil
	}

	// If container is healthy, check PostgreSQL connection
	return hc.CheckPostgreSQLConnection(ctx, dsn)
}

// WaitForHealthy waits for a PostgreSQL instance to become healthy.
func (hc *HealthChecker) WaitForHealthy(ctx context.Context, containerID, dsn string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	slog.Info("Waiting for PostgreSQL instance to become healthy",
		"container_id", containerID,
		"timeout", timeout)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for PostgreSQL instance to become healthy: %w", ctx.Err())
		case <-ticker.C:
			health, err := hc.CheckPostgreSQLInstance(ctx, containerID, dsn)
			if err != nil {
				slog.Warn("Health check failed", "error", err)
				continue
			}

			slog.Debug("Health check result",
				"status", health.Status,
				"message", health.Message,
				"duration", health.Duration)

			switch health.Status {
			case HealthStatusHealthy:
				slog.Info("PostgreSQL instance is healthy", "container_id", containerID)
				return nil
			case HealthStatusUnhealthy:
				return fmt.Errorf("PostgreSQL instance became unhealthy: %s", health.Message)
			case HealthStatusStopped:
				return fmt.Errorf("PostgreSQL container stopped unexpectedly")
			case HealthStatusStarting:
				// Continue waiting
				continue
			default:
				slog.Warn("Unknown health status", "status", health.Status, "message", health.Message)
				continue
			}
		}
	}
}
