package scheduler_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/scottylabs/scottylabs-agent/services/scheduler/internal/scheduler"
)

type fakeRT struct {
	submitted  []string
	failRecipe string
}

func (f *fakeRT) Submit(_ context.Context, recipe, _ string, _ map[string]string) (string, error) {
	f.submitted = append(f.submitted, recipe)
	if recipe == f.failRecipe {
		return "", errors.New("runtime unreachable")
	}
	return "task-" + recipe, nil
}

// nextMonday1300 returns the first Monday 13:00:00 UTC on or after a fixed base, so the test does
// not hard-code which calendar date is a Monday.
func nextMonday1300() time.Time {
	t := time.Date(2026, 6, 1, 13, 0, 0, 0, time.UTC)
	for t.Weekday() != time.Monday {
		t = t.AddDate(0, 0, 1)
	}
	return t
}

func names(jobs []scheduler.Job) map[string]bool {
	m := map[string]bool{}
	for _, j := range jobs {
		m[j.Name] = true
	}
	return m
}

func TestDue(t *testing.T) {
	s := scheduler.New(scheduler.DefaultSchedule(), nil)
	mon := nextMonday1300()

	// Monday 13:00 — both the weekly digest and the daily screening fire.
	due, err := s.Due(mon.Add(20 * time.Second)) // mid-minute still counts
	if err != nil {
		t.Fatal(err)
	}
	got := names(due)
	if !got["weekly-leadership-digest"] || !got["daily-reimbursement-screening"] {
		t.Fatalf("Monday 13:00 should fire both, got %v", got)
	}

	// Tuesday 13:00 — only the daily job.
	due, _ = s.Due(mon.AddDate(0, 0, 1))
	got = names(due)
	if got["weekly-leadership-digest"] || !got["daily-reimbursement-screening"] {
		t.Fatalf("Tuesday 13:00 should fire only the daily job, got %v", got)
	}

	// Monday 13:05 — neither (they fire at :00).
	due, _ = s.Due(mon.Add(5 * time.Minute))
	if len(due) != 0 {
		t.Fatalf("13:05 should fire nothing, got %v", names(due))
	}
}

func TestRunDueSurfacesErrors(t *testing.T) {
	rt := &fakeRT{failRecipe: "finance/screen-reimbursement"}
	s := scheduler.New(scheduler.DefaultSchedule(), rt)

	results, err := s.RunDue(context.Background(), nextMonday1300())
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 due jobs run, got %d", len(results))
	}
	var digest, screen scheduler.RunResult
	for _, r := range results {
		switch r.Job {
		case "weekly-leadership-digest":
			digest = r
		case "daily-reimbursement-screening":
			screen = r
		}
	}
	if digest.TaskID == "" || digest.Err != "" {
		t.Fatalf("digest should submit cleanly: %+v", digest)
	}
	if screen.Err == "" {
		t.Fatalf("a failed submit should surface its error, not abort: %+v", screen)
	}
	if len(rt.submitted) != 2 {
		t.Fatalf("both jobs should be submitted even when one fails, got %d", len(rt.submitted))
	}
}

func TestInvalidSpec(t *testing.T) {
	s := scheduler.New([]scheduler.Job{{Name: "bad", Spec: "not a cron", Recipe: "x"}}, nil)
	if _, err := s.Due(nextMonday1300()); err == nil {
		t.Fatal("an invalid cron spec should error")
	}
}
