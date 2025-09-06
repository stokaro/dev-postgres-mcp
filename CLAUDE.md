# Claude Development Instructions

This document contains specific instructions for Claude when working on the dev-postgres-mcp project.

## Project Context

This is a Go-based MCP (Model Context Protocol) server for managing ephemeral PostgreSQL instances in Docker containers. The project follows patterns established in the inventario reference project.

## Development Standards

### Go Code Style
- Use Go 1.24+ features
- Follow standard Go naming conventions
- Write comprehensive godoc comments for all public interfaces
- Use structured error handling with proper error wrapping
- Implement proper context handling for cancellation and timeouts

### Testing Requirements
- Use frankban/quicktest testing framework with import alias `qt`
- Structure tests as table-driven tests where appropriate
- Separate test functions for happy path and error scenarios
- Use `_test` package suffix for testing only public interfaces
- Target >90% test coverage
- Write integration tests using testcontainers for Docker operations

### Code Organization
- Follow the established project structure with cmd/, internal/, pkg/, test/ directories
- Keep internal packages focused and cohesive
- Use interfaces for testability and modularity
- Implement proper dependency injection patterns

### Documentation
- Write all comments and documentation in English
- Provide comprehensive godoc for public APIs
- Include usage examples in documentation
- Maintain up-to-date README with clear installation and usage instructions

### Error Handling
- Use structured error types for different failure modes
- Wrap errors with appropriate context using fmt.Errorf
- Implement proper error logging with structured logging
- Provide user-friendly error messages in CLI commands

### Docker Integration
- Handle Docker daemon connectivity gracefully
- Implement proper container lifecycle management
- Use appropriate resource limits and cleanup
- Handle Docker API errors appropriately

### MCP Integration
- Follow MCP protocol specifications strictly
- Implement proper tool validation and error responses
- Use structured JSON responses for all tools
- Handle MCP client disconnections gracefully

## Development Workflow

1. **Before Making Changes**: Always run tests to ensure current functionality works
2. **During Development**: Write tests alongside implementation code
3. **After Changes**: Run full test suite including integration tests
4. **Code Review**: Ensure all public APIs have proper documentation

## Command Patterns

Follow the Cobra CLI patterns established in the inventario project:
- Separate command files for each major command group
- Use shared utilities for common functionality
- Implement proper flag validation and help text
- Handle configuration through environment variables and flags

## Security Considerations

- Generate secure random passwords for PostgreSQL instances
- Implement proper container isolation
- Avoid logging sensitive information like passwords
- Use appropriate Docker security settings

## Performance Guidelines

- Implement proper connection pooling for Docker client
- Use context with timeouts for all Docker operations
- Implement efficient port allocation algorithms
- Monitor and limit resource usage

## Debugging and Logging

- Use structured logging with appropriate log levels
- Include relevant context in log messages
- Implement debug modes for troubleshooting
- Provide clear error messages for common failure scenarios

## Dependencies

- Prefer standard library when possible
- Use well-maintained third-party libraries
- Keep dependency tree minimal and focused
- Regularly update dependencies for security

## Testing Strategy

### Unit Tests
- Test all business logic in isolation
- Mock external dependencies (Docker, network)
- Test error conditions and edge cases
- Use quicktest for assertions

### Integration Tests
- Test Docker container operations with real containers
- Test MCP protocol communication
- Test CLI command execution
- Use testcontainers for reliable test environments

### End-to-End Tests
- Test complete workflows from CLI to container creation
- Test MCP client integration scenarios
- Verify cleanup and resource management
- Test failure recovery scenarios

Remember: This is a development tool, so prioritize developer experience, clear error messages, and reliable cleanup over performance optimization.
