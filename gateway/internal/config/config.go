// Package config loads the gateway's typed settings from the environment (no secrets in code; in
// production these come from Railway secrets / the .env the platform assumes).
package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port string // HTTP port (Railway injects PORT)

	// Rate limits (design 10.2): per-committee and global requests/minute. <= 0 disables a dimension.
	RateCommitteePerMin int
	RateGlobalPerMin    int

	// ServiceToken is the bearer the deployed agent and the dashboard present to the gateway.
	// Human callers authenticate via ContextForge's OAuth 2.1/PKCE in front of this service.
	ServiceToken string

	// DatabaseURL is the Postgres DSN for the registry, approvals, and audit log (WP-01).
	DatabaseURL string

	// ContextForgeURL is the adopted MCP gateway this policy layer fronts/cooperates with.
	ContextForgeURL string

	// DownstreamToken is presented to downstream MCP servers (their MCP_AUTH_TOKEN). Per-server
	// tokens can be added later; the credential is held by the gateway, never by a client.
	DownstreamToken string

	// RepoRoot is where manifests are read from when syncing the registry from the monorepo.
	RepoRoot string
}

func Load() Config {
	return Config{
		Port:                firstNonEmpty(os.Getenv("PORT"), "8080"),
		ServiceToken:        os.Getenv("GATEWAY_SERVICE_TOKEN"),
		DatabaseURL:         os.Getenv("DATABASE_URL"),
		ContextForgeURL:     os.Getenv("CONTEXTFORGE_URL"),
		DownstreamToken:     os.Getenv("MCP_DOWNSTREAM_TOKEN"),
		RepoRoot:            firstNonEmpty(os.Getenv("REPO_ROOT"), "."),
		RateCommitteePerMin: envInt("GATEWAY_RATE_COMMITTEE_PER_MIN", 120),
		RateGlobalPerMin:    envInt("GATEWAY_RATE_GLOBAL_PER_MIN", 600),
	}
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
