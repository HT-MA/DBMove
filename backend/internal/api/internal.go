package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/dbmove/dbmove/backend/internal/model"
	"github.com/dbmove/dbmove/backend/internal/redact"
	"github.com/dbmove/dbmove/backend/internal/repository"
	"github.com/dbmove/dbmove/backend/internal/sse"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// internalHandlers expose worker-facing endpoints. These are only reachable
// inside the cluster/network and are protected by a shared token when set.
type internalHandlers struct {
	repo     *repository.Repository
	hub      *sse.Hub
	redactor func(string) string
}

func newInternalHandlers(repo *repository.Repository, hub *sse.Hub, token string) *internalHandlers {
	return &internalHandlers{
		repo:     repo,
		hub:      hub,
		redactor: redact.New(token),
	}
}

// taskPayload is the config the worker receives. It never includes passwords.
func (h *internalHandlers) task(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	task, err := h.repo.GetTask(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fail(c, http.StatusNotFound, CodeMigrationNotFound, "migration task not found")
			return
		}
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	ok(c, gin.H{
		"id":               task.ID,
		"name":             task.Name,
		"migration_type":   task.MigrationType,
		"engine":           task.Engine,
		"target_db_policy": task.TargetDBPolicy,
		"databases":        task.DatabasePairs(),
		"source": gin.H{
			"type":     task.SourceConnection.Type,
			"host":     task.SourceConnection.Host,
			"port":     task.SourceConnection.Port,
			"username": task.SourceConnection.Username,
			"database": task.SourceDatabase,
			"ssl_mode": task.SourceConnection.SSLMode,
		},
		"target": gin.H{
			"type":     task.TargetConnection.Type,
			"host":     task.TargetConnection.Host,
			"port":     task.TargetConnection.Port,
			"username": task.TargetConnection.Username,
			"database": task.TargetDatabase,
			"ssl_mode": task.TargetConnection.SSLMode,
		},
	})
}

func (h *internalHandlers) status(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var body struct {
		Status string `json:"status"`
		Error  string `json:"error_message"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidInput, err.Error())
		return
	}
	body.Status = strings.ToUpper(body.Status)
	if !validStatus(body.Status) {
		fail(c, http.StatusBadRequest, CodeInvalidInput, "invalid status")
		return
	}
	body.Error = h.redactor(body.Error)

	task, err := h.repo.GetTask(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fail(c, http.StatusNotFound, CodeMigrationNotFound, "migration task not found")
			return
		}
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	// Terminal states are final: a late report from a worker that was still
	// running when the task was cancelled/failed must not overwrite them.
	if isTerminalStatus(task.Status) && body.Status != task.Status {
		ok(c, gin.H{"status": task.Status})
		return
	}
	now := time.Now()
	fields := map[string]any{"status": body.Status}
	if body.Error != "" {
		fields["error_message"] = body.Error
	}
	if task.StartedAt == nil && (body.Status == model.TaskStatusPreparing || body.Status == model.TaskStatusRunning) {
		fields["started_at"] = now
	}
	if isTerminalStatus(body.Status) {
		fields["finished_at"] = now
	}
	if err := h.repo.UpdateTaskFields(c.Request.Context(), id, fields); err != nil {
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	msg := ""
	if body.Status == model.TaskStatusSuccess {
		msg = "migration completed"
	}
	if body.Status == model.TaskStatusFailed {
		msg = "migration failed"
	}
	if body.Error != "" {
		msg = "migration failed: " + body.Error
	}
	if msg != "" {
		level := "INFO"
		if body.Status == model.TaskStatusFailed {
			level = "ERROR"
		}
		_ = h.repo.AddLog(c.Request.Context(), &model.MigrationLog{TaskID: id, Level: level, Message: msg, CreatedAt: now})
	}
	if h.hub != nil {
		h.hub.Publish(id, sse.Event{Type: "status", Data: map[string]any{"status": body.Status, "error_message": body.Error}})
	}
	ok(c, gin.H{"status": body.Status})
}

func (h *internalHandlers) progress(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var body struct {
		Progress         int   `json:"progress"`
		TablesTotal      int64 `json:"tables_total"`
		TablesCompleted  int64 `json:"tables_completed"`
		RowsTotal        int64 `json:"rows_total"`
		RowsCompleted    int64 `json:"rows_completed"`
		BytesTotal       int64 `json:"bytes_total"`
		BytesTransferred int64 `json:"bytes_transferred"`
		Speed            int64 `json:"speed"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidInput, err.Error())
		return
	}
	if body.Progress < 0 || body.Progress > 100 {
		fail(c, http.StatusBadRequest, CodeInvalidInput, "progress must be between 0 and 100")
		return
	}
	fields := map[string]any{
		"progress":          body.Progress,
		"tables_total":      body.TablesTotal,
		"tables_completed":  body.TablesCompleted,
		"rows_total":        body.RowsTotal,
		"rows_completed":    body.RowsCompleted,
		"bytes_total":       body.BytesTotal,
		"bytes_transferred": body.BytesTransferred,
		"speed":             body.Speed,
	}
	if err := h.repo.UpdateTaskFields(c.Request.Context(), id, fields); err != nil {
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	if h.hub != nil {
		h.hub.Publish(id, sse.Event{Type: "progress", Data: fields})
	}
	ok(c, gin.H{"updated": true})
}

func (h *internalHandlers) logs(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var body struct {
		Logs []struct {
			Level   string `json:"level"`
			Message string `json:"message"`
		} `json:"logs"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidInput, err.Error())
		return
	}
	if len(body.Logs) == 0 {
		fail(c, http.StatusBadRequest, CodeInvalidInput, "logs must not be empty")
		return
	}
	now := time.Now()
	entries := make([]model.MigrationLog, 0, len(body.Logs))
	for _, l := range body.Logs {
		level := strings.ToUpper(l.Level)
		if level == "" {
			level = "INFO"
		}
		message := h.redactor(l.Message)
		if len(message) > 4000 {
			message = message[:4000]
		}
		entries = append(entries, model.MigrationLog{TaskID: id, Level: level, Message: message, CreatedAt: now})
	}
	if err := h.repo.AddLogs(c.Request.Context(), entries); err != nil {
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	if h.hub != nil {
		for i := range entries {
			h.hub.Publish(id, sse.Event{Type: "log", Data: entries[i]})
		}
	}
	ok(c, gin.H{"accepted": len(entries)})
}

func validStatus(s string) bool {
	switch s {
	case model.TaskStatusPending, model.TaskStatusPreparing, model.TaskStatusRunning,
		model.TaskStatusSuccess, model.TaskStatusFailed, model.TaskStatusCancelled:
		return true
	}
	return false
}

func isTerminalStatus(s string) bool {
	switch s {
	case model.TaskStatusSuccess, model.TaskStatusFailed, model.TaskStatusCancelled:
		return true
	}
	return false
}

// internalAuth middleware checks the shared internal token when configured.
func internalAuth(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		got := c.GetHeader("Authorization")
		got = strings.TrimPrefix(got, "Bearer ")
		if got == "" {
			got = c.GetHeader("X-DBMove-Internal")
		}
		if got != token {
			fail(c, http.StatusUnauthorized, CodeAuthFailed, "unauthorized")
			return
		}
		c.Next()
	}
}
