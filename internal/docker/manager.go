package docker

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
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
	postgres    *PostgreSQLManager
}

// NewManager creates a new Docker manager with the specified port range.
func NewManager(startPort, endPort int) (*Manager, error) {
	client, err := NewClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	portManager := NewPortManager(startPort, endPort)
	postgres := NewPostgreSQLManager(client)

	return &Manager{
		client:      client,
		portManager: portManager,
		postgres:    postgres,
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

// PostgreSQL returns the PostgreSQL manager.
func (m *Manager) PostgreSQL() *PostgreSQLManager {
	return m.postgres
}

// GetClient returns the underlying Docker client.
func (m *Manager) GetClient() *Client {
	return m.client
}
