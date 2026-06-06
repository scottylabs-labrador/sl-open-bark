// Package config loads typed settings from the environment.
package config

import "os"

type Config struct {
	Port      string
	LogLevel  string
	AuthToken string // bearer the gateway presents (optional locally)

	// StandardsFile optionally overrides the embedded, reviewed standards with a file path.
	StandardsFile string
}

func Load() Config {
	return Config{
		Port:          firstNonEmpty(os.Getenv("PORT"), os.Getenv("MCP_PORT"), "8080"),
		LogLevel:      firstNonEmpty(os.Getenv("MCP_LOG_LEVEL"), "info"),
		AuthToken:     os.Getenv("MCP_AUTH_TOKEN"),
		StandardsFile: os.Getenv("FINANCE_STANDARDS_FILE"),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
