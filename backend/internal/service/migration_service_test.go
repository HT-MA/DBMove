package service

import (
	"strings"
	"testing"

	"github.com/dbmove/dbmove/backend/internal/model"
)

func TestResolveDatabasePairsMulti(t *testing.T) {
	in := MigrationInput{
		Databases: []model.DatabasePair{
			{Source: "db1", Target: "t1"},
			{Source: "db2", Target: "t2"},
		},
	}
	pairs, err := resolveDatabasePairs(in)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
}

func TestResolveDatabasePairsLegacyFallback(t *testing.T) {
	in := MigrationInput{SourceDatabase: "old", TargetDatabase: "new"}
	pairs, err := resolveDatabasePairs(in)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(pairs) != 1 || pairs[0].Source != "old" || pairs[0].Target != "new" {
		t.Fatalf("legacy fallback mismatch: %+v", pairs)
	}
}

func TestResolveDatabasePairsRejectsInvalid(t *testing.T) {
	cases := []MigrationInput{
		{},
		{Databases: []model.DatabasePair{{Source: "", Target: "t"}}},
		{Databases: []model.DatabasePair{{Source: "s", Target: ""}}},
		{Databases: []model.DatabasePair{{Source: "dup", Target: "a"}, {Source: "dup", Target: "b"}}},
		{Databases: []model.DatabasePair{{Source: "a", Target: "same"}, {Source: "b", Target: "same"}}},
	}
	for i, in := range cases {
		if _, err := resolveDatabasePairs(in); err == nil {
			t.Fatalf("case %d must be rejected", i)
		}
	}
}

func TestBuildDSN(t *testing.T) {
	mysql := &model.Connection{
		Type:     model.ConnTypeMySQL,
		Host:     "10.0.0.10",
		Port:     3306,
		Username: "root",
		SSLMode:  "prefer",
	}
	dsn, err := buildDSN(mysql, "p@ss", "order_db")
	if err != nil {
		t.Fatalf("buildDSN mysql: %v", err)
	}
	if !strings.Contains(dsn, "root:p@ss@tcp(10.0.0.10:3306)/order_db") {
		t.Fatalf("unexpected mysql dsn: %s", dsn)
	}

	pg := &model.Connection{
		Type:     model.ConnTypePostgreSQL,
		Host:     "db.example.com",
		Port:     5432,
		Username: "app",
		SSLMode:  "require",
	}
	dsn, err = buildDSN(pg, "pw", "db1")
	if err != nil {
		t.Fatalf("buildDSN pg: %v", err)
	}
	if !strings.Contains(dsn, "host=db.example.com port=5432 user=app password=pw dbname=db1 sslmode=require") {
		t.Fatalf("unexpected pg dsn: %s", dsn)
	}

	dsn, err = buildDSN(pg, "pw", "")
	if err != nil {
		t.Fatalf("buildDSN pg empty db: %v", err)
	}
	if !strings.Contains(dsn, "dbname=postgres") {
		t.Fatalf("empty pg database must fall back to postgres: %s", dsn)
	}
}

func TestBuildDSNRejectsUnsupportedType(t *testing.T) {
	conn := &model.Connection{Type: "oracle", Host: "h", Port: 1521}
	if _, err := buildDSN(conn, "pw", "db"); err == nil {
		t.Fatalf("unsupported type must be rejected")
	}
}
