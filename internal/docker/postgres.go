package docker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/go-connections/nat"

	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

// PostgreSQLContainerConfig holds configuration for creating a PostgreSQL container.
type PostgreSQLContainerConfig struct {
	Version  string
	Database string
	Username string
	Password string
	Port     int
}

// PostgreSQLManager manages PostgreSQL containers.
type PostgreSQLManager struct {
	client *Client
}

// NewPostgreSQLManager creates a new PostgreSQL container manager.
func NewPostgreSQLManager(client *Client) *PostgreSQLManager {
	return &PostgreSQLManager{
		client: client,
	}
}

// CreatePostgreSQLContainer creates and starts a new PostgreSQL container.
func (m *PostgreSQLManager) CreatePostgreSQLContainer(ctx context.Context, instanceID string, config PostgreSQLContainerConfig) (*types.PostgreSQLInstance, error) {
	image := fmt.Sprintf("postgres:%s", config.Version)
	containerName := fmt.Sprintf("mcp-postgres-%s", instanceID)

	slog.Info("Creating PostgreSQL container",
		"instance_id", instanceID,
		"image", image,
		"port", config.Port,
		"database", config.Database)

	// Pull the image if needed
	if err := m.client.PullImage(ctx, image); err != nil {
		return nil, fmt.Errorf("failed to pull PostgreSQL image: %w", err)
	}

	// Configure container
	containerConfig := &container.Config{
		Image: image,
		Env: []string{
			fmt.Sprintf("POSTGRES_DB=%s", config.Database),
			fmt.Sprintf("POSTGRES_USER=%s", config.Username),
			fmt.Sprintf("POSTGRES_PASSWORD=%s", config.Password),
			"POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256",
		},
		ExposedPorts: nat.PortSet{
			"5432/tcp": struct{}{},
		},
		Labels: map[string]string{
			"dev-postgres-mcp.managed":     "true",
			"dev-postgres-mcp.instance-id": instanceID,
			"dev-postgres-mcp.database":    config.Database,
			"dev-postgres-mcp.username":    config.Username,
			"dev-postgres-mcp.version":     config.Version,
			"dev-postgres-mcp.port":        strconv.Itoa(config.Port),
			"dev-postgres-mcp.created-at":  time.Now().UTC().Format(time.RFC3339),
		},
		Healthcheck: &container.HealthConfig{
			Test: []string{
				"CMD-SHELL",
				fmt.Sprintf("pg_isready -U %s -d %s", config.Username, config.Database),
			},
			Interval:    10 * time.Second,
			Timeout:     5 * time.Second,
			Retries:     5,
			StartPeriod: 30 * time.Second,
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"5432/tcp": []nat.PortBinding{
				{
					HostIP:   "127.0.0.1",
					HostPort: strconv.Itoa(config.Port),
				},
			},
		},
		RestartPolicy: container.RestartPolicy{
			Name: "no",
		},
		// Set resource limits
		Resources: container.Resources{
			Memory:   512 * 1024 * 1024, // 512MB
			NanoCPUs: 1000000000,        // 1 CPU core
		},
	}

	// Create container
	containerID, err := m.client.CreateContainer(ctx, containerConfig, hostConfig, containerName)
	if err != nil {
		return nil, fmt.Errorf("failed to create PostgreSQL container: %w", err)
	}

	// Start container
	if err := m.client.StartContainer(ctx, containerID); err != nil {
		// Clean up on failure
		_ = m.client.RemoveContainer(ctx, containerID)
		return nil, fmt.Errorf("failed to start PostgreSQL container: %w", err)
	}

	// Wait for container to be healthy
	if err := m.waitForHealthy(ctx, containerID, 60*time.Second); err != nil {
		// Clean up on failure
		_ = m.client.RemoveContainer(ctx, containerID)
		return nil, fmt.Errorf("PostgreSQL container failed to become healthy: %w", err)
	}

	// Create instance object
	instance := &types.PostgreSQLInstance{
		ID:          instanceID,
		ContainerID: containerID,
		Port:        config.Port,
		Database:    config.Database,
		Username:    config.Username,
		Password:    config.Password,
		Version:     config.Version,
		DSN:         fmt.Sprintf("postgres://%s:%s@localhost:%d/%s?sslmode=disable", config.Username, config.Password, config.Port, config.Database),
		CreatedAt:   time.Now(),
		Status:      "running",
	}

	slog.Info("PostgreSQL container created and started successfully",
		"instance_id", instanceID,
		"container_id", containerID,
		"port", config.Port)

	return instance, nil
}

// StopPostgreSQLContainer stops a PostgreSQL container.
func (m *PostgreSQLManager) StopPostgreSQLContainer(ctx context.Context, containerID string) error {
	return m.client.StopContainer(ctx, containerID)
}

// RemovePostgreSQLContainer removes a PostgreSQL container.
func (m *PostgreSQLManager) RemovePostgreSQLContainer(ctx context.Context, containerID string) error {
	return m.client.RemoveContainer(ctx, containerID)
}

// GetPostgreSQLContainerStatus returns the status of a PostgreSQL container.
func (m *PostgreSQLManager) GetPostgreSQLContainerStatus(ctx context.Context, containerID string) (string, error) {
	inspect, err := m.client.InspectContainer(ctx, containerID)
	if err != nil {
		return "", err
	}

	if !inspect.State.Running {
		return "stopped", nil
	}

	// Check health status
	if inspect.State.Health != nil {
		switch inspect.State.Health.Status {
		case "healthy":
			return "running", nil
		case "unhealthy":
			return "unhealthy", nil
		case "starting":
			return "starting", nil
		default:
			return "unknown", nil
		}
	}

	return "running", nil
}

// ListPostgreSQLContainers lists all PostgreSQL containers managed by this service.
func (m *PostgreSQLManager) ListPostgreSQLContainers(ctx context.Context) ([]container.Summary, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "dev-postgres-mcp.managed=true")

	containers, err := m.client.ListContainers(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, err
	}

	return containers, nil
}

// waitForHealthy waits for a container to become healthy.
func (m *PostgreSQLManager) waitForHealthy(ctx context.Context, containerID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	slog.Info("Waiting for PostgreSQL container to become healthy", "container_id", containerID)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for container to become healthy: %w", ctx.Err())
		case <-ticker.C:
			inspect, err := m.client.InspectContainer(ctx, containerID)
			if err != nil {
				return fmt.Errorf("failed to inspect container: %w", err)
			}

			if !inspect.State.Running {
				return fmt.Errorf("container stopped unexpectedly")
			}

			if inspect.State.Health != nil {
				switch inspect.State.Health.Status {
				case "healthy":
					slog.Info("PostgreSQL container is healthy", "container_id", containerID)
					return nil
				case "unhealthy":
					logs, _ := m.client.ContainerLogs(ctx, containerID, container.LogsOptions{
						ShowStdout: true,
						ShowStderr: true,
						Tail:       "50",
					})
					return fmt.Errorf("container became unhealthy, logs: %s", logs)
				case "starting":
					slog.Debug("PostgreSQL container is still starting", "container_id", containerID)
					continue
				}
			}

			// If no health check is configured, assume it's ready after a short delay
			// This shouldn't happen with our configuration, but it's a fallback
			time.Sleep(5 * time.Second)
			return nil
		}
	}
}
