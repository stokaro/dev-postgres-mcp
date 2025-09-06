package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/stokaro/dev-postgres-mcp/internal/database"
	"github.com/stokaro/dev-postgres-mcp/internal/docker"
)

// Server represents the MCP server for database instance management.
type Server struct {
	mcpServer      *server.MCPServer
	stdioServer    *server.StdioServer
	toolHandler    *ToolHandler
	unifiedManager *database.UnifiedManager
	dockerMgr      *docker.Manager
}

// ServerConfig holds configuration for the MCP server.
type ServerConfig struct {
	Name      string
	Version   string
	StartPort int
	EndPort   int
	LogLevel  string
}

// NewServer creates a new MCP server.
func NewServer(config ServerConfig) (*Server, error) {
	// Create Docker manager
	dockerMgr, err := docker.NewManager(config.StartPort, config.EndPort)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker manager: %w", err)
	}

	// Test Docker connection
	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		dockerMgr.Close()
		return nil, fmt.Errorf("Docker daemon is not accessible: %w", err)
	}

	// Create unified database manager
	unifiedManager := database.NewUnifiedManager(dockerMgr)

	// Create tool handler
	toolHandler := NewToolHandler(unifiedManager)

	// Create MCP server
	mcpServer := server.NewMCPServer(config.Name, config.Version)

	// Add tools to the server
	tools := toolHandler.GetTools()
	for _, tool := range tools {
		mcpServer.AddTool(tool, toolHandler.HandleTool)
	}

	// Create stdio server wrapper
	stdioServer := server.NewStdioServer(mcpServer)

	return &Server{
		mcpServer:      mcpServer,
		stdioServer:    stdioServer,
		toolHandler:    toolHandler,
		unifiedManager: unifiedManager,
		dockerMgr:      dockerMgr,
	}, nil
}

// Start starts the MCP server.
func (s *Server) Start(ctx context.Context) error {
	slog.Info("Starting MCP server for database instance management")

	// Start the stdio server
	slog.Info("MCP server started, waiting for requests...")
	return s.stdioServer.Listen(ctx, os.Stdin, os.Stdout)
}

// Stop stops the MCP server and cleans up resources.
func (s *Server) Stop(ctx context.Context) error {
	slog.Info("Stopping MCP server")

	// Cleanup all database instances
	if err := s.unifiedManager.Cleanup(ctx); err != nil {
		slog.Error("Failed to cleanup database instances", "error", err)
	}

	// Close Docker manager
	if err := s.dockerMgr.Close(); err != nil {
		slog.Error("Failed to close Docker manager", "error", err)
	}

	slog.Info("MCP server stopped")
	return nil
}

// Close closes the MCP server and cleans up resources.
func (s *Server) Close() error {
	ctx := context.Background()

	// Cleanup all database instances
	if err := s.unifiedManager.Cleanup(ctx); err != nil {
		slog.Error("Failed to cleanup database instances", "error", err)
	}

	// Close Docker manager
	if err := s.dockerMgr.Close(); err != nil {
		slog.Error("Failed to close Docker manager", "error", err)
		return err
	}

	return nil
}

// GetInstanceCount returns the number of currently managed instances.
func (s *Server) GetInstanceCount() int {
	return s.unifiedManager.GetInstanceCount()
}

// GetServerInfo returns information about the MCP server.
func (s *Server) GetServerInfo() map[string]any {
	return map[string]any{
		"name":    "dev-postgres-mcp",
		"version": "1.0.0",
	}
}

// GetServerCapabilities returns the capabilities of the MCP server.
func (s *Server) GetServerCapabilities() map[string]any {
	return map[string]any{
		"tools": map[string]any{
			"listChanged": false,
		},
	}
}
