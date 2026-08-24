package executor

import (
	"strings"
	"testing"
)

func TestLineWriterSplitsLines(t *testing.T) {
	var lines []string
	w := NewLineWriter(func(line string) { lines = append(lines, line) }, 10)
	_, _ = w.Write([]byte("hello\nworld\r\npart"))
	_, _ = w.Write([]byte("ial\n"))
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "hello" || lines[1] != "world" || lines[2] != "partial" {
		t.Fatalf("unexpected lines: %v", lines)
	}
}

func TestLineWriterSkipsEmptyLines(t *testing.T) {
	var lines []string
	w := NewLineWriter(func(line string) { lines = append(lines, line) }, 10)
	_, _ = w.Write([]byte("a\n\n\nb\n"))
	if len(lines) != 2 {
		t.Fatalf("expected 2 non-empty lines, got %d", len(lines))
	}
}

func TestLineWriterTail(t *testing.T) {
	var lines []string
	w := NewLineWriter(func(line string) { lines = append(lines, line) }, 3)
	for i := 0; i < 5; i++ {
		_, _ = w.Write([]byte("line" + string(rune('0'+i)) + "\n"))
	}
	tail := w.Tail()
	if len(tail) != 3 {
		t.Fatalf("expected tail of 3, got %d: %v", len(tail), tail)
	}
	if strings.Join(tail, ",") != "line2,line3,line4" {
		t.Fatalf("unexpected tail: %v", tail)
	}
}
