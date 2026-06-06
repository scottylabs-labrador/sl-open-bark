package egress

import (
	"net"
	"testing"
)

func TestAllowedHosts(t *testing.T) {
	allow := []string{"github.com", "api.github.com", "codeload.github.com", "registry.npmjs.org",
		"proxy.golang.org", "pypi.org", "files.pythonhosted.org", "crates.io", "deb.debian.org"}
	for _, h := range allow {
		if !Allowed(h) {
			t.Errorf("%s should be allowed", h)
		}
	}
	// subdomains of allowed hosts pass
	if !Allowed("fastly.crates.io") {
		t.Error("subdomain of an allowed host should pass")
	}

	deny := []string{
		"evil.com", "metadata.google.internal", "metadata", "foo.internal", "x.local",
		"169.254.169.254", "10.0.0.1", "172.16.5.4", "192.168.1.1", "127.0.0.1", "::1", "",
	}
	for _, h := range deny {
		if Allowed(h) {
			t.Errorf("%s should be blocked", h)
		}
	}
}

func TestAllowedIP(t *testing.T) {
	blocked := []string{"169.254.169.254", "10.1.2.3", "172.20.0.1", "192.168.0.5", "127.0.0.1", "::1", "169.254.1.1"}
	for _, s := range blocked {
		if AllowedIP(net.ParseIP(s)) {
			t.Errorf("IP %s should be blocked", s)
		}
	}
	// A public IP is allowed at the IP layer (the host allowlist is the primary gate).
	if !AllowedIP(net.ParseIP("140.82.112.3")) { // github.com range
		t.Error("a public IP should pass the IP-layer check")
	}
	if AllowedIP(nil) {
		t.Error("nil IP should be blocked")
	}
}
