package docker

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/go-connections/nat"

	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

// PortManager manages port allocation for containers.
type PortManager struct {
	mu        sync.Mutex
	startPort int
	endPort   int
	allocated map[int]bool
}

// NewPortManager creates a new port manager with the specified port range.
func NewPortManager(startPort, endPort int) *PortManager {
	return &PortManager{
		startPort: startPort,
		endPort:   endPort,
		allocated: make(map[int]bool),
	}
}

// AllocatePort allocates an available port in the configured range.
func (pm *PortManager) AllocatePort(_ context.Context) (int, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Try to find an available port
	for port := pm.startPort; port <= pm.endPort; port++ {
		if pm.allocated[port] {
			continue
		}

		// Check if port is actually available on the system
		if pm.isPortAvailable(port) {
			pm.allocated[port] = true
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", pm.startPort, pm.endPort)
}

// ReleasePort releases a previously allocated port.
func (pm *PortManager) ReleasePort(port int) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	delete(pm.allocated, port)
}

// IsPortAllocated checks if a port is currently allocated.
func (pm *PortManager) IsPortAllocated(port int) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	return pm.allocated[port]
}

// GetAllocatedPorts returns a slice of all currently allocated ports.
func (pm *PortManager) GetAllocatedPorts() []int {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	ports := make([]int, 0, len(pm.allocated))
	for port := range pm.allocated {
		ports = append(ports, port)
	}

	return ports
}

// isPortAvailable checks if a port is available on the local system.
func (pm *PortManager) isPortAvailable(port int) bool {
	// Try to bind to the port to see if it's available
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	defer listener.Close()

	return true
}

// Manager combines Docker client and port management functionality.
type Manager struct {
	client      *Client
	portManager *PortManager
}

// NewManager creates a new Docker manager with the specified port range.
func NewManager(startPort, endPort int) (*Manager, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	portManager := NewPortManager(startPort, endPort)

	return &Manager{
		client:      client,
		portManager: portManager,
	}, nil
}

// Close closes the Docker manager and releases resources.
func (m *Manager) Close() error {
	return m.client.Close()
}

// Ping checks if the Docker daemon is accessible.
func (m *Manager) Ping(ctx context.Context) error {
	return m.client.Ping(ctx)
}

// AllocatePort allocates an available port.
func (m *Manager) AllocatePort(ctx context.Context) (int, error) {
	return m.portManager.AllocatePort(ctx)
}

// ReleasePort releases a previously allocated port.
func (m *Manager) ReleasePort(port int) {
	m.portManager.ReleasePort(port)
}

// GetClient returns the underlying Docker client.
func (m *Manager) GetClient() *Client {
	return m.client
}

// GenericContainerConfig holds configuration for creating a generic database container.
type GenericContainerConfig struct {
	Image         string
	ContainerName string
	Environment   []string
	Port          int
	ContainerPort string
	HealthCheck   []string
	Labels        map[string]string
}

// CreateGenericContainer creates a generic database container.
func (m *Manager) CreateGenericContainer(ctx context.Context, config GenericContainerConfig) (string, error) {
	// Pull the image if needed
	if err := m.client.PullImage(ctx, config.Image); err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}

	// Configure container
	containerConfig := &container.Config{
		Image:  config.Image,
		Env:    config.Environment,
		Labels: config.Labels,
		ExposedPorts: nat.PortSet{
			nat.Port(config.ContainerPort): struct{}{},
		},
		Healthcheck: &container.HealthConfig{
			Test:        config.HealthCheck,
			Interval:    10 * time.Second,
			Timeout:     5 * time.Second,
			Retries:     5,
			StartPeriod: 30 * time.Second,
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			nat.Port(config.ContainerPort): []nat.PortBinding{
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
	containerID, err := m.client.CreateContainer(ctx, containerConfig, hostConfig, config.ContainerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return containerID, nil
}

// ListContainersByType lists all containers of a specific database type.
func (m *Manager) ListContainersByType(ctx context.Context, dbType types.DatabaseType) ([]container.Summary, error) {
	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "dev-postgres-mcp.managed=true")
	filterArgs.Add("label", fmt.Sprintf("dev-postgres-mcp.type=%s", dbType))

	containers, err := m.client.ListContainers(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, err
	}

	return containers, nil
}

// StartContainer starts a container.
func (m *Manager) StartContainer(ctx context.Context, containerID string) error {
	return m.client.StartContainer(ctx, containerID)
}

// StopContainer stops a container.
func (m *Manager) StopContainer(ctx context.Context, containerID string) error {
	return m.client.StopContainer(ctx, containerID)
}

// RemoveContainer removes a container.
func (m *Manager) RemoveContainer(ctx context.Context, containerID string) error {
	return m.client.RemoveContainer(ctx, containerID)
}

// InspectContainer inspects a container.
func (m *Manager) InspectContainer(ctx context.Context, containerID string) (*container.InspectResponse, error) {
	inspect, err := m.client.InspectContainer(ctx, containerID)
	if err != nil {
		return nil, err
	}
	return &inspect, nil
}

// ContainerLogs gets container logs.
func (m *Manager) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (string, error) {
	return m.client.ContainerLogs(ctx, containerID, options)
}

// PullImage pulls a Docker image.
func (m *Manager) PullImage(ctx context.Context, image string) error {
	return m.client.PullImage(ctx, image)
}
