package unit_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/stokaro/dev-postgres-mcp/internal/postgres"
	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

func TestDSNGeneration(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name     string
		config   postgres.DSNConfig
		expected string
	}{
		{
			name: "basic DSN",
			config: postgres.DSNConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				Username: "testuser",
				Password: "testpass",
				SSLMode:  "disable",
			},
			expected: "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable",
		},
		{
			name: "DSN with custom port",
			config: postgres.DSNConfig{
				Host:     "localhost",
				Port:     15432,
				Database: "mydb",
				Username: "postgres",
				Password: "secret",
				SSLMode:  "require",
			},
			expected: "postgres://postgres:secret@localhost:15432/mydb?sslmode=require",
		},
		{
			name: "DSN with special characters",
			config: postgres.DSNConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "test@db",
				Username: "user+name",
				Password: "pass word",
				SSLMode:  "disable",
			},
			expected: "postgres://user%2Bname:pass+word@localhost:5432/test%40db?sslmode=disable",
		},
		{
			name: "DSN with options",
			config: postgres.DSNConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				Username: "testuser",
				Password: "testpass",
				SSLMode:  "disable",
				Options: map[string]string{
					"connect_timeout": "10",
					"application_name": "test_app",
				},
			},
			expected: "postgres://testuser:testpass@localhost:5432/testdb?application_name=test_app&connect_timeout=10&sslmode=disable",
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			dsn := postgres.GenerateDSN(tt.config)
			c.Assert(dsn, qt.Equals, tt.expected)
		})
	}
}

func TestDSNParsing(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name     string
		dsn      string
		expected postgres.DSNConfig
		hasError bool
	}{
		{
			name: "basic DSN parsing",
			dsn:  "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable",
			expected: postgres.DSNConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "testdb",
				Username: "testuser",
				Password: "testpass",
				SSLMode:  "disable",
				Options:  map[string]string{},
			},
			hasError: false,
		},
		{
			name: "DSN with custom port",
			dsn:  "postgres://postgres:secret@localhost:15432/mydb?sslmode=require",
			expected: postgres.DSNConfig{
				Host:     "localhost",
				Port:     15432,
				Database: "mydb",
				Username: "postgres",
				Password: "secret",
				SSLMode:  "require",
				Options:  map[string]string{},
			},
			hasError: false,
		},
		{
			name: "DSN with options",
			dsn:  "postgres://user:pass@localhost:5432/db?sslmode=disable&connect_timeout=10&application_name=test",
			expected: postgres.DSNConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "db",
				Username: "user",
				Password: "pass",
				SSLMode:  "disable",
				Options: map[string]string{
					"connect_timeout":  "10",
					"application_name": "test",
				},
			},
			hasError: false,
		},
		{
			name:     "invalid DSN",
			dsn:      "invalid://dsn",
			expected: postgres.DSNConfig{},
			hasError: true,
		},
		{
			name:     "malformed URL",
			dsn:      "not-a-url",
			expected: postgres.DSNConfig{},
			hasError: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			config, err := postgres.ParseDSN(tt.dsn)

			if tt.hasError {
				c.Assert(err, qt.IsNotNil)
				return
			}

			c.Assert(err, qt.IsNil)
			c.Assert(config.Host, qt.Equals, tt.expected.Host)
			c.Assert(config.Port, qt.Equals, tt.expected.Port)
			c.Assert(config.Database, qt.Equals, tt.expected.Database)
			c.Assert(config.Username, qt.Equals, tt.expected.Username)
			c.Assert(config.Password, qt.Equals, tt.expected.Password)
			c.Assert(config.SSLMode, qt.Equals, tt.expected.SSLMode)
			c.Assert(len(config.Options), qt.Equals, len(tt.expected.Options))

			for key, expectedValue := range tt.expected.Options {
				c.Assert(config.Options[key], qt.Equals, expectedValue)
			}
		})
	}
}

func TestDSNRoundTrip(t *testing.T) {
	c := qt.New(t)

	originalConfig := postgres.DSNConfig{
		Host:     "localhost",
		Port:     15432,
		Database: "testdb",
		Username: "testuser",
		Password: "testpass",
		SSLMode:  "require",
		Options: map[string]string{
			"connect_timeout": "10",
		},
	}

	// Generate DSN from config
	dsn := postgres.GenerateDSN(originalConfig)

	// Parse DSN back to config
	parsedConfig, err := postgres.ParseDSN(dsn)
	c.Assert(err, qt.IsNil)

	// Compare configs
	c.Assert(parsedConfig.Host, qt.Equals, originalConfig.Host)
	c.Assert(parsedConfig.Port, qt.Equals, originalConfig.Port)
	c.Assert(parsedConfig.Database, qt.Equals, originalConfig.Database)
	c.Assert(parsedConfig.Username, qt.Equals, originalConfig.Username)
	c.Assert(parsedConfig.Password, qt.Equals, originalConfig.Password)
	c.Assert(parsedConfig.SSLMode, qt.Equals, originalConfig.SSLMode)
	c.Assert(len(parsedConfig.Options), qt.Equals, len(originalConfig.Options))

	for key, expectedValue := range originalConfig.Options {
		c.Assert(parsedConfig.Options[key], qt.Equals, expectedValue)
	}
}

func TestBuildLocalDSN(t *testing.T) {
	c := qt.New(t)

	dsn := postgres.BuildLocalDSN(15432, "testdb", "testuser", "testpass")
	expected := "postgres://testuser:testpass@localhost:15432/testdb?sslmode=disable"

	c.Assert(dsn, qt.Equals, expected)
}

func TestMaskPassword(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name     string
		dsn      string
		expected string
	}{
		{
			name:     "mask password in DSN",
			dsn:      "postgres://user:secret@localhost:5432/db",
			expected: "postgres://user:%2A%2A%2A%2A@localhost:5432/db?sslmode=disable",
		},
		{
			name:     "no password to mask",
			dsn:      "postgres://user@localhost:5432/db",
			expected: "postgres://user:@localhost:5432/db?sslmode=disable",
		},
		{
			name:     "invalid DSN",
			dsn:      "not-a-dsn",
			expected: "not-a-dsn",
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			masked := postgres.MaskPassword(tt.dsn)
			c.Assert(masked, qt.Equals, tt.expected)
		})
	}
}

func TestGetDatabaseFromDSN(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name     string
		dsn      string
		expected string
		hasError bool
	}{
		{
			name:     "extract database name",
			dsn:      "postgres://user:pass@localhost:5432/mydb",
			expected: "mydb",
			hasError: false,
		},
		{
			name:     "invalid DSN",
			dsn:      "invalid",
			expected: "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			db, err := postgres.GetDatabaseFromDSN(tt.dsn)

			if tt.hasError {
				c.Assert(err, qt.IsNotNil)
				return
			}

			c.Assert(err, qt.IsNil)
			c.Assert(db, qt.Equals, tt.expected)
		})
	}
}

func TestGetHostPortFromDSN(t *testing.T) {
	c := qt.New(t)

	host, port, err := postgres.GetHostPortFromDSN("postgres://user:pass@localhost:15432/db")
	c.Assert(err, qt.IsNil)
	c.Assert(host, qt.Equals, "localhost")
	c.Assert(port, qt.Equals, 15432)
}

func TestGetCredentialsFromDSN(t *testing.T) {
	c := qt.New(t)

	username, password, err := postgres.GetCredentialsFromDSN("postgres://testuser:testpass@localhost:5432/db")
	c.Assert(err, qt.IsNil)
	c.Assert(username, qt.Equals, "testuser")
	c.Assert(password, qt.Equals, "testpass")
}

func TestCreateInstanceOptions(t *testing.T) {
	c := qt.New(t)

	// Test default values
	opts := types.CreateInstanceOptions{}
	c.Assert(opts.Version, qt.Equals, "")
	c.Assert(opts.Database, qt.Equals, "")
	c.Assert(opts.Username, qt.Equals, "")
	c.Assert(opts.Password, qt.Equals, "")

	// Test with values
	opts = types.CreateInstanceOptions{
		Version:  "16",
		Database: "mydb",
		Username: "myuser",
		Password: "mypass",
	}
	c.Assert(opts.Version, qt.Equals, "16")
	c.Assert(opts.Database, qt.Equals, "mydb")
	c.Assert(opts.Username, qt.Equals, "myuser")
	c.Assert(opts.Password, qt.Equals, "mypass")
}

func TestPartialIDMatching(t *testing.T) {
	c := qt.New(t)

	// Create mock instances for testing
	instances := []*types.PostgreSQLInstance{
		{
			ID:          "abcd1234-5678-90ef-ghij-klmnopqrstuv",
			ContainerID: "container1",
			Port:        5432,
			Database:    "db1",
			Username:    "user1",
			Version:     "17",
			Status:      "running",
		},
		{
			ID:          "abcd5678-1234-90ef-ghij-klmnopqrstuv",
			ContainerID: "container2",
			Port:        5433,
			Database:    "db2",
			Username:    "user2",
			Version:     "17",
			Status:      "running",
		},
		{
			ID:          "efgh1234-5678-90ef-ghij-klmnopqrstuv",
			ContainerID: "container3",
			Port:        5434,
			Database:    "db3",
			Username:    "user3",
			Version:     "17",
			Status:      "running",
		},
	}

	c.Run("exact_match", func(c *qt.C) {
		fullID := "abcd1234-5678-90ef-ghij-klmnopqrstuv"
		var match *types.PostgreSQLInstance
		for _, instance := range instances {
			if instance.ID == fullID {
				match = instance
				break
			}
		}
		c.Assert(match, qt.IsNotNil)
		c.Assert(match.ID, qt.Equals, fullID)
	})

	c.Run("partial_match_unique", func(c *qt.C) {
		partialID := "efgh"
		var matches []*types.PostgreSQLInstance
		for _, instance := range instances {
			if len(instance.ID) >= len(partialID) && instance.ID[:len(partialID)] == partialID {
				matches = append(matches, instance)
			}
		}
		c.Assert(len(matches), qt.Equals, 1)
		c.Assert(matches[0].Database, qt.Equals, "db3")
	})

	c.Run("partial_match_ambiguous", func(c *qt.C) {
		partialID := "abcd"
		var matches []*types.PostgreSQLInstance
		for _, instance := range instances {
			if len(instance.ID) >= len(partialID) && instance.ID[:len(partialID)] == partialID {
				matches = append(matches, instance)
			}
		}
		c.Assert(len(matches), qt.Equals, 2)
	})

	c.Run("no_match", func(c *qt.C) {
		partialID := "xyz"
		var matches []*types.PostgreSQLInstance
		for _, instance := range instances {
			if len(instance.ID) >= len(partialID) && instance.ID[:len(partialID)] == partialID {
				matches = append(matches, instance)
			}
		}
		c.Assert(len(matches), qt.Equals, 0)
	})
}
