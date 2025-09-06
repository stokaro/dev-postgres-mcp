# dev-postgres-mcp

A Model Context Protocol (MCP) server that provides tools for creating, managing, and accessing ephemeral database instances running in Docker containers. Supports PostgreSQL, MySQL, and MariaDB.

## Features

- **Multi-Database Support**: Create temporary PostgreSQL, MySQL, and MariaDB instances in Docker containers
- **Dynamic Port Allocation**: Automatic port assignment to prevent conflicts (range: 15432-25432)
- **Multiple Database Versions**: Support for various versions of each database type
  - PostgreSQL: 15, 16, 17 (default: 17)
  - MySQL: 5.7, 8.0 (default: 8.0)
  - MariaDB: 10.6, 11 (default: 11)
- **Superuser Access**: Auto-generated credentials with full database access
- **MCP Integration**: Compatible with Augment Code and other MCP clients
- **CLI Management**: Command-line tools for instance management outside of MCP
- **Health Monitoring**: Built-in health checks for all database types
- **Comprehensive Logging**: Structured logging with configurable levels and formats

## Quick Start

### Prerequisites

- Go 1.24 or later
- Docker Desktop or Docker Engine
- Windows, macOS, or Linux

### Installation

```bash
# Clone the repository
git clone https://github.com/stokaro/dev-postgres-mcp.git
cd dev-postgres-mcp

# Build the application
go build -o dev-postgres-mcp ./cmd/dev-postgres-mcp

# Or install directly
go install ./cmd/dev-postgres-mcp
```

### Basic Usage

#### Start the MCP Server

```bash
# Start MCP server with default settings
dev-postgres-mcp mcp serve

# Start with custom port range
dev-postgres-mcp mcp serve --start-port 20000 --end-port 30000

# Start with debug logging
dev-postgres-mcp mcp serve --log-level debug
```

#### CLI Commands

```bash
# List all running database instances
dev-postgres-mcp database list

# List instances of a specific type
dev-postgres-mcp database list --type postgresql
dev-postgres-mcp database list --type mysql
dev-postgres-mcp database list --type mariadb

# List instances in JSON format
dev-postgres-mcp database list --format json

# Get details of a specific instance
dev-postgres-mcp database get <instance-id>

# Drop a specific instance
dev-postgres-mcp database drop <instance-id>

# Force drop without confirmation
dev-postgres-mcp database drop <instance-id> --force

# Show version information
dev-postgres-mcp version

# Show help
dev-postgres-mcp --help
```

## MCP Tools

The server provides the following MCP tools:

#### `create_database_instance`

Creates a new ephemeral database instance.

**Parameters:**
- `type` (optional): Database type - postgresql, mysql, mariadb (default: postgresql)
- `version` (optional): Database version - defaults vary by type
- `database` (optional): Database name - defaults vary by type
- `username` (optional): Database username - defaults vary by type
- `password` (optional): Database password - auto-generated if not provided

**Returns:**
- Instance ID (without dashes)
- Database type
- Connection DSN
- Port number
- Database details

#### `list_database_instances`

Lists all running database instances.

**Parameters:**
- `type` (optional): Filter by database type - postgresql, mysql, mariadb

**Returns:**
- Array of instance details including ID, type, port, database, version, status, and creation time

#### `get_database_instance`

Gets details of a specific database instance.

**Parameters:**
- `instance_id` (required): The instance ID to retrieve

**Returns:**
- Complete instance details including connection information

#### `drop_database_instance`

Removes a database instance and cleans up resources.

**Parameters:**
- `instance_id` (required): The instance ID to remove

**Returns:**
- Confirmation of removal

#### `health_check_database`

Performs a health check on a database instance.

**Parameters:**
- `instance_id` (required): The instance ID to check

**Returns:**
- Health status and diagnostic information

## Configuration

### Environment Variables

- `DEV_POSTGRES_MCP_LOG_LEVEL`: Log level (debug, info, warn, error) - default: info
- `DEV_POSTGRES_MCP_LOG_FORMAT`: Log format (text, json) - default: text

### Command-Line Flags

#### MCP Serve Command

- `--start-port`: Start of port range for PostgreSQL instances (default: 15432)
- `--end-port`: End of port range for PostgreSQL instances (default: 25432)
- `--log-level`: Log level override (debug, info, warn, error)

#### Postgres Commands

- `--format`: Output format for list command (table, json) - default: table
- `--force`: Force operations without confirmation prompts

## Docker Requirements

The application requires Docker to be running and accessible. It will:

1. Pull PostgreSQL images as needed (postgres:15, postgres:16, postgres:17)
2. Create containers with proper networking and resource limits
3. Manage container lifecycle (start, stop, remove)
4. Monitor container health status

### Container Configuration

Each PostgreSQL instance runs in a Docker container with:

- **Image**: Official PostgreSQL images from Docker Hub
- **Port Binding**: Dynamic allocation from configured range
- **Environment**: Configured with database, username, and password
- **Health Check**: Built-in PostgreSQL health monitoring
- **Resource Limits**: Reasonable defaults for development use
- **Auto-removal**: Containers are removed when instances are dropped

## Development

### Project Structure

```
dev-postgres-mcp/
├── cmd/
│   ├── common/version/          # Version information
│   └── dev-postgres-mcp/        # Main application entry point
├── internal/
│   ├── docker/                  # Docker client and container management
│   ├── mcp/                     # MCP server implementation
│   └── postgres/                # PostgreSQL instance management
├── pkg/
│   └── types/                   # Shared type definitions
├── test/
│   ├── unit/                    # Unit tests
│   ├── integration/             # Integration tests
│   └── e2e/                     # End-to-end tests
├── .github/workflows/           # CI/CD workflows
├── .golangci.yml               # Linting configuration
└── go.mod                      # Go module definition
```

### Building from Source

```bash
# Install dependencies
go mod download

# Run tests
go test ./...

# Run linting
golangci-lint run

# Build for current platform
go build -o dev-postgres-mcp ./cmd/dev-postgres-mcp

# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o dev-postgres-mcp-linux ./cmd/dev-postgres-mcp
GOOS=darwin GOARCH=amd64 go build -o dev-postgres-mcp-darwin ./cmd/dev-postgres-mcp
GOOS=windows GOARCH=amd64 go build -o dev-postgres-mcp.exe ./cmd/dev-postgres-mcp
```

### Testing

```bash
# Run unit tests
go test ./test/unit/... -v

# Run integration tests (requires Docker)
go test ./test/integration/... -v

# Run end-to-end tests
go test ./test/e2e/... -v

# Run all tests with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Integration with MCP Clients

### Augment Code

This server is designed to work seamlessly with Augment Code. To integrate:

1. Start the MCP server: `dev-postgres-mcp mcp serve`
2. Configure Augment Code to use the server via stdio transport
3. Use the provided tools to create and manage PostgreSQL instances

### Other MCP Clients

The server implements the standard MCP protocol and should work with any compliant MCP client that supports stdio transport.

## Troubleshooting

### Common Issues

**Docker not available**
- Ensure Docker Desktop is running
- Check Docker daemon accessibility: `docker ps`
- Verify Docker permissions for your user

**Port conflicts**
- Adjust port range using `--start-port` and `--end-port` flags
- Check for other services using the same ports: `netstat -an | grep :15432`

**Container startup failures**
- Check Docker logs: `docker logs <container-id>`
- Verify sufficient system resources (memory, disk space)
- Ensure PostgreSQL images can be pulled: `docker pull postgres:17`

**Permission issues**
- Ensure your user has Docker permissions
- On Linux, add user to docker group: `sudo usermod -aG docker $USER`

### Logging

Enable debug logging for detailed troubleshooting:

```bash
# Environment variable
export DEV_POSTGRES_MCP_LOG_LEVEL=debug
dev-postgres-mcp mcp serve

# Command-line flag
dev-postgres-mcp mcp serve --log-level debug

# JSON format for structured logs
export DEV_POSTGRES_MCP_LOG_FORMAT=json
dev-postgres-mcp mcp serve
```

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feature-name`
3. Make your changes and add tests
4. Run the test suite: `go test ./...`
5. Run linting: `golangci-lint run`
6. Commit your changes: `git commit -am 'Add feature'`
7. Push to the branch: `git push origin feature-name`
8. Create a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) - MCP protocol implementation
- [spf13/cobra](https://github.com/spf13/cobra) - CLI framework
- [docker/docker](https://github.com/docker/docker) - Docker client library
- [testcontainers/testcontainers-go](https://github.com/testcontainers/testcontainers-go) - Integration testing
- [frankban/quicktest](https://github.com/frankban/quicktest) - Testing framework
