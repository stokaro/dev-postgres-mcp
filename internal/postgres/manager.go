// Package postgres provides PostgreSQL instance management functionality.
package postgres

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/stokaro/dev-postgres-mcp/internal/docker"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

// Manager manages PostgreSQL instances.
type Manager struct {
	mu        sync.RWMutex
	instances map[string]*types.PostgreSQLInstance
	docker    *docker.Manager
}

// NewManager creates a new PostgreSQL instance manager.
func NewManager(dockerManager *docker.Manager) *Manager {
	return &Manager{
		instances: make(map[string]*types.PostgreSQLInstance),
		docker:    dockerManager,
	}
}

// CreateInstance creates a new PostgreSQL instance.
func (m *Manager) CreateInstance(ctx context.Context, opts types.CreateInstanceOptions) (*types.PostgreSQLInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate instance ID
	instanceID := uuid.New().String()

	// Set defaults
	if opts.Version == "" {
		opts.Version = "17"
	}
	if opts.Database == "" {
		opts.Database = "postgres"
	}
	if opts.Username == "" {
		opts.Username = "postgres"
	}
	if opts.Password == "" {
		var err error
		opts.Password, err = generatePassword(16)
		if err != nil {
			return nil, fmt.Errorf("failed to generate password: %w", err)
		}
	}

	slog.Info("Creating PostgreSQL instance",
		"instance_id", instanceID,
		"version", opts.Version,
		"database", opts.Database,
		"username", opts.Username)

	// Allocate port
	port, err := m.docker.AllocatePort(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate port: %w", err)
	}

	// Create container configuration
	config := docker.PostgreSQLContainerConfig{
		Version:  opts.Version,
		Database: opts.Database,
		Username: opts.Username,
		Password: opts.Password,
		Port:     port,
	}

	// Create and start container
	instance, err := m.docker.PostgreSQL().CreatePostgreSQLContainer(ctx, instanceID, config)
	if err != nil {
		// Release port on failure
		m.docker.ReleasePort(port)
		return nil, fmt.Errorf("failed to create PostgreSQL container: %w", err)
	}

	// Store instance
	m.instances[instanceID] = instance

	slog.Info("PostgreSQL instance created successfully",
		"instance_id", instanceID,
		"port", port,
		"dsn", instance.DSN)

	return instance, nil
}

// ListInstances returns all PostgreSQL instances by discovering them from Docker containers.
func (m *Manager) ListInstances(ctx context.Context) ([]*types.PostgreSQLInstance, error) {
	// Get all PostgreSQL containers managed by this service
	containers, err := m.docker.PostgreSQL().ListPostgreSQLContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list PostgreSQL containers: %w", err)
	}

	instances := make([]*types.PostgreSQLInstance, 0, len(containers))
	for _, container := range containers {
		// Extract instance information from container labels
		labels := container.Labels
		if labels == nil {
			continue
		}

		// Check if this is a managed container
		if labels["dev-postgres-mcp.managed"] != "true" {
			continue
		}

		// Extract instance details from labels
		instanceID := labels["dev-postgres-mcp.instance-id"]
		database := labels["dev-postgres-mcp.database"]
		username := labels["dev-postgres-mcp.username"]
		version := labels["dev-postgres-mcp.version"]
		portStr := labels["dev-postgres-mcp.port"]
		createdAtStr := labels["dev-postgres-mcp.created-at"]

		if instanceID == "" || database == "" || username == "" || version == "" || portStr == "" {
			slog.Warn("Container missing required labels", "container_id", container.ID)
			continue
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			slog.Warn("Invalid port in container labels", "container_id", container.ID, "port", portStr)
			continue
		}

		var createdAt time.Time
		if createdAtStr != "" {
			if parsed, err := time.Parse(time.RFC3339, createdAtStr); err == nil {
				createdAt = parsed
			}
		}
		if createdAt.IsZero() {
			createdAt = time.Unix(container.Created, 0)
		}

		// Get container status
		status := "unknown"
		if len(container.Names) > 0 {
			containerName := container.Names[0]
			if containerName[0] == '/' {
				containerName = containerName[1:] // Remove leading slash
			}
			_ = containerName // containerName is extracted but not used in current logic
			if containerStatus, err := m.docker.PostgreSQL().GetPostgreSQLContainerStatus(ctx, container.ID); err == nil {
				status = containerStatus
			}
		}

		// Build DSN (we need to extract password from environment or use a placeholder)
		dsn := fmt.Sprintf("postgres://%s:****@localhost:%d/%s?sslmode=disable", username, port, database)

		instance := &types.PostgreSQLInstance{
			ID:          instanceID,
			ContainerID: container.ID,
			Port:        port,
			Database:    database,
			Username:    username,
			Password:    "****", // Don't expose password in listings
			Version:     version,
			DSN:         dsn,
			Status:      status,
			CreatedAt:   createdAt,
		}

		instances = append(instances, instance)
	}

	return instances, nil
}

// GetInstance returns a specific PostgreSQL instance by ID (supports partial ID matching).
func (m *Manager) GetInstance(ctx context.Context, id string) (*types.PostgreSQLInstance, error) {
	// First try exact match in-memory instances (for MCP server context)
	m.mu.RLock()
	if instance, exists := m.instances[id]; exists {
		m.mu.RUnlock()

		// Update status from Docker
		status, err := m.docker.PostgreSQL().GetPostgreSQLContainerStatus(ctx, instance.ContainerID)
		if err != nil {
			slog.Warn("Failed to get container status", "instance_id", id, "error", err)
			status = "unknown"
		}

		// Create a copy to avoid modifying the original
		instanceCopy := *instance
		instanceCopy.Status = status
		return &instanceCopy, nil
	}
	m.mu.RUnlock()

	// If not found in memory, try partial ID matching (for CLI context or partial IDs)
	return m.FindInstanceByPartialID(ctx, id)
}

// FindInstanceByPartialID finds an instance by partial ID match (like Docker's container ID matching).
// Returns the instance if exactly one match is found, or an error if no matches or multiple matches.
func (m *Manager) FindInstanceByPartialID(ctx context.Context, partialID string) (*types.PostgreSQLInstance, error) {
	if partialID == "" {
		return nil, fmt.Errorf("instance ID cannot be empty")
	}

	instances, err := m.ListInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	var matches []*types.PostgreSQLInstance
	for _, instance := range instances {
		if strings.HasPrefix(instance.ID, partialID) {
			matches = append(matches, instance)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("no such instance: %s", partialID)
	case 1:
		return matches[0], nil
	default:
		var ids []string
		for _, match := range matches {
			ids = append(ids, match.ID[:12]) // Show first 12 chars like Docker
		}
		return nil, fmt.Errorf("multiple instances found with prefix %s: %s", partialID, strings.Join(ids, ", "))
	}
}

// DropInstance removes a PostgreSQL instance.
func (m *Manager) DropInstance(ctx context.Context, id string) error {
	// First try to find instance in memory (for MCP server context)
	m.mu.Lock()
	instance, exists := m.instances[id]
	if exists {
		// Remove from instances map first
		delete(m.instances, id)
		m.mu.Unlock()

		slog.Info("Dropping PostgreSQL instance", "instance_id", id, "container_id", instance.ContainerID)

		// Stop and remove container
		if err := m.docker.PostgreSQL().StopPostgreSQLContainer(ctx, instance.ContainerID); err != nil {
			slog.Warn("Failed to stop container", "instance_id", id, "error", err)
			// Continue with removal even if stop fails
		}

		if err := m.docker.PostgreSQL().RemovePostgreSQLContainer(ctx, instance.ContainerID); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}

		// Release port
		m.docker.ReleasePort(instance.Port)

		slog.Info("PostgreSQL instance dropped successfully", "instance_id", id)
		return nil
	}
	m.mu.Unlock()

	// If not found in memory, try to discover from Docker (for CLI context)
	discoveredInstance, err := m.GetInstance(ctx, id)
	if err != nil {
		return fmt.Errorf("instance %s not found", id)
	}

	slog.Info("Dropping PostgreSQL instance", "instance_id", id, "container_id", discoveredInstance.ContainerID)

	// Stop and remove container
	if err := m.docker.PostgreSQL().StopPostgreSQLContainer(ctx, discoveredInstance.ContainerID); err != nil {
		slog.Warn("Failed to stop container", "instance_id", id, "error", err)
		// Continue with removal even if stop fails
	}

	if err := m.docker.PostgreSQL().RemovePostgreSQLContainer(ctx, discoveredInstance.ContainerID); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	// Release port (best effort - may not work if port manager doesn't know about it)
	m.docker.ReleasePort(discoveredInstance.Port)

	slog.Info("PostgreSQL instance dropped successfully", "instance_id", id)
	return nil
}

// HealthCheck performs a health check on a PostgreSQL instance (supports partial ID matching).
func (m *Manager) HealthCheck(ctx context.Context, id string) (*docker.HealthCheck, error) {
	instance, err := m.GetInstance(ctx, id)
	if err != nil {
		return nil, err
	}

	// Create health checker
	healthChecker := docker.NewHealthChecker(m.docker.GetClient())

	// Perform comprehensive health check
	return healthChecker.CheckPostgreSQLInstance(ctx, instance.ContainerID, instance.DSN)
}

// Cleanup removes all instances and cleans up resources.
func (m *Manager) Cleanup(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	slog.Info("Cleaning up all PostgreSQL instances", "count", len(m.instances))

	var errors []error

	for id, instance := range m.instances {
		slog.Info("Cleaning up instance", "instance_id", id)

		// Stop and remove container
		if err := m.docker.PostgreSQL().StopPostgreSQLContainer(ctx, instance.ContainerID); err != nil {
			slog.Warn("Failed to stop container during cleanup", "instance_id", id, "error", err)
		}

		if err := m.docker.PostgreSQL().RemovePostgreSQLContainer(ctx, instance.ContainerID); err != nil {
			slog.Error("Failed to remove container during cleanup", "instance_id", id, "error", err)
			errors = append(errors, fmt.Errorf("failed to remove container %s: %w", id, err))
		}

		// Release port
		m.docker.ReleasePort(instance.Port)
	}

	// Clear instances map
	m.instances = make(map[string]*types.PostgreSQLInstance)

	if len(errors) > 0 {
		return fmt.Errorf("cleanup completed with %d errors: %v", len(errors), errors)
	}

	slog.Info("Cleanup completed successfully")
	return nil
}

// GetInstanceCount returns the number of currently managed instances.
func (m *Manager) GetInstanceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.instances)
}

// generatePassword generates a secure random password.
func generatePassword(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Use base64 encoding to ensure printable characters
	password := base64.URLEncoding.EncodeToString(bytes)

	// Trim to desired length
	if len(password) > length {
		password = password[:length]
	}

	return password, nil
}
