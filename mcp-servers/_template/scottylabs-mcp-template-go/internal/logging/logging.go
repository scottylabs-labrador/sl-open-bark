// Package logging configures structured JSON logging and a redaction helper.
package logging

import (
	"log/slog"
	"os"
	"strings"
)

func Setup(level string) {
	var l slog.Level
	switch strings.ToLower(level) {
	case "debug":
		l = slog.LevelDebug
	case "warn":
		l = slog.LevelWarn
	case "error":
		l = slog.LevelError
	default:
		l = slog.LevelInfo
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: l})))
}

var redactKeys = map[string]bool{
	"authorization": true, "token": true, "api_key": true, "password": true, "secret": true,
}

// Redact returns a copy of payload with sensitive values masked, for safe logging.
func Redact(payload map[string]any) map[string]any {
	out := make(map[string]any, len(payload))
	for k, v := range payload {
		if redactKeys[strings.ToLower(k)] {
			out[k] = "***"
		} else {
			out[k] = v
		}
	}
	return out
}
