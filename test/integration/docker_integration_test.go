package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stokaro/dev-postgres-mcp/internal/docker"
	"github.com/stokaro/dev-postgres-mcp/internal/postgres"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

func TestPostgreSQLInstanceIntegration(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(23000, 23010)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer dockerMgr.Close()

	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		c.Skip("Docker daemon not accessible:", err)
	}

	// Create PostgreSQL manager
	postgresManager := postgres.NewManager(dockerMgr)

	c.Run("create_and_connect_to_instance", func(c *qt.C) {
		// Create instance
		opts := types.CreateInstanceOptions{
			Version:  "17",
			Database: "testdb",
			Username: "testuser",
			Password: "testpass",
		}

		instance, err := postgresManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		c.Assert(instance, qt.IsNotNil)
		c.Assert(instance.ID, qt.Not(qt.Equals), "")
		c.Assert(instance.Port >= 23000, qt.IsTrue)
		c.Assert(instance.Port <= 23010, qt.IsTrue)
		c.Assert(instance.DSN, qt.Contains, "testuser")
		c.Assert(instance.DSN, qt.Contains, "testpass")
		c.Assert(instance.DSN, qt.Contains, "testdb")

		// Clean up
		defer func() {
			err := postgresManager.DropInstance(ctx, instance.ID)
			c.Assert(err, qt.IsNil)
		}()

		// Wait a moment for the instance to be fully ready
		time.Sleep(5 * time.Second)

		// Test database connection
		db, err := sql.Open("postgres", instance.DSN)
		c.Assert(err, qt.IsNil)
		defer db.Close()

		// Set connection timeout
		db.SetConnMaxLifetime(10 * time.Second)
		db.SetMaxOpenConns(1)

		// Test connection
		err = db.Ping()
		c.Assert(err, qt.IsNil)

		// Test basic query
		var result int
		err = db.QueryRow("SELECT 1").Scan(&result)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.Equals, 1)

		// Test database creation
		_, err = db.Exec("CREATE TABLE test_table (id SERIAL PRIMARY KEY, name TEXT)")
		c.Assert(err, qt.IsNil)

		// Test data insertion
		_, err = db.Exec("INSERT INTO test_table (name) VALUES ($1)", "test_value")
		c.Assert(err, qt.IsNil)

		// Test data retrieval
		var name string
		err = db.QueryRow("SELECT name FROM test_table WHERE id = 1").Scan(&name)
		c.Assert(err, qt.IsNil)
		c.Assert(name, qt.Equals, "test_value")
	})

	c.Run("multiple_instances", func(c *qt.C) {
		// Create multiple instances
		var instances []*types.PostgreSQLInstance

		for i := 0; i < 3; i++ {
			opts := types.CreateInstanceOptions{
				Version:  "17",
				Database: fmt.Sprintf("testdb%d", i),
				Username: "testuser",
				Password: "testpass",
			}

			instance, err := postgresManager.CreateInstance(ctx, opts)
			c.Assert(err, qt.IsNil)
			instances = append(instances, instance)
		}

		// Clean up
		defer func() {
			for _, instance := range instances {
				_ = postgresManager.DropInstance(ctx, instance.ID)
			}
		}()

		// Verify all instances are different
		for i := 0; i < len(instances); i++ {
			for j := i + 1; j < len(instances); j++ {
				c.Assert(instances[i].ID, qt.Not(qt.Equals), instances[j].ID)
				c.Assert(instances[i].Port, qt.Not(qt.Equals), instances[j].Port)
				c.Assert(instances[i].ContainerID, qt.Not(qt.Equals), instances[j].ContainerID)
			}
		}

		// List instances
		listedInstances, err := postgresManager.ListInstances(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(listedInstances) >= 3, qt.IsTrue)

		// Verify all created instances are in the list
		instanceIDs := make(map[string]bool)
		for _, instance := range listedInstances {
			instanceIDs[instance.ID] = true
		}

		for _, instance := range instances {
			c.Assert(instanceIDs[instance.ID], qt.IsTrue)
		}
	})

	c.Run("instance_lifecycle", func(c *qt.C) {
		// Create instance
		opts := types.CreateInstanceOptions{
			Version:  "17",
			Database: "lifecycledb",
			Username: "lifecycleuser",
			Password: "lifecyclepass",
		}

		instance, err := postgresManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)

		// Get instance
		retrievedInstance, err := postgresManager.GetInstance(ctx, instance.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(retrievedInstance.ID, qt.Equals, instance.ID)
		c.Assert(retrievedInstance.Port, qt.Equals, instance.Port)
		c.Assert(retrievedInstance.Database, qt.Equals, instance.Database)

		// Health check
		health, err := postgresManager.HealthCheck(ctx, instance.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(health, qt.IsNotNil)

		// Drop instance
		err = postgresManager.DropInstance(ctx, instance.ID)
		c.Assert(err, qt.IsNil)

		// Verify instance is gone
		_, err = postgresManager.GetInstance(ctx, instance.ID)
		c.Assert(err, qt.IsNotNil)
	})
}

func TestPostgreSQLVersions(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(23100, 23110)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer dockerMgr.Close()

	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		c.Skip("Docker daemon not accessible:", err)
	}

	postgresManager := postgres.NewManager(dockerMgr)

	versions := []string{"17", "16", "15"}

	for _, version := range versions {
		c.Run(fmt.Sprintf("postgresql_%s", version), func(c *qt.C) {
			opts := types.CreateInstanceOptions{
				Version:  version,
				Database: "versiontest",
				Username: "testuser",
				Password: "testpass",
			}

			instance, err := postgresManager.CreateInstance(ctx, opts)
			c.Assert(err, qt.IsNil)
			c.Assert(instance.Version, qt.Equals, version)

			// Clean up
			defer func() {
				_ = postgresManager.DropInstance(ctx, instance.ID)
			}()

			// Wait for instance to be ready
			time.Sleep(10 * time.Second)

			// Test connection
			db, err := sql.Open("postgres", instance.DSN)
			c.Assert(err, qt.IsNil)
			defer db.Close()

			db.SetConnMaxLifetime(10 * time.Second)
			db.SetMaxOpenConns(1)

			err = db.Ping()
			c.Assert(err, qt.IsNil)

			// Verify PostgreSQL version
			var pgVersion string
			err = db.QueryRow("SELECT version()").Scan(&pgVersion)
			c.Assert(err, qt.IsNil)
			c.Assert(pgVersion, qt.Contains, "PostgreSQL")
			c.Assert(pgVersion, qt.Contains, version)
		})
	}
}

func TestDockerContainerManagement(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(23200, 23210)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer dockerMgr.Close()

	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		c.Skip("Docker daemon not accessible:", err)
	}

	c.Run("container_with_testcontainers", func(c *qt.C) {
		// Use testcontainers to create a PostgreSQL container for comparison
		req := testcontainers.ContainerRequest{
			Image:        "postgres:17",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_DB":       "testdb",
				"POSTGRES_USER":     "testuser",
				"POSTGRES_PASSWORD": "testpass",
			},
			WaitingFor: wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second),
		}

		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			c.Skip("Failed to start testcontainer:", err)
		}
		defer container.Terminate(ctx)

		// Get container details
		host, err := container.Host(ctx)
		c.Assert(err, qt.IsNil)

		port, err := container.MappedPort(ctx, "5432")
		c.Assert(err, qt.IsNil)

		// Test connection
		dsn := fmt.Sprintf("postgres://testuser:testpass@%s:%s/testdb?sslmode=disable", host, port.Port())
		db, err := sql.Open("postgres", dsn)
		c.Assert(err, qt.IsNil)
		defer db.Close()

		db.SetConnMaxLifetime(10 * time.Second)
		db.SetMaxOpenConns(1)

		err = db.Ping()
		c.Assert(err, qt.IsNil)

		// Test basic functionality
		var result int
		err = db.QueryRow("SELECT 1").Scan(&result)
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.Equals, 1)
	})
}
