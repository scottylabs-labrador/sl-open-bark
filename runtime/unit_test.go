package runtime_test

import (
	"testing"

	rt "github.com/scottylabs/scottylabs-agent/runtime"
)

func TestModelSelect(t *testing.T) {
	mc := rt.ModelConfig{Default: "anthropic/claude-sonnet", Escalation: "anthropic/claude-opus"}
	cases := []struct {
		recipeModel string
		hard        bool
		want        string
	}{
		{"", false, "anthropic/claude-sonnet"},  // default
		{"", true, "anthropic/claude-opus"},     // escalate hard tasks
		{"custom/model", true, "custom/model"},  // recipe override wins
		{"custom/model", false, "custom/model"}, // recipe override wins
	}
	for _, c := range cases {
		if got := mc.Select(c.recipeModel, c.hard); got != c.want {
			t.Fatalf("Select(%q, %v) = %q, want %q", c.recipeModel, c.hard, got, c.want)
		}
	}
	// With no escalation model configured, a hard task stays on the default.
	noEsc := rt.ModelConfig{Default: "anthropic/claude-sonnet"}
	if got := noEsc.Select("", true); got != "anthropic/claude-sonnet" {
		t.Fatalf("no escalation configured should keep default, got %q", got)
	}
}

func TestLoadRecipe(t *testing.T) {
	r, err := rt.LoadRecipeByID("testdata", "screen-reimbursement")
	if err != nil {
		t.Fatal(err)
	}
	if r.Title == "" || r.Instructions == "" || r.Owner != "finance-committee" {
		t.Fatalf("recipe not loaded: %+v", r)
	}
	if len(r.Response.RequireHumanApprovalFor) != 1 || r.Response.RequireHumanApprovalFor[0] != "google.gmail.send" {
		t.Fatalf("response block not parsed: %+v", r.Response)
	}

	if _, err := rt.LoadRecipeByID("testdata", "does-not-exist"); err == nil {
		t.Fatal("missing recipe should error")
	}
	// Path traversal must be rejected.
	if _, err := rt.LoadRecipeByID("testdata", "../config"); err == nil {
		t.Fatal("traversal id should be rejected")
	}
}
