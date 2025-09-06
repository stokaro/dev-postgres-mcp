package integration_test

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/stokaro/dev-postgres-mcp/internal/docker"
)

// getBinaryName returns the appropriate binary name for the current OS
func getBinaryName() string {
	name := "dev-postgres-mcp-test"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	// Add path prefix for current directory
	return "./" + name
}

// buildTestBinary builds the CLI binary for testing
func buildTestBinary(c *qt.C) string {
	// Get the base name without path for building
	baseName := "dev-postgres-mcp-test"
	if runtime.GOOS == "windows" {
		baseName += ".exe"
	}

	buildCmd := exec.Command("go", "build", "-o", baseName, "../../cmd/dev-postgres-mcp")
	if err := buildCmd.Run(); err != nil {
		c.Skip("Failed to build CLI binary:", err)
	}

	// Return the path-prefixed name for execution
	return getBinaryName()
}

// cleanupTestBinary removes the test binary in a cross-platform way
func cleanupTestBinary(binaryName string) {
	// Remove the path prefix for cleanup
	baseName := strings.TrimPrefix(binaryName, "./")
	os.Remove(baseName) // Cross-platform file removal
}

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
	binaryName := buildTestBinary(c)
	defer cleanupTestBinary(binaryName)

	c.Run("version_command", func(c *qt.C) {
		cmd := exec.Command(binaryName, "version")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		c.Assert(string(output), qt.Contains, "dev-postgres-mcp")
	})

	c.Run("help_command", func(c *qt.C) {
		cmd := exec.Command(binaryName, "--help")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "dev-postgres-mcp")
		c.Assert(outputStr, qt.Contains, "mcp")
		c.Assert(outputStr, qt.Contains, "database")
	})

	c.Run("database_list_empty", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "list", "--start-port", "21000", "--end-port", "21010")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		c.Assert(string(output), qt.Contains, "No database instances are currently running")
	})

	c.Run("database_list_json_format", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "list", "--format", "json", "--start-port", "21000", "--end-port", "21010")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "count")
		c.Assert(outputStr, qt.Contains, "instances")
	})

	c.Run("database_drop_nonexistent", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "drop", "nonexistent-id", "--force", "--start-port", "21000", "--end-port", "21010")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		c.Assert(string(output), qt.Contains, "not found")
	})

	c.Run("mcp_serve_help", func(c *qt.C) {
		cmd := exec.Command(binaryName, "mcp", "serve", "--help")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "Start the Model Context Protocol server")
		c.Assert(outputStr, qt.Contains, "start-port")
		c.Assert(outputStr, qt.Contains, "end-port")
	})
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
	binaryName := buildTestBinary(c)
	defer cleanupTestBinary(binaryName)

	c.Run("invalid_format_flag", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "list", "--format", "invalid", "--start-port", "21100", "--end-port", "21110")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		c.Assert(string(output), qt.Contains, "unsupported format")
	})

	c.Run("invalid_port_range", func(c *qt.C) {
		// Test with start port higher than end port
		cmd := exec.Command(binaryName, "database", "list", "--start-port", "21110", "--end-port", "21100")
		output, err := cmd.CombinedOutput()
		// This should either fail or handle gracefully
		if err == nil {
			// If it doesn't fail, it should at least not crash
			c.Assert(string(output), qt.Not(qt.Contains), "panic")
		}
	})

	c.Run("database_drop_missing_args", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "drop")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		outputStr := string(output)
		// Should indicate missing argument
		c.Assert(outputStr, qt.Contains, "accepts 1 arg")
	})

	c.Run("database_drop_too_many_args", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "drop", "id1", "id2")
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
	binaryName := buildTestBinary(c)
	defer cleanupTestBinary(binaryName)

	c.Run("unknown_command", func(c *qt.C) {
		cmd := exec.Command(binaryName, "unknown")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "unknown command")
	})

	c.Run("unknown_subcommand", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "unknown")
		output, err := cmd.CombinedOutput()
		// Cobra shows help for unknown subcommands instead of erroring
		c.Assert(err, qt.IsNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "Available Commands")
	})

	c.Run("invalid_flag", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "list", "--invalid-flag")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNotNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "unknown flag")
	})

	// Test Docker unavailable scenario by using an invalid port range that would fail
	c.Run("docker_unavailable", func(c *qt.C) {
		// Use a port range that's likely to cause issues or be unavailable
		cmd := exec.Command(binaryName, "database", "list", "--start-port", "1", "--end-port", "2")
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
	binaryName := buildTestBinary(c)
	defer cleanupTestBinary(binaryName)

	c.Run("command_timeout", func(c *qt.C) {
		// Test that commands don't hang indefinitely
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binaryName, "database", "list", "--start-port", "22000", "--end-port", "22010")
		output, err := cmd.CombinedOutput()

		// Command should complete within timeout
		c.Assert(ctx.Err(), qt.IsNil, qt.Commentf("Command timed out"))

		// Even if Docker is not available, it should fail quickly, not hang
		if err != nil {
			outputStr := string(output)
			c.Assert(outputStr, qt.Not(qt.Equals), "")
		}
	})
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
	binaryName := buildTestBinary(c)
	defer cleanupTestBinary(binaryName)

	c.Run("table_format_structure", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "list", "--format", "table", "--start-port", "22100", "--end-port", "22110")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)

		if strings.Contains(outputStr, "No database instances") {
			// Empty case is fine
			return
		}

		// Should have table headers if there are instances
		c.Assert(outputStr, qt.Contains, "ID")
		c.Assert(outputStr, qt.Contains, "TYPE")
		c.Assert(outputStr, qt.Contains, "PORT")
	})

	c.Run("json_format_structure", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "list", "--format", "json", "--start-port", "22100", "--end-port", "22110")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)

		// Should be valid JSON with expected structure
		c.Assert(outputStr, qt.Contains, `"count"`)
		c.Assert(outputStr, qt.Contains, `"instances"`)
		c.Assert(outputStr, qt.Contains, `[`)
		c.Assert(outputStr, qt.Contains, `]`)
	})
}
