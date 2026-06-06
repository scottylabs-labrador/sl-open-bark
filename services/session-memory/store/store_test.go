package store_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

// These tests run against a real Postgres. Set TEST_DATABASE_URL (or DATABASE_URL) to a disposable
// database; without one, the whole package is skipped so `make test` stays green on a machine with
// no database. CI provides a Postgres service.
var (
	testDB   *sql.DB
	testRepo *store.Repository
)

func TestMain(m *testing.M) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	if dsn == "" {
		fmt.Println("store: no TEST_DATABASE_URL/DATABASE_URL set; skipping database tests")
		os.Exit(0)
	}

	ctx := context.Background()
	db, err := store.Open(ctx, dsn)
	if err != nil {
		fmt.Println("store: cannot open test database:", err)
		os.Exit(1)
	}
	if err := store.Migrate(db); err != nil {
		fmt.Println("store: migrate:", err)
		os.Exit(1)
	}
	testDB = db
	testRepo = store.New(db)

	code := m.Run()
	_ = db.Close()
	os.Exit(code)
}

// reset truncates every table so each test starts clean.
func reset(t *testing.T) {
	t.Helper()
	_, err := testDB.Exec(`TRUNCATE users, committee_roles, sessions, turns, summaries,
		memory_facts, approvals, audit_log, mcp_servers, mcp_tools, server_committees
		RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("reset: %v", err)
	}
}

func ctx() context.Context { return context.Background() }

func TestUsersAndRoles(t *testing.T) {
	reset(t)
	r := testRepo

	u, err := r.UpsertUser(ctx(), "U1", "Alice", "alice@scottylabs.org")
	if err != nil {
		t.Fatal(err)
	}
	if u.ID == "" || u.SlackID != "U1" || u.Name != "Alice" {
		t.Fatalf("unexpected user: %+v", u)
	}

	// Upsert is idempotent on slack_id and updates name/email.
	u2, err := r.UpsertUser(ctx(), "U1", "Alice B", "ab@scottylabs.org")
	if err != nil {
		t.Fatal(err)
	}
	if u2.ID != u.ID || u2.Name != "Alice B" || u2.Email != "ab@scottylabs.org" {
		t.Fatalf("upsert did not update in place: %+v", u2)
	}

	got, err := r.GetUserBySlackID(ctx(), "U1")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID {
		t.Fatalf("get mismatch: %+v", got)
	}

	if _, err := r.GetUserBySlackID(ctx(), "missing"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}

	if err := r.SetCommitteeRole(ctx(), u.ID, "finance", "member"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetCommitteeRole(ctx(), u.ID, "finance", "lead"); err != nil { // update role
		t.Fatal(err)
	}
	if err := r.SetCommitteeRole(ctx(), u.ID, "events", "member"); err != nil {
		t.Fatal(err)
	}
	roles, err := r.ListCommitteeRoles(ctx(), u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) != 2 {
		t.Fatalf("want 2 roles, got %d: %+v", len(roles), roles)
	}
	if roles[0].Committee != "events" || roles[1].Committee != "finance" || roles[1].Role != "lead" {
		t.Fatalf("unexpected roles: %+v", roles)
	}

	removed, err := r.RemoveCommitteeRole(ctx(), u.ID, "events")
	if err != nil || !removed {
		t.Fatalf("remove role: removed=%v err=%v", removed, err)
	}
}

func TestSessionsTurnsSummaries(t *testing.T) {
	reset(t)
	r := testRepo
	u, _ := r.UpsertUser(ctx(), "U1", "Alice", "")

	s, err := r.CreateSession(ctx(), u.ID, "C1", "1700000000.000100", "screen-reimbursement")
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != "active" || s.UserID != u.ID {
		t.Fatalf("unexpected session: %+v", s)
	}

	// A session with no user resolves UserID to "".
	anon, err := r.CreateSession(ctx(), "", "C1", "t2", "")
	if err != nil {
		t.Fatal(err)
	}
	if anon.UserID != "" {
		t.Fatalf("anon session should have empty UserID, got %q", anon.UserID)
	}

	got, err := r.GetSessionByThread(ctx(), "C1", "1700000000.000100")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != s.ID {
		t.Fatalf("by-thread mismatch: %+v", got)
	}

	for i, role := range []string{"user", "assistant", "user"} {
		if _, err := r.AddTurn(ctx(), s.ID, role, fmt.Sprintf("msg %d", i), i*5); err != nil {
			t.Fatal(err)
		}
	}
	all, err := r.ListTurns(ctx(), s.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || all[0].Content != "msg 0" || all[2].Content != "msg 2" {
		t.Fatalf("turns not chronological: %+v", all)
	}
	last2, err := r.ListTurns(ctx(), s.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(last2) != 2 || last2[0].Content != "msg 1" || last2[1].Content != "msg 2" {
		t.Fatalf("last-2 wrong: %+v", last2)
	}

	if _, err := r.UpsertSummary(ctx(), s.ID, "v1"); err != nil {
		t.Fatal(err)
	}
	sum, err := r.UpsertSummary(ctx(), s.ID, "v2")
	if err != nil {
		t.Fatal(err)
	}
	if sum.Summary != "v2" {
		t.Fatalf("summary not updated: %+v", sum)
	}
	if got, _ := r.GetSummary(ctx(), s.ID); got.Summary != "v2" {
		t.Fatalf("get summary: %+v", got)
	}

	if err := r.UpdateSessionStatus(ctx(), s.ID, "done"); err != nil {
		t.Fatal(err)
	}
	if got, _ := r.GetSession(ctx(), s.ID); got.Status != "done" {
		t.Fatalf("status not updated: %+v", got)
	}
	if err := r.UpdateSessionStatus(ctx(), "00000000-0000-0000-0000-000000000000", "x"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound for missing session, got %v", err)
	}
}

// TestFactScoping is the load-bearing guarantee: a search in one scope never returns another
// scope's facts, even when keys, tags, and values collide.
func TestFactScoping(t *testing.T) {
	reset(t)
	r := testRepo

	mustWrite := func(scopeType, scopeID, key, value string, tags []string) {
		t.Helper()
		if _, err := r.WriteFact(ctx(), store.FactInput{
			ScopeType: scopeType, ScopeID: scopeID, Key: key, Value: value, Tags: tags, Source: "test",
		}); err != nil {
			t.Fatalf("write fact: %v", err)
		}
	}
	// Same key "diet" for two different users, plus a committee and org fact.
	mustWrite(store.ScopeUser, "alice", "diet", "vegetarian", []string{"pref"})
	mustWrite(store.ScopeUser, "bob", "diet", "vegan", []string{"pref"})
	mustWrite(store.ScopeCommittee, "finance", "fiscal_year", "starts July", []string{"policy"})
	mustWrite(store.ScopeOrg, "scottylabs", "name", "ScottyLabs", []string{"org"})

	alice, err := r.SearchFacts(ctx(), store.FactQuery{ScopeType: store.ScopeUser, ScopeID: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if len(alice) != 1 || alice[0].Value != "vegetarian" {
		t.Fatalf("alice should see only her fact, got %+v", alice)
	}

	bob, err := r.SearchFacts(ctx(), store.FactQuery{ScopeType: store.ScopeUser, ScopeID: "bob"})
	if err != nil {
		t.Fatal(err)
	}
	if len(bob) != 1 || bob[0].Value != "vegan" {
		t.Fatalf("bob should see only his fact, got %+v", bob)
	}

	// A committee search must not surface any user fact, and vice versa.
	fin, _ := r.SearchFacts(ctx(), store.FactQuery{ScopeType: store.ScopeCommittee, ScopeID: "finance"})
	if len(fin) != 1 || fin[0].Key != "fiscal_year" {
		t.Fatalf("finance scope leaked: %+v", fin)
	}
	// Same scope_id string but different scope_type is still a different scope.
	crossType, _ := r.SearchFacts(ctx(), store.FactQuery{ScopeType: store.ScopeUser, ScopeID: "finance"})
	if len(crossType) != 0 {
		t.Fatalf("scope_type must partition: user:finance should be empty, got %+v", crossType)
	}

	// Forget-by-key is scoped: forgetting alice's "diet" leaves bob's intact.
	n, err := r.ForgetFactsByKey(ctx(), store.ScopeUser, "alice", "diet")
	if err != nil || n != 1 {
		t.Fatalf("forget by key: n=%d err=%v", n, err)
	}
	bobAfter, _ := r.SearchFacts(ctx(), store.FactQuery{ScopeType: store.ScopeUser, ScopeID: "bob"})
	if len(bobAfter) != 1 {
		t.Fatalf("forgetting alice's key affected bob: %+v", bobAfter)
	}
}

func TestFactTagsRecencyAndExpiry(t *testing.T) {
	reset(t)
	r := testRepo

	f1, _ := r.WriteFact(ctx(), store.FactInput{ScopeType: store.ScopeUser, ScopeID: "u", Key: "a", Value: "1", Tags: []string{"travel"}})
	_, _ = r.WriteFact(ctx(), store.FactInput{ScopeType: store.ScopeUser, ScopeID: "u", Key: "b", Value: "2", Tags: []string{"food"}})

	// Tag filter returns only matching facts.
	travel, err := r.SearchFacts(ctx(), store.FactQuery{ScopeType: store.ScopeUser, ScopeID: "u", Tags: []string{"travel"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(travel) != 1 || travel[0].ID != f1.ID {
		t.Fatalf("tag filter wrong: %+v", travel)
	}
	if len(travel[0].Tags) != 1 || travel[0].Tags[0] != "travel" {
		t.Fatalf("tags did not round-trip: %+v", travel[0].Tags)
	}

	// Limit caps results.
	limited, _ := r.SearchFacts(ctx(), store.FactQuery{ScopeType: store.ScopeUser, ScopeID: "u", Limit: 1})
	if len(limited) != 1 {
		t.Fatalf("limit not applied: %d", len(limited))
	}

	// An already-expired fact is hidden by default, visible with IncludeExpired, and reaped.
	past := time.Now().Add(-time.Hour)
	exp, _ := r.WriteFact(ctx(), store.FactInput{ScopeType: store.ScopeUser, ScopeID: "u", Key: "old", Value: "x", ExpiresAt: &past})
	if exp.ExpiresAt == nil {
		t.Fatal("expires_at not stored")
	}
	visible, _ := r.SearchFacts(ctx(), store.FactQuery{ScopeType: store.ScopeUser, ScopeID: "u"})
	if len(visible) != 2 { // a, b — not the expired one
		t.Fatalf("expired fact should be hidden, got %d: %+v", len(visible), visible)
	}
	withExpired, _ := r.SearchFacts(ctx(), store.FactQuery{ScopeType: store.ScopeUser, ScopeID: "u", IncludeExpired: true})
	if len(withExpired) != 3 {
		t.Fatalf("IncludeExpired should show all, got %d", len(withExpired))
	}
	reaped, err := r.DeleteExpiredFacts(ctx(), time.Now())
	if err != nil || reaped != 1 {
		t.Fatalf("reap expired: n=%d err=%v", reaped, err)
	}

	// ForgetFact reports whether a row was removed.
	ok, _ := r.ForgetFact(ctx(), f1.ID)
	if !ok {
		t.Fatal("forget existing fact returned false")
	}
	if ok, _ := r.ForgetFact(ctx(), f1.ID); ok {
		t.Fatal("forget already-gone fact returned true")
	}
}

func TestApprovals(t *testing.T) {
	reset(t)
	r := testRepo
	s, _ := r.CreateSession(ctx(), "", "C1", "t", "")

	a, err := r.CreateApproval(ctx(), s.ID, "google.gmail.send", json.RawMessage(`{"to":"***"}`))
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != "pending" {
		t.Fatalf("new approval should be pending: %+v", a)
	}

	pending, _ := r.ListPendingApprovals(ctx())
	if len(pending) != 1 {
		t.Fatalf("want 1 pending, got %d", len(pending))
	}

	decided, err := r.DecideApproval(ctx(), a.ID, "approved", "lead@scottylabs.org")
	if err != nil {
		t.Fatal(err)
	}
	if decided.Status != "approved" || decided.DecidedBy != "lead@scottylabs.org" || decided.DecidedAt == nil {
		t.Fatalf("decision not recorded: %+v", decided)
	}
	if pending, _ := r.ListPendingApprovals(ctx()); len(pending) != 0 {
		t.Fatalf("approved item still pending: %d", len(pending))
	}
	if bySession, _ := r.ListApprovalsBySession(ctx(), s.ID); len(bySession) != 1 {
		t.Fatalf("want 1 approval for session, got %d", len(bySession))
	}
	if _, err := r.DecideApproval(ctx(), "00000000-0000-0000-0000-000000000000", "approved", "x"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestAudit(t *testing.T) {
	reset(t)
	r := testRepo

	for i := 0; i < 3; i++ {
		if _, err := r.WriteAudit(ctx(), store.AuditInput{
			Actor: "agent", Tool: "finance.rules.evaluate", Result: "ok", LatencyMS: 12,
			ArgsRedacted: json.RawMessage(fmt.Sprintf(`{"i":%d}`, i)),
		}); err != nil {
			t.Fatal(err)
		}
	}
	recent, err := r.ListAudit(ctx(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(recent) != 3 || recent[0].Tool != "finance.rules.evaluate" {
		t.Fatalf("audit list wrong: %+v", recent)
	}
	// Default args produce a valid JSON object, not empty bytes.
	e, _ := r.WriteAudit(ctx(), store.AuditInput{Actor: "x"})
	if string(e.ArgsRedacted) != "{}" {
		t.Fatalf("empty args should default to {}, got %q", e.ArgsRedacted)
	}

	// Age-out removes rows older than the cutoff.
	aged, err := r.AgeOutAudit(ctx(), time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if aged != 4 {
		t.Fatalf("expected to age out 4 rows, got %d", aged)
	}
}

// TestMigrateRollback proves the migrations roll back cleanly, then restores the schema for any
// later tests in the package.
func TestMigrateRollback(t *testing.T) {
	reset(t)
	if !tableExists(t, "users") {
		t.Fatal("users table should exist before rollback")
	}
	if err := store.Rollback(testDB); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if tableExists(t, "users") || tableExists(t, "audit_log") {
		t.Fatal("tables should be gone after rollback")
	}
	if err := store.Migrate(testDB); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}
	if !tableExists(t, "memory_facts") {
		t.Fatal("schema should be restored after re-migrate")
	}
}

func tableExists(t *testing.T, name string) bool {
	t.Helper()
	var exists bool
	err := testDB.QueryRow(`SELECT EXISTS (
		SELECT 1 FROM information_schema.tables
		WHERE table_schema = 'public' AND table_name = $1)`, name).Scan(&exists)
	if err != nil {
		t.Fatalf("table exists check: %v", err)
	}
	return exists
}
