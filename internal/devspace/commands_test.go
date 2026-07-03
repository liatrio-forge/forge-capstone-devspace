package devspace

import (
	"strings"
	"testing"
)

func TestResolveServeAddrRejectsExplicitPublicWithoutOptIn(t *testing.T) {
	_, err := resolveServeAddr("0.0.0.0:8787", true, "", false)
	if err == nil || !strings.Contains(err.Error(), "refusing to bind public address") {
		t.Fatalf("expected public-bind guard error, got %v", err)
	}
}

func TestResolveServeAddrAllowsPublicWithFlag(t *testing.T) {
	got, err := resolveServeAddr("0.0.0.0:8787", true, "", true)
	if err != nil {
		t.Fatalf("unexpected error with --allow-public-http: %v", err)
	}
	if got.addr != "0.0.0.0:8787" {
		t.Fatalf("addr = %q", got.addr)
	}
	if !got.public {
		t.Fatal("expected public=true for 0.0.0.0 bind")
	}
}

func TestResolveServeAddrAllowsPublicWithPortEnv(t *testing.T) {
	// PORT env implies a PaaS/proxy that terminates TLS upstream, so a public
	// bind is sanctioned without --allow-public-http.
	got, err := resolveServeAddr("127.0.0.1:8787", false, "8080", false)
	if err != nil {
		t.Fatalf("unexpected error with PORT env: %v", err)
	}
	if got.addr != "0.0.0.0:8080" {
		t.Fatalf("addr = %q, want 0.0.0.0:8080", got.addr)
	}
	if !got.public {
		t.Fatal("expected public=true for PORT-driven 0.0.0.0 bind")
	}
}

func TestResolveServeAddrAllowsLoopback(t *testing.T) {
	got, err := resolveServeAddr("127.0.0.1:8787", true, "", false)
	if err != nil {
		t.Fatalf("loopback bind should not require opt-in: %v", err)
	}
	if got.public {
		t.Fatal("expected public=false for loopback bind")
	}
	if got.addr != "127.0.0.1:8787" {
		t.Fatalf("addr = %q", got.addr)
	}
}

func TestResolveServeAddrRejectsInvalidAddr(t *testing.T) {
	if _, err := resolveServeAddr("not-a-valid-addr", true, "", false); err == nil {
		t.Fatal("expected error for invalid listen address")
	}
}
