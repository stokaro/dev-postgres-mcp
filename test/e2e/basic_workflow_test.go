package e2e_test

import (
	"context"
	"encoding/json"
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
	name := "dev-postgres-mcp-e2e"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	// Add path prefix for current directory
	return "../../" + name
}

// buildTestBinary builds the CLI binary for testing
func buildTestBinary(c *qt.C) string {
	// Get the base name without path for building
	baseName := "dev-postgres-mcp-e2e"
	if runtime.GOOS == "windows" {
		baseName += ".exe"
	}

	buildCmd := exec.Command("go", "build", "-o", "../../"+baseName, "../../cmd/dev-postgres-mcp")
	if err := buildCmd.Run(); err != nil {
		c.Skip("Failed to build CLI binary:", err)
	}

	// Return the path-prefixed name for execution
	return getBinaryName()
}

// cleanupTestBinary removes the test binary in a cross-platform way
func cleanupTestBinary(binaryName string) {
	// Remove the path prefix for cleanup
	baseName := strings.TrimPrefix(binaryName, "../../")
	os.Remove("../../" + baseName) // Cross-platform file removal
}

// TestBasicWorkflow tests the complete end-to-end workflow of the application
func TestBasicWorkflow(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(24000, 24010)
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

	c.Run("complete_workflow", func(c *qt.C) {
		// Step 1: Verify no instances are running initially
		cmd := exec.Command(binaryName, "database", "list", "--format", "json")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)

		var result map[string]any
		err = json.Unmarshal(output, &result)
		c.Assert(err, qt.IsNil)
		c.Assert(result["count"], qt.Equals, float64(0))
		instances := result["instances"].([]any)
		c.Assert(len(instances), qt.Equals, 0)

		// Step 2: Test version command
		cmd = exec.Command(binaryName, "version")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		c.Assert(string(output), qt.Contains, "dev-postgres-mcp")
		c.Assert(string(output), qt.Contains, "Go version:")

		// Step 3: Test help command
		cmd = exec.Command(binaryName, "--help")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "dev-postgres-mcp")
		c.Assert(outputStr, qt.Contains, "mcp")
		c.Assert(outputStr, qt.Contains, "database")

		// Step 4: Test MCP serve help
		cmd = exec.Command(binaryName, "mcp", "serve", "--help")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		c.Assert(string(output), qt.Contains, "Start the Model Context Protocol server")

		// Step 5: Test database commands help
		cmd = exec.Command(binaryName, "database", "--help")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		c.Assert(string(output), qt.Contains, "Commands for managing database instances")

		// Step 6: Test invalid command
		cmd = exec.Command(binaryName, "invalid-command")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(string(output), qt.Contains, "unknown command")

		// Step 7: Test invalid flag
		cmd = exec.Command(binaryName, "database", "list", "--invalid-flag")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(string(output), qt.Contains, "unknown flag")

		// Step 8: Test drop non-existent instance
		cmd = exec.Command(binaryName, "database", "drop", "nonexistent-id")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(string(output), qt.Contains, "not found")
	})

	c.Run("mcp_server_startup", func(c *qt.C) {
		// Test that MCP server can start (but don't wait for it to run)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binaryName, "mcp", "serve")
		err := cmd.Start()
		c.Assert(err, qt.IsNil)

		// Give it a moment to start
		time.Sleep(100 * time.Millisecond)

		// Kill the process
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})
}

// TestCLIErrorHandling tests various error conditions
func TestCLIErrorHandling(t *testing.T) {
	c := qt.New(t)

	// Build the CLI binary first
	binaryName := buildTestBinary(c)
	defer cleanupTestBinary(binaryName)

	c.Run("missing_arguments", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "drop")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.Not(qt.IsNil))
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "accepts 1 arg")
	})

	c.Run("too_many_arguments", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "drop", "id1", "id2")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.Not(qt.IsNil))
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "accepts 1 arg")
	})

	c.Run("invalid_format", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "list", "--format", "invalid")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.Not(qt.IsNil))
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "unsupported format")
	})
}

// TestOutputFormats tests different output formats
func TestOutputFormats(t *testing.T) {
	c := qt.New(t)

	// Skip if Docker is not available
	dockerMgr, err := docker.NewManager(24100, 24110)
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

	c.Run("table_format", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "list", "--format", "table")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)
		// Should contain the message about no instances or table headers
		c.Assert(len(strings.TrimSpace(outputStr)) > 0, qt.IsTrue)
	})

	c.Run("json_format", func(c *qt.C) {
		cmd := exec.Command(binaryName, "database", "list", "--format", "json")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)

		// Should be valid JSON
		var result map[string]any
		err = json.Unmarshal(output, &result)
		c.Assert(err, qt.IsNil)
		c.Assert(result["count"], qt.Equals, float64(0))
		instances := result["instances"].([]any)
		c.Assert(len(instances), qt.Equals, 0) // No instances running
	})
}
