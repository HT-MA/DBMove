package redact

import (
	"strings"
	"testing"
)

func TestRedactRemovesKnownSecrets(t *testing.T) {
	r := New("hunter2", "my-long-token")
	msg := "connecting with hunter2 and token my-long-token"
	out := r(msg)
	if strings.Contains(out, "hunter2") || strings.Contains(out, "my-long-token") {
		t.Fatalf("known secrets must be removed: %q", out)
	}
}

func TestRedactMasksKeyValuePatterns(t *testing.T) {
	r := New()
	cases := []string{
		"password=abc123",
		"passwd: xyz",
		"token = tok_123",
		"authorization=Bearer abc.def",
	}
	for _, c := range cases {
		out := r(c)
		if strings.Contains(out, "abc123") || strings.Contains(out, "xyz") ||
			strings.Contains(out, "tok_123") || strings.Contains(out, "abc.def") {
			t.Fatalf("credential value leaked: %q -> %q", c, out)
		}
	}
}

func TestRedactMasksDSNCredentials(t *testing.T) {
	r := New()
	msg := "mysql://root:supersecret@10.0.0.1:3306/db"
	out := r(msg)
	if strings.Contains(out, "supersecret") {
		t.Fatalf("dsn password leaked: %q", out)
	}
	if !strings.Contains(out, "******@10.0.0.1") {
		t.Fatalf("dsn password not masked: %q", out)
	}
}

func TestRedactKeepsOrdinaryText(t *testing.T) {
	r := New("secret1")
	in := "starting dump of database source_db"
	if out := r(in); out != in {
		t.Fatalf("ordinary text must be unchanged: %q", out)
	}
}
