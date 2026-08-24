package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/dbmove/dbmove/backend/internal/config"
	"github.com/dbmove/dbmove/backend/internal/repository"
	"github.com/dbmove/dbmove/backend/internal/service"
	"github.com/dbmove/dbmove/backend/internal/sse"
	"github.com/gin-gonic/gin"
)

// Handlers bundles all HTTP handlers.
type Handlers struct {
	cfg         *config.Config
	repo        *repository.Repository
	hub         *sse.Hub
	connections *connectionHandlers
	migrations  *migrationHandlers
	internal    *internalHandlers
	sse         *sseHandlers
}

func NewHandlers(cfg *config.Config, repo *repository.Repository, hub *sse.Hub,
	connSvc *service.ConnectionService, migSvc *service.MigrationService,
	cancelRunner func(ctx context.Context, id uint64) error) *Handlers {
	return &Handlers{
		cfg:         cfg,
		repo:        repo,
		hub:         hub,
		connections: &connectionHandlers{svc: connSvc},
		migrations:  &migrationHandlers{svc: migSvc, cancelRunner: cancelRunner},
		internal:    newInternalHandlers(repo, hub, cfg.InternalToken),
		sse:         &sseHandlers{repo: repo, hub: hub},
	}
}

// Build creates the Gin engine with all routes and middleware.
func (h *Handlers) Build() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware(h.cfg.CORSOrigins))

	r.GET("/healthz", func(c *gin.Context) { ok(c, gin.H{"status": "ok"}) })
	r.GET("/api/v1/info", func(c *gin.Context) {
		ok(c, gin.H{
			"name":                      "DBMove",
			"version":                   "0.1.0",
			"execution_mode":            h.cfg.ExecutionMode,
			"max_concurrent_migrations": h.cfg.MaxConcurrent,
			"supported_databases":       []string{"mysql", "postgresql"},
			"supported_migration_types": []string{"FULL"},
		})
	})

	v1 := r.Group("/api/v1")
	{
		v1.GET("/stats", h.migrations.stats)

		conn := v1.Group("/connections")
		{
			conn.POST("", h.connections.create)
			conn.GET("", h.connections.list)
			conn.POST("/test", h.connections.testByValues)
			conn.GET("/:id", h.connections.get)
			conn.PUT("/:id", h.connections.update)
			conn.DELETE("/:id", h.connections.delete)
			conn.POST("/:id/test", h.connections.testByID)
			conn.GET("/:id/databases", h.connections.databases)
		}

		mig := v1.Group("/migrations")
		{
			mig.POST("", h.migrations.create)
			mig.GET("", h.migrations.list)
			mig.GET("/:id", h.migrations.get)
			mig.POST("/:id/start", h.migrations.start)
			mig.POST("/:id/cancel", h.migrations.cancel)
			mig.POST("/:id/retry", h.migrations.retry)
			mig.GET("/:id/logs", h.migrations.logs)
			mig.GET("/:id/logs/stream", h.sse.streamLogs)
			mig.GET("/:id/progress", h.migrations.progress)
		}

		internal := v1.Group("/internal", internalAuth(h.cfg.InternalToken))
		{
			internal.GET("/tasks/:id", h.internal.task)
			internal.POST("/tasks/:id/status", h.internal.status)
			internal.POST("/tasks/:id/progress", h.internal.progress)
			internal.POST("/tasks/:id/logs", h.internal.logs)
		}
	}
	return r
}

func corsMiddleware(origins string) gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, o := range strings.Split(origins, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowed[o] = true
		}
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && (allowed["*"] || allowed[origin]) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		} else if allowed["*"] {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-DBMove-Internal")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
