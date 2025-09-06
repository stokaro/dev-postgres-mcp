package integration_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/stokaro/dev-postgres-mcp/internal/database"
	"github.com/stokaro/dev-postgres-mcp/internal/docker"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

func TestUnifiedDatabaseManager(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(20100, 20200)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer dockerMgr.Close()

	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		c.Skip("Docker daemon not accessible:", err)
	}

	// Create unified database manager
	unifiedManager := database.NewUnifiedManager(dockerMgr)

	t.Run("Create PostgreSQL instance", func(t *testing.T) {
		c := qt.New(t)

		opts := types.CreateInstanceOptions{
			Type:     types.DatabaseTypePostgreSQL,
			Version:  "17",
			Database: "testdb",
			Username: "testuser",
			Password: "testpass",
		}

		instance, err := unifiedManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		c.Assert(instance, qt.IsNotNil)
		c.Assert(instance.Type, qt.Equals, types.DatabaseTypePostgreSQL)
		c.Assert(instance.Version, qt.Equals, "17")
		c.Assert(instance.Database, qt.Equals, "testdb")
		c.Assert(instance.Username, qt.Equals, "testuser")
		c.Assert(instance.Password, qt.Equals, "testpass")
		c.Assert(instance.Port, qt.Not(qt.Equals), 0)
		c.Assert(instance.DSN, qt.Not(qt.Equals), "")

		// Clean up
		defer func() {
			err := unifiedManager.DropInstance(ctx, instance.ID)
			c.Assert(err, qt.IsNil)
		}()

		// Test health check
		health, err := unifiedManager.HealthCheck(ctx, instance.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(health, qt.IsNotNil)
		c.Assert(health.Status, qt.Not(qt.Equals), types.HealthStatusUnknown)
	})

	t.Run("Create MySQL instance", func(t *testing.T) {
		c := qt.New(t)

		opts := types.CreateInstanceOptions{
			Type:     types.DatabaseTypeMySQL,
			Version:  "8.0",
			Database: "testdb",
			Username: "root",
			Password: "testpass",
		}

		instance, err := unifiedManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		c.Assert(instance, qt.IsNotNil)
		c.Assert(instance.Type, qt.Equals, types.DatabaseTypeMySQL)
		c.Assert(instance.Version, qt.Equals, "8.0")
		c.Assert(instance.Database, qt.Equals, "testdb")
		c.Assert(instance.Username, qt.Equals, "root")
		c.Assert(instance.Password, qt.Equals, "testpass")
		c.Assert(instance.Port, qt.Not(qt.Equals), 0)
		c.Assert(instance.DSN, qt.Not(qt.Equals), "")

		// Clean up
		defer func() {
			err := unifiedManager.DropInstance(ctx, instance.ID)
			c.Assert(err, qt.IsNil)
		}()

		// Test health check
		health, err := unifiedManager.HealthCheck(ctx, instance.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(health, qt.IsNotNil)
		c.Assert(health.Status, qt.Not(qt.Equals), types.HealthStatusUnknown)
	})

	t.Run("Create MariaDB instance", func(t *testing.T) {
		c := qt.New(t)

		opts := types.CreateInstanceOptions{
			Type:     types.DatabaseTypeMariaDB,
			Version:  "11",
			Database: "testdb",
			Username: "root",
			Password: "testpass",
		}

		instance, err := unifiedManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		c.Assert(instance, qt.IsNotNil)
		c.Assert(instance.Type, qt.Equals, types.DatabaseTypeMariaDB)
		c.Assert(instance.Version, qt.Equals, "11")
		c.Assert(instance.Database, qt.Equals, "testdb")
		c.Assert(instance.Username, qt.Equals, "root")
		c.Assert(instance.Password, qt.Equals, "testpass")
		c.Assert(instance.Port, qt.Not(qt.Equals), 0)
		c.Assert(instance.DSN, qt.Not(qt.Equals), "")

		// Clean up
		defer func() {
			err := unifiedManager.DropInstance(ctx, instance.ID)
			c.Assert(err, qt.IsNil)
		}()

		// Test health check
		health, err := unifiedManager.HealthCheck(ctx, instance.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(health, qt.IsNotNil)
		c.Assert(health.Status, qt.Not(qt.Equals), types.HealthStatusUnknown)
	})

	t.Run("List instances by type", func(t *testing.T) {
		c := qt.New(t)

		// Create instances of different types
		pgOpts := types.CreateInstanceOptions{
			Type:     types.DatabaseTypePostgreSQL,
			Database: "pgtest",
		}
		pgInstance, err := unifiedManager.CreateInstance(ctx, pgOpts)
		c.Assert(err, qt.IsNil)
		defer unifiedManager.DropInstance(ctx, pgInstance.ID)

		mysqlOpts := types.CreateInstanceOptions{
			Type:     types.DatabaseTypeMySQL,
			Database: "mysqltest",
		}
		mysqlInstance, err := unifiedManager.CreateInstance(ctx, mysqlOpts)
		c.Assert(err, qt.IsNil)
		defer unifiedManager.DropInstance(ctx, mysqlInstance.ID)

		// List all instances
		allInstances, err := unifiedManager.ListInstances(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(allInstances), qt.Equals, 2)

		// List PostgreSQL instances only
		pgInstances, err := unifiedManager.ListInstancesByType(ctx, types.DatabaseTypePostgreSQL)
		c.Assert(err, qt.IsNil)
		c.Assert(len(pgInstances), qt.Equals, 1)
		c.Assert(pgInstances[0].Type, qt.Equals, types.DatabaseTypePostgreSQL)

		// List MySQL instances only
		mysqlInstances, err := unifiedManager.ListInstancesByType(ctx, types.DatabaseTypeMySQL)
		c.Assert(err, qt.IsNil)
		c.Assert(len(mysqlInstances), qt.Equals, 1)
		c.Assert(mysqlInstances[0].Type, qt.Equals, types.DatabaseTypeMySQL)

		// List MariaDB instances (should be empty)
		mariadbInstances, err := unifiedManager.ListInstancesByType(ctx, types.DatabaseTypeMariaDB)
		c.Assert(err, qt.IsNil)
		c.Assert(len(mariadbInstances), qt.Equals, 0)
	})

	t.Run("Get instance by ID", func(t *testing.T) {
		c := qt.New(t)

		opts := types.CreateInstanceOptions{
			Type:     types.DatabaseTypePostgreSQL,
			Database: "gettest",
		}
		instance, err := unifiedManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		defer unifiedManager.DropInstance(ctx, instance.ID)

		// Get by full ID
		retrieved, err := unifiedManager.GetInstance(ctx, instance.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(retrieved.ID, qt.Equals, instance.ID)
		c.Assert(retrieved.Type, qt.Equals, instance.Type)

		// Get by partial ID (first 8 characters)
		partialID := instance.ID[:8]
		retrieved, err = unifiedManager.GetInstance(ctx, partialID)
		c.Assert(err, qt.IsNil)
		c.Assert(retrieved.ID, qt.Equals, instance.ID)
	})

	t.Run("Drop instance", func(t *testing.T) {
		c := qt.New(t)

		opts := types.CreateInstanceOptions{
			Type:     types.DatabaseTypePostgreSQL,
			Database: "droptest",
		}
		instance, err := unifiedManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)

		// Verify instance exists
		_, err = unifiedManager.GetInstance(ctx, instance.ID)
		c.Assert(err, qt.IsNil)

		// Drop instance
		err = unifiedManager.DropInstance(ctx, instance.ID)
		c.Assert(err, qt.IsNil)

		// Verify instance no longer exists
		_, err = unifiedManager.GetInstance(ctx, instance.ID)
		c.Assert(err, qt.IsNotNil)
	})

	t.Run("Cleanup all instances", func(t *testing.T) {
		c := qt.New(t)

		// Create multiple instances
		for i := 0; i < 3; i++ {
			opts := types.CreateInstanceOptions{
				Type:     types.DatabaseTypePostgreSQL,
				Database: "cleanuptest",
			}
			_, err := unifiedManager.CreateInstance(ctx, opts)
			c.Assert(err, qt.IsNil)
		}

		// Verify instances exist
		instances, err := unifiedManager.ListInstances(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(instances), qt.Equals, 3)

		// Cleanup all
		err = unifiedManager.Cleanup(ctx)
		c.Assert(err, qt.IsNil)

		// Verify no instances remain
		instances, err = unifiedManager.ListInstances(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(instances), qt.Equals, 0)
	})

	t.Run("Invalid database type", func(t *testing.T) {
		c := qt.New(t)

		opts := types.CreateInstanceOptions{
			Type: "invalid",
		}
		_, err := unifiedManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNotNil)
	})

	t.Run("Instance count", func(t *testing.T) {
		c := qt.New(t)

		// Start with clean state
		err := unifiedManager.Cleanup(ctx)
		c.Assert(err, qt.IsNil)

		// Verify count is 0
		count := unifiedManager.GetInstanceCount()
		c.Assert(count, qt.Equals, 0)

		// Create an instance
		opts := types.CreateInstanceOptions{
			Type: types.DatabaseTypePostgreSQL,
		}
		instance, err := unifiedManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		defer unifiedManager.DropInstance(ctx, instance.ID)

		// Verify count is 1
		count = unifiedManager.GetInstanceCount()
		c.Assert(count, qt.Equals, 1)
	})
}
