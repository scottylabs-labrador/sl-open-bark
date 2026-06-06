package authz_test

import (
	"testing"

	"github.com/scottylabs/scottylabs-agent/services/engineering-agent/internal/authz"
)

func TestCanTrigger(t *testing.T) {
	a := authz.New([]string{"alice", "bob"}, []string{"scottylabs-labrador/site", "scottylabs-labrador/api"})

	if !a.CanTrigger("alice", "scottylabs-labrador/site") {
		t.Fatal("allowlisted maintainer + opt-in repo should be allowed")
	}
	if a.CanTrigger("alice", "scottylabs-labrador/secret-repo") {
		t.Fatal("repo not opted in must be denied")
	}
	if a.CanTrigger("eve", "scottylabs-labrador/site") {
		t.Fatal("non-maintainer must be denied")
	}
	if a.CanTrigger("", "") {
		t.Fatal("empty must be denied (default-deny)")
	}
}
