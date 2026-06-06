// Package config loads typed settings from the environment. No magic globals; secrets come from
// the environment (Railway secrets), never from code.
package config

import "os"

type Config struct {
	Port      string // Railway injects PORT; falls back to MCP_PORT then 8080
	LogLevel  string
	AuthToken string // bearer token the gateway must present (optional)
	AuditURL  string // external audit endpoint; empty means log to stdout
}

func Load() Config {
	return Config{
		Port:      firstNonEmpty(os.Getenv("PORT"), os.Getenv("MCP_PORT"), "8080"),
		LogLevel:  firstNonEmpty(os.Getenv("MCP_LOG_LEVEL"), "info"),
		AuthToken: os.Getenv("MCP_AUTH_TOKEN"),
		AuditURL:  os.Getenv("MCP_AUDIT_URL"),
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
