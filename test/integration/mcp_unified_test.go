package integration_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stokaro/dev-postgres-mcp/internal/database"
	"github.com/stokaro/dev-postgres-mcp/internal/docker"
	"github.com/stokaro/dev-postgres-mcp/internal/mcp"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

func TestUnifiedMCPTools(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(20300, 20400)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer dockerMgr.Close()

	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		c.Skip("Docker daemon not accessible:", err)
	}

	// Create unified database manager and tool handler
	unifiedManager := database.NewUnifiedManager(dockerMgr)
	toolHandler := mcp.NewToolHandler(unifiedManager)

	t.Run("Unified tools definition", func(t *testing.T) {
		c := qt.New(t)

		tools := toolHandler.GetTools()
		c.Assert(len(tools), qt.Equals, 5) // 5 unified tools

		expectedTools := []string{
			"create_database_instance",
			"list_database_instances",
			"get_database_instance",
			"drop_database_instance",
			"health_check_database",
		}

		toolNames := make([]string, len(tools))
		for i, tool := range tools {
			toolNames[i] = tool.Name
		}

		for _, expectedTool := range expectedTools {
			c.Assert(toolNames, qt.Contains, expectedTool)
		}
	})

	t.Run("Create database instance tool", func(t *testing.T) {
		c := qt.New(t)

		// Test PostgreSQL creation
		result, err := callTool(ctx, toolHandler, "create_database_instance", map[string]any{
			"type":     "postgresql",
			"version":  "17",
			"database": "testdb",
			"username": "testuser",
			"password": "testpass",
		})
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		content := getTextContent(result, 0)
		c.Assert(content, qt.Contains, "Database instance created successfully")
		c.Assert(content, qt.Contains, "postgresql")

		// Clean up - extract instance ID from response
		defer func() {
			instances, _ := unifiedManager.ListInstancesByType(ctx, types.DatabaseTypePostgreSQL)
			for _, instance := range instances {
				unifiedManager.DropInstance(ctx, instance.ID)
			}
		}()
	})

	t.Run("List database instances tool", func(t *testing.T) {
		c := qt.New(t)

		// Create a test instance first
		opts := types.CreateInstanceOptions{
			Type:     types.DatabaseTypePostgreSQL,
			Database: "listtest",
		}
		instance, err := unifiedManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		defer unifiedManager.DropInstance(ctx, instance.ID)

		// Test listing all instances
		result, err := callTool(ctx, toolHandler, "list_database_instances", map[string]any{})
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		content := getTextContent(result, 0)
		c.Assert(content, qt.Contains, "Database instances")
		c.Assert(content, qt.Contains, instance.ID)

		// Test filtering by type
		result, err = callTool(ctx, toolHandler, "list_database_instances", map[string]any{
			"type": "postgresql",
		})
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		content = getTextContent(result, 0)
		c.Assert(content, qt.Contains, "postgresql")
	})

	t.Run("Get database instance tool", func(t *testing.T) {
		c := qt.New(t)

		// Create a test instance first
		opts := types.CreateInstanceOptions{
			Type:     types.DatabaseTypePostgreSQL,
			Database: "gettest",
		}
		instance, err := unifiedManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		defer unifiedManager.DropInstance(ctx, instance.ID)

		// Test getting instance details
		result, err := callTool(ctx, toolHandler, "get_database_instance", map[string]any{
			"instance_id": instance.ID,
		})
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		content := getTextContent(result, 0)
		c.Assert(content, qt.Contains, "Database instance details")
		c.Assert(content, qt.Contains, instance.ID)
		c.Assert(content, qt.Contains, "postgresql")
	})

	t.Run("Health check database tool", func(t *testing.T) {
		c := qt.New(t)

		// Create a test instance first
		opts := types.CreateInstanceOptions{
			Type:     types.DatabaseTypePostgreSQL,
			Database: "healthtest",
		}
		instance, err := unifiedManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)
		defer unifiedManager.DropInstance(ctx, instance.ID)

		// Wait a bit for the instance to be ready
		time.Sleep(5 * time.Second)

		// Test health check
		result, err := callTool(ctx, toolHandler, "health_check_database", map[string]any{
			"instance_id": instance.ID,
		})
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		content := getTextContent(result, 0)
		c.Assert(content, qt.Contains, "Health check results")
		c.Assert(content, qt.Contains, instance.ID)
	})

	t.Run("Drop database instance tool", func(t *testing.T) {
		c := qt.New(t)

		// Create a test instance first
		opts := types.CreateInstanceOptions{
			Type:     types.DatabaseTypePostgreSQL,
			Database: "droptest",
		}
		instance, err := unifiedManager.CreateInstance(ctx, opts)
		c.Assert(err, qt.IsNil)

		// Test dropping the instance
		result, err := callTool(ctx, toolHandler, "drop_database_instance", map[string]any{
			"instance_id": instance.ID,
		})
		c.Assert(err, qt.IsNil)
		c.Assert(result, qt.IsNotNil)

		content := getTextContent(result, 0)
		c.Assert(content, qt.Contains, "Database instance dropped")
		c.Assert(content, qt.Contains, instance.ID)

		// Verify instance is gone
		_, err = unifiedManager.GetInstance(ctx, instance.ID)
		c.Assert(err, qt.IsNotNil)
	})

	t.Run("Error handling", func(t *testing.T) {
		c := qt.New(t)

		// Test unknown tool
		result, err := callTool(ctx, toolHandler, "unknown_tool", map[string]any{})
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsTrue)
		c.Assert(getTextContent(result, 0), qt.Contains, "Unknown tool")

		// Test missing instance ID
		result, err = callTool(ctx, toolHandler, "get_database_instance", map[string]any{})
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsTrue)
		c.Assert(getTextContent(result, 0), qt.Contains, "instance_id parameter is required")

		// Test nonexistent instance
		result, err = callTool(ctx, toolHandler, "get_database_instance", map[string]any{
			"instance_id": "nonexistent",
		})
		c.Assert(err, qt.IsNil)
		c.Assert(result.IsError, qt.IsTrue)
		c.Assert(getTextContent(result, 0), qt.Contains, "Failed to get database instance")
	})
}

// Helper functions from the original test file
func callTool(ctx context.Context, handler *mcp.ToolHandler, name string, arguments map[string]any) (*mcplib.CallToolResult, error) {
	request := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name:      name,
			Arguments: arguments,
		},
	}
	return handler.HandleTool(ctx, request)
}

func getTextContent(result *mcplib.CallToolResult, index int) string {
	if index >= len(result.Content) {
		return ""
	}
	if textContent, ok := result.Content[index].(mcplib.TextContent); ok {
		return textContent.Text
	}
	return ""
}
