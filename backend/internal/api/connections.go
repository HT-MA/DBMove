package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/dbmove/dbmove/backend/internal/service"
	"github.com/gin-gonic/gin"
)

type connectionHandlers struct {
	svc *service.ConnectionService
}

func (h *connectionHandlers) create(c *gin.Context) {
	var in service.ConnectionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidInput, err.Error())
		return
	}
	dto, err := h.svc.Create(c.Request.Context(), in)
	if err != nil {
		failErr(c, http.StatusBadRequest, CodeInvalidInput, err)
		return
	}
	created(c, dto)
}

func (h *connectionHandlers) list(c *gin.Context) {
	items, err := h.svc.List(c.Request.Context())
	if err != nil {
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	ok(c, items)
}

func (h *connectionHandlers) get(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	dto, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrConnectionNotFound) {
			fail(c, http.StatusNotFound, CodeConnectionNotFound, "connection not found")
			return
		}
		failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		return
	}
	ok(c, dto)
}

func (h *connectionHandlers) update(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	var in service.ConnectionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidInput, err.Error())
		return
	}
	dto, err := h.svc.Update(c.Request.Context(), id, in)
	if err != nil {
		if errors.Is(err, service.ErrConnectionNotFound) {
			fail(c, http.StatusNotFound, CodeConnectionNotFound, "connection not found")
			return
		}
		failErr(c, http.StatusBadRequest, CodeInvalidInput, err)
		return
	}
	ok(c, dto)
}

func (h *connectionHandlers) delete(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	err := h.svc.Delete(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrConnectionNotFound):
			fail(c, http.StatusNotFound, CodeConnectionNotFound, "connection not found")
		case errors.Is(err, service.ErrConnectionInUse):
			fail(c, http.StatusConflict, CodeConnectionInUse, err.Error())
		default:
			failErr(c, http.StatusInternalServerError, CodeInternalError, err)
		}
		return
	}
	ok(c, gin.H{"deleted": true})
}

func (h *connectionHandlers) testByID(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	result, err := h.svc.TestByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrConnectionNotFound) {
			fail(c, http.StatusNotFound, CodeConnectionNotFound, "connection not found")
			return
		}
		fail(c, http.StatusBadRequest, CodeConnectionFailed, err.Error())
		return
	}
	ok(c, result)
}

func (h *connectionHandlers) testByValues(c *gin.Context) {
	var in service.ConnectionInput
	if err := c.ShouldBindJSON(&in); err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidInput, err.Error())
		return
	}
	result, err := h.svc.TestByValues(c.Request.Context(), in)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeConnectionFailed, err.Error())
		return
	}
	ok(c, result)
}

func (h *connectionHandlers) databases(c *gin.Context) {
	id, okID := parseID(c)
	if !okID {
		return
	}
	items, err := h.svc.Databases(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrConnectionNotFound) {
			fail(c, http.StatusNotFound, CodeConnectionNotFound, "connection not found")
			return
		}
		fail(c, http.StatusBadRequest, CodeConnectionFailed, err.Error())
		return
	}
	ok(c, gin.H{"databases": items})
}

func parseID(c *gin.Context) (uint64, bool) {
	raw := c.Param("id")
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, CodeInvalidInput, "invalid id")
		return 0, false
	}
	return id, true
}
