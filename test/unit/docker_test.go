package unit_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/stokaro/dev-postgres-mcp/internal/docker"
)

func TestPortManager(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name      string
		startPort int
		endPort   int
		allocate  int
		expectErr bool
	}{
		{
			name:      "allocate single port",
			startPort: 15432,
			endPort:   15440,
			allocate:  1,
			expectErr: false,
		},
		{
			name:      "allocate multiple ports",
			startPort: 15441,
			endPort:   15450,
			allocate:  5,
			expectErr: false,
		},
		{
			name:      "allocate more ports than available",
			startPort: 15451,
			endPort:   15452,
			allocate:  5,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			pm := docker.NewPortManager(tt.startPort, tt.endPort)
			ctx := context.Background()

			var allocatedPorts []int
			var err error

			for i := 0; i < tt.allocate; i++ {
				port, allocErr := pm.AllocatePort(ctx)
				if allocErr != nil {
					err = allocErr
					break
				}
				allocatedPorts = append(allocatedPorts, port)
			}

			if tt.expectErr {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(len(allocatedPorts), qt.Equals, tt.allocate)

				// Verify all allocated ports are in range
				for _, port := range allocatedPorts {
					c.Assert(port >= tt.startPort, qt.IsTrue, qt.Commentf("Port %d should be >= %d", port, tt.startPort))
					c.Assert(port <= tt.endPort, qt.IsTrue, qt.Commentf("Port %d should be <= %d", port, tt.endPort))
				}

				// Verify ports are marked as allocated
				for _, port := range allocatedPorts {
					c.Assert(pm.IsPortAllocated(port), qt.IsTrue)
				}

				// Release ports and verify they're no longer allocated
				for _, port := range allocatedPorts {
					pm.ReleasePort(port)
					c.Assert(pm.IsPortAllocated(port), qt.IsFalse)
				}
			}
		})
	}
}

func TestPortManagerConcurrency(t *testing.T) {
	c := qt.New(t)

	pm := docker.NewPortManager(16000, 16010)
	ctx := context.Background()

	// Test concurrent allocation
	const numGoroutines = 5
	results := make(chan int, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			port, err := pm.AllocatePort(ctx)
			if err != nil {
				errors <- err
				return
			}
			results <- port
		}()
	}

	// Collect results
	var allocatedPorts []int
	var allocErrors []error

	for i := 0; i < numGoroutines; i++ {
		select {
		case port := <-results:
			allocatedPorts = append(allocatedPorts, port)
		case err := <-errors:
			allocErrors = append(allocErrors, err)
		}
	}

	// Should have allocated some ports without errors
	c.Assert(len(allocatedPorts) > 0, qt.IsTrue)
	c.Assert(len(allocErrors), qt.Equals, 0)

	// All allocated ports should be unique
	portSet := make(map[int]bool)
	for _, port := range allocatedPorts {
		c.Assert(portSet[port], qt.IsFalse, qt.Commentf("Port %d allocated multiple times", port))
		portSet[port] = true
	}
}

func TestPortManagerGetAllocatedPorts(t *testing.T) {
	c := qt.New(t)

	pm := docker.NewPortManager(17000, 17010)
	ctx := context.Background()

	// Initially no ports should be allocated
	allocated := pm.GetAllocatedPorts()
	c.Assert(len(allocated), qt.Equals, 0)

	// Allocate some ports
	port1, err := pm.AllocatePort(ctx)
	c.Assert(err, qt.IsNil)

	port2, err := pm.AllocatePort(ctx)
	c.Assert(err, qt.IsNil)

	// Check allocated ports
	allocated = pm.GetAllocatedPorts()
	c.Assert(len(allocated), qt.Equals, 2)

	// Should contain both ports
	portSet := make(map[int]bool)
	for _, port := range allocated {
		portSet[port] = true
	}
	c.Assert(portSet[port1], qt.IsTrue)
	c.Assert(portSet[port2], qt.IsTrue)

	// Release one port
	pm.ReleasePort(port1)
	allocated = pm.GetAllocatedPorts()
	c.Assert(len(allocated), qt.Equals, 1)
	c.Assert(allocated[0], qt.Equals, port2)
}

func TestHealthStatus(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name     string
		status   docker.HealthStatus
		expected string
	}{
		{
			name:     "healthy status",
			status:   docker.HealthStatusHealthy,
			expected: "healthy",
		},
		{
			name:     "unhealthy status",
			status:   docker.HealthStatusUnhealthy,
			expected: "unhealthy",
		},
		{
			name:     "starting status",
			status:   docker.HealthStatusStarting,
			expected: "starting",
		},
		{
			name:     "stopped status",
			status:   docker.HealthStatusStopped,
			expected: "stopped",
		},
		{
			name:     "unknown status",
			status:   docker.HealthStatusUnknown,
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			c.Assert(string(tt.status), qt.Equals, tt.expected)
		})
	}
}

// TestDockerManagerCreation tests the creation of a Docker manager.
func TestDockerManagerCreation(t *testing.T) {
	c := qt.New(t)

	// Test valid port range
	manager, err := docker.NewManager(18000, 18010)
	if err != nil {
		// If Docker is not available, skip this test
		c.Skip("Docker not available:", err)
	}
	defer manager.Close()

	c.Assert(manager, qt.IsNotNil)
	c.Assert(manager.PostgreSQL(), qt.IsNotNil)
	c.Assert(manager.GetClient(), qt.IsNotNil)
}

// TestDockerManagerPortAllocation tests port allocation through the manager.
func TestDockerManagerPortAllocation(t *testing.T) {
	c := qt.New(t)

	manager, err := docker.NewManager(19000, 19010)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer manager.Close()

	ctx := context.Background()

	// Allocate a port
	port, err := manager.AllocatePort(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(port >= 19000, qt.IsTrue, qt.Commentf("Port %d should be >= 19000", port))
	c.Assert(port <= 19010, qt.IsTrue, qt.Commentf("Port %d should be <= 19010", port))

	// Release the port
	manager.ReleasePort(port)
}
