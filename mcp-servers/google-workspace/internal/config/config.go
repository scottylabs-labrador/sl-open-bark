// Package config loads typed settings from the environment. Credentials (the service-account key,
// the bearer token) come from the environment / Railway secrets, never from code.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port      string
	LogLevel  string
	AuthToken string // bearer the gateway presents (optional locally)

	// GoogleSAJSON is the service-account key (raw JSON), from GOOGLE_SA_JSON or the file at
	// GOOGLE_SA_JSON_FILE. DelegatedSubject is the Workspace user to impersonate via domain-wide
	// delegation (e.g. agent@scottylabs.org).
	GoogleSAJSON     []byte
	DelegatedSubject string
}

func Load() (Config, error) {
	cfg := Config{
		Port:             firstNonEmpty(os.Getenv("PORT"), os.Getenv("MCP_PORT"), "8080"),
		LogLevel:         firstNonEmpty(os.Getenv("MCP_LOG_LEVEL"), "info"),
		AuthToken:        os.Getenv("MCP_AUTH_TOKEN"),
		DelegatedSubject: os.Getenv("GOOGLE_DELEGATED_SUBJECT"),
	}
	if raw := os.Getenv("GOOGLE_SA_JSON"); raw != "" {
		cfg.GoogleSAJSON = []byte(raw)
	} else if path := os.Getenv("GOOGLE_SA_JSON_FILE"); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return Config{}, fmt.Errorf("config: read GOOGLE_SA_JSON_FILE: %w", err)
		}
		cfg.GoogleSAJSON = b
	}
	return cfg, nil
}

// HasGoogleCredentials reports whether the server has what it needs to reach Google.
func (c Config) HasGoogleCredentials() bool {
	return len(c.GoogleSAJSON) > 0 && c.DelegatedSubject != ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
