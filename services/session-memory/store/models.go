package store

import (
	"encoding/json"
	"time"
)

// User is a Slack-identified person the agent acts for.
type User struct {
	ID        string    `json:"id"`
	SlackID   string    `json:"slack_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// CommitteeRole is one membership, e.g. {Committee: "finance", Role: "member"}.
type CommitteeRole struct {
	UserID    string    `json:"user_id"`
	Committee string    `json:"committee"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// Session is an in-progress task bound to a Slack thread. UserID is empty if not yet resolved.
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Channel   string    `json:"channel"`
	ThreadTS  string    `json:"thread_ts"`
	Recipe    string    `json:"recipe"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Turn is one message in a session's conversation.
type Turn struct {
	ID        int64     `json:"id"`
	SessionID string    `json:"session_id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Tokens    int       `json:"tokens"`
	CreatedAt time.Time `json:"created_at"`
}

// Summary is the rolling summary of a session's thread.
type Summary struct {
	SessionID string    `json:"session_id"`
	Summary   string    `json:"summary"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Fact is a durable, scoped memory. ExpiresAt is nil when the fact does not expire.
type Fact struct {
	ID        string     `json:"id"`
	ScopeType string     `json:"scope_type"`
	ScopeID   string     `json:"scope_id"`
	Key       string     `json:"key"`
	Value     string     `json:"value"`
	Tags      []string   `json:"tags"`
	Source    string     `json:"source"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// FactInput is the payload for writing a fact.
type FactInput struct {
	ScopeType string
	ScopeID   string
	Key       string
	Value     string
	Tags      []string
	Source    string
	ExpiresAt *time.Time
}

// FactQuery scopes and filters a memory search. ScopeType and ScopeID are required and are always
// applied, so a search can never cross into another scope. Tags, when set, match facts carrying
// any of them. Expired facts are excluded unless IncludeExpired is set.
type FactQuery struct {
	ScopeType      string
	ScopeID        string
	Tags           []string
	Limit          int
	IncludeExpired bool
}

// Approval is a human-in-the-loop decision record for a high-impact action.
type Approval struct {
	ID           string          `json:"id"`
	SessionID    string          `json:"session_id"`
	Tool         string          `json:"tool"`
	ArgsRedacted json.RawMessage `json:"args_redacted"`
	Status       string          `json:"status"`
	DecidedBy    string          `json:"decided_by"`
	DecidedAt    *time.Time      `json:"decided_at,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
}

// AuditEntry is one logged side effect.
type AuditEntry struct {
	ID           int64           `json:"id"`
	Actor        string          `json:"actor"`
	Tool         string          `json:"tool"`
	ArgsRedacted json.RawMessage `json:"args_redacted"`
	Result       string          `json:"result"`
	LatencyMS    int             `json:"latency_ms"`
	CreatedAt    time.Time       `json:"created_at"`
}

// AuditInput is the payload for writing an audit entry.
type AuditInput struct {
	Actor        string
	Tool         string
	ArgsRedacted json.RawMessage
	Result       string
	LatencyMS    int
}
