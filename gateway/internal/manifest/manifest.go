// Package manifest loads MCP server manifest.yaml files (the registry contract, design Section
// 7.4) into the registry's ServerInput. Loading is fail-closed: a tool with no scope or an invalid
// impact is rejected, so a misdeclared capability never registers.
package manifest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/scottylabs/scottylabs-agent/services/session-memory/store"
)

type manifestFile struct {
	Name        string `yaml:"name"`
	Owner       string `yaml:"owner"`
	Description string `yaml:"description"`
	Endpoint    string `yaml:"endpoint"`
	Transport   string `yaml:"transport"`
	Auth        string `yaml:"auth"`
	Tools       []struct {
		Name   string `yaml:"name"`
		Scope  string `yaml:"scope"`
		Impact string `yaml:"impact"`
	} `yaml:"tools"`
	AllowedCommittees []string `yaml:"allowed_committees"`
	Lifecycle         string   `yaml:"lifecycle"`
}

var validImpact = map[string]bool{"read": true, "write": true, "high": true}

// Load parses and validates one manifest.yaml into a ServerInput.
func Load(path string) (store.ServerInput, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return store.ServerInput{}, fmt.Errorf("manifest: read %s: %w", path, err)
	}
	var m manifestFile
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return store.ServerInput{}, fmt.Errorf("manifest: parse %s: %w", path, err)
	}
	return validate(m, path)
}

func validate(m manifestFile, path string) (store.ServerInput, error) {
	if m.Name == "" {
		return store.ServerInput{}, fmt.Errorf("manifest %s: name is required", path)
	}
	if m.Endpoint == "" {
		return store.ServerInput{}, fmt.Errorf("manifest %s: endpoint is required", path)
	}
	if len(m.Tools) == 0 {
		return store.ServerInput{}, fmt.Errorf("manifest %s: at least one tool is required", path)
	}
	in := store.ServerInput{
		Name: m.Name, Owner: m.Owner, Description: m.Description, Endpoint: m.Endpoint,
		Transport: m.Transport, Auth: m.Auth, Committees: m.AllowedCommittees,
	}
	for i, t := range m.Tools {
		if t.Name == "" {
			return store.ServerInput{}, fmt.Errorf("manifest %s: tool %d has no name", path, i)
		}
		if t.Scope == "" {
			return store.ServerInput{}, fmt.Errorf("manifest %s: tool %q has no scope (fail closed)", path, t.Name)
		}
		if !validImpact[t.Impact] {
			return store.ServerInput{}, fmt.Errorf("manifest %s: tool %q has invalid impact %q (read|write|high)", path, t.Name, t.Impact)
		}
		in.Tools = append(in.Tools, store.ToolInput{Name: t.Name, Scope: t.Scope, Impact: t.Impact})
	}
	return in, nil
}

// LoadDir walks root for every mcp-servers/**/manifest.yaml and loads them. Used to seed the
// registry from the monorepo. Returns the loaded servers in sorted path order.
func LoadDir(root string) ([]store.ServerInput, error) {
	var paths []string
	serversDir := filepath.Join(root, "mcp-servers")
	err := filepath.WalkDir(serversDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return fs.SkipDir
			}
			return err
		}
		if !d.IsDir() && d.Name() == "manifest.yaml" {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var out []store.ServerInput
	for _, p := range paths {
		in, err := Load(p)
		if err != nil {
			return nil, err
		}
		out = append(out, in)
	}
	return out, nil
}
