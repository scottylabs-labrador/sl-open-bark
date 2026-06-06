package manifest_test

import (
	"testing"

	"github.com/scottylabs/scottylabs-agent/gateway/internal/manifest"
)

const repoRoot = "../../.."

func TestLoadTemplateManifest(t *testing.T) {
	in, err := manifest.Load(repoRoot + "/mcp-servers/_template/scottylabs-mcp-template-go/manifest.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if in.Name != "scottylabs.example" {
		t.Fatalf("unexpected name: %q", in.Name)
	}
	if len(in.Tools) != 2 {
		t.Fatalf("want 2 tools, got %d", len(in.Tools))
	}
	if len(in.Committees) == 0 {
		t.Fatal("expected allowed_committees")
	}
}

func TestLoadRejectsBadManifests(t *testing.T) {
	for _, f := range []string{"testdata/no_scope.yaml", "testdata/bad_impact.yaml"} {
		if _, err := manifest.Load(f); err == nil {
			t.Fatalf("%s should be rejected (fail closed)", f)
		}
	}
}

func TestLoadDir(t *testing.T) {
	servers, err := manifest.LoadDir(repoRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) == 0 {
		t.Fatal("expected to load at least the _template manifest")
	}
}
