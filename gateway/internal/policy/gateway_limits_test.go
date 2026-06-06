package policy_test

import (
	"context"
	"errors"
	"testing"

	"github.com/scottylabs/scottylabs-agent/gateway/internal/limits"
	"github.com/scottylabs/scottylabs-agent/gateway/internal/policy"
)

// A caller that exceeds its per-committee rate limit is denied before any work, and the denial is
// audited (design 10.2).
func TestRateLimited(t *testing.T) {
	_, fs, caller := setup(t) // google registered + approved, committee finance
	lim := limits.NewLimiter(1, 1, 1000, 1000)
	gw := policy.New(fs, caller, policy.WithLimiter(lim))
	req := policy.CallRequest{Identity: financeID(), ServerName: "google", ToolName: "sheets_read"}

	if _, err := gw.Call(context.Background(), req); err != nil {
		t.Fatalf("first call (within burst) should pass: %v", err)
	}
	if _, err := gw.Call(context.Background(), req); !errors.Is(err, policy.ErrRateLimited) {
		t.Fatalf("second call should be rate limited, got %v", err)
	}
	if caller.calls != 1 {
		t.Fatalf("a rate-limited call must not reach the downstream tool, got %d calls", caller.calls)
	}
}
