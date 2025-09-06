// Package database provides database instance management functionality.
package database

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/docker/docker/api/types/container"

	"github.com/stokaro/dev-postgres-mcp/internal/docker"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

// DatabaseConfig holds database-specific configuration.
type DatabaseConfig struct {
	Type                types.DatabaseType
	DefaultPort         int
	DefaultVersion      string
	DefaultDatabase     string
	DefaultUsername     string
	EnvironmentTemplate map[string]string // Template strings for environment variables
	HealthCheckCommand  []string          // Health check command template strings
	ContainerPort       string            // Internal container port

	// Compiled templates (populated during initialization)
	envTemplates    map[string]*template.Template
	healthTemplates []*template.Template
}

// GetDatabaseConfig returns configuration for a specific database type.
func GetDatabaseConfig(dbType types.DatabaseType) DatabaseConfig {
	var config DatabaseConfig

	switch dbType {
	case types.DatabaseTypePostgreSQL:
		config = DatabaseConfig{
			Type:            types.DatabaseTypePostgreSQL,
			DefaultPort:     5432,
			DefaultVersion:  "17",
			DefaultDatabase: "postgres",
			DefaultUsername: "postgres",
			EnvironmentTemplate: map[string]string{
				"POSTGRES_DB":       "{{.Database}}",
				"POSTGRES_USER":     "{{.Username}}",
				"POSTGRES_PASSWORD": "{{.Password}}",
			},
			HealthCheckCommand: []string{"CMD-SHELL", "pg_isready -U {{.Username}} -d {{.Database}}"},
			ContainerPort:      "5432/tcp",
		}
	case types.DatabaseTypeMySQL:
		config = DatabaseConfig{
			Type:            types.DatabaseTypeMySQL,
			DefaultPort:     3306,
			DefaultVersion:  "8.0",
			DefaultDatabase: "mysql",
			DefaultUsername: "root",
			EnvironmentTemplate: map[string]string{
				"MYSQL_DATABASE":      "{{.Database}}",
				"MYSQL_USER":          "{{.Username}}",
				"MYSQL_PASSWORD":      "{{.Password}}",
				"MYSQL_ROOT_PASSWORD": "{{.Password}}",
			},
			HealthCheckCommand: []string{"CMD-SHELL", "mysqladmin ping -h localhost -u {{.Username}} -p{{.Password}}"},
			ContainerPort:      "3306/tcp",
		}
	case types.DatabaseTypeMariaDB:
		config = DatabaseConfig{
			Type:            types.DatabaseTypeMariaDB,
			DefaultPort:     3306,
			DefaultVersion:  "11",
			DefaultDatabase: "mysql",
			DefaultUsername: "root",
			EnvironmentTemplate: map[string]string{
				"MARIADB_DATABASE":      "{{.Database}}",
				"MARIADB_USER":          "{{.Username}}",
				"MARIADB_PASSWORD":      "{{.Password}}",
				"MARIADB_ROOT_PASSWORD": "{{.Password}}",
			},
			HealthCheckCommand: []string{"CMD-SHELL", "mariadb-admin ping -h localhost -u {{.Username}} -p{{.Password}}"},
			ContainerPort:      "3306/tcp",
		}
	default:
		panic(fmt.Sprintf("unsupported database type: %s", dbType))
	}

	// Compile templates
	config.envTemplates = make(map[string]*template.Template)
	for key, tmplStr := range config.EnvironmentTemplate {
		tmpl, err := template.New(key).Parse(tmplStr)
		if err != nil {
			panic(fmt.Sprintf("failed to parse environment template for %s.%s: %v", dbType, key, err))
		}
		config.envTemplates[key] = tmpl
	}

	config.healthTemplates = make([]*template.Template, len(config.HealthCheckCommand))
	for i, tmplStr := range config.HealthCheckCommand {
		tmpl, err := template.New(fmt.Sprintf("health_%d", i)).Parse(tmplStr)
		if err != nil {
			panic(fmt.Sprintf("failed to parse health check template for %s[%d]: %v", dbType, i, err))
		}
		config.healthTemplates[i] = tmpl
	}

	return config
}

// GenericManager implements DatabaseManager for any database type using configuration.
type GenericManager struct {
	mu        sync.RWMutex
	instances map[string]*types.DatabaseInstance
	docker    *docker.Manager
	config    DatabaseConfig
}

// NewGenericManager creates a new generic database manager for the specified type.
func NewGenericManager(dockerManager *docker.Manager, dbType types.DatabaseType) *GenericManager {
	return &GenericManager{
		instances: make(map[string]*types.DatabaseInstance),
		docker:    dockerManager,
		config:    GetDatabaseConfig(dbType),
	}
}

// CreateInstance creates a new database instance.
func (m *GenericManager) CreateInstance(ctx context.Context, opts types.CreateInstanceOptions) (*types.DatabaseInstance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate instance ID
	instanceID := types.GenerateInstanceID()

	slog.Info("Creating database instance",
		"type", m.config.Type,
		"instance_id", instanceID,
		"version", opts.Version,
		"database", opts.Database,
		"username", opts.Username)

	// Allocate port
	port, err := m.docker.AllocatePort(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate port: %w", err)
	}

	// Create and start container
	instance, err := m.createContainer(ctx, instanceID, opts, port)
	if err != nil {
		// Release port on failure
		m.docker.ReleasePort(port)
		return nil, fmt.Errorf("failed to create %s container: %w", m.config.Type, err)
	}

	// Store instance
	m.instances[instanceID] = instance

	slog.Info("Database instance created successfully",
		"type", m.config.Type,
		"instance_id", instanceID,
		"port", port,
		"dsn", instance.DSN)

	return instance, nil
}

// ListInstances returns all database instances of this type.
func (m *GenericManager) ListInstances(ctx context.Context) ([]*types.DatabaseInstance, error) {
	// Get all containers of this database type
	containers, err := m.listContainers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list %s containers: %w", m.config.Type, err)
	}

	var instances []*types.DatabaseInstance
	for _, cont := range containers {
		// Extract instance information from container labels
		instanceID := cont.Labels["dev-postgres-mcp.instance-id"]
		if instanceID == "" {
			continue
		}

		database := cont.Labels["dev-postgres-mcp.database"]
		username := cont.Labels["dev-postgres-mcp.username"]
		version := cont.Labels["dev-postgres-mcp.version"]
		portStr := cont.Labels["dev-postgres-mcp.port"]
		createdAtStr := cont.Labels["dev-postgres-mcp.created-at"]

		port, _ := strconv.Atoi(portStr)
		createdAt, _ := time.Parse(time.RFC3339, createdAtStr)

		// Determine status
		status := "unknown"
		if len(cont.Names) > 0 {
			if cont.State == "running" {
				status = "running"
			} else {
				status = cont.State
			}
		}

		instance := &types.DatabaseInstance{
			ID:          instanceID,
			Type:        m.config.Type,
			ContainerID: cont.ID,
			Port:        port,
			Database:    database,
			Username:    username,
			Version:     version,
			CreatedAt:   createdAt,
			Status:      status,
		}

		// We don't store password in labels for security, so we can't retrieve it
		// The DSN will be incomplete, but that's acceptable for listing
		instance.DSN = types.BuildDSN(instance)

		instances = append(instances, instance)
	}

	// Update in-memory instances
	m.mu.Lock()
	m.instances = make(map[string]*types.DatabaseInstance)
	for _, instance := range instances {
		m.instances[instance.ID] = instance
	}
	m.mu.Unlock()

	return instances, nil
}

// GetInstance returns a specific database instance by ID.
func (m *GenericManager) GetInstance(ctx context.Context, id string) (*types.DatabaseInstance, error) {
	// First try exact match in-memory instances
	m.mu.RLock()
	if instance, exists := m.instances[id]; exists {
		m.mu.RUnlock()

		// Update status from Docker
		status, err := m.getContainerStatus(ctx, instance.ContainerID)
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

	// Try partial ID matching by listing all instances
	instances, err := m.ListInstances(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	// Find matching instances (exact match or prefix match)
	var matches []*types.DatabaseInstance
	for _, instance := range instances {
		if instance.ID == id || strings.HasPrefix(instance.ID, id) {
			matches = append(matches, instance)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("%s instance %s not found", m.config.Type, id)
	}

	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple %s instances match %s", m.config.Type, id)
	}

	return matches[0], nil
}

// DropInstance removes a database instance.
func (m *GenericManager) DropInstance(ctx context.Context, id string) error {
	// Get instance details first
	instance, err := m.GetInstance(ctx, id)
	if err != nil {
		return err
	}

	slog.Info("Dropping database instance", "type", m.config.Type, "instance_id", instance.ID)

	// Stop and remove container
	if err := m.docker.StopContainer(ctx, instance.ContainerID); err != nil {
		slog.Warn("Failed to stop container", "type", m.config.Type, "instance_id", instance.ID, "error", err)
	}

	if err := m.docker.RemoveContainer(ctx, instance.ContainerID); err != nil {
		return fmt.Errorf("failed to remove %s container: %w", m.config.Type, err)
	}

	// Release port
	m.docker.ReleasePort(instance.Port)

	// Remove from in-memory instances
	m.mu.Lock()
	delete(m.instances, instance.ID)
	m.mu.Unlock()

	slog.Info("Database instance dropped successfully", "type", m.config.Type, "instance_id", instance.ID)
	return nil
}

// HealthCheck performs a health check on a database instance.
func (m *GenericManager) HealthCheck(ctx context.Context, id string) (*types.HealthCheckResult, error) {
	instance, err := m.GetInstance(ctx, id)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	status, err := m.getContainerStatus(ctx, instance.ContainerID)
	duration := time.Since(start)

	result := &types.HealthCheckResult{
		Duration:  duration.String(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	if err != nil {
		result.Status = types.HealthStatusUnknown
		result.Message = fmt.Sprintf("Failed to check status: %v", err)
		return result, nil
	}

	switch status {
	case "running":
		result.Status = types.HealthStatusHealthy
		result.Message = fmt.Sprintf("%s instance is running and healthy", m.config.Type)
	case "starting":
		result.Status = types.HealthStatusStarting
		result.Message = fmt.Sprintf("%s instance is starting up", m.config.Type)
	case "unhealthy":
		result.Status = types.HealthStatusUnhealthy
		result.Message = fmt.Sprintf("%s instance is unhealthy", m.config.Type)
	default:
		result.Status = types.HealthStatusUnknown
		result.Message = fmt.Sprintf("%s instance status: %s", m.config.Type, status)
	}

	return result, nil
}

// Cleanup removes all database instances of this type.
func (m *GenericManager) Cleanup(ctx context.Context) error {
	instances, err := m.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("failed to list %s instances for cleanup: %w", m.config.Type, err)
	}

	var errors []error
	for _, instance := range instances {
		if err := m.DropInstance(ctx, instance.ID); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to cleanup some %s instances: %v", m.config.Type, errors)
	}

	return nil
}

// GetInstanceCount returns the number of database instances of this type.
func (m *GenericManager) GetInstanceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.instances)
}

// GetDatabaseType returns the database type this manager handles.
func (m *GenericManager) GetDatabaseType() types.DatabaseType {
	return m.config.Type
}

// Helper methods

// TemplateData holds the data for template execution.
type TemplateData struct {
	Database string
	Username string
	Password string
}

// executeTemplate executes a template with the given data.
func (m *GenericManager) executeTemplate(tmpl *template.Template, data TemplateData) (string, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}

// createContainer creates and starts a database container.
func (m *GenericManager) createContainer(ctx context.Context, instanceID string, opts types.CreateInstanceOptions, port int) (*types.DatabaseInstance, error) {
	image := types.GetDockerImage(m.config.Type, opts.Version)
	containerName := types.GetContainerName(instanceID, m.config.Type)

	slog.Info("Creating database container",
		"type", m.config.Type,
		"instance_id", instanceID,
		"image", image,
		"port", port,
		"database", opts.Database)

	// Pull the image if needed
	if err := m.docker.PullImage(ctx, image); err != nil {
		return nil, fmt.Errorf("failed to pull %s image: %w", m.config.Type, err)
	}

	// Prepare template data
	data := TemplateData{
		Database: opts.Database,
		Username: opts.Username,
		Password: opts.Password,
	}

	// Build environment variables using compiled templates
	var env []string
	for key, tmpl := range m.config.envTemplates {
		value, err := m.executeTemplate(tmpl, data)
		if err != nil {
			return nil, fmt.Errorf("failed to execute environment template for %s: %w", key, err)
		}
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}

	// Build health check command using compiled templates
	var healthCmd []string
	for _, tmpl := range m.config.healthTemplates {
		cmdPart, err := m.executeTemplate(tmpl, data)
		if err != nil {
			return nil, fmt.Errorf("failed to execute health check template: %w", err)
		}
		healthCmd = append(healthCmd, cmdPart)
	}

	// Create container using the generic Docker client
	containerID, err := m.docker.CreateGenericContainer(ctx, docker.GenericContainerConfig{
		Image:         image,
		ContainerName: containerName,
		Environment:   env,
		Port:          port,
		ContainerPort: m.config.ContainerPort,
		HealthCheck:   healthCmd,
		Labels: map[string]string{
			"dev-postgres-mcp.managed":     "true",
			"dev-postgres-mcp.type":        string(m.config.Type),
			"dev-postgres-mcp.instance-id": instanceID,
			"dev-postgres-mcp.database":    opts.Database,
			"dev-postgres-mcp.username":    opts.Username,
			"dev-postgres-mcp.version":     opts.Version,
			"dev-postgres-mcp.port":        strconv.Itoa(port),
			"dev-postgres-mcp.created-at":  time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create %s container: %w", m.config.Type, err)
	}

	// Start container
	if err := m.docker.StartContainer(ctx, containerID); err != nil {
		// Clean up on failure
		_ = m.docker.RemoveContainer(ctx, containerID)
		return nil, fmt.Errorf("failed to start %s container: %w", m.config.Type, err)
	}

	// Wait for container to be healthy
	if err := m.waitForHealthy(ctx, containerID, 60*time.Second); err != nil {
		// Clean up on failure
		_ = m.docker.RemoveContainer(ctx, containerID)
		return nil, fmt.Errorf("%s container failed to become healthy: %w", m.config.Type, err)
	}

	// Create instance object
	instance := &types.DatabaseInstance{
		ID:          instanceID,
		Type:        m.config.Type,
		ContainerID: containerID,
		Port:        port,
		Database:    opts.Database,
		Username:    opts.Username,
		Password:    opts.Password,
		Version:     opts.Version,
		CreatedAt:   time.Now(),
		Status:      "running",
	}
	instance.DSN = types.BuildDSN(instance)

	slog.Info("Database container created and started successfully",
		"type", m.config.Type,
		"instance_id", instanceID,
		"container_id", containerID,
		"port", port)

	return instance, nil
}

// listContainers lists all containers of this database type.
func (m *GenericManager) listContainers(ctx context.Context) ([]container.Summary, error) {
	return m.docker.ListContainersByType(ctx, m.config.Type)
}

// getContainerStatus returns the status of a container.
func (m *GenericManager) getContainerStatus(ctx context.Context, containerID string) (string, error) {
	inspect, err := m.docker.InspectContainer(ctx, containerID)
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

// waitForHealthy waits for a container to become healthy.
func (m *GenericManager) waitForHealthy(ctx context.Context, containerID string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	slog.Info("Waiting for database container to become healthy", "type", m.config.Type, "container_id", containerID)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for container to become healthy: %w", ctx.Err())
		case <-ticker.C:
			inspect, err := m.docker.InspectContainer(ctx, containerID)
			if err != nil {
				return fmt.Errorf("failed to inspect container: %w", err)
			}

			if !inspect.State.Running {
				return fmt.Errorf("container stopped unexpectedly")
			}

			if inspect.State.Health != nil {
				switch inspect.State.Health.Status {
				case "healthy":
					slog.Info("Database container is healthy", "type", m.config.Type, "container_id", containerID)
					return nil
				case "unhealthy":
					logs, _ := m.docker.ContainerLogs(ctx, containerID, container.LogsOptions{
						ShowStdout: true,
						ShowStderr: true,
						Tail:       "50",
					})
					return fmt.Errorf("container became unhealthy, logs: %s", logs)
				case "starting":
					slog.Debug("Database container is still starting", "type", m.config.Type, "container_id", containerID)
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
