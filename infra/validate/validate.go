// Package main implements slvalidate: the CI gate that checks every MCP server manifest and every
// recipe against its JSON Schema (the registry and workflow contracts from design Section 7).
//
// The logic is split so it is testable without the CLI: loadSchema compiles a schema, validateFile
// checks one YAML document against it, and the find* helpers locate the files in the repo. Side
// effects (reading files, printing, exit codes) stay in main.go.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"
)

// loadSchema compiles a JSON Schema from a file on disk. It registers the document under its own
// $id (falling back to a synthetic URL) so $ref resolution inside the schema works.
func loadSchema(path string) (*jsonschema.Schema, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read schema %s: %w", path, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("parse schema %s: %w", path, err)
	}
	id := "mem:///schema"
	if m, ok := doc.(map[string]any); ok {
		if s, ok := m["$id"].(string); ok && s != "" {
			id = s
		}
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource(id, doc); err != nil {
		return nil, fmt.Errorf("add schema %s: %w", path, err)
	}
	sch, err := c.Compile(id)
	if err != nil {
		return nil, fmt.Errorf("compile schema %s: %w", path, err)
	}
	return sch, nil
}

// validateFile checks a single YAML document against the schema. YAML is decoded to a plain Go
// value, then round-tripped through JSON so numbers and maps match what the validator expects.
func validateFile(schema *jsonschema.Schema, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	var parsed any
	if err := yaml.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("parse YAML %s: %w", path, err)
	}
	jb, err := json.Marshal(parsed)
	if err != nil {
		return fmt.Errorf("normalize %s: %w", path, err)
	}
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(jb))
	if err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	if err := schema.Validate(inst); err != nil {
		return err
	}
	return nil
}

// findManifests returns every mcp-servers/**/manifest.yaml under root, sorted for stable output.
func findManifests(root string) ([]string, error) {
	return findFiles(filepath.Join(root, "mcp-servers"), func(d fs.DirEntry) bool {
		return d.Name() == "manifest.yaml"
	})
}

// findRecipes returns every recipes/**/*.yaml or *.yml under root, sorted for stable output.
func findRecipes(root string) ([]string, error) {
	return findFiles(filepath.Join(root, "recipes"), func(d fs.DirEntry) bool {
		ext := filepath.Ext(d.Name())
		return ext == ".yaml" || ext == ".yml"
	})
}

// findFiles walks dir and returns the paths of files matching keep. A missing dir is not an error
// (it just yields no files), so the validator passes cleanly before any servers or recipes exist.
func findFiles(dir string, keep func(fs.DirEntry) bool) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return fs.SkipDir
			}
			return err
		}
		if !d.IsDir() && keep(d) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// validateAll validates every file against the schema and returns one error per failing file.
func validateAll(schema *jsonschema.Schema, files []string) []error {
	var problems []error
	for _, f := range files {
		if err := validateFile(schema, f); err != nil {
			problems = append(problems, fmt.Errorf("%s:\n%w", f, err))
		}
	}
	return problems
}
