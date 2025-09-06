package unit_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/stokaro/dev-postgres-mcp/pkg/types"
)

func TestDatabaseType(t *testing.T) {
	t.Run("Valid database types", func(t *testing.T) {
		c := qt.New(t)

		validTypes := []types.DatabaseType{
			types.DatabaseTypePostgreSQL,
			types.DatabaseTypeMySQL,
			types.DatabaseTypeMariaDB,
		}

		for _, dbType := range validTypes {
			c.Assert(dbType.IsValid(), qt.IsTrue, qt.Commentf("Database type %s should be valid", dbType))
		}
	})

	t.Run("Invalid database types", func(t *testing.T) {
		c := qt.New(t)

		invalidTypes := []types.DatabaseType{
			"invalid",
			"",
			"postgres", // Should be "postgresql"
			"mongo",
		}

		for _, dbType := range invalidTypes {
			c.Assert(dbType.IsValid(), qt.IsFalse, qt.Commentf("Database type %s should be invalid", dbType))
		}
	})

	t.Run("Default ports", func(t *testing.T) {
		c := qt.New(t)

		tests := []struct {
			dbType       types.DatabaseType
			expectedPort int
		}{
			{types.DatabaseTypePostgreSQL, 5432},
			{types.DatabaseTypeMySQL, 3306},
			{types.DatabaseTypeMariaDB, 3306},
		}

		for _, test := range tests {
			c.Assert(test.dbType.DefaultPort(), qt.Equals, test.expectedPort,
				qt.Commentf("Database type %s should have default port %d", test.dbType, test.expectedPort))
		}
	})

	t.Run("Default versions", func(t *testing.T) {
		c := qt.New(t)

		tests := []struct {
			dbType          types.DatabaseType
			expectedVersion string
		}{
			{types.DatabaseTypePostgreSQL, "17"},
			{types.DatabaseTypeMySQL, "8.0"},
			{types.DatabaseTypeMariaDB, "11"},
		}

		for _, test := range tests {
			c.Assert(test.dbType.DefaultVersion(), qt.Equals, test.expectedVersion,
				qt.Commentf("Database type %s should have default version %s", test.dbType, test.expectedVersion))
		}
	})

	t.Run("Default databases", func(t *testing.T) {
		c := qt.New(t)

		tests := []struct {
			dbType           types.DatabaseType
			expectedDatabase string
		}{
			{types.DatabaseTypePostgreSQL, "postgres"},
			{types.DatabaseTypeMySQL, "mysql"},
			{types.DatabaseTypeMariaDB, "mysql"},
		}

		for _, test := range tests {
			c.Assert(test.dbType.DefaultDatabase(), qt.Equals, test.expectedDatabase,
				qt.Commentf("Database type %s should have default database %s", test.dbType, test.expectedDatabase))
		}
	})

	t.Run("Default usernames", func(t *testing.T) {
		c := qt.New(t)

		tests := []struct {
			dbType           types.DatabaseType
			expectedUsername string
		}{
			{types.DatabaseTypePostgreSQL, "postgres"},
			{types.DatabaseTypeMySQL, "root"},
			{types.DatabaseTypeMariaDB, "root"},
		}

		for _, test := range tests {
			c.Assert(test.dbType.DefaultUsername(), qt.Equals, test.expectedUsername,
				qt.Commentf("Database type %s should have default username %s", test.dbType, test.expectedUsername))
		}
	})
}

func TestGenerateInstanceID(t *testing.T) {
	t.Run("Generate unique IDs", func(t *testing.T) {
		c := qt.New(t)

		// Generate multiple IDs and ensure they're unique
		ids := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := types.GenerateInstanceID()
			c.Assert(id, qt.Not(qt.Equals), "", qt.Commentf("Generated ID should not be empty"))
			c.Assert(ids[id], qt.IsFalse, qt.Commentf("Generated ID %s should be unique", id))
			ids[id] = true
		}
	})

	t.Run("ID format", func(t *testing.T) {
		c := qt.New(t)

		id := types.GenerateInstanceID()

		// Should not contain dashes
		c.Assert(id, qt.Not(qt.Contains), "-", qt.Commentf("Generated ID should not contain dashes"))

		// Should be alphanumeric
		for _, char := range id {
			c.Assert((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9'),
				qt.IsTrue, qt.Commentf("Character %c should be alphanumeric", char))
		}

		// Should have reasonable length (UUID without dashes is 32 characters)
		c.Assert(len(id), qt.Equals, 32, qt.Commentf("Generated ID should be 32 characters long"))
	})
}

func TestGeneratePassword(t *testing.T) {
	t.Run("Generate passwords of different lengths", func(t *testing.T) {
		c := qt.New(t)

		lengths := []int{8, 16, 32, 64}
		for _, length := range lengths {
			password, err := types.GeneratePassword(length)
			c.Assert(err, qt.IsNil, qt.Commentf("Should generate password of length %d", length))
			c.Assert(len(password), qt.Equals, length, qt.Commentf("Password should have length %d", length))
		}
	})

	t.Run("Generate unique passwords", func(t *testing.T) {
		c := qt.New(t)

		passwords := make(map[string]bool)
		for i := 0; i < 100; i++ {
			password, err := types.GeneratePassword(16)
			c.Assert(err, qt.IsNil)
			c.Assert(passwords[password], qt.IsFalse, qt.Commentf("Password %s should be unique", password))
			passwords[password] = true
		}
	})

	t.Run("Invalid length", func(t *testing.T) {
		c := qt.New(t)

		_, err := types.GeneratePassword(0)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Should return error for zero length"))

		_, err = types.GeneratePassword(-1)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Should return error for negative length"))
	})
}

func TestValidateCreateInstanceOptions(t *testing.T) {
	t.Run("Valid options with defaults", func(t *testing.T) {
		c := qt.New(t)

		opts := &types.CreateInstanceOptions{}
		err := types.ValidateCreateInstanceOptions(opts)
		c.Assert(err, qt.IsNil)

		// Should set defaults
		c.Assert(opts.Type, qt.Equals, types.DatabaseTypePostgreSQL)
		c.Assert(opts.Version, qt.Equals, "17")
		c.Assert(opts.Database, qt.Equals, "postgres")
		c.Assert(opts.Username, qt.Equals, "postgres")
		c.Assert(opts.Password, qt.Not(qt.Equals), "", qt.Commentf("Password should be generated"))
	})

	t.Run("Valid options with MySQL", func(t *testing.T) {
		c := qt.New(t)

		opts := &types.CreateInstanceOptions{
			Type: types.DatabaseTypeMySQL,
		}
		err := types.ValidateCreateInstanceOptions(opts)
		c.Assert(err, qt.IsNil)

		// Should set MySQL defaults
		c.Assert(opts.Type, qt.Equals, types.DatabaseTypeMySQL)
		c.Assert(opts.Version, qt.Equals, "8.0")
		c.Assert(opts.Database, qt.Equals, "mysql")
		c.Assert(opts.Username, qt.Equals, "root")
	})

	t.Run("Invalid database type", func(t *testing.T) {
		c := qt.New(t)

		opts := &types.CreateInstanceOptions{
			Type: "invalid",
		}
		err := types.ValidateCreateInstanceOptions(opts)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Should return error for invalid database type"))
	})
}

func TestBuildDSN(t *testing.T) {
	t.Run("PostgreSQL DSN", func(t *testing.T) {
		c := qt.New(t)

		instance := &types.DatabaseInstance{
			Type:     types.DatabaseTypePostgreSQL,
			Username: "postgres",
			Password: "secret",
			Port:     5432,
			Database: "testdb",
		}

		dsn := types.BuildDSN(instance)
		expected := "postgres://postgres:secret@localhost:5432/testdb?sslmode=disable"
		c.Assert(dsn, qt.Equals, expected)
	})

	t.Run("MySQL DSN", func(t *testing.T) {
		c := qt.New(t)

		instance := &types.DatabaseInstance{
			Type:     types.DatabaseTypeMySQL,
			Username: "root",
			Password: "secret",
			Port:     3306,
			Database: "testdb",
		}

		dsn := types.BuildDSN(instance)
		expected := "root:secret@tcp(localhost:3306)/testdb"
		c.Assert(dsn, qt.Equals, expected)
	})

	t.Run("MariaDB DSN", func(t *testing.T) {
		c := qt.New(t)

		instance := &types.DatabaseInstance{
			Type:     types.DatabaseTypeMariaDB,
			Username: "root",
			Password: "secret",
			Port:     3306,
			Database: "testdb",
		}

		dsn := types.BuildDSN(instance)
		expected := "root:secret@tcp(localhost:3306)/testdb"
		c.Assert(dsn, qt.Equals, expected)
	})
}

func TestGetDockerImage(t *testing.T) {
	tests := []struct {
		dbType        types.DatabaseType
		version       string
		expectedImage string
	}{
		{types.DatabaseTypePostgreSQL, "17", "postgres:17"},
		{types.DatabaseTypePostgreSQL, "16", "postgres:16"},
		{types.DatabaseTypeMySQL, "8.0", "mysql:8.0"},
		{types.DatabaseTypeMySQL, "5.7", "mysql:5.7"},
		{types.DatabaseTypeMariaDB, "11", "mariadb:11"},
		{types.DatabaseTypeMariaDB, "10.6", "mariadb:10.6"},
	}

	for _, test := range tests {
		c := qt.New(t)
		image := types.GetDockerImage(test.dbType, test.version)
		c.Assert(image, qt.Equals, test.expectedImage,
			qt.Commentf("Database type %s version %s should have image %s", test.dbType, test.version, test.expectedImage))
	}
}

func TestGetContainerName(t *testing.T) {
	tests := []struct {
		instanceID   string
		dbType       types.DatabaseType
		expectedName string
	}{
		{"abc123", types.DatabaseTypePostgreSQL, "dev-postgresql-mcp-abc123"},
		{"def456", types.DatabaseTypeMySQL, "dev-mysql-mcp-def456"},
		{"ghi789", types.DatabaseTypeMariaDB, "dev-mariadb-mcp-ghi789"},
	}

	for _, test := range tests {
		c := qt.New(t)
		name := types.GetContainerName(test.instanceID, test.dbType)
		c.Assert(name, qt.Equals, test.expectedName,
			qt.Commentf("Instance %s of type %s should have container name %s", test.instanceID, test.dbType, test.expectedName))
	}
}
