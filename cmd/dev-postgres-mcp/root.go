package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/stokaro/dev-postgres-mcp/cmd/common/version"
	"github.com/stokaro/dev-postgres-mcp/internal/docker"
	"github.com/stokaro/dev-postgres-mcp/internal/mcp"
	"github.com/stokaro/dev-postgres-mcp/internal/postgres"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(args ...string) {
	var rootCmd = &cobra.Command{
		Use:   "dev-postgres-mcp",
		Short: "MCP server for managing ephemeral PostgreSQL instances",
		Long: `dev-postgres-mcp is a Model Context Protocol (MCP) server that provides
tools for creating, managing, and accessing ephemeral PostgreSQL database instances
running in Docker containers.

Each PostgreSQL instance is completely ephemeral and can be created and destroyed
on demand, making it perfect for development, testing, and experimentation.

FEATURES:
  • Create ephemeral PostgreSQL instances in Docker containers
  • Dynamic port allocation to prevent conflicts
  • Support for multiple PostgreSQL versions (default: PostgreSQL 17)
  • Superuser access with auto-generated credentials
  • MCP integration compatible with Augment Code and other MCP clients
  • CLI tools for instance management

EXAMPLES:
  # Start the MCP server
  dev-postgres-mcp mcp serve

  # List all running PostgreSQL instances
  dev-postgres-mcp postgres list

  # Drop a specific instance
  dev-postgres-mcp postgres drop <instance-id>

Use "dev-postgres-mcp [command] --help" for detailed information about each command.`,
		Args: cobra.NoArgs, // Disallow unknown subcommands
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	rootCmd.SetArgs(args)

	// Add subcommands
	rootCmd.AddCommand(newMCPCommand())
	rootCmd.AddCommand(newPostgresCommand())
	rootCmd.AddCommand(version.New())

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1) //revive:disable-line:deep-exit
	}
}

// newMCPCommand creates the mcp command group.
func newMCPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP server commands",
		Long:  "Commands for managing the MCP (Model Context Protocol) server.",
	}

	cmd.AddCommand(newMCPServeCommand())

	return cmd
}

// newMCPServeCommand creates the mcp serve command.
func newMCPServeCommand() *cobra.Command {
	var startPort int
	var endPort int
	var logLevel string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server",
		Long: `Start the MCP server using stdio transport.

This command starts the MCP server that provides tools for managing PostgreSQL
instances. The server communicates using the Model Context Protocol over stdio,
making it compatible with MCP clients like Augment Code.

The server provides the following tools:
  • create_postgres_instance - Create a new PostgreSQL instance
  • list_postgres_instances - List all running instances
  • get_postgres_instance - Get details of a specific instance
  • drop_postgres_instance - Remove a PostgreSQL instance
  • health_check_postgres - Check instance health

ENVIRONMENT VARIABLES:
  DEV_POSTGRES_MCP_LOG_LEVEL    Log level (debug, info, warn, error)
  DEV_POSTGRES_MCP_LOG_FORMAT   Log format (text, json)`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runMCPServe(startPort, endPort, logLevel)
		},
	}

	cmd.Flags().IntVar(&startPort, "start-port", 15432, "Start of port range for PostgreSQL instances")
	cmd.Flags().IntVar(&endPort, "end-port", 25432, "End of port range for PostgreSQL instances")
	cmd.Flags().StringVar(&logLevel, "log-level", "", "Log level (debug, info, warn, error)")

	return cmd
}

// newPostgresCommand creates the postgres command group.
func newPostgresCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "postgres",
		Short: "PostgreSQL instance management commands",
		Long:  "Commands for managing PostgreSQL instances outside of the MCP protocol.",
	}

	cmd.AddCommand(newPostgresListCommand())
	cmd.AddCommand(newPostgresDropCommand())

	return cmd
}

// newPostgresListCommand creates the postgres list command.
func newPostgresListCommand() *cobra.Command {
	var format string
	var startPort int
	var endPort int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all running PostgreSQL instances",
		Long: `List all currently running PostgreSQL instances managed by this server.

This command shows details about each instance including:
  • Instance ID
  • Container ID
  • Port number
  • Database name
  • Status
  • Creation time`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runPostgresList(format, startPort, endPort)
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&startPort, "start-port", 15432, "Start of port range for PostgreSQL instances")
	cmd.Flags().IntVar(&endPort, "end-port", 25432, "End of port range for PostgreSQL instances")

	return cmd
}

// newPostgresDropCommand creates the postgres drop command.
func newPostgresDropCommand() *cobra.Command {
	var startPort int
	var endPort int
	var force bool

	cmd := &cobra.Command{
		Use:   "drop <instance-id>",
		Short: "Drop a PostgreSQL instance",
		Long: `Drop (remove) a specific PostgreSQL instance by its ID.

This command will:
  • Stop the PostgreSQL container
  • Remove the container and all associated data
  • Free up the allocated port
  • Clean up all resources

WARNING: This action is irreversible. All data in the instance will be lost.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			instanceID := args[0]
			return runPostgresDrop(instanceID, startPort, endPort, force)
		},
	}

	cmd.Flags().IntVar(&startPort, "start-port", 15432, "Start of port range for PostgreSQL instances")
	cmd.Flags().IntVar(&endPort, "end-port", 25432, "End of port range for PostgreSQL instances")
	cmd.Flags().BoolVar(&force, "force", false, "Force removal without confirmation")

	return cmd
}

// runPostgresList lists all PostgreSQL instances.
func runPostgresList(format string, startPort, endPort int) error {
	// Create Docker manager
	dockerMgr, err := docker.NewManager(startPort, endPort)
	if err != nil {
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer dockerMgr.Close()

	// Test Docker connection
	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		return fmt.Errorf("Docker daemon is not accessible: %w", err)
	}

	// Create PostgreSQL manager
	postgresManager := postgres.NewManager(dockerMgr)

	// List instances
	instances, err := postgresManager.ListInstances(ctx)
	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	// Output results
	switch format {
	case "json":
		return outputInstancesJSON(instances)
	case "table":
		return outputInstancesTable(instances)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// outputInstancesJSON outputs instances in JSON format.
func outputInstancesJSON(instances []*types.PostgreSQLInstance) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]interface{}{
		"count":     len(instances),
		"instances": instances,
	})
}

// outputInstancesTable outputs instances in table format.
func outputInstancesTable(instances []*types.PostgreSQLInstance) error {
	if len(instances) == 0 {
		fmt.Println("No PostgreSQL instances are currently running.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Header
	fmt.Fprintln(w, "INSTANCE ID\tCONTAINER ID\tPORT\tDATABASE\tUSERNAME\tVERSION\tSTATUS\tCREATED")

	// Rows
	for _, instance := range instances {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
			instance.ID, // Show full ID like Docker
			instance.ContainerID, // Show full container ID like Docker
			instance.Port,
			instance.Database,
			instance.Username,
			instance.Version,
			instance.Status,
			instance.CreatedAt.Format(time.RFC3339),
		)
	}

	return nil
}

// runPostgresDrop drops a PostgreSQL instance.
func runPostgresDrop(instanceID string, startPort, endPort int, force bool) error {
	// Create Docker manager
	dockerMgr, err := docker.NewManager(startPort, endPort)
	if err != nil {
		return fmt.Errorf("failed to create Docker manager: %w", err)
	}
	defer dockerMgr.Close()

	// Test Docker connection
	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		return fmt.Errorf("Docker daemon is not accessible: %w", err)
	}

	// Create PostgreSQL manager
	postgresManager := postgres.NewManager(dockerMgr)

	// Get instance details first to verify it exists
	instance, err := postgresManager.GetInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("instance %s not found: %w", instanceID, err)
	}

	// Confirmation prompt unless force is used
	if !force {
		fmt.Printf("WARNING: This will permanently delete PostgreSQL instance %s and all its data.\n", instanceID)
		fmt.Printf("Instance details:\n")
		fmt.Printf("  ID: %s\n", instance.ID)
		fmt.Printf("  Port: %d\n", instance.Port)
		fmt.Printf("  Database: %s\n", instance.Database)
		fmt.Printf("  Version: %s\n", instance.Version)
		fmt.Printf("  Created: %s\n", instance.CreatedAt.Format(time.RFC3339))
		fmt.Print("\nAre you sure you want to continue? (y/N): ")

		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" && response != "yes" && response != "YES" {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	// Drop the instance
	if err := postgresManager.DropInstance(ctx, instanceID); err != nil {
		return fmt.Errorf("failed to drop instance: %w", err)
	}

	fmt.Printf("PostgreSQL instance %s has been successfully dropped.\n", instanceID)
	return nil
}

// runMCPServe starts the MCP server.
func runMCPServe(startPort, endPort int, logLevel string) error {
	// Setup logging
	loggingConfig := mcp.GetLoggingConfigFromEnv()
	if logLevel != "" {
		switch logLevel {
		case "debug":
			loggingConfig.Level = mcp.LogLevelDebug
		case "info":
			loggingConfig.Level = mcp.LogLevelInfo
		case "warn":
			loggingConfig.Level = mcp.LogLevelWarn
		case "error":
			loggingConfig.Level = mcp.LogLevelError
		default:
			return fmt.Errorf("invalid log level: %s", logLevel)
		}
	}
	mcp.SetupLogging(loggingConfig)

	// Create server configuration
	config := mcp.ServerConfig{
		Name:      "dev-postgres-mcp",
		Version:   version.Version,
		StartPort: startPort,
		EndPort:   endPort,
		LogLevel:  string(loggingConfig.Level),
	}

	// Create and start server
	server, err := mcp.NewServer(config)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		cancel()
	}()

	// Start server in a goroutine
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- server.Start(ctx)
	}()

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
		// Graceful shutdown
		if err := server.Stop(context.Background()); err != nil {
			return fmt.Errorf("failed to stop server gracefully: %w", err)
		}
		return nil
	case err := <-serverErr:
		// Server error
		return err
	}
}
