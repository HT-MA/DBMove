package api

import (
	"log"
	"net/http"
	"time"

	"github.com/dbmove/dbmove/backend/internal/repository"
	"github.com/dbmove/dbmove/backend/internal/sse"
	"github.com/gin-gonic/gin"
)

type sseHandlers struct {
	repo *repository.Repository
	hub  *sse.Hub
}

// streamLogs serves a Server-Sent Events stream of logs and status updates.
func (h *sseHandlers) streamLogs(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	if _, err := h.repo.GetTask(c.Request.Context(), id); err != nil {
		fail(c, http.StatusNotFound, CodeMigrationNotFound, "migration task not found")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	ch, unsub := h.hub.Subscribe(id)
	defer unsub()

	// replay recent logs
	logs, err := h.repo.ListLogs(c.Request.Context(), id, 500)
	if err == nil {
		for i := range logs {
			if _, werr := c.Writer.Write(sse.Encode(sse.Event{Type: "log", Data: logs[i]})); werr != nil {
				return
			}
		}
		c.Writer.Flush()
	}

	// heartbeat keeps proxies alive
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case ev := <-ch:
			if _, err := c.Writer.Write(sse.Encode(ev)); err != nil {
				log.Printf("sse: write to task %d: %v", id, err)
				return
			}
			c.Writer.Flush()
		case <-heartbeat.C:
			if _, err := c.Writer.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}
