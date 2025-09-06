# MCP PostgreSQL Server Implementation Plan

## Project Overview

This document outlines the implementation plan for creating a Go-based MCP (Model Context Protocol) Server that manages ephemeral PostgreSQL instances in Docker containers. The project will follow the patterns established in the inventario reference project and use the mark3labs/mcp-go library.

## Project Structure

```
dev-postgres-mcp/
├── .github/
│   ├── workflows/
│   │   ├── ci.yml
│   │   ├── release.yml
│   │   └── test.yml
│   └── copilot-instructions.md
├── cmd/
│   ├── dev-postgres-mcp/
│   │   ├── main.go
│   │   ├── root.go
│   │   ├── mcp/
│   │   │   └── serve.go
│   │   └── postgres/
│   │       ├── list.go
│   │       └── drop.go
│   └── common/
│       └── version/
│           └── version.go
├── internal/
│   ├── docker/
│   │   ├── client.go
│   │   ├── postgres.go
│   │   └── manager.go
│   ├── mcp/
│   │   ├── server.go
│   │   ├── tools.go
│   │   └── handlers.go
│   └── postgres/
│       ├── instance.go
│       ├── manager.go
│       └── dsn.go
├── pkg/
│   └── types/
│       └── instance.go
├── test/
│   ├── integration/
│   │   └── docker_test.go
│   └── unit/
│       └── postgres_test.go
├── .gitignore
├── .golangci.yml
├── CLAUDE.md
├── go.mod
├── go.sum
├── LICENSE
├── Makefile
└── README.md
```

## Core Components Design

### 1. PostgreSQL Instance Management

**Instance Structure:**
```go
type PostgreSQLInstance struct {
    ID          string    `json:"id"`
    ContainerID string    `json:"container_id"`
    Port        int       `json:"port"`
    Database    string    `json:"database"`
    Username    string    `json:"username"`
    Password    string    `json:"password"`
    Version     string    `json:"version"`
    DSN         string    `json:"dsn"`
    CreatedAt   time.Time `json:"created_at"`
    Status      string    `json:"status"`
}
```

**Manager Interface:**
```go
type PostgreSQLManager interface {
    CreateInstance(ctx context.Context, opts CreateOptions) (*PostgreSQLInstance, error)
    ListInstances(ctx context.Context) ([]*PostgreSQLInstance, error)
    GetInstance(ctx context.Context, id string) (*PostgreSQLInstance, error)
    DropInstance(ctx context.Context, id string) error
    HealthCheck(ctx context.Context, id string) error
}
```

### 2. Docker Container Management

**Docker Client Wrapper:**
- Manage PostgreSQL container lifecycle
- Handle port allocation (dynamic port assignment)
- Container health monitoring
- Cleanup and resource management

**Key Features:**
- Automatic port discovery to prevent conflicts
- Container naming convention: `mcp-postgres-{instance-id}`
- Volume management for ephemeral storage
- Network isolation

### 3. MCP Server Implementation

**Tools to Implement:**
1. `create_postgres_instance` - Create new PostgreSQL instance
2. `list_postgres_instances` - List all running instances
3. `get_postgres_instance` - Get details of specific instance
4. `drop_postgres_instance` - Remove PostgreSQL instance
5. `health_check_postgres` - Check instance health

**Tool Schemas:**
```go
// create_postgres_instance
{
    "version": "17",      // optional, default "17"
    "database": "mydb",   // optional, default "postgres"
    "username": "user",   // optional, default "postgres"
    "password": "pass"    // optional, auto-generated if not provided
}

// drop_postgres_instance
{
    "id": "instance-uuid"
}

// get_postgres_instance
{
    "id": "instance-uuid"
}
```

### 4. CLI Command Structure

**Root Command:**
```
dev-postgres-mcp [command]
```

**Subcommands:**
1. `mcp serve` - Start MCP server (stdio transport)
2. `postgres list` - List running PostgreSQL instances
3. `postgres drop <id>` - Drop specific instance
4. `version` - Show version information

**Command Implementation Pattern:**
Following inventario's pattern with separate command files and shared utilities.

## Implementation Steps

### Phase 1: Project Setup and Foundation
1. Initialize Go module with proper dependencies
2. Set up project structure following inventario patterns
3. Configure linting (.golangci.yml) based on inventario config
4. Set up CI/CD workflows
5. Create basic CLI structure with Cobra

### Phase 2: Docker Integration
1. Implement Docker client wrapper
2. Create PostgreSQL container management
3. Implement port allocation strategy
4. Add container health checking
5. Write unit tests for Docker operations

### Phase 3: PostgreSQL Instance Management
1. Implement PostgreSQL instance data structures
2. Create instance manager with CRUD operations
3. Implement DSN generation
4. Add instance lifecycle management
5. Write comprehensive unit tests

### Phase 4: MCP Server Implementation
1. Integrate mark3labs/mcp-go library
2. Implement MCP tools and handlers
3. Create tool validation and error handling
4. Add proper logging and monitoring
5. Write integration tests

### Phase 5: CLI Commands
1. Implement `mcp serve` command
2. Create `postgres list` command
3. Implement `postgres drop` command
4. Add proper flag handling and validation
5. Write CLI integration tests

### Phase 6: Testing and Documentation
1. Write comprehensive unit tests (target >90% coverage)
2. Create integration tests with real Docker containers
3. Add end-to-end testing scenarios
4. Write detailed README and usage documentation
5. Create example usage scenarios

## Technical Specifications

### Dependencies
```go
// Core dependencies
github.com/mark3labs/mcp-go v0.39.1
github.com/spf13/cobra v1.8.0
github.com/docker/docker v24.0.0+incompatible
github.com/google/uuid v1.6.0

// Testing
github.com/frankban/quicktest v1.14.6
github.com/testcontainers/testcontainers-go v0.26.0
```

### Configuration
- Environment variable prefix: `DEV_POSTGRES_MCP_`
- Default PostgreSQL version: `postgres:17`
- Port range for allocation: 15432-25432
- Container resource limits: 512MB RAM, 1 CPU core

### Error Handling Strategy
1. Structured error types for different failure modes
2. Proper error wrapping with context
3. Graceful degradation for Docker connectivity issues
4. Comprehensive logging for debugging

### Security Considerations
1. Auto-generated secure passwords
2. Container network isolation
3. Resource limits to prevent abuse
4. Proper cleanup of sensitive data

## Testing Strategy

### Unit Tests
- Docker client operations
- PostgreSQL instance management
- MCP tool handlers
- CLI command logic
- Error handling scenarios

### Integration Tests
- Real Docker container operations
- MCP server communication
- End-to-end instance lifecycle
- CLI command execution

### Test Coverage Goals
- Minimum 90% code coverage
- All error paths tested
- Performance benchmarks for critical operations
- Memory leak detection

## Compatibility Requirements

### MCP Protocol
- Compatible with Augment Code's MCP client
- Follows MCP specification for tool definitions
- Proper error response formatting
- Support for stdio transport

### Docker Requirements
- Docker Engine 20.10+
- Support for Linux containers
- Network bridge capability
- Volume management support

### Go Version
- Go 1.24+ (following inventario pattern)
- Module-aware build system
- Cross-platform compatibility (Windows, macOS, Linux)

## Deployment Considerations

### Binary Distribution
- Single binary with no external dependencies (except Docker)
- Cross-platform builds via GitHub Actions
- Semantic versioning following inventario pattern

### Docker Requirements
- Clear documentation for Docker setup
- Error messages for missing Docker daemon
- Graceful handling of Docker permission issues

### Resource Management
- Automatic cleanup of orphaned containers
- Configurable resource limits
- Monitoring of system resource usage

This implementation plan provides a comprehensive roadmap for creating a robust, well-tested MCP PostgreSQL Server that follows established Go patterns and integrates seamlessly with the MCP ecosystem.
