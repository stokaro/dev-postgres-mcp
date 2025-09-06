// Package version provides version information for the application.
package version

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Build information. These will be set by the build process.
var (
	// Version is the current version of the application.
	Version = "dev"
	// Commit is the git commit hash.
	Commit = "unknown"
	// Date is the build date.
	Date = "unknown"
	// BuiltBy is the entity that built the binary.
	BuiltBy = "unknown"
)

// Info represents version information.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
	BuiltBy   string `json:"built_by"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// GetInfo returns the version information.
func GetInfo() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		BuiltBy:   BuiltBy,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string.
func (i Info) String() string {
	return fmt.Sprintf("dev-postgres-mcp %s (%s) built on %s by %s\nGo version: %s\nPlatform: %s",
		i.Version, i.Commit, i.Date, i.BuiltBy, i.GoVersion, i.Platform)
}

// New creates a new version command.
func New() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display version information including build details and platform information.",
		Run: func(_ *cobra.Command, _ []string) {
			info := GetInfo()
			fmt.Println(info.String())
		},
	}
}
