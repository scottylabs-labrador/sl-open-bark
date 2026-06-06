// Package config loads the Slack gateway's typed settings from the environment. Slack tokens and
// the runtime token come from Railway secrets, never from code.
package config

import "os"

type Config struct {
	SigningSecret string // SLACK_SIGNING_SECRET (empty disables signature checks — dev only)
	BotToken      string // SLACK_BOT_TOKEN (empty = log-only poster)
	BotUserID     string // SLACK_BOT_USER_ID (to ignore the bot's own messages)
	RuntimeURL    string // RUNTIME_URL
	RuntimeToken  string // RUNTIME_SERVICE_TOKEN
	Port          string
}

func Load() Config {
	return Config{
		SigningSecret: os.Getenv("SLACK_SIGNING_SECRET"),
		BotToken:      os.Getenv("SLACK_BOT_TOKEN"),
		BotUserID:     os.Getenv("SLACK_BOT_USER_ID"),
		RuntimeURL:    os.Getenv("RUNTIME_URL"),
		RuntimeToken:  os.Getenv("RUNTIME_SERVICE_TOKEN"),
		Port:          firstNonEmpty(os.Getenv("PORT"), "8080"),
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
