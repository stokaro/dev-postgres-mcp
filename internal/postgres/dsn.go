package postgres

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// DSNConfig holds configuration for generating a PostgreSQL DSN.
type DSNConfig struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
	Options  map[string]string
}

// GenerateDSN creates a PostgreSQL connection string (DSN) from the given configuration.
func GenerateDSN(config DSNConfig) string {
	// Set defaults
	if config.Host == "" {
		config.Host = "localhost"
	}
	if config.Port == 0 {
		config.Port = 5432
	}
	if config.Database == "" {
		config.Database = "postgres"
	}
	if config.Username == "" {
		config.Username = "postgres"
	}
	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}

	// Build the DSN
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		url.QueryEscape(config.Username),
		url.QueryEscape(config.Password),
		config.Host,
		config.Port,
		url.QueryEscape(config.Database))

	// Add query parameters
	params := url.Values{}
	params.Set("sslmode", config.SSLMode)

	// Add custom options
	for key, value := range config.Options {
		params.Set(key, value)
	}

	if len(params) > 0 {
		dsn += "?" + params.Encode()
	}

	return dsn
}

// ParseDSN parses a PostgreSQL DSN and returns the configuration.
func ParseDSN(dsn string) (*DSNConfig, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid DSN format: %w", err)
	}

	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return nil, fmt.Errorf("unsupported scheme: %s", u.Scheme)
	}

	config := &DSNConfig{
		Host:     u.Hostname(),
		Database: strings.TrimPrefix(u.Path, "/"),
		Options:  make(map[string]string),
	}

	// Parse port
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
		config.Port = port
	} else {
		config.Port = 5432
	}

	// Parse username and password
	if u.User != nil {
		config.Username = u.User.Username()
		if password, ok := u.User.Password(); ok {
			config.Password = password
		}
	}

	// Parse query parameters
	for key, values := range u.Query() {
		if len(values) > 0 {
			switch key {
			case "sslmode":
				config.SSLMode = values[0]
			default:
				config.Options[key] = values[0]
			}
		}
	}

	// Set default SSL mode if not specified
	if config.SSLMode == "" {
		config.SSLMode = "disable"
	}

	return config, nil
}

// ValidateDSN validates a PostgreSQL DSN format.
func ValidateDSN(dsn string) error {
	_, err := ParseDSN(dsn)
	return err
}

// BuildDSNFromInstance creates a DSN from a PostgreSQL instance.
func BuildDSNFromInstance(host string, port int, database, username, password string) string {
	return GenerateDSN(DSNConfig{
		Host:     host,
		Port:     port,
		Database: database,
		Username: username,
		Password: password,
		SSLMode:  "disable", // For local development instances
	})
}

// BuildLocalDSN creates a DSN for a local PostgreSQL instance.
func BuildLocalDSN(port int, database, username, password string) string {
	return BuildDSNFromInstance("localhost", port, database, username, password)
}

// MaskPassword returns a DSN with the password masked for logging purposes.
func MaskPassword(dsn string) string {
	config, err := ParseDSN(dsn)
	if err != nil {
		// If we can't parse it, just return the original (might not be a DSN)
		return dsn
	}

	// Replace password with asterisks
	maskedConfig := *config
	if maskedConfig.Password != "" {
		maskedConfig.Password = "****"
	}

	return GenerateDSN(maskedConfig)
}

// GetDatabaseFromDSN extracts the database name from a DSN.
func GetDatabaseFromDSN(dsn string) (string, error) {
	config, err := ParseDSN(dsn)
	if err != nil {
		return "", err
	}
	return config.Database, nil
}

// GetHostPortFromDSN extracts the host and port from a DSN.
func GetHostPortFromDSN(dsn string) (string, int, error) {
	config, err := ParseDSN(dsn)
	if err != nil {
		return "", 0, err
	}
	return config.Host, config.Port, nil
}

// GetCredentialsFromDSN extracts the username and password from a DSN.
func GetCredentialsFromDSN(dsn string) (username, password string, err error) {
	config, err := ParseDSN(dsn)
	if err != nil {
		return "", "", err
	}
	return config.Username, config.Password, nil
}
