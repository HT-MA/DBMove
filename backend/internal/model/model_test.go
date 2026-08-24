package model

import (
	"encoding/json"
	"testing"
)

func TestDatabaseMappingValue(t *testing.T) {
	m := DatabaseMapping{{Source: "a", Target: "b"}}
	v, err := m.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	s, ok := v.(string)
	if !ok {
		t.Fatalf("Value must return a string, got %T", v)
	}
	var parsed []DatabasePair
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Fatalf("Value is not valid JSON: %v", err)
	}
	if len(parsed) != 1 || parsed[0].Source != "a" || parsed[0].Target != "b" {
		t.Fatalf("unexpected JSON: %s", s)
	}
}

func TestDatabaseMappingScan(t *testing.T) {
	var m DatabaseMapping
	if err := m.Scan([]byte(`[{"source":"x","target":"y"}]`)); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(m) != 1 || m[0].Source != "x" || m[0].Target != "y" {
		t.Fatalf("unexpected mapping: %+v", m)
	}
}

func TestDatabaseMappingScanStringAndNil(t *testing.T) {
	var m DatabaseMapping
	if err := m.Scan(`[{"source":"a","target":"b"}]`); err != nil {
		t.Fatalf("Scan string: %v", err)
	}
	if len(m) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(m))
	}
	var empty DatabaseMapping
	if err := empty.Scan(nil); err != nil {
		t.Fatalf("Scan nil: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("nil scan must produce empty mapping")
	}
}

func TestTaskDatabasePairsFallback(t *testing.T) {
	task := &MigrationTask{SourceDatabase: "legacy_src", TargetDatabase: "legacy_tgt"}
	pairs := task.DatabasePairs()
	if len(pairs) != 1 || pairs[0].Source != "legacy_src" || pairs[0].Target != "legacy_tgt" {
		t.Fatalf("legacy fallback failed: %+v", pairs)
	}

	task2 := &MigrationTask{
		SourceDatabase: "legacy_src",
		TargetDatabase: "legacy_tgt",
		Databases:      DatabaseMapping{{Source: "a", Target: "b"}, {Source: "c", Target: "d"}},
	}
	pairs = task2.DatabasePairs()
	if len(pairs) != 2 {
		t.Fatalf("mapping must take precedence over legacy fields")
	}
}

func TestTaskDatabasePairsEmpty(t *testing.T) {
	task := &MigrationTask{}
	if pairs := task.DatabasePairs(); pairs != nil {
		t.Fatalf("expected nil pairs, got %+v", pairs)
	}
}
