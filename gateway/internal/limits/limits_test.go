package limits

import (
	"testing"
	"time"
)

func TestRateLimiterBurstAndRefill(t *testing.T) {
	cur := time.Unix(1000, 0)
	rl := NewRateLimiter(60, 3).withClock(func() time.Time { return cur }) // 1 token/sec, burst 3

	for i := 0; i < 3; i++ {
		if !rl.Allow("k") {
			t.Fatalf("burst token %d should be allowed", i)
		}
	}
	if rl.Allow("k") {
		t.Fatal("4th call should be denied (burst exhausted)")
	}

	cur = cur.Add(time.Second) // refill exactly one token
	if !rl.Allow("k") {
		t.Fatal("after 1s, one token should be available")
	}
	if rl.Allow("k") {
		t.Fatal("only one token should have refilled")
	}

	if !rl.Allow("other-key") {
		t.Fatal("a different key has its own full bucket")
	}
}

func TestLimiterPerCommittee(t *testing.T) {
	cur := time.Unix(1000, 0)
	l := NewLimiter(2, 2, 1000, 1000).WithClock(func() time.Time { return cur })

	if !l.Allow("finance") {
		t.Fatal("finance call 1 should pass (burst)")
	}
	if !l.Allow("finance") {
		t.Fatal("finance call 2 should pass (burst of 2)")
	}
	if l.Allow("finance") {
		t.Fatal("finance's 3rd call should be rate limited")
	}
	if !l.Allow("events") {
		t.Fatal("events has an independent per-committee bucket")
	}
}

func TestLimiterGlobalCap(t *testing.T) {
	cur := time.Unix(1000, 0)
	l := NewLimiter(1000, 1000, 100, 2).WithClock(func() time.Time { return cur }) // global burst 2

	if !l.Allow("a") || !l.Allow("b") {
		t.Fatal("the first two calls (different committees) should pass the global cap")
	}
	if l.Allow("c") {
		t.Fatal("a global burst of 2 should rate limit the third call regardless of committee")
	}
}

func TestLimiterDisabledDimensions(t *testing.T) {
	l := NewLimiter(0, 0, 0, 0) // both disabled
	for i := 0; i < 100; i++ {
		if !l.Allow("anything") {
			t.Fatal("a limiter with no configured rates allows everything")
		}
	}
}
