package engine

import (
	"strings"
	"testing"
)

func TestParseMajor(t *testing.T) {
	cases := map[string]int{
		"8.0.46":                     8,
		"8.4.0":                      8,
		"16.15":                      16,
		"17.11 (Debian 17.11-1)":     17,
		"5.5.5-10.11.8-MariaDB":      10,
		"PostgreSQL 16.15 on x86_64": 16,
		"not-a-version":              0,
		"":                           0,
	}
	for in, want := range cases {
		if got := parseMajor(in); got != want {
			t.Fatalf("parseMajor(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestMySQLVersionCompat(t *testing.T) {
	if w, f := mysqlVersionCompat("8.0.46", "8.0.36"); w != "" || f != "" {
		t.Fatalf("same major must pass: %q %q", w, f)
	}
	if _, f := mysqlVersionCompat("9.0.0", "8.0.36"); f == "" {
		t.Fatalf("target older than source must fail")
	}
	w, f := mysqlVersionCompat("8.0.36", "9.0.0")
	if f != "" || w == "" {
		t.Fatalf("target newer than source must warn: %q %q", w, f)
	}
	if w, f := mysqlVersionCompat("unknown", "8.0.36"); w == "" || f != "" {
		t.Fatalf("unparseable versions must warn, not fail")
	}
}

func TestPGVersionCompat(t *testing.T) {
	if w, f := pgVersionCompat(16, 16, 16); w != "" || f != "" {
		t.Fatalf("matching versions must pass: %q %q", w, f)
	}
	if _, f := pgVersionCompat(16, 17, 17); f == "" || !strings.Contains(f, "cannot dump") {
		t.Fatalf("client older than source must fail: %q", f)
	}
	if _, f := pgVersionCompat(17, 17, 16); f == "" || !strings.Contains(f, "older than source") {
		t.Fatalf("target older than source must fail: %q", f)
	}
	w, f := pgVersionCompat(17, 16, 17)
	if f != "" || w == "" {
		t.Fatalf("upgrade should warn: %q %q", w, f)
	}
	if w, f := pgVersionCompat(0, 16, 16); w == "" || f != "" {
		t.Fatalf("unknown client version must warn, not fail")
	}
}
