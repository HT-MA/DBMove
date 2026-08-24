package engine

import (
	"strings"
	"testing"

	"github.com/dbmove/dbmove/worker/internal/reporter"
)

func TestTaskConfigDatabasePairs(t *testing.T) {
	cfg := &TaskConfig{Databases: []DatabasePair{{Source: "a", Target: "b"}, {Source: "c", Target: "d"}}}
	if pairs := cfg.DatabasePairs(); len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}

	legacy := &TaskConfig{Source: ConnectionInfo{Database: "s"}, Target: ConnectionInfo{Database: "t"}}
	pairs := legacy.DatabasePairs()
	if len(pairs) != 1 || pairs[0].Source != "s" || pairs[0].Target != "t" {
		t.Fatalf("legacy fallback mismatch: %+v", pairs)
	}
}

func TestMySQLProgressPercent(t *testing.T) {
	p := &mysqlProgress{tablesTotal: 4}
	if got := p.percent(); got != 0 {
		t.Fatalf("expected 0%%, got %d", got)
	}
	p.tablesCompleted = 2
	if got := p.percent(); got != 50 {
		t.Fatalf("expected 50%%, got %d", got)
	}
	p.tablesCompleted = 4
	if got := p.percent(); got != 100 {
		t.Fatalf("expected 100%%, got %d", got)
	}
	empty := &mysqlProgress{}
	if got := empty.percent(); got != 0 {
		t.Fatalf("empty progress must be 0, got %d", got)
	}
}

func TestMySQLProgressClampsAt99DuringDump(t *testing.T) {
	reported := make([]int, 0, 4)
	p := &mysqlProgress{rep: &fakeReporter{onProgress: func(v int) { reported = append(reported, v) }}}
	p.tablesTotal = 2
	p.tablesCompleted = 1
	p.reportWith(1) // 1 completed + 1 in-flight = 2/2 -> clamped to 99
	if len(reported) != 1 || reported[0] != 99 {
		t.Fatalf("expected clamp to 99, got %v", reported)
	}
}

func TestPGProgressPercent(t *testing.T) {
	p := &pgProgress{tablesTotal: 3, tablesCompleted: 1}
	if got := p.percent(); got != 33 {
		t.Fatalf("expected 33%%, got %d", got)
	}
	if got := (&pgProgress{}).percent(); got != 0 {
		t.Fatalf("empty progress must be 0")
	}
}

func TestTailSuffix(t *testing.T) {
	if s := tailSuffix(nil); s != "" {
		t.Fatalf("nil tail must produce empty suffix, got %q", s)
	}
	short := tailSuffix([]string{"a", "b"})
	if short != ": a | b" {
		t.Fatalf("unexpected short suffix: %q", short)
	}
	long := tailSuffix([]string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "10"})
	if strings.Count(long, "|") != 7 {
		t.Fatalf("long tail must be truncated to 8 lines: %q", long)
	}
	if !strings.Contains(long, "10") || strings.Contains(long, "1 |") {
		t.Fatalf("truncation must keep the newest lines: %q", long)
	}
}

// fakeReporter records progress updates for tests.
type fakeReporter struct {
	onProgress func(int)
}

func (f *fakeReporter) Log(level, format string, args ...any) {}
func (f *fakeReporter) Progress(p reporter.ProgressUpdate) {
	if f.onProgress != nil {
		f.onProgress(p.Progress)
	}
}
func (f *fakeReporter) Status(status, errMsg string) {}
func (f *fakeReporter) Close()                       {}
