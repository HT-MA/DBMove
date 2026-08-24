package executor

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// LineWriter collects stdout/stderr and invokes onLine for each complete line.
type LineWriter struct {
	onLine func(string)
	buf    bytes.Buffer
	tail   []string
}

func NewLineWriter(onLine func(string), tailLimit int) *LineWriter {
	return &LineWriter{onLine: onLine, tail: make([]string, 0, tailLimit)}
}

func (w *LineWriter) Write(p []byte) (int, error) {
	n := len(p)
	w.buf.Write(p)
	for {
		line, err := w.buf.ReadString('\n')
		if err != nil {
			w.buf.WriteString(line)
			break
		}
		text := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(text) != "" {
			if len(w.tail) == cap(w.tail) {
				copy(w.tail, w.tail[1:])
				w.tail[len(w.tail)-1] = text
			} else {
				w.tail = append(w.tail, text)
			}
			if w.onLine != nil {
				w.onLine(text)
			}
		}
	}
	return n, nil
}

// Tail returns the last captured lines.
func (w *LineWriter) Tail() []string {
	out := make([]string, len(w.tail))
	copy(out, w.tail)
	return out
}

// Result describes a finished command.
type Result struct {
	ExitCode int
	Tail     []string
}

// Run executes a command, streaming output to onLine. Returns nil error when
// the process exits 0.
func Run(ctx context.Context, bin string, args []string, env []string, onLine func(string)) (Result, error) {
	return RunFile(ctx, bin, args, env, "", "", onLine)
}

// RunFile executes a command with optional stdin/stdout files while streaming
// stderr lines to onLine.
func RunFile(ctx context.Context, bin string, args []string, env []string, stdinPath, stdoutPath string, onLine func(string)) (Result, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	w := NewLineWriter(onLine, 50)
	if stdoutPath != "" {
		f, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			return Result{}, fmt.Errorf("open output file: %w", err)
		}
		defer f.Close()
		cmd.Stdout = f
	} else {
		cmd.Stdout = w
	}
	cmd.Stderr = w
	if stdinPath != "" {
		f, err := os.Open(stdinPath)
		if err != nil {
			return Result{}, fmt.Errorf("open input file: %w", err)
		}
		defer f.Close()
		cmd.Stdin = f
	}
	err := cmd.Run()
	res := Result{Tail: w.Tail()}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			res.ExitCode = exitErr.ExitCode()
			return res, fmt.Errorf("command %s exited with code %d", bin, res.ExitCode)
		}
		return res, fmt.Errorf("command %s: %w", bin, err)
	}
	return res, nil
}

// LookPath checks a binary is available in PATH.
func LookPath(bin string) error {
	_, err := exec.LookPath(bin)
	return err
}

// Scanner is a convenience wrapper around bufio.Scanner for tests.
var _ = bufio.ScanLines
