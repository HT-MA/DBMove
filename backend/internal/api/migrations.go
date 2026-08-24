package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/dbmove/dbmove/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type migrationHandlers struct {
	svc          *service.MigrationService
	cancelRunner func(ctx context.Context, id uint64) error
}

func (h *migrationHandlers) create(c *gin.Context) {
	var in service.MigrationInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidInput, err.Error())
		return
	}
	task, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUnsupportedMigrate):
			fail(c, http.StatusBadRequest, CodeUnsupportedMigration, err.Error())
		case errors.Is(err, service.ErrConnectionNotFound):
			fail(c, http.StatusNotFound, CodeConnectionNotFound, err.Error())
		default:
			failErr(c, http.StatusBadRequest, CodeInvalidInput, err)
		}
		return
	}
	created(c, gin.H{"id": task.ID, "status": task.Status})
}

func (h *migrationHandlers) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	status := c.Query("status")
	createdBy := c.Query("created_by")
	items, total, err := h.svc.List(c.Request.Context(), page, pageSize, status, createdBy)
	if err != nil {
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	ok(c, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *migrationHandlers) get(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	task, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			fail(c, http.StatusNotFound, CodeMigrationNotFound, "migration task not found")
			return
		}
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	ok(c, task)
}

func (h *migrationHandlers) start(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	task, err := h.svc.Start(c.Request.Context(), id)
	if err != nil {
		h.startErr(c, err)
		return
	}
	ok(c, gin.H{"id": task.ID, "status": task.Status})
}

func (h *migrationHandlers) startErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrTaskNotFound):
		fail(c, http.StatusNotFound, CodeMigrationNotFound, "migration task not found")
	case errors.Is(err, service.ErrAlreadyRunning):
		fail(c, http.StatusConflict, CodeMigrationAlreadyRun, err.Error())
	default:
		failErr(c, http.StatusBadRequest, CodeInvalidState, err)
	}
}

func (h *migrationHandlers) cancel(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	task, err := h.svc.Cancel(c.Request.Context(), id, func(ctx context.Context, tid uint64) error {
		if h.cancelRunner != nil {
			return h.cancelRunner(ctx, tid)
		}
		return nil
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			fail(c, http.StatusNotFound, CodeMigrationNotFound, "migration task not found")
		default:
			failErr(c, http.StatusBadRequest, CodeInvalidState, err)
		}
		return
	}
	ok(c, gin.H{"id": task.ID, "status": task.Status})
}

func (h *migrationHandlers) retry(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	task, err := h.svc.Retry(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			fail(c, http.StatusNotFound, CodeMigrationNotFound, "migration task not found")
		default:
			failErr(c, http.StatusBadRequest, CodeInvalidState, err)
		}
		return
	}
	ok(c, gin.H{"id": task.ID, "status": task.Status})
}

func (h *migrationHandlers) logs(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "1000"))
	items, err := h.svc.Logs(c.Request.Context(), id, limit)
	if err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			fail(c, http.StatusNotFound, CodeMigrationNotFound, "migration task not found")
			return
		}
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	ok(c, gin.H{"items": items})
}

func (h *migrationHandlers) progress(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	task, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrTaskNotFound) {
			fail(c, http.StatusNotFound, CodeMigrationNotFound, "migration task not found")
			return
		}
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	ok(c, map[string]any{
		"status":            task.Status,
		"progress":          task.Progress,
		"tables_total":      task.TablesTotal,
		"tables_completed":  task.TablesCompleted,
		"rows_total":        task.RowsTotal,
		"rows_completed":    task.RowsCompleted,
		"bytes_total":       task.BytesTotal,
		"bytes_transferred": task.BytesTransferred,
		"speed":             task.Speed,
		"error_message":     task.ErrorMessage,
	})
}

func (h *migrationHandlers) stats(c *gin.Context) {
	data, err := h.svc.Stats(c.Request.Context())
	if err != nil {
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	ok(c, data)
}
