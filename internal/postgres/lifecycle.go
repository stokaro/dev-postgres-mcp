package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/stokaro/dev-postgres-mcp/internal/docker"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

// LifecycleManager handles advanced lifecycle operations for PostgreSQL instances.
type LifecycleManager struct {
	manager       *Manager
	healthChecker *docker.HealthChecker
	mu            sync.RWMutex
	monitors      map[string]*InstanceMonitor
}

// InstanceMonitor monitors a single PostgreSQL instance.
type InstanceMonitor struct {
	instanceID    string
	ctx           context.Context
	cancel        context.CancelFunc
	healthChecker *docker.HealthChecker
	manager       *Manager
	interval      time.Duration
	lastHealth    *docker.HealthCheck
	mu            sync.RWMutex
}

// NewLifecycleManager creates a new lifecycle manager.
func NewLifecycleManager(manager *Manager, dockerClient *docker.Client) *LifecycleManager {
	return &LifecycleManager{
		manager:       manager,
		healthChecker: docker.NewHealthChecker(dockerClient),
		monitors:      make(map[string]*InstanceMonitor),
	}
}

// StartMonitoring starts monitoring an instance for health and lifecycle events.
func (lm *LifecycleManager) StartMonitoring(instanceID string, interval time.Duration) error {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	// Check if already monitoring
	if _, exists := lm.monitors[instanceID]; exists {
		return fmt.Errorf("instance %s is already being monitored", instanceID)
	}

	// Verify instance exists
	_, err := lm.manager.GetInstance(context.Background(), instanceID)
	if err != nil {
		return fmt.Errorf("instance %s not found: %w", instanceID, err)
	}

	// Create monitor
	ctx, cancel := context.WithCancel(context.Background())
	monitor := &InstanceMonitor{
		instanceID:    instanceID,
		ctx:           ctx,
		cancel:        cancel,
		healthChecker: lm.healthChecker,
		manager:       lm.manager,
		interval:      interval,
	}

	lm.monitors[instanceID] = monitor

	// Start monitoring goroutine
	go monitor.run()

	slog.Info("Started monitoring instance", "instance_id", instanceID, "interval", interval)
	return nil
}

// StopMonitoring stops monitoring an instance.
func (lm *LifecycleManager) StopMonitoring(instanceID string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	monitor, exists := lm.monitors[instanceID]
	if !exists {
		return
	}

	monitor.cancel()
	delete(lm.monitors, instanceID)

	slog.Info("Stopped monitoring instance", "instance_id", instanceID)
}

// GetInstanceHealth returns the last known health status of an instance.
func (lm *LifecycleManager) GetInstanceHealth(instanceID string) (*docker.HealthCheck, error) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	monitor, exists := lm.monitors[instanceID]
	if !exists {
		return nil, fmt.Errorf("instance %s is not being monitored", instanceID)
	}

	monitor.mu.RLock()
	defer monitor.mu.RUnlock()

	if monitor.lastHealth == nil {
		return nil, fmt.Errorf("no health data available for instance %s", instanceID)
	}

	return monitor.lastHealth, nil
}

// GetMonitoredInstances returns a list of all monitored instance IDs.
func (lm *LifecycleManager) GetMonitoredInstances() []string {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	instances := make([]string, 0, len(lm.monitors))
	for instanceID := range lm.monitors {
		instances = append(instances, instanceID)
	}

	return instances
}

// Shutdown stops all monitoring and cleans up resources.
func (lm *LifecycleManager) Shutdown() {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	slog.Info("Shutting down lifecycle manager", "monitored_instances", len(lm.monitors))

	for instanceID, monitor := range lm.monitors {
		monitor.cancel()
		slog.Debug("Stopped monitoring during shutdown", "instance_id", instanceID)
	}

	lm.monitors = make(map[string]*InstanceMonitor)
}

// run is the main monitoring loop for an instance.
func (im *InstanceMonitor) run() {
	ticker := time.NewTicker(im.interval)
	defer ticker.Stop()

	slog.Debug("Starting instance monitor", "instance_id", im.instanceID, "interval", im.interval)

	for {
		select {
		case <-im.ctx.Done():
			slog.Debug("Instance monitor stopped", "instance_id", im.instanceID)
			return
		case <-ticker.C:
			im.performHealthCheck()
		}
	}
}

// performHealthCheck performs a health check on the monitored instance.
func (im *InstanceMonitor) performHealthCheck() {
	// Get instance details
	instance, err := im.manager.GetInstance(im.ctx, im.instanceID)
	if err != nil {
		slog.Warn("Failed to get instance for health check", "instance_id", im.instanceID, "error", err)
		return
	}

	// Perform health check
	health, err := im.healthChecker.CheckPostgreSQLInstance(im.ctx, instance.ContainerID, instance.DSN)
	if err != nil {
		slog.Warn("Health check failed", "instance_id", im.instanceID, "error", err)
		return
	}

	// Update last health status
	im.mu.Lock()
	im.lastHealth = health
	im.mu.Unlock()

	// Log health status changes
	slog.Debug("Health check completed",
		"instance_id", im.instanceID,
		"status", health.Status,
		"duration", health.Duration,
		"message", health.Message)

	// Handle unhealthy instances
	if health.Status == docker.HealthStatusUnhealthy {
		slog.Warn("Instance is unhealthy",
			"instance_id", im.instanceID,
			"message", health.Message)
		// Could implement automatic recovery here
	}
}

// CreateInstanceWithMonitoring creates a new instance and starts monitoring it.
func (lm *LifecycleManager) CreateInstanceWithMonitoring(ctx context.Context, opts types.CreateInstanceOptions, monitorInterval time.Duration) (*types.PostgreSQLInstance, error) {
	// Create the instance
	instance, err := lm.manager.CreateInstance(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Start monitoring
	if err := lm.StartMonitoring(instance.ID, monitorInterval); err != nil {
		slog.Warn("Failed to start monitoring for new instance", "instance_id", instance.ID, "error", err)
		// Don't fail the creation, just log the warning
	}

	return instance, nil
}

// DropInstanceWithCleanup drops an instance and stops monitoring it.
func (lm *LifecycleManager) DropInstanceWithCleanup(ctx context.Context, instanceID string) error {
	// Stop monitoring first
	lm.StopMonitoring(instanceID)

	// Drop the instance
	return lm.manager.DropInstance(ctx, instanceID)
}

// RestartInstance restarts a PostgreSQL instance.
func (lm *LifecycleManager) RestartInstance(ctx context.Context, instanceID string) error {
	instance, err := lm.manager.GetInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("instance %s not found: %w", instanceID, err)
	}

	slog.Info("Restarting PostgreSQL instance", "instance_id", instanceID)

	// Stop the container
	dockerManager := lm.manager.docker
	if err := dockerManager.PostgreSQL().StopPostgreSQLContainer(ctx, instance.ContainerID); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	// Start the container
	if err := dockerManager.GetClient().StartContainer(ctx, instance.ContainerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	// Wait for it to become healthy
	healthChecker := docker.NewHealthChecker(dockerManager.GetClient())
	if err := healthChecker.WaitForHealthy(ctx, instance.ContainerID, instance.DSN, 60*time.Second); err != nil {
		return fmt.Errorf("instance failed to become healthy after restart: %w", err)
	}

	slog.Info("PostgreSQL instance restarted successfully", "instance_id", instanceID)
	return nil
}
