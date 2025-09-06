// Package main is the entry point for the dev-postgres-mcp application.
package main

import (
	"log/slog"
	"os"
)

func setupSlog() {
	var handler slog.Handler
	if os.Getenv("DEV_POSTGRES_MCP_LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: true,
		})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			AddSource: true,
		})
	}

	slog.SetDefault(slog.New(handler))
}

func main() {
	setupSlog()

	Execute()
}
