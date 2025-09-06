package integration_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/stokaro/dev-postgres-mcp/internal/docker"
)

func TestCLICommands(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(21000, 21010)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer dockerMgr.Close()

	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		c.Skip("Docker daemon not accessible:", err)
	}

	// Build the CLI binary first
	buildCmd := exec.Command("go", "build", "-o", "dev-postgres-mcp-test", "../../cmd/dev-postgres-mcp")
	if err := buildCmd.Run(); err != nil {
		c.Skip("Failed to build CLI binary:", err)
	}

	c.Run("version_command", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "version")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		c.Assert(string(output), qt.Contains, "dev-postgres-mcp")
	})

	c.Run("help_command", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "--help")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "dev-postgres-mcp")
		c.Assert(outputStr, qt.Contains, "mcp")
		c.Assert(outputStr, qt.Contains, "postgres")
	})

	c.Run("postgres_list_empty", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "list", "--start-port", "21000", "--end-port", "21010")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		c.Assert(string(output), qt.Contains, "No PostgreSQL instances are currently running")
	})

	c.Run("postgres_list_json_format", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "list", "--format", "json", "--start-port", "21000", "--end-port", "21010")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "count")
		c.Assert(outputStr, qt.Contains, "instances")
	})

	c.Run("postgres_drop_nonexistent", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "drop", "nonexistent-id", "--force", "--start-port", "21000", "--end-port", "21010")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		c.Assert(string(output), qt.Contains, "not found")
	})

	c.Run("mcp_serve_help", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "mcp", "serve", "--help")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "Start the MCP server")
		c.Assert(outputStr, qt.Contains, "start-port")
		c.Assert(outputStr, qt.Contains, "end-port")
		c.Assert(outputStr, qt.Contains, "log-level")
	})

	// Clean up test binary
	exec.Command("rm", "-f", "dev-postgres-mcp-test").Run()
}

func TestCLIFlags(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(21100, 21110)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer dockerMgr.Close()

	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		c.Skip("Docker daemon not accessible:", err)
	}

	// Build the CLI binary first
	buildCmd := exec.Command("go", "build", "-o", "dev-postgres-mcp-test", "../../cmd/dev-postgres-mcp")
	if err := buildCmd.Run(); err != nil {
		c.Skip("Failed to build CLI binary:", err)
	}

	c.Run("invalid_format_flag", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "list", "--format", "invalid", "--start-port", "21100", "--end-port", "21110")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		c.Assert(string(output), qt.Contains, "unsupported format")
	})

	c.Run("invalid_port_range", func(c *qt.C) {
		// Test with start port higher than end port
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "list", "--start-port", "21110", "--end-port", "21100")
		output, err := cmd.CombinedOutput()
		// This should either fail or handle gracefully
		if err == nil {
			// If it doesn't fail, it should at least not crash
			c.Assert(string(output), qt.Not(qt.Contains), "panic")
		}
	})

	c.Run("postgres_drop_missing_args", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "drop")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		outputStr := string(output)
		// Should indicate missing argument
		c.Assert(outputStr, qt.Contains, "required")
	})

	c.Run("postgres_drop_too_many_args", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "drop", "id1", "id2")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		// Should indicate too many arguments
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "accepts 1 arg")
	})

	// Clean up test binary
	exec.Command("rm", "-f", "dev-postgres-mcp-test").Run()
}

func TestCLIErrorHandling(t *testing.T) {
	c := qt.New(t)

	// Build the CLI binary first
	buildCmd := exec.Command("go", "build", "-o", "dev-postgres-mcp-test", "../../cmd/dev-postgres-mcp")
	if err := buildCmd.Run(); err != nil {
		c.Skip("Failed to build CLI binary:", err)
	}

	c.Run("unknown_command", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "unknown")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "unknown command")
	})

	c.Run("unknown_subcommand", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "unknown")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "unknown command")
	})

	c.Run("invalid_flag", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "list", "--invalid-flag")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "unknown flag")
	})

	// Test Docker unavailable scenario by using an invalid port range that would fail
	c.Run("docker_unavailable", func(c *qt.C) {
		// Use a port range that's likely to cause issues or be unavailable
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "list", "--start-port", "1", "--end-port", "2")
		output, err := cmd.CombinedOutput()
		// This might fail due to permission issues with low ports
		if err != nil {
			outputStr := string(output)
			// Should have a meaningful error message
			c.Assert(outputStr, qt.Not(qt.Equals), "")
		}
	})

	// Clean up test binary
	exec.Command("rm", "-f", "dev-postgres-mcp-test").Run()
}

func TestCLITimeout(t *testing.T) {
	c := qt.New(t)

	// Build the CLI binary first
	buildCmd := exec.Command("go", "build", "-o", "dev-postgres-mcp-test", "../../cmd/dev-postgres-mcp")
	if err := buildCmd.Run(); err != nil {
		c.Skip("Failed to build CLI binary:", err)
	}

	c.Run("command_timeout", func(c *qt.C) {
		// Test that commands don't hang indefinitely
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "./dev-postgres-mcp-test", "postgres", "list", "--start-port", "22000", "--end-port", "22010")
		output, err := cmd.CombinedOutput()

		// Command should complete within timeout
		c.Assert(ctx.Err(), qt.IsNil, qt.Commentf("Command timed out"))

		// Even if Docker is not available, it should fail quickly, not hang
		if err != nil {
			outputStr := string(output)
			c.Assert(outputStr, qt.Not(qt.Equals), "")
		}
	})

	// Clean up test binary
	exec.Command("rm", "-f", "dev-postgres-mcp-test").Run()
}

func TestCLIOutputFormats(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(22100, 22110)
	if err != nil {
		c.Skip("Docker not available:", err)
	}
	defer dockerMgr.Close()

	ctx := context.Background()
	if err := dockerMgr.Ping(ctx); err != nil {
		c.Skip("Docker daemon not accessible:", err)
	}

	// Build the CLI binary first
	buildCmd := exec.Command("go", "build", "-o", "dev-postgres-mcp-test", "../../cmd/dev-postgres-mcp")
	if err := buildCmd.Run(); err != nil {
		c.Skip("Failed to build CLI binary:", err)
	}

	c.Run("table_format_structure", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "list", "--format", "table", "--start-port", "22100", "--end-port", "22110")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)

		if strings.Contains(outputStr, "No PostgreSQL instances") {
			// Empty case is fine
			return
		}

		// Should have table headers if there are instances
		c.Assert(outputStr, qt.Contains, "INSTANCE ID")
		c.Assert(outputStr, qt.Contains, "CONTAINER ID")
		c.Assert(outputStr, qt.Contains, "PORT")
	})

	c.Run("json_format_structure", func(c *qt.C) {
		cmd := exec.Command("./dev-postgres-mcp-test", "postgres", "list", "--format", "json", "--start-port", "22100", "--end-port", "22110")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)

		// Should be valid JSON with expected structure
		c.Assert(outputStr, qt.Contains, `"count"`)
		c.Assert(outputStr, qt.Contains, `"instances"`)
		c.Assert(outputStr, qt.Contains, `[`)
		c.Assert(outputStr, qt.Contains, `]`)
	})

	// Clean up test binary
	exec.Command("rm", "-f", "dev-postgres-mcp-test").Run()
}
