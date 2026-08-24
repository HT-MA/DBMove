package redact

import (
	"strings"
	"testing"
)

func TestRedactRemovesKnownSecrets(t *testing.T) {
	r := New("pass123", "very-secret")
	out := r("source pass123 target very-secret done")
	if strings.Contains(out, "pass123") || strings.Contains(out, "very-secret") {
		t.Fatalf("secrets leaked: %q", out)
	}
}

func TestRedactMasksCredentialPatterns(t *testing.T) {
	r := New()
	out := r("password=abc123 mysql://user:pw@host/db")
	if strings.Contains(out, "abc123") || strings.Contains(out, "pw@") {
		t.Fatalf("credentials leaked: %q", out)
	}
}

func TestRedactKeepsOrdinaryText(t *testing.T) {
	r := New("secret")
	in := "restoring into mysql-target:3306/target_db"
	if out := r(in); out != in {
		t.Fatalf("ordinary text changed: %q", out)
	}
}
