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
	"github.com/stokaro/dev-postgres-mcp/internal/database"
	"github.com/stokaro/dev-postgres-mcp/internal/docker"
	"github.com/stokaro/dev-postgres-mcp/internal/mcp"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(args ...string) {
	var rootCmd = &cobra.Command{
		Use:   "dev-postgres-mcp",
		Short: "MCP server for managing ephemeral database instances",
		Long: `dev-postgres-mcp is a Model Context Protocol (MCP) server that provides
tools for creating, managing, and accessing ephemeral database instances
running in Docker containers.

Supports PostgreSQL, MySQL, and MariaDB databases. Each instance is completely 
ephemeral and can be created and destroyed on demand, making it perfect for 
development, testing, and experimentation.

FEATURES:
  • Create ephemeral PostgreSQL, MySQL, and MariaDB instances in Docker containers
  • Dynamic port allocation to prevent conflicts
  • Support for multiple database versions
  • Superuser access with auto-generated credentials
  • MCP integration compatible with Augment Code and other MCP clients
  • CLI tools for instance management

EXAMPLES:
  # Start the MCP server
  dev-postgres-mcp mcp serve

  # List all running database instances
  dev-postgres-mcp database list

  # List only MySQL instances
  dev-postgres-mcp database list --type mysql

  # Get details of a specific instance
  dev-postgres-mcp database get <instance-id>

  # Drop a specific instance
  dev-postgres-mcp database drop <instance-id>

Use "dev-postgres-mcp [command] --help" for detailed information about each command.`,
		Args: cobra.NoArgs, // Disallow unknown subcommands
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	if len(args) > 0 {
		rootCmd.SetArgs(args)
	}

	// Add subcommands
	rootCmd.AddCommand(newMCPCommand())
	rootCmd.AddCommand(newDatabaseCommand())
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
		Long:  "Commands for running the Model Context Protocol server.",
	}

	cmd.AddCommand(newMCPServeCommand())

	return cmd
}

// newMCPServeCommand creates the mcp serve command.
func newMCPServeCommand() *cobra.Command {
	var startPort int
	var endPort int

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server",
		Long: `Start the Model Context Protocol server for database instance management.

This command starts the MCP server that provides tools for managing database
instances (PostgreSQL, MySQL, MariaDB). The server communicates using the Model 
Context Protocol over stdio, making it compatible with MCP clients like Augment Code.

The server provides the following unified tools:
  • create_database_instance - Create a new database instance
  • list_database_instances - List all running instances
  • get_database_instance - Get details of a specific instance
  • drop_database_instance - Remove a database instance
  • health_check_database - Check instance health

The server will run until interrupted (Ctrl+C) and will automatically clean up
all managed database instances on shutdown.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runMCPServe(startPort, endPort)
		},
	}

	cmd.Flags().IntVar(&startPort, "start-port", 15432, "Start of port range for database instances")
	cmd.Flags().IntVar(&endPort, "end-port", 25432, "End of port range for database instances")

	return cmd
}

// runMCPServe runs the MCP server.
func runMCPServe(startPort, endPort int) error {
	// Create MCP server
	config := mcp.ServerConfig{
		Name:      "dev-postgres-mcp",
		Version:   "1.0.0",
		StartPort: startPort,
		EndPort:   endPort,
		LogLevel:  "info",
	}
	server, err := mcp.NewServer(config)
	if err != nil {
		return fmt.Errorf("failed to create MCP server: %w", err)
	}
	defer server.Close()

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nReceived interrupt signal, shutting down...")
		cancel()
	}()

	// Start the server
	if err := server.Start(ctx); err != nil {
		if ctx.Err() != nil {
			// Context was cancelled, this is expected during shutdown
			return nil
		}
		// Server error
		return err
	}

	return nil
}

// newDatabaseCommand creates the database command group.
func newDatabaseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "database",
		Short:   "Database instance management commands",
		Long:    "Commands for managing database instances (PostgreSQL, MySQL, MariaDB) outside of the MCP protocol.",
		Aliases: []string{"db"},
	}

	cmd.AddCommand(newDatabaseListCommand())
	cmd.AddCommand(newDatabaseGetCommand())
	cmd.AddCommand(newDatabaseDropCommand())

	return cmd
}

// DropOptions holds options for dropping instances.
type DropOptions struct {
	Force bool
}

// Database command implementations

// newDatabaseListCommand creates the database list command.
func newDatabaseListCommand() *cobra.Command {
	var format string
	var startPort int
	var endPort int
	var dbType string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all running database instances",
		Long: `List all currently running database instances managed by this server.

This command shows details about each instance including:
  • Instance ID
  • Database Type (PostgreSQL, MySQL, MariaDB)
  • Container ID
  • Port number
  • Database name
  • Status
  • Creation time`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDatabaseList(format, startPort, endPort, dbType)
		},
	}

	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	cmd.Flags().IntVar(&startPort, "start-port", 15432, "Start of port range for database instances")
	cmd.Flags().IntVar(&endPort, "end-port", 25432, "End of port range for database instances")
	cmd.Flags().StringVar(&dbType, "type", "", "Filter by database type (postgresql, mysql, mariadb)")

	return cmd
}

// newDatabaseGetCommand creates the database get command.
func newDatabaseGetCommand() *cobra.Command {
	var startPort int
	var endPort int

	cmd := &cobra.Command{
		Use:   "get <instance-id>",
		Short: "Get details of a specific database instance",
		Long: `Get detailed information about a specific database instance by its ID.

This command shows comprehensive details about the instance including:
  • Instance ID and type
  • Container information
  • Connection details (DSN)
  • Current status
  • Resource allocation`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			instanceID := args[0]
			return runDatabaseGet(instanceID, startPort, endPort)
		},
	}

	cmd.Flags().IntVar(&startPort, "start-port", 15432, "Start of port range for database instances")
	cmd.Flags().IntVar(&endPort, "end-port", 25432, "End of port range for database instances")

	return cmd
}

// newDatabaseDropCommand creates the database drop command.
func newDatabaseDropCommand() *cobra.Command {
	var startPort int
	var endPort int
	var force bool

	cmd := &cobra.Command{
		Use:   "drop <instance-id>",
		Short: "Drop a database instance",
		Long: `Drop (remove) a specific database instance by its ID.

This command will:
  • Stop the database container
  • Remove the container and all associated data
  • Free up the allocated port
  • Clean up all resources

WARNING: This action is irreversible. All data in the instance will be lost.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			instanceID := args[0]
			return runDatabaseDrop(instanceID, startPort, endPort, DropOptions{Force: force})
		},
	}

	cmd.Flags().IntVar(&startPort, "start-port", 15432, "Start of port range for database instances")
	cmd.Flags().IntVar(&endPort, "end-port", 25432, "End of port range for database instances")
	cmd.Flags().BoolVar(&force, "force", false, "Force removal without confirmation")

	return cmd
}

// runDatabaseList lists all database instances.
func runDatabaseList(format string, startPort, endPort int, dbType string) error {
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

	// Create unified database manager
	unifiedManager := database.NewUnifiedManager(dockerMgr)

	// List instances
	var instances []*types.DatabaseInstance
	if dbType != "" {
		dbTypeEnum := types.DatabaseType(dbType)
		if !dbTypeEnum.IsValid() {
			return fmt.Errorf("invalid database type: %s", dbType)
		}
		instances, err = unifiedManager.ListInstancesByType(ctx, dbTypeEnum)
	} else {
		instances, err = unifiedManager.ListInstances(ctx)
	}

	if err != nil {
		return fmt.Errorf("failed to list instances: %w", err)
	}

	// Format output
	switch format {
	case "json":
		// Ensure instances is an empty array instead of null when empty
		if instances == nil {
			instances = []*types.DatabaseInstance{}
		}
		// Create JSON response with count and instances
		response := map[string]any{
			"count":     len(instances),
			"instances": instances,
		}
		output, err := json.MarshalIndent(response, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal instances to JSON: %w", err)
		}
		fmt.Println(string(output))
	case "table":
		if len(instances) == 0 {
			fmt.Println("No database instances are currently running.")
			return nil
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tTYPE\tPORT\tDATABASE\tSTATUS\tCREATED")
		for _, instance := range instances {
			fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\n",
				instance.ID,
				instance.Type,
				instance.Port,
				instance.Database,
				instance.Status,
				instance.CreatedAt.Format(time.RFC3339))
		}
		w.Flush()
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return nil
}

// runDatabaseGet gets details of a specific database instance.
func runDatabaseGet(instanceID string, startPort, endPort int) error {
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

	// Create unified database manager
	unifiedManager := database.NewUnifiedManager(dockerMgr)

	// Get instance
	instance, err := unifiedManager.GetInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	// Format output as JSON
	output, err := json.MarshalIndent(instance, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal instance to JSON: %w", err)
	}

	fmt.Println(string(output))
	return nil
}

// runDatabaseDrop drops a database instance.
func runDatabaseDrop(instanceID string, startPort, endPort int, opts DropOptions) error {
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

	// Create unified database manager
	unifiedManager := database.NewUnifiedManager(dockerMgr)

	// Get instance details first to verify it exists
	instance, err := unifiedManager.GetInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("instance %s not found: %w", instanceID, err)
	}

	// Confirm deletion unless force flag is set
	if !opts.Force {
		fmt.Printf("Are you sure you want to drop database instance %s (%s)? This action cannot be undone.\n", instance.ID, instance.Type)
		fmt.Printf("Instance details:\n")
		fmt.Printf("  Type: %s\n", instance.Type)
		fmt.Printf("  Port: %d\n", instance.Port)
		fmt.Printf("  Database: %s\n", instance.Database)
		fmt.Printf("  Status: %s\n", instance.Status)
		fmt.Printf("\nType 'yes' to confirm: ")

		var confirmation string
		if _, err := fmt.Scanln(&confirmation); err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		if confirmation != "yes" {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	// Drop the instance
	if err := unifiedManager.DropInstance(ctx, instanceID); err != nil {
		return fmt.Errorf("failed to drop instance: %w", err)
	}

	fmt.Printf("Database instance %s (%s) has been successfully dropped.\n", instance.ID, instance.Type)
	return nil
}
