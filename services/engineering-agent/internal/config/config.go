// Package config loads the Engineering Agent's settings from the environment. The GitHub App key,
// the maintainer/repo allowlists, and the sandbox/Slack config come from Railway secrets — never
// from code. This subsystem holds NO gateway, Google, finance, or other production credentials.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/githubapp"
)

type Config struct {
	Port         string
	ServiceToken string // bearer the Slack gateway presents to enqueue /fix-bug tasks

	GitHub          githubapp.Config
	Maintainers     []string // ENG_MAINTAINERS (comma-separated)
	Repos           []string // ENG_REPOS (comma-separated owner/repo)
	SandboxTemplate string
	TimeLimit       time.Duration
	SlackWebhookURL string
}

func Load() Config {
	return Config{
		Port:         firstNonEmpty(os.Getenv("PORT"), "8080"),
		ServiceToken: os.Getenv("ENG_SERVICE_TOKEN"),
		GitHub: githubapp.Config{
			AppID:          os.Getenv("GITHUB_APP_ID"),
			InstallationID: os.Getenv("GITHUB_APP_INSTALLATION_ID"),
			PrivateKeyPEM:  os.Getenv("GITHUB_APP_PRIVATE_KEY"),
		},
		Maintainers:     splitList(os.Getenv("ENG_MAINTAINERS")),
		Repos:           splitList(os.Getenv("ENG_REPOS")),
		SandboxTemplate: firstNonEmpty(os.Getenv("SANDBOX_TEMPLATE"), "dev"),
		TimeLimit:       time.Duration(envInt("ENG_TIME_LIMIT_MIN", 20)) * time.Minute,
		SlackWebhookURL: os.Getenv("SLACK_WEBHOOK_URL"),
	}
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
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
