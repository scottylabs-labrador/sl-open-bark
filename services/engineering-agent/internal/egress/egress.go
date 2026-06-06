// Package egress implements the Engineering Agent sandbox's default-deny egress allowlist (design
// 12.4, Appendix H): outbound traffic is limited to package registries and the GitHub API; cloud
// metadata endpoints and private network ranges are blocked to prevent SSRF and lateral movement.
// This is a pure decision function the sandbox enforces.
package egress

import (
	"net"
	"strings"
)

// allowedHosts are the exact hosts (and their subdomains) the build is permitted to reach.
var allowedHosts = []string{
	// GitHub (clone + API + release downloads)
	"github.com", "api.github.com", "codeload.github.com", "objects.githubusercontent.com",
	"raw.githubusercontent.com", "ghcr.io",
	// Go
	"proxy.golang.org", "sum.golang.org", "go.dev", "golang.org",
	// npm
	"registry.npmjs.org",
	// Python
	"pypi.org", "files.pythonhosted.org",
	// Rust
	"crates.io", "static.crates.io",
	// Ruby
	"rubygems.org",
	// Debian/Ubuntu (sandbox base image package installs)
	"deb.debian.org", "security.debian.org", "archive.ubuntu.com", "ports.ubuntu.com",
}

// AllowedHosts returns a copy of the egress allowlist, for configuring the sandbox.
func AllowedHosts() []string {
	out := make([]string, len(allowedHosts))
	copy(out, allowedHosts)
	return out
}

// Allowed reports whether the sandbox may make an outbound connection to host. It is default-deny:
// blocked metadata/private/loopback/link-local addresses are refused, and only allowlisted hosts (or
// their subdomains) pass.
func Allowed(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	// If the host is (or resolves to a literal) blocked IP, refuse.
	if ip := net.ParseIP(host); ip != nil {
		return false // never allow raw-IP egress; only named allowlisted hosts
	}
	if blockedName(host) {
		return false
	}
	for _, a := range allowedHosts {
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}

// AllowedIP reports whether a resolved IP is permitted — used to validate at connection time
// (defense against DNS rebinding). Metadata, private, loopback, and link-local are blocked.
func AllowedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return false
	}
	// Cloud metadata endpoints.
	if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("fd00:ec2::254")) {
		return false
	}
	return true
}

// blockedName catches well-known metadata hostnames.
func blockedName(host string) bool {
	switch host {
	case "metadata.google.internal", "metadata", "instance-data", "instance-data.ec2.internal":
		return true
	}
	return strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".local")
}
