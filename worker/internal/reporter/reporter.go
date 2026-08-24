package reporter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dbmove/dbmove/worker/internal/redact"
)

// LogEntry is one log line sent to the backend.
type LogEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// ProgressUpdate is the progress payload sent to the backend.
type ProgressUpdate struct {
	Progress         int   `json:"progress"`
	TablesTotal      int64 `json:"tables_total"`
	TablesCompleted  int64 `json:"tables_completed"`
	RowsTotal        int64 `json:"rows_total"`
	RowsCompleted    int64 `json:"rows_completed"`
	BytesTotal       int64 `json:"bytes_total"`
	BytesTransferred int64 `json:"bytes_transferred"`
	Speed            int64 `json:"speed"`
}

// Reporter batches log lines and progress updates to the backend internal
// API asynchronously so migration commands are never blocked by HTTP.
type Reporter struct {
	apiBase string
	taskID  uint64
	token   string
	client  *http.Client
	redact  func(string) string

	mu      sync.Mutex
	pending []LogEntry
	done    chan struct{}
	wg      sync.WaitGroup

	lastProgress time.Time
	lastSpeed    time.Time
}

func New(apiBase string, taskID uint64, token string, secrets ...string) *Reporter {
	r := &Reporter{
		apiBase: strings.TrimRight(apiBase, "/"),
		taskID:  taskID,
		token:   token,
		client:  &http.Client{Timeout: 15 * time.Second},
		redact:  redact.New(secrets...),
		done:    make(chan struct{}),
	}
	r.wg.Add(1)
	go r.flushLoop()
	return r
}

func (r *Reporter) Log(level, format string, args ...any) {
	msg := r.redact(fmt.Sprintf(format, args...))
	r.mu.Lock()
	r.pending = append(r.pending, LogEntry{Level: strings.ToUpper(level), Message: msg})
	// flush immediately when the buffer is large
	flush := len(r.pending) >= 50
	r.mu.Unlock()
	if flush {
		r.flush()
	}
}

func (r *Reporter) Progress(p ProgressUpdate) {
	r.mu.Lock()
	now := time.Now()
	throttled := now.Sub(r.lastProgress) < 500*time.Millisecond
	r.lastProgress = now
	r.mu.Unlock()
	if throttled && p.Progress < 100 {
		return
	}
	r.post("/progress", p)
}

func (r *Reporter) Status(status, errMsg string) {
	payload := map[string]any{"status": status, "error_message": r.redact(errMsg)}
	r.postRetry("/status", payload, 5)
}

func (r *Reporter) Close() {
	close(r.done)
	r.wg.Wait()
	r.flush()
}

func (r *Reporter) flushLoop() {
	defer r.wg.Done()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.flush()
		case <-r.done:
			return
		}
	}
}

func (r *Reporter) flush() {
	r.mu.Lock()
	if len(r.pending) == 0 {
		r.mu.Unlock()
		return
	}
	logs := r.pending
	r.pending = nil
	r.mu.Unlock()
	r.postRetry("/logs", map[string]any{"logs": logs}, 3)
}

func (r *Reporter) post(path string, body any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, r.apiBase+"/api/v1/internal/tasks/"+fmt.Sprint(r.taskID)+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if r.token != "" {
		req.Header.Set("Authorization", "Bearer "+r.token)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resp, err := r.client.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("backend returned %d", resp.StatusCode)
	}
	return nil
}

func (r *Reporter) postRetry(path string, body any, attempts int) {
	for i := 0; i < attempts; i++ {
		if err := r.post(path, body); err == nil {
			return
		} else if i == attempts-1 {
			log.Printf("reporter: %s failed: %v", path, err)
		}
		time.Sleep(300 * time.Millisecond)
	}
}
