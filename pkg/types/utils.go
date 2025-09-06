// Package types provides utility functions for working with database types.
package types

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// GenerateInstanceID generates a unique instance ID without dashes.
// The ID is alphanumeric and URL-safe.
func GenerateInstanceID() string {
	// Generate a UUID and remove dashes
	id := uuid.New().String()
	return strings.ReplaceAll(id, "-", "")
}

// GeneratePassword generates a random password of the specified length.
func GeneratePassword(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("password length must be positive")
	}

	// Generate random bytes
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Encode to base64 and trim to desired length
	password := base64.URLEncoding.EncodeToString(bytes)
	if len(password) > length {
		password = password[:length]
	}

	return password, nil
}

// ValidateCreateInstanceOptions validates and sets defaults for CreateInstanceOptions.
func ValidateCreateInstanceOptions(opts *CreateInstanceOptions) error {
	// Validate database type
	if opts.Type == "" {
		opts.Type = DatabaseTypePostgreSQL // Default to PostgreSQL for backward compatibility
	}

	if !opts.Type.IsValid() {
		return fmt.Errorf("invalid database type: %s", opts.Type)
	}

	// Set defaults based on database type
	if opts.Version == "" {
		opts.Version = opts.Type.DefaultVersion()
	}

	if opts.Database == "" {
		opts.Database = opts.Type.DefaultDatabase()
	}

	if opts.Username == "" {
		opts.Username = opts.Type.DefaultUsername()
	}

	if opts.Password == "" {
		var err error
		opts.Password, err = GeneratePassword(16)
		if err != nil {
			return fmt.Errorf("failed to generate password: %w", err)
		}
	}

	return nil
}

// BuildDSN builds a Data Source Name (DSN) for the given database instance.
func BuildDSN(instance *DatabaseInstance) string {
	switch instance.Type {
	case DatabaseTypePostgreSQL:
		return fmt.Sprintf("postgres://%s:%s@localhost:%d/%s?sslmode=disable",
			instance.Username, instance.Password, instance.Port, instance.Database)
	case DatabaseTypeMySQL:
		return fmt.Sprintf("%s:%s@tcp(localhost:%d)/%s",
			instance.Username, instance.Password, instance.Port, instance.Database)
	case DatabaseTypeMariaDB:
		return fmt.Sprintf("%s:%s@tcp(localhost:%d)/%s",
			instance.Username, instance.Password, instance.Port, instance.Database)
	default:
		return ""
	}
}

// GetDockerImage returns the Docker image name for the given database type and version.
func GetDockerImage(dbType DatabaseType, version string) string {
	switch dbType {
	case DatabaseTypePostgreSQL:
		return fmt.Sprintf("postgres:%s", version)
	case DatabaseTypeMySQL:
		return fmt.Sprintf("mysql:%s", version)
	case DatabaseTypeMariaDB:
		return fmt.Sprintf("mariadb:%s", version)
	default:
		return ""
	}
}

// GetContainerName generates a container name for the given instance ID and database type.
func GetContainerName(instanceID string, dbType DatabaseType) string {
	return fmt.Sprintf("dev-%s-mcp-%s", dbType.String(), instanceID)
}
