package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIError is the unified error payload.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Response is the unified API envelope.
type Response struct {
	Success bool      `json:"success"`
	Data    any       `json:"data,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

// Error codes.
const (
	CodeConnectionFailed      = "CONNECTION_FAILED"
	CodeAuthFailed            = "AUTH_FAILED"
	CodeDatabaseNotFound      = "DATABASE_NOT_FOUND"
	CodePermissionDenied      = "PERMISSION_DENIED"
	CodeDiskSpaceInsufficient = "DISK_SPACE_INSUFFICIENT"
	CodeMigrationFailed       = "MIGRATION_FAILED"
	CodeMigrationNotFound     = "MIGRATION_NOT_FOUND"
	CodeMigrationAlreadyRun   = "MIGRATION_ALREADY_RUNNING"
	CodeMigrationCancelled    = "MIGRATION_CANCELLED"
	CodeEngineNotFound        = "ENGINE_NOT_FOUND"
	CodeInvalidDatabaseType   = "INVALID_DATABASE_TYPE"
	CodeInvalidInput          = "INVALID_INPUT"
	CodeUnsupportedMigration  = "UNSUPPORTED_MIGRATION"
	CodeInvalidState          = "INVALID_STATE"
	CodeConnectionNotFound    = "CONNECTION_NOT_FOUND"
	CodeConnectionInUse       = "CONNECTION_IN_USE"
	CodeInternalError         = "INTERNAL_ERROR"
)

func ok(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{Success: true, Data: data})
}

func created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{Success: true, Data: data})
}

func fail(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, Response{Success: false, Error: &APIError{Code: code, Message: message}})
}

func failErr(c *gin.Context, status int, code string, err error) {
	fail(c, status, code, err.Error())
}
