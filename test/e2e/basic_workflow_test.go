package e2e_test

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/stokaro/dev-postgres-mcp/internal/docker"
)

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
	buildCmd := exec.Command("go", "build", "-o", "../../dev-postgres-mcp-e2e.exe", "../../cmd/dev-postgres-mcp")
	if err := buildCmd.Run(); err != nil {
		c.Skip("Failed to build CLI binary:", err)
	}

	c.Run("complete_workflow", func(c *qt.C) {
		// Step 1: Verify no instances are running initially
		cmd := exec.Command("../../dev-postgres-mcp-e2e.exe", "postgres", "list", "--format", "json")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)

		var result map[string]any
		err = json.Unmarshal(output, &result)
		c.Assert(err, qt.IsNil)
		c.Assert(result["count"], qt.Equals, float64(0))
		instances := result["instances"].([]any)
		c.Assert(len(instances), qt.Equals, 0)

		// Step 2: Test version command
		cmd = exec.Command("../../dev-postgres-mcp-e2e.exe", "version")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		c.Assert(string(output), qt.Contains, "dev-postgres-mcp")
		c.Assert(string(output), qt.Contains, "Go version:")

		// Step 3: Test help command
		cmd = exec.Command("../../dev-postgres-mcp-e2e.exe", "--help")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "dev-postgres-mcp")
		c.Assert(outputStr, qt.Contains, "mcp")
		c.Assert(outputStr, qt.Contains, "postgres")

		// Step 4: Test MCP serve help
		cmd = exec.Command("../../dev-postgres-mcp-e2e.exe", "mcp", "serve", "--help")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		c.Assert(string(output), qt.Contains, "Start the MCP server")

		// Step 5: Test postgres commands help
		cmd = exec.Command("../../dev-postgres-mcp-e2e.exe", "postgres", "--help")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		c.Assert(string(output), qt.Contains, "managing PostgreSQL instances")

		// Step 6: Test invalid command
		cmd = exec.Command("../../dev-postgres-mcp-e2e.exe", "invalid-command")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(string(output), qt.Contains, "unknown command")

		// Step 7: Test invalid flag
		cmd = exec.Command("../../dev-postgres-mcp-e2e.exe", "postgres", "list", "--invalid-flag")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(string(output), qt.Contains, "unknown flag")

		// Step 8: Test drop non-existent instance
		cmd = exec.Command("../../dev-postgres-mcp-e2e.exe", "postgres", "drop", "nonexistent-id")
		output, err = cmd.CombinedOutput()
		c.Assert(err, qt.Not(qt.IsNil))
		c.Assert(string(output), qt.Contains, "not found")
	})

	c.Run("mcp_server_startup", func(c *qt.C) {
		// Test that MCP server can start (but don't wait for it to run)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "../../dev-postgres-mcp-e2e.exe", "mcp", "serve")
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
	buildCmd := exec.Command("go", "build", "-o", "../../dev-postgres-mcp-e2e.exe", "../../cmd/dev-postgres-mcp")
	if err := buildCmd.Run(); err != nil {
		c.Skip("Failed to build CLI binary:", err)
	}

	c.Run("missing_arguments", func(c *qt.C) {
		cmd := exec.Command("../../dev-postgres-mcp-e2e.exe", "postgres", "drop")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.Not(qt.IsNil))
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "accepts 1 arg")
	})

	c.Run("too_many_arguments", func(c *qt.C) {
		cmd := exec.Command("../../dev-postgres-mcp-e2e.exe", "postgres", "drop", "id1", "id2")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.Not(qt.IsNil))
		outputStr := string(output)
		c.Assert(outputStr, qt.Contains, "accepts 1 arg")
	})

	c.Run("invalid_format", func(c *qt.C) {
		cmd := exec.Command("../../dev-postgres-mcp-e2e.exe", "postgres", "list", "--format", "invalid")
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
	buildCmd := exec.Command("go", "build", "-o", "../../dev-postgres-mcp-e2e.exe", "../../cmd/dev-postgres-mcp")
	if err := buildCmd.Run(); err != nil {
		c.Skip("Failed to build CLI binary:", err)
	}

	c.Run("table_format", func(c *qt.C) {
		cmd := exec.Command("../../dev-postgres-mcp-e2e.exe", "postgres", "list", "--format", "table")
		output, err := cmd.CombinedOutput()
		c.Assert(err, qt.IsNil)
		outputStr := string(output)
		// Should contain the message about no instances or table headers
		c.Assert(len(strings.TrimSpace(outputStr)) > 0, qt.IsTrue)
	})

	c.Run("json_format", func(c *qt.C) {
		cmd := exec.Command("../../dev-postgres-mcp-e2e.exe", "postgres", "list", "--format", "json")
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
