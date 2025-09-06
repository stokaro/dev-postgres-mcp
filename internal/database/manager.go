// Package database provides unified database instance management functionality.
package database

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/stokaro/dev-postgres-mcp/internal/docker"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

// UnifiedManager manages database instances across multiple database types.
type UnifiedManager struct {
	mu        sync.RWMutex
	instances map[string]*types.DatabaseInstance
	docker    *docker.Manager
	managers  map[types.DatabaseType]types.DatabaseManager
}

// NewUnifiedManager creates a new unified database manager.
func NewUnifiedManager(dockerManager *docker.Manager) *UnifiedManager {
	managers := make(map[types.DatabaseType]types.DatabaseManager)

	// Create database-specific managers using the generic manager
	managers[types.DatabaseTypePostgreSQL] = NewGenericManager(dockerManager, types.DatabaseTypePostgreSQL)
	managers[types.DatabaseTypeMySQL] = NewGenericManager(dockerManager, types.DatabaseTypeMySQL)
	managers[types.DatabaseTypeMariaDB] = NewGenericManager(dockerManager, types.DatabaseTypeMariaDB)

	return &UnifiedManager{
		instances: make(map[string]*types.DatabaseInstance),
		docker:    dockerManager,
		managers:  managers,
	}
}

// CreateInstance creates a new database instance of the specified type.
func (m *UnifiedManager) CreateInstance(ctx context.Context, opts types.CreateInstanceOptions) (*types.DatabaseInstance, error) {
	// Validate and set defaults
	if err := types.ValidateCreateInstanceOptions(&opts); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	// Get the appropriate manager
	manager, exists := m.managers[opts.Type]
	if !exists {
		return nil, fmt.Errorf("unsupported database type: %s", opts.Type)
	}

	// Create the instance
	instance, err := manager.CreateInstance(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Store in unified registry
	m.mu.Lock()
	m.instances[instance.ID] = instance
	m.mu.Unlock()

	slog.Info("Database instance created",
		"instance_id", instance.ID,
		"type", instance.Type,
		"version", instance.Version,
		"port", instance.Port)

	return instance, nil
}

// ListInstances returns all database instances across all types.
func (m *UnifiedManager) ListInstances(ctx context.Context) ([]*types.DatabaseInstance, error) {
	var allInstances []*types.DatabaseInstance

	// Get instances from each database manager
	for dbType, manager := range m.managers {
		instances, err := manager.ListInstances(ctx)
		if err != nil {
			slog.Warn("Failed to list instances for database type", "type", dbType, "error", err)
			continue
		}
		allInstances = append(allInstances, instances...)
	}

	// Update in-memory registry
	m.mu.Lock()
	m.instances = make(map[string]*types.DatabaseInstance)
	for _, instance := range allInstances {
		m.instances[instance.ID] = instance
	}
	m.mu.Unlock()

	return allInstances, nil
}

// ListInstancesByType returns all database instances of a specific type.
func (m *UnifiedManager) ListInstancesByType(ctx context.Context, dbType types.DatabaseType) ([]*types.DatabaseInstance, error) {
	manager, exists := m.managers[dbType]
	if !exists {
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}

	return manager.ListInstances(ctx)
}

// GetInstance returns a specific database instance by ID.
func (m *UnifiedManager) GetInstance(ctx context.Context, id string) (*types.DatabaseInstance, error) {
	// First try exact match in-memory instances
	m.mu.RLock()
	if instance, exists := m.instances[id]; exists {
		m.mu.RUnlock()

		// Get the appropriate manager and update status
		manager, exists := m.managers[instance.Type]
		if exists {
			if updatedInstance, err := manager.GetInstance(ctx, id); err == nil {
				return updatedInstance, nil
			}
		}

		return instance, nil
	}
	m.mu.RUnlock()

	// Try to find in each database manager (supports partial ID matching)
	for _, manager := range m.managers {
		if instance, err := manager.GetInstance(ctx, id); err == nil {
			// Update in-memory registry
			m.mu.Lock()
			m.instances[instance.ID] = instance
			m.mu.Unlock()
			return instance, nil
		}
	}

	return nil, fmt.Errorf("instance %s not found", id)
}

// DropInstance removes a database instance.
func (m *UnifiedManager) DropInstance(ctx context.Context, id string) error {
	// Get the instance to determine its type
	instance, err := m.GetInstance(ctx, id)
	if err != nil {
		return err
	}

	// Get the appropriate manager
	manager, exists := m.managers[instance.Type]
	if !exists {
		return fmt.Errorf("unsupported database type: %s", instance.Type)
	}

	// Drop the instance
	if err := manager.DropInstance(ctx, id); err != nil {
		return err
	}

	// Remove from in-memory registry
	m.mu.Lock()
	delete(m.instances, id)
	m.mu.Unlock()

	slog.Info("Database instance dropped",
		"instance_id", id,
		"type", instance.Type)

	return nil
}

// HealthCheck performs a health check on a database instance.
func (m *UnifiedManager) HealthCheck(ctx context.Context, id string) (*types.HealthCheckResult, error) {
	// Get the instance to determine its type
	instance, err := m.GetInstance(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get the appropriate manager
	manager, exists := m.managers[instance.Type]
	if !exists {
		return nil, fmt.Errorf("unsupported database type: %s", instance.Type)
	}

	return manager.HealthCheck(ctx, id)
}

// Cleanup removes all instances managed by this manager.
func (m *UnifiedManager) Cleanup(ctx context.Context) error {
	var errors []error

	// Cleanup each database manager
	for dbType, manager := range m.managers {
		if err := manager.Cleanup(ctx); err != nil {
			slog.Error("Failed to cleanup database instances", "type", dbType, "error", err)
			errors = append(errors, fmt.Errorf("failed to cleanup %s instances: %w", dbType, err))
		}
	}

	// Clear in-memory registry
	m.mu.Lock()
	m.instances = make(map[string]*types.DatabaseInstance)
	m.mu.Unlock()

	if len(errors) > 0 {
		return fmt.Errorf("cleanup failed for some database types: %v", errors)
	}

	return nil
}

// GetInstanceCount returns the total number of instances across all database types.
func (m *UnifiedManager) GetInstanceCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.instances)
}

// GetInstanceCountByType returns the number of instances for a specific database type.
func (m *UnifiedManager) GetInstanceCountByType(dbType types.DatabaseType) int {
	manager, exists := m.managers[dbType]
	if !exists {
		return 0
	}
	return manager.GetInstanceCount()
}

// GetSupportedDatabaseTypes returns a list of supported database types.
func (m *UnifiedManager) GetSupportedDatabaseTypes() []types.DatabaseType {
	supportedTypes := make([]types.DatabaseType, 0, len(m.managers))
	for dbType := range m.managers {
		supportedTypes = append(supportedTypes, dbType)
	}
	return supportedTypes
}
