// Package config loads the scheduler's settings and job set from the environment.
package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/scottylabs/scottylabs-agent/services/scheduler/internal/scheduler"
)

type Config struct {
	RuntimeURL   string
	RuntimeToken string
	ScheduleFile string // optional JSON array of jobs; overrides the built-in DefaultSchedule
	Port         string
}

func Load() Config {
	return Config{
		RuntimeURL:   os.Getenv("RUNTIME_URL"),
		RuntimeToken: os.Getenv("RUNTIME_SERVICE_TOKEN"),
		ScheduleFile: os.Getenv("SCHEDULE_FILE"),
		Port:         firstNonEmpty(os.Getenv("PORT"), "8080"),
	}
}

// Jobs returns the configured job set: the file at ScheduleFile if set, otherwise the built-in
// default.
func (c Config) Jobs() ([]scheduler.Job, error) {
	if c.ScheduleFile == "" {
		return scheduler.DefaultSchedule(), nil
	}
	raw, err := os.ReadFile(c.ScheduleFile)
	if err != nil {
		return nil, fmt.Errorf("config: read schedule file: %w", err)
	}
	var jobs []scheduler.Job
	if err := json.Unmarshal(raw, &jobs); err != nil {
		return nil, fmt.Errorf("config: parse schedule file: %w", err)
	}
	return jobs, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
