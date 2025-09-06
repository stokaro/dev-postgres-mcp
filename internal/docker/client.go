// Package docker provides Docker client functionality for managing containers.
package docker

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

// Client wraps the Docker client with additional functionality.
type Client struct {
	cli *client.Client
}

// NewClient creates a new Docker client wrapper.
func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	return &Client{cli: cli}, nil
}

// Close closes the Docker client connection.
func (c *Client) Close() error {
	if c.cli != nil {
		return c.cli.Close()
	}
	return nil
}

// Ping checks if the Docker daemon is accessible.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf("failed to ping Docker daemon: %w", err)
	}
	return nil
}

// PullImage pulls a Docker image if it doesn't exist locally.
func (c *Client) PullImage(ctx context.Context, imageName string) error {
	slog.Info("Checking if image exists locally", "image", imageName)

	// Check if image exists locally
	_, err := c.cli.ImageInspect(ctx, imageName)
	if err == nil {
		slog.Info("Image already exists locally", "image", imageName)
		return nil
	}

	slog.Info("Pulling image", "image", imageName)
	reader, err := c.cli.ImagePull(ctx, imageName, imagetypes.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	// Read the response to ensure the pull completes
	// In a real implementation, you might want to parse the JSON response
	// and show progress, but for now we'll just read it all
	buf := make([]byte, 1024)
	for {
		_, err := reader.Read(buf)
		if err != nil {
			break
		}
	}

	slog.Info("Image pulled successfully", "image", imageName)
	return nil
}

// CreateContainer creates a new container with the given configuration.
func (c *Client) CreateContainer(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, containerName string) (string, error) {
	slog.Info("Creating container", "name", containerName, "image", config.Image)

	resp, err := c.cli.ContainerCreate(ctx, config, hostConfig, nil, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("failed to create container %s: %w", containerName, err)
	}

	if len(resp.Warnings) > 0 {
		for _, warning := range resp.Warnings {
			slog.Warn("Container creation warning", "warning", warning)
		}
	}

	slog.Info("Container created successfully", "id", resp.ID, "name", containerName)
	return resp.ID, nil
}

// StartContainer starts a container by ID.
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	slog.Info("Starting container", "id", containerID)

	err := c.cli.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return fmt.Errorf("failed to start container %s: %w", containerID, err)
	}

	slog.Info("Container started successfully", "id", containerID)
	return nil
}

// StopContainer stops a container by ID.
func (c *Client) StopContainer(ctx context.Context, containerID string) error {
	slog.Info("Stopping container", "id", containerID)

	timeout := 10 // seconds
	err := c.cli.ContainerStop(ctx, containerID, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		return fmt.Errorf("failed to stop container %s: %w", containerID, err)
	}

	slog.Info("Container stopped successfully", "id", containerID)
	return nil
}

// RemoveContainer removes a container by ID.
func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	slog.Info("Removing container", "id", containerID)

	err := c.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true, // Force removal even if running
	})
	if err != nil {
		return fmt.Errorf("failed to remove container %s: %w", containerID, err)
	}

	slog.Info("Container removed successfully", "id", containerID)
	return nil
}

// InspectContainer returns detailed information about a container.
func (c *Client) InspectContainer(ctx context.Context, containerID string) (container.InspectResponse, error) {
	inspect, err := c.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return container.InspectResponse{}, fmt.Errorf("failed to inspect container %s: %w", containerID, err)
	}

	return inspect, nil
}

// ListContainers lists containers with optional filters.
func (c *Client) ListContainers(ctx context.Context, options container.ListOptions) ([]container.Summary, error) {
	containers, err := c.cli.ContainerList(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	return containers, nil
}

// ContainerLogs retrieves logs from a container.
func (c *Client) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (string, error) {
	reader, err := c.cli.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", fmt.Errorf("failed to get logs for container %s: %w", containerID, err)
	}
	defer reader.Close()

	// Read all logs
	buf := make([]byte, 4096)
	var logs string
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			logs += string(buf[:n])
		}
		if err != nil {
			break
		}
	}

	return logs, nil
}

// IsContainerRunning checks if a container is currently running.
func (c *Client) IsContainerRunning(ctx context.Context, containerID string) (bool, error) {
	inspect, err := c.InspectContainer(ctx, containerID)
	if err != nil {
		return false, err
	}

	return inspect.State.Running, nil
}
