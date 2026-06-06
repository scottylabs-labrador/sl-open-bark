package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Recipe is the loaded form of a workflow file (recipes/<committee>/<name>.yaml), matching the
// recipe schema. Model is an optional per-recipe override (design 5.5).
type Recipe struct {
	Version      any               `yaml:"version"`
	Title        string            `yaml:"title"`
	Description  string            `yaml:"description"`
	Owner        string            `yaml:"owner"`
	Model        string            `yaml:"model"`
	Parameters   []RecipeParam     `yaml:"parameters"`
	Extensions   []RecipeExtension `yaml:"extensions"`
	Instructions string            `yaml:"instructions"`
	Response     RecipeResponse    `yaml:"response"`
}

type RecipeParam struct {
	Key         string `yaml:"key"`
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
}

type RecipeExtension struct {
	Gateway string `yaml:"gateway"`
}

type RecipeResponse struct {
	RequireHumanApprovalFor []string `yaml:"require_human_approval_for"`
}

// LoadRecipe parses and lightly validates a recipe file (the gateway/CI enforce the full schema).
func LoadRecipe(path string) (Recipe, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Recipe{}, fmt.Errorf("runtime: read recipe %s: %w", path, err)
	}
	var r Recipe
	if err := yaml.Unmarshal(raw, &r); err != nil {
		return Recipe{}, fmt.Errorf("runtime: parse recipe %s: %w", path, err)
	}
	if r.Title == "" || r.Instructions == "" || r.Owner == "" {
		return Recipe{}, fmt.Errorf("runtime: recipe %s missing title, owner, or instructions", path)
	}
	return r, nil
}

// LoadRecipeByID resolves a recipe id like "finance/screen-reimbursement" to
// <recipesDir>/finance/screen-reimbursement.yaml (or .yml) and loads it. The id is sanitized so it
// cannot escape recipesDir.
func LoadRecipeByID(recipesDir, id string) (Recipe, error) {
	clean := filepath.Clean("/" + id) // anchor, then strip leading slash to defeat ../ traversal
	clean = strings.TrimPrefix(clean, string(filepath.Separator))
	if clean == "" || strings.Contains(clean, "..") {
		return Recipe{}, fmt.Errorf("runtime: invalid recipe id %q", id)
	}
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(recipesDir, clean+ext)
		if _, err := os.Stat(path); err == nil {
			return LoadRecipe(path)
		}
	}
	return Recipe{}, fmt.Errorf("runtime: recipe %q not found under %s", id, recipesDir)
}
