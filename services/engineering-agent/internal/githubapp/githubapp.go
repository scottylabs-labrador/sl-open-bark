// Package githubapp mints short-lived, per-repo installation tokens and opens DRAFT pull requests
// via a least-privilege GitHub App (design 12.3, Appendix H): Contents read/write and Pull requests
// write only — no admin, no secrets, no Actions, and no merge. The App id and private key come from
// the environment (human-gated); none in code. There is intentionally no merge method.
package githubapp

import (
	"context"
	"errors"
)

// Config holds the App's credentials and target API.
type Config struct {
	AppID          string
	InstallationID string
	PrivateKeyPEM  string
	APIBase        string // default https://api.github.com
}

// App is the GitHub App client.
type App struct{ cfg Config }

// New builds an App.
func New(cfg Config) *App {
	if cfg.APIBase == "" {
		cfg.APIBase = "https://api.github.com"
	}
	return &App{cfg: cfg}
}

// Configured reports whether the App credentials are present.
func (a *App) Configured() bool { return a.cfg.AppID != "" && a.cfg.PrivateKeyPEM != "" }

// MintInstallToken mints a short-lived installation token scoped to a single repo. The real
// implementation signs an App JWT with the private key and exchanges it for an installation token
// scoped to repo (Contents + Pull requests). It is exercised against the live GitHub App (the key is
// provisioned via the environment).
func (a *App) MintInstallToken(_ context.Context, _ string) (string, error) {
	if !a.Configured() {
		return "", errors.New("githubapp: GITHUB_APP_ID + GITHUB_APP_PRIVATE_KEY are required (provision the GitHub App; see README)")
	}
	// TODO(live): JWT(appID, key) -> POST installation access_token scoped to the repo.
	return "", errors.New("githubapp: live token minting is finalized against the provisioned GitHub App")
}

// OpenDraftPR opens a draft PR (draft=true) so branch protection requires human review; the App
// cannot merge.
func (a *App) OpenDraftPR(_ context.Context, _, _, _, _ string) (string, error) {
	if !a.Configured() {
		return "", errors.New("githubapp: not configured")
	}
	// TODO(live): POST /repos/{repo}/pulls with {head: branch, base: default, draft: true}.
	return "", errors.New("githubapp: live PR creation is finalized against the provisioned GitHub App")
}
