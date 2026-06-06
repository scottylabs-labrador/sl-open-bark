package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// The security checklist (design 10, WP-12): programmatic, fail-closed invariants over the repo.
//
//  1. Every registered tool declares a scope and an impact (read|write|high).
//  2. Irreversible-looking tools (send/delete/submit/…) are impact:high, so the gateway gates them.
//  3. No committed secrets (a real OpenRouter/Slack/GitHub/AWS credential or a private key).
//
// "The checklist passes" == RunChecklist returns no problems.

var irreversibleHints = []string{
	"send", "delete", "remove", "submit", "book", "pay", "transfer", "purchase", "destroy", "withdraw",
}

// secretPatterns match real credentials, not placeholders (e.g. "sk-or-..." in docs is ignored).
var secretPatterns = map[string]*regexp.Regexp{
	"OpenRouter API key":   regexp.MustCompile(`sk-or-v1-[0-9a-f]{48,}`),
	"Slack bot token":      regexp.MustCompile(`xoxb-[0-9]{8,}-[0-9]{8,}-[0-9A-Za-z]{20,}`),
	"GitHub PAT":           regexp.MustCompile(`gh[pousr]_[0-9A-Za-z]{36,}`),
	"GitHub fine-grained":  regexp.MustCompile(`github_pat_[0-9A-Za-z_]{60,}`),
	"AWS access key":       regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	"private key":          regexp.MustCompile(`-----BEGIN (?:RSA |EC |OPENSSH |PGP )?PRIVATE KEY-----`),
	"Google SA private id": regexp.MustCompile(`"private_key_id"\s*:\s*"[0-9a-f]{40}"`),
}

type clManifest struct {
	Name  string `yaml:"name"`
	Tools []struct {
		Name   string `yaml:"name"`
		Scope  string `yaml:"scope"`
		Impact string `yaml:"impact"`
	} `yaml:"tools"`
}

// RunChecklist evaluates the security invariants over the repo at root and returns a list of
// problems (empty == pass).
func RunChecklist(root string) ([]string, error) {
	var problems []string

	manifests, err := findManifests(root)
	if err != nil {
		return nil, err
	}
	for _, path := range manifests {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var m clManifest
		if err := yaml.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("checklist: parse %s: %w", path, err)
		}
		for _, tool := range m.Tools {
			if tool.Scope == "" {
				problems = append(problems, fmt.Sprintf("%s: tool %q has no scope (fail closed)", m.Name, tool.Name))
			}
			if tool.Impact != "read" && tool.Impact != "write" && tool.Impact != "high" {
				problems = append(problems, fmt.Sprintf("%s: tool %q has invalid impact %q", m.Name, tool.Name, tool.Impact))
			}
			if looksIrreversible(tool.Name) && tool.Impact != "high" {
				problems = append(problems, fmt.Sprintf("%s: irreversible-looking tool %q is impact:%s — must be high (gated)", m.Name, tool.Name, tool.Impact))
			}
		}
	}

	secrets, err := scanSecrets(root)
	if err != nil {
		return nil, err
	}
	problems = append(problems, secrets...)
	return problems, nil
}

func looksIrreversible(name string) bool {
	n := strings.ToLower(name)
	for _, h := range irreversibleHints {
		if strings.Contains(n, h) {
			return true
		}
	}
	return false
}

// scanSecrets walks the repo for committed credentials, skipping VCS, build, and example files.
func scanSecrets(root string) ([]string, error) {
	var problems []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "vendor", "out", "bin":
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".example") || strings.HasSuffix(name, ".sum") {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > 2<<20 { // skip files > 2 MiB
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for label, re := range secretPatterns {
			if re.Match(content) {
				rel, _ := filepath.Rel(root, path)
				problems = append(problems, fmt.Sprintf("COMMITTED SECRET: %s in %s", label, rel))
			}
		}
		return nil
	})
	return problems, err
}
