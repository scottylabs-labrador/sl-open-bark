package main

import (
	"testing"
)

const (
	repoRoot       = "../.."
	manifestSchema = repoRoot + "/mcp-servers/manifest.schema.json"
	recipeSchema   = repoRoot + "/recipes/recipe.schema.json"
)

func TestManifestSchema(t *testing.T) {
	schema, err := loadSchema(manifestSchema)
	if err != nil {
		t.Fatalf("load manifest schema: %v", err)
	}
	cases := []struct {
		file string
		want bool // true = should validate
	}{
		{"testdata/manifest_valid.yaml", true},
		{"testdata/manifest_invalid_missing_scope.yaml", false},
		{"testdata/manifest_invalid_bad_impact.yaml", false},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			err := validateFile(schema, c.file)
			if c.want && err != nil {
				t.Fatalf("expected valid, got error: %v", err)
			}
			if !c.want && err == nil {
				t.Fatalf("expected invalid, but validation passed")
			}
		})
	}
}

func TestRecipeSchema(t *testing.T) {
	schema, err := loadSchema(recipeSchema)
	if err != nil {
		t.Fatalf("load recipe schema: %v", err)
	}
	cases := []struct {
		file string
		want bool
	}{
		{"testdata/recipe_valid.yaml", true},
		{"testdata/recipe_invalid_missing_owner.yaml", false},
		{"testdata/recipe_invalid_bad_extension.yaml", false},
	}
	for _, c := range cases {
		t.Run(c.file, func(t *testing.T) {
			err := validateFile(schema, c.file)
			if c.want && err != nil {
				t.Fatalf("expected valid, got error: %v", err)
			}
			if !c.want && err == nil {
				t.Fatalf("expected invalid, but validation passed")
			}
		})
	}
}

// TestTemplateManifestValidates is the integration check that the shipped schema matches the real
// _template manifest. If someone changes one without the other, CI fails here.
func TestTemplateManifestValidates(t *testing.T) {
	schema, err := loadSchema(manifestSchema)
	if err != nil {
		t.Fatalf("load schema: %v", err)
	}
	manifests, err := findManifests(repoRoot)
	if err != nil {
		t.Fatalf("find manifests: %v", err)
	}
	if len(manifests) == 0 {
		t.Fatal("expected to find at least the _template manifest, found none")
	}
	for _, m := range manifests {
		if err := validateFile(schema, m); err != nil {
			t.Errorf("%s should validate: %v", m, err)
		}
	}
}

// TestRunOverRepo exercises the CLI entry over the real tree: the template manifest is valid and
// there are no invalid recipes, so both commands succeed.
func TestRunOverRepo(t *testing.T) {
	if err := run([]string{"manifests", repoRoot}); err != nil {
		t.Errorf("manifests: %v", err)
	}
	if err := run([]string{"recipes", repoRoot}); err != nil {
		t.Errorf("recipes: %v", err)
	}
	if err := run([]string{"all", repoRoot}); err != nil {
		t.Errorf("all: %v", err)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	if err := run([]string{"bogus"}); err == nil {
		t.Fatal("expected error for unknown command")
	}
	if err := run(nil); err == nil {
		t.Fatal("expected error for no command")
	}
}
