// Package authz is the Engineering Agent's maintainer allowlist (design 12.6): who may trigger a
// coding task, and on which opt-in repositories. Default-deny.
package authz

import "strings"

// Allowlist permits a set of maintainers to trigger tasks on a set of opt-in repositories.
type Allowlist struct {
	maintainers map[string]bool
	repos       map[string]bool
}

// New builds an allowlist from maintainer ids and "owner/repo" names.
func New(maintainers, repos []string) *Allowlist {
	a := &Allowlist{maintainers: map[string]bool{}, repos: map[string]bool{}}
	for _, m := range maintainers {
		if m = strings.TrimSpace(m); m != "" {
			a.maintainers[m] = true
		}
	}
	for _, r := range repos {
		if r = strings.TrimSpace(r); r != "" {
			a.repos[r] = true
		}
	}
	return a
}

// Maintainer reports whether user may trigger coding tasks.
func (a *Allowlist) Maintainer(user string) bool { return a.maintainers[user] }

// Repo reports whether repo is opted in.
func (a *Allowlist) Repo(repo string) bool { return a.repos[repo] }

// CanTrigger reports whether user may run a coding task on repo (both must be allowlisted).
func (a *Allowlist) CanTrigger(user, repo string) bool {
	return a.Maintainer(user) && a.Repo(repo)
}
