// Package limits implements the gateway's cost and rate controls (design 10.2): per-committee and
// global token-bucket rate limiting. A runaway or abused caller hits a wall quickly. The clock is
// injectable so the behavior is deterministic in tests.
package limits

import (
	"sync"
	"time"
)

type bucket struct {
	tokens float64
	last   time.Time
}

// RateLimiter is a keyed token-bucket limiter: each key gets its own bucket refilling at rate
// tokens/sec up to burst.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64 // tokens per second
	burst   float64
	now     func() time.Time
}

// NewRateLimiter allows perMinute requests per key with a burst of burst.
func NewRateLimiter(perMinute, burst int) *RateLimiter {
	if burst < 1 {
		burst = 1
	}
	return &RateLimiter{
		buckets: map[string]*bucket{},
		rate:    float64(perMinute) / 60.0,
		burst:   float64(burst),
		now:     time.Now,
	}
}

// Allow consumes one token for key, refilling first. Returns false when the bucket is empty.
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.now()
	b := r.buckets[key]
	if b == nil {
		b = &bucket{tokens: r.burst, last: now}
		r.buckets[key] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	if elapsed > 0 {
		b.tokens += elapsed * r.rate
		if b.tokens > r.burst {
			b.tokens = r.burst
		}
		b.last = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Limiter combines a per-committee limiter and a global limiter (design 10.2).
type Limiter struct {
	committee *RateLimiter
	global    *RateLimiter
}

// NewLimiter builds a combined limiter. A non-positive rate disables that dimension.
func NewLimiter(committeePerMinute, committeeBurst, globalPerMinute, globalBurst int) *Limiter {
	l := &Limiter{}
	if committeePerMinute > 0 {
		l.committee = NewRateLimiter(committeePerMinute, committeeBurst)
	}
	if globalPerMinute > 0 {
		l.global = NewRateLimiter(globalPerMinute, globalBurst)
	}
	return l
}

// Allow reports whether a call from committee may proceed under both the per-committee and global
// limits. The committee bucket is checked first so a global denial does not burn a committee token.
func (l *Limiter) Allow(committee string) bool {
	if committee == "" {
		committee = "_unknown"
	}
	if l.committee != nil && !l.committee.Allow(committee) {
		return false
	}
	if l.global != nil && !l.global.Allow("_global") {
		return false
	}
	return true
}

// withClock overrides the clock (test helper).
func (r *RateLimiter) withClock(now func() time.Time) *RateLimiter { r.now = now; return r }

// WithClock overrides both limiters' clocks for deterministic tests.
func (l *Limiter) WithClock(now func() time.Time) *Limiter {
	if l.committee != nil {
		l.committee.withClock(now)
	}
	if l.global != nil {
		l.global.withClock(now)
	}
	return l
}
