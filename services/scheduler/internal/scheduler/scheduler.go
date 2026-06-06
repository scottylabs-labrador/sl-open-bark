// Package scheduler runs recurring recipes on a cadence (design 4.2, 11.1): each turn it asks which
// jobs are due and submits them through the runtime. Cron matching and due-selection are pure and
// testable; submission is the only side effect.
package scheduler

import (
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

// Runtime is the slice of the runtime API the scheduler needs (defined at the consumer).
type Runtime interface {
	Submit(ctx context.Context, recipeID, committee string, params map[string]string) (taskID string, err error)
}

// Job is one recurring task: a 5-field cron Spec (UTC), the Recipe to run, its committee, and params.
type Job struct {
	Name      string            `json:"name"`
	Spec      string            `json:"spec"` // standard 5-field cron, UTC, min 5-minute granularity
	Recipe    string            `json:"recipe"`
	Committee string            `json:"committee"`
	Params    map[string]string `json:"params,omitempty"`
}

// RunResult records the outcome of running one due job.
type RunResult struct {
	Job    string
	TaskID string
	Err    string
}

// Scheduler holds the job set and a runtime to submit to.
type Scheduler struct {
	jobs  []Job
	rt    Runtime
	parse func(string) (cron.Schedule, error)
}

// New builds a Scheduler over a job set and a runtime.
func New(jobs []Job, rt Runtime) *Scheduler {
	return &Scheduler{jobs: jobs, rt: rt, parse: func(s string) (cron.Schedule, error) {
		return cron.ParseStandard(s)
	}}
}

// Due returns the jobs whose cron spec fires at the minute containing now (UTC).
func (s *Scheduler) Due(now time.Time) ([]Job, error) {
	minute := now.UTC().Truncate(time.Minute)
	var due []Job
	for _, j := range s.jobs {
		sched, err := s.parse(j.Spec)
		if err != nil {
			return nil, fmt.Errorf("scheduler: job %q has invalid spec %q: %w", j.Name, j.Spec, err)
		}
		// A job is due this minute if its next activation after one second before the minute is the
		// minute itself.
		if sched.Next(minute.Add(-time.Second)).Equal(minute) {
			due = append(due, j)
		}
	}
	return due, nil
}

// RunDue submits every job due at now and returns a result per job (errors are surfaced, never
// fatal — one failing job does not stop the others).
func (s *Scheduler) RunDue(ctx context.Context, now time.Time) ([]RunResult, error) {
	due, err := s.Due(now)
	if err != nil {
		return nil, err
	}
	results := make([]RunResult, 0, len(due))
	for _, j := range due {
		taskID, err := s.rt.Submit(ctx, j.Recipe, j.Committee, j.Params)
		r := RunResult{Job: j.Name, TaskID: taskID}
		if err != nil {
			r.Err = err.Error()
		}
		results = append(results, r)
	}
	return results, nil
}

// DefaultSchedule is the built-in job set (UTC). Override via a schedule file. 13:00 UTC ≈ 09:00 ET.
func DefaultSchedule() []Job {
	return []Job{
		{
			Name: "weekly-leadership-digest", Spec: "0 13 * * 1", // Mondays 13:00 UTC
			Recipe: "shared/weekly-digest", Committee: "leadership",
		},
		{
			Name: "daily-reimbursement-screening", Spec: "0 13 * * *", // every day 13:00 UTC
			Recipe: "finance/screen-reimbursement", Committee: "finance",
			Params: map[string]string{"sheet_id": ""}, // set the real sheet id via the schedule file
		},
	}
}
