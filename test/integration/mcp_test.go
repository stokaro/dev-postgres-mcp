package integration_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stokaro/dev-postgres-mcp/internal/docker"
	"github.com/stokaro/dev-postgres-mcp/internal/mcp"
	"github.com/stokaro/dev-postgres-mcp/internal/postgres"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

// Helper function to call MCP tools
func callTool(ctx context.Context, toolHandler *mcp.ToolHandler, name string, arguments map[string]interface{}) (*mcplib.CallToolResult, error) {
	request := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      name,
			Arguments: arguments,
		},
	}
	return toolHandler.HandleTool(ctx, request)
}

// Helper function to get text content from result
func getTextContent(result *mcplib.CallToolResult, index int) string {
	if len(result.Content) <= index {
		return ""
	}
	if textContent, ok := mcplib.AsTextContent(result.Content[index]); ok {
		return textContent.Text
	}
	return ""
}

func TestMCPToolHandler(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(20000, 20010)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer dockerMgr.Close()

	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		c.Skip("Docker daemon not accessible:", err)
	}

	// Create PostgreSQL manager and tool handler
	postgresManager := postgres.NewManager(dockerMgr)
	toolHandler := mcp.NewToolHandler(postgresManager)

	// Test create_postgres_instance tool
	c.Run("create_postgres_instance", func(c *qt.C) {
		arguments := map[string]interface{}{
			"version":  "17",
			"database": "testdb",
			"username": "testuser",
			"password": "testpass",
		}

		result, err := callTool(ctx, toolHandler, "create_postgres_instance", arguments)
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsFalse)
		c.Assert(len(result.Content) > 0, qt.IsTrue)
		textContent := getTextContent(result, 0)
		c.Assert(textContent, qt.Contains, "PostgreSQL instance created successfully")

		// Clean up - we'll get the instance ID from the list
		defer func() {
			instances, _ := postgresManager.ListInstances(ctx)
			for _, instance := range instances {
				_ = postgresManager.DropInstance(ctx, instance.ID)
			}
		}()
	})

	c.Run("list_postgres_instances", func(c *qt.C) {
		// First create an instance
		opts := types.CreateInstanceOptions{
			Version:  "17",
			Database: "testdb",
			Username: "testuser",
			Password: "testpass",
		}

		instance, err := postgresManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		defer postgresManager.DropInstance(ctx, instance.ID)

		// Test list tool
		result, err := callTool(ctx, toolHandler, "list_postgres_instances", map[string]interface{}{})
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsFalse)
		c.Assert(len(result.Content) > 0, qt.IsTrue)
		textContent := getTextContent(result, 0)
		c.Assert(textContent, qt.Contains, "Found 1 PostgreSQL instance")
	})

	c.Run("get_postgres_instance", func(c *qt.C) {
		// First create an instance
		opts := types.CreateInstanceOptions{
			Version:  "17",
			Database: "testdb",
			Username: "testuser",
			Password: "testpass",
		}

		instance, err := postgresManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		defer postgresManager.DropInstance(ctx, instance.ID)

		// Test get tool
		arguments := map[string]interface{}{
			"instance_id": instance.ID,
		}

		result, err := callTool(ctx, toolHandler, "get_postgres_instance", arguments)
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsFalse)
		c.Assert(len(result.Content) > 0, qt.IsTrue)
		textContent := getTextContent(result, 0)
		c.Assert(textContent, qt.Contains, "PostgreSQL instance details")
		c.Assert(textContent, qt.Contains, instance.ID)
	})

	c.Run("health_check_postgres", func(c *qt.C) {
		// First create an instance
		opts := types.CreateInstanceOptions{
			Version:  "17",
			Database: "testdb",
			Username: "testuser",
			Password: "testpass",
		}

		instance, err := postgresManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		defer postgresManager.DropInstance(ctx, instance.ID)

		// Wait a moment for the instance to be fully ready
		time.Sleep(2 * time.Second)

		// Test health check tool
		arguments := map[string]interface{}{
			"instance_id": instance.ID,
		}

		result, err := callTool(ctx, toolHandler, "health_check_postgres", arguments)
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsFalse)
		c.Assert(len(result.Content) > 0, qt.IsTrue)
		textContent := getTextContent(result, 0)
		c.Assert(textContent, qt.Contains, "Health check results")
	})

	c.Run("drop_postgres_instance", func(c *qt.C) {
		// First create an instance
		opts := types.CreateInstanceOptions{
			Version:  "17",
			Database: "testdb",
			Username: "testuser",
			Password: "testpass",
		}

		instance, err := postgresManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)

		// Test drop tool
		arguments := map[string]interface{}{
			"instance_id": instance.ID,
		}

		result, err := callTool(ctx, toolHandler, "drop_postgres_instance", arguments)
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsFalse)
		c.Assert(len(result.Content) > 0, qt.IsTrue)
		textContent := getTextContent(result, 0)
		c.Assert(textContent, qt.Contains, "successfully dropped")

		// Verify instance is gone
		_, err = postgresManager.GetInstance(ctx, instance.ID)
		c.Assert(err, qt.IsNotNil)
	})
}

func TestMCPToolHandlerErrors(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(20100, 20110)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer dockerMgr.Close()

	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		c.Skip("Docker daemon not accessible:", err)
	}

	postgresManager := postgres.NewManager(dockerMgr)
	toolHandler := mcp.NewToolHandler(postgresManager)

	c.Run("unknown_tool", func(c *qt.C) {
		result, err := callTool(ctx, toolHandler, "unknown_tool", map[string]interface{}{})
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsTrue)
		c.Assert(getTextContent(result, 0), qt.Contains, "Unknown tool")
	})

	c.Run("get_instance_missing_id", func(c *qt.C) {
		result, err := callTool(ctx, toolHandler, "get_postgres_instance", map[string]interface{}{})
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsTrue)
		c.Assert(getTextContent(result, 0), qt.Contains, "instance_id is required")
	})

	c.Run("get_instance_not_found", func(c *qt.C) {
		arguments := map[string]interface{}{
			"instance_id": "nonexistent-id",
		}

		result, err := callTool(ctx, toolHandler, "get_postgres_instance", arguments)
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsTrue)
		c.Assert(getTextContent(result, 0), qt.Contains, "Failed to get PostgreSQL instance")
	})

	c.Run("drop_instance_missing_id", func(c *qt.C) {
		result, err := callTool(ctx, toolHandler, "drop_postgres_instance", map[string]interface{}{})
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsTrue)
		c.Assert(getTextContent(result, 0), qt.Contains, "instance_id is required")
	})

	c.Run("drop_instance_not_found", func(c *qt.C) {
		arguments := map[string]interface{}{
			"instance_id": "nonexistent-id",
		}

		result, err := callTool(ctx, toolHandler, "drop_postgres_instance", arguments)
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsTrue)
		c.Assert(getTextContent(result, 0), qt.Contains, "Failed to drop PostgreSQL instance")
	})

	c.Run("health_check_missing_id", func(c *qt.C) {
		result, err := callTool(ctx, toolHandler, "health_check_postgres", map[string]interface{}{})
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsTrue)
		c.Assert(getTextContent(result, 0), qt.Contains, "instance_id is required")
	})
}

func TestMCPToolsDefinition(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(20200, 20210)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer dockerMgr.Close()

	postgresManager := postgres.NewManager(dockerMgr)
	toolHandler := mcp.NewToolHandler(postgresManager)

	tools := toolHandler.GetTools()
	c.Assert(len(tools), qt.Equals, 5)

	expectedTools := []string{
		"create_postgres_instance",
		"list_postgres_instances",
		"get_postgres_instance",
		"drop_postgres_instance",
		"health_check_postgres",
	}

	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}

	for _, expectedTool := range expectedTools {
		c.Assert(toolNames, qt.Contains, expectedTool)
	}

	// Verify tool schemas
	for _, tool := range tools {
		c.Assert(tool.Name, qt.Not(qt.Equals), "")
		c.Assert(tool.Description, qt.Not(qt.Equals), "")
		c.Assert(tool.InputSchema.Type, qt.Equals, "object")
	}
}
