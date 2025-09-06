// Package mcp provides Model Context Protocol integration for database instance management.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/stokaro/dev-postgres-mcp/internal/database"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

// ToolHandler handles MCP tool calls for database instance management.
type ToolHandler struct {
	manager *database.UnifiedManager
}

// NewToolHandler creates a new MCP tool handler with unified database support.
func NewToolHandler(manager *database.UnifiedManager) *ToolHandler {
	return &ToolHandler{
		manager: manager,
	}
}

// GetTools returns the list of available MCP tools.
func (h *ToolHandler) GetTools() []mcp.Tool {
	return []mcp.Tool{
		mcp.NewTool("create_database_instance",
			mcp.WithDescription("Create a new ephemeral database instance in a Docker container"),
			mcp.WithString("type", mcp.Description("Database type: postgresql, mysql, or mariadb (default: postgresql)")),
			mcp.WithString("version", mcp.Description("Database version to use (defaults vary by type)")),
			mcp.WithString("database", mcp.Description("Database name to create (defaults vary by type)")),
			mcp.WithString("username", mcp.Description("Database username (defaults vary by type)")),
			mcp.WithString("password", mcp.Description("Database password (auto-generated if not provided)")),
		),
		mcp.NewTool("list_database_instances",
			mcp.WithDescription("List all running database instances"),
			mcp.WithString("type", mcp.Description("Filter by database type: postgresql, mysql, mariadb (optional)")),
		),
		mcp.NewTool("get_database_instance",
			mcp.WithDescription("Get details of a specific database instance"),
			mcp.WithString("instance_id", mcp.Description("The unique identifier of the database instance"), mcp.Required()),
		),
		mcp.NewTool("drop_database_instance",
			mcp.WithDescription("Remove a database instance and all its data"),
			mcp.WithString("instance_id", mcp.Description("The unique identifier of the database instance to remove"), mcp.Required()),
		),
		mcp.NewTool("health_check_database",
			mcp.WithDescription("Check the health status of a database instance"),
			mcp.WithString("instance_id", mcp.Description("The unique identifier of the database instance to check"), mcp.Required()),
		),
	}
}

// HandleTool handles an MCP tool call.
func (h *ToolHandler) HandleTool(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	name := request.Params.Name
	arguments := request.Params.Arguments
	slog.Info("Handling MCP tool call", "tool", name, "arguments", arguments)

	// Convert arguments to map[string]any
	args, ok := arguments.(map[string]any)
	if !ok {
		args = make(map[string]any)
	}

	switch name {
	case "create_database_instance":
		return h.handleCreateDatabaseInstance(ctx, args)
	case "list_database_instances":
		return h.handleListDatabaseInstances(ctx, args)
	case "get_database_instance":
		return h.handleGetDatabaseInstance(ctx, args)
	case "drop_database_instance":
		return h.handleDropDatabaseInstance(ctx, args)
	case "health_check_database":
		return h.handleHealthCheckDatabase(ctx, args)
	default:
		return mcp.NewToolResultError(fmt.Sprintf("Unknown tool: %s", name)), nil
	}
}

// handleCreateDatabaseInstance handles the create_database_instance tool call.
func (h *ToolHandler) handleCreateDatabaseInstance(ctx context.Context, arguments map[string]any) (*mcp.CallToolResult, error) {
	opts := types.CreateInstanceOptions{}

	// Parse arguments
	if dbType, ok := arguments["type"].(string); ok {
		opts.Type = types.DatabaseType(dbType)
	}
	if version, ok := arguments["version"].(string); ok {
		opts.Version = version
	}
	if databaseName, ok := arguments["database"].(string); ok {
		opts.Database = databaseName
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
		return mcp.NewToolResultError(fmt.Sprintf("Failed to create database instance: %v", err)), nil
	}

	// Format response
	response := map[string]any{
		"instance_id":  instance.ID,
		"type":         instance.Type,
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

	return mcp.NewToolResultText(fmt.Sprintf("Database instance created successfully:\n\n```json\n%s\n```", string(responseJSON))), nil
}

// handleListDatabaseInstances handles the list_database_instances tool call.
func (h *ToolHandler) handleListDatabaseInstances(ctx context.Context, arguments map[string]any) (*mcp.CallToolResult, error) {
	var instances []*types.DatabaseInstance
	var err error

	// Check if filtering by type
	if dbTypeStr, ok := arguments["type"].(string); ok && dbTypeStr != "" {
		dbType := types.DatabaseType(dbTypeStr)
		if !dbType.IsValid() {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid database type: %s", dbTypeStr)), nil
		}
		instances, err = h.manager.ListInstancesByType(ctx, dbType)
	} else {
		instances, err = h.manager.ListInstances(ctx)
	}

	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to list database instances: %v", err)), nil
	}

	if len(instances) == 0 {
		return mcp.NewToolResultText("No database instances are currently running."), nil
	}

	// Format response
	response := map[string]any{
		"count":     len(instances),
		"instances": instances,
	}

	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Database instances:\n\n```json\n%s\n```", string(responseJSON))), nil
}

// handleGetDatabaseInstance handles the get_database_instance tool call.
func (h *ToolHandler) handleGetDatabaseInstance(ctx context.Context, arguments map[string]any) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instance_id"].(string)
	if !ok || instanceID == "" {
		return mcp.NewToolResultError("instance_id parameter is required"), nil
	}

	instance, err := h.manager.GetInstance(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get database instance: %v", err)), nil
	}

	responseJSON, err := json.MarshalIndent(instance, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Database instance details:\n\n```json\n%s\n```", string(responseJSON))), nil
}

// handleDropDatabaseInstance handles the drop_database_instance tool call.
func (h *ToolHandler) handleDropDatabaseInstance(ctx context.Context, arguments map[string]any) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instance_id"].(string)
	if !ok || instanceID == "" {
		return mcp.NewToolResultError("instance_id parameter is required"), nil
	}

	// Get instance details before dropping for response
	instance, err := h.manager.GetInstance(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to find database instance: %v", err)), nil
	}

	err = h.manager.DropInstance(ctx, instanceID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to drop database instance: %v", err)), nil
	}

	response := map[string]any{
		"message":     "Database instance dropped successfully",
		"instance_id": instance.ID,
		"type":        instance.Type,
		"port":        instance.Port,
	}

	responseJSON, err := json.MarshalIndent(response, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to format response: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Database instance dropped:\n\n```json\n%s\n```", string(responseJSON))), nil
}

// handleHealthCheckDatabase handles the health_check_database tool call.
func (h *ToolHandler) handleHealthCheckDatabase(ctx context.Context, arguments map[string]any) (*mcp.CallToolResult, error) {
	instanceID, ok := arguments["instance_id"].(string)
	if !ok || instanceID == "" {
		return mcp.NewToolResultError("instance_id parameter is required"), nil
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
