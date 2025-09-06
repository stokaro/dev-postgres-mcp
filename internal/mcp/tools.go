// Package mcp provides Model Context Protocol integration for PostgreSQL instance management.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/stokaro/dev-postgres-mcp/internal/postgres"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

// ToolHandler handles MCP tool calls for PostgreSQL instance management.
type ToolHandler struct {
	manager *postgres.Manager
}

// NewToolHandler creates a new MCP tool handler.
func NewToolHandler(manager *postgres.Manager) *ToolHandler {
	return &ToolHandler{
		manager: manager,
	}
}

// GetTools returns the list of available MCP tools.
func (h *ToolHandler) GetTools() []mcp.Tool {
	return []mcp.Tool{
		mcp.NewTool("create_postgres_instance",
			mcp.WithDescription("Create a new ephemeral PostgreSQL instance in a Docker container"),
			mcp.WithString("version", mcp.Description("PostgreSQL version to use (default: 17)")),
			mcp.WithString("database", mcp.Description("Database name to create (default: postgres)")),
			mcp.WithString("username", mcp.Description("PostgreSQL username (default: postgres)")),
			mcp.WithString("password", mcp.Description("PostgreSQL password (auto-generated if not provided)")),
		),
		mcp.NewTool("list_postgres_instances",
			mcp.WithDescription("List all running PostgreSQL instances"),
		),
		mcp.NewTool("get_postgres_instance",
			mcp.WithDescription("Get details of a specific PostgreSQL instance"),
			mcp.WithString("instance_id", mcp.Description("The unique identifier of the PostgreSQL instance"), mcp.Required()),
		),
		mcp.NewTool("drop_postgres_instance",
			mcp.WithDescription("Remove a PostgreSQL instance and all its data"),
			mcp.WithString("instance_id", mcp.Description("The unique identifier of the PostgreSQL instance to remove"), mcp.Required()),
		),
		mcp.NewTool("health_check_postgres",
			mcp.WithDescription("Check the health status of a PostgreSQL instance"),
			mcp.WithString("instance_id", mcp.Description("The unique identifier of the PostgreSQL instance to check"), mcp.Required()),
		),
	}
}

// HandleTool handles an MCP tool call.
func (h *ToolHandler) HandleTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := request.Params.Name
	arguments := request.Params.Arguments
	slog.Info("Handling MCP tool call", "tool", name, "arguments", arguments)

	// Convert arguments to map[string]interface{}
	args, ok := arguments.(map[string]interface{})
	if !ok {
		args = make(map[string]interface{})
	}

	switch name {
	case "create_postgres_instance":
		return h.handleCreateInstance(ctx, args)
	case "list_postgres_instances":
		return h.handleListInstances(ctx, args)
	case "get_postgres_instance":
		return h.handleGetInstance(ctx, args)
	case "drop_postgres_instance":
		return h.handleDropInstance(ctx, args)
	case "health_check_postgres":
		return h.handleHealthCheck(ctx, args)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Unknown tool: %s", name)), nil
	}
}

// handleCreateInstance handles the create_postgres_instance tool call.
func (h *ToolHandler) handleCreateInstance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	opts := types.CreateInstanceOptions{}

	// Parse arguments
	if version, ok := arguments["version"].(string); ok {
		opts.Version = version
	}
	if database, ok := arguments["database"].(string); ok {
		opts.Database = database
	}
	if username, ok := arguments["username"].(string); ok {
		opts.Username = username
	}
	if password, ok := arguments["password"].(string); ok {
		opts.Password = password
	}

	// Create instance
	instance, err := h.manager.CreateInstance(ctx, opts)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create PostgreSQL instance: %v", err)), nil
	}

	// Format response
	response := map[string]interface{}{
		"instance_id":  instance.ID,
		"container_id": instance.ContainerID,
		"port":         instance.Port,
		"database":     instance.Database,
		"username":     instance.Username,
		"password":     instance.Password,
		"version":      instance.Version,
		"dsn":          instance.DSN,
		"created_at":   instance.CreatedAt,
		"status":       instance.Status,
	}

	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("PostgreSQL instance created successfully:\n\n```json\n%s\n```", string(responseJSON))), nil
}

// handleListInstances handles the list_postgres_instances tool call.
func (h *ToolHandler) handleListInstances(ctx context.Context, _ map[string]interface{}) (*mcp.CallToolResult, error) {
	instances, err := h.manager.ListInstances(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list PostgreSQL instances: %v", err)), nil
	}

	if len(instances) == 0 {
		return mcp.NewToolResultText("No PostgreSQL instances are currently running."), nil
	}

	// Format response
	response := map[string]interface{}{
		"count":     len(instances),
		"instances": instances,
	}

	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Found %d PostgreSQL instance(s):\n\n```json\n%s\n```", len(instances), string(responseJSON))), nil
}

// handleGetInstance handles the get_postgres_instance tool call.
func (h *ToolHandler) handleGetInstance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instance_id"].(string)
	if !ok || instanceID == "" {
		return mcp.NewToolResultError("instance_id is required and must be a string"), nil
	}

	instance, err := h.manager.GetInstance(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get PostgreSQL instance: %v", err)), nil
	}

	responseJSON, err := json.MarshalIndent(instance, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("PostgreSQL instance details:\n\n```json\n%s\n```", string(responseJSON))), nil
}

// handleDropInstance handles the drop_postgres_instance tool call.
func (h *ToolHandler) handleDropInstance(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instance_id"].(string)
	if !ok || instanceID == "" {
		return mcp.NewToolResultError("instance_id is required and must be a string"), nil
	}

	err := h.manager.DropInstance(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to drop PostgreSQL instance: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("PostgreSQL instance %s has been successfully dropped and all data has been removed.", instanceID)), nil
}

// handleHealthCheck handles the health_check_postgres tool call.
func (h *ToolHandler) handleHealthCheck(ctx context.Context, arguments map[string]interface{}) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instance_id"].(string)
	if !ok || instanceID == "" {
		return mcp.NewToolResultError("instance_id is required and must be a string"), nil
	}

	health, err := h.manager.HealthCheck(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to perform health check: %v", err)), nil
	}

	responseJSON, err := json.MarshalIndent(health, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Health check results for instance %s:\n\n```json\n%s\n```", instanceID, string(responseJSON))), nil
}
