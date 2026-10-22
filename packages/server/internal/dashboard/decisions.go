package dashboard

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/httperr"
	"github.com/cerebellum-labs/cerebellum/packages/server/internal/view"
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

var sources = map[string]bool{"device": true, "server": true}

var statuses = map[string]bool{
	"decided_on_device":  true,
	"waiting_for_server": true,
	"decided_on_server":  true,
	"executed":           true,
	"corrected":          true,
	"cancelled":          true,
}

type decisionPage struct {
	Decisions []view.Decision `json:"decisions"`
	// NextBefore is the cursor of the next page, empty after the last one.
	NextBefore *uuid.UUID `json:"next_before"`
}

// listDecisions serves a page of the journal, newest first. Query:
// device_id, status, source, changed (true for the decisions where the
// cloud disagreed with the robot), before (the next_before of the previous
// page) and limit, all optional.
func (h *Handler) listDecisions(c *gin.Context) {
	var params db.ListDecisionsParams
	var ok bool
	if params.DeviceID, ok = queryID(c, "device_id"); !ok {
		return
	}
	if params.Before, ok = queryID(c, "before"); !ok {
		return
	}
	if params.Status, ok = queryOneOf(c, "status", statuses); !ok {
		return
	}
	if params.Source, ok = queryOneOf(c, "source", sources); !ok {
		return
	}
	if s := c.Query("changed"); s != "" {
		changed, err := strconv.ParseBool(s)
		if err != nil {
			httperr.BadRequest(c, "changed must be true or false")
			return
		}
		params.Changed = changed
	}
	limit, ok := queryLimit(c)
	if !ok {
		return
	}
	// One row more than the page tells whether another page follows.
	params.RowLimit = int32(limit + 1) //nolint:gosec // queryLimit caps it at maxPageSize

	rows, err := h.q.ListDecisions(c.Request.Context(), params)
	if err != nil {
		h.internalError(c, "list decisions", err)
		return
	}
	page := decisionPage{Decisions: make([]view.Decision, 0, min(len(rows), limit))}
	for i, r := range rows {
		if i == limit {
			next := page.Decisions[limit-1].ID
			page.NextBefore = &next
			break
		}
		page.Decisions = append(page.Decisions, view.NewDecision(r))
	}
	c.JSON(http.StatusOK, page)
}

type decisionResponse struct {
	Decision    view.Decision     `json:"decision"`
	Corrections []view.Correction `json:"corrections"`
}

func (h *Handler) getDecision(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	row, err := h.q.GetDecision(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		httperr.Missing(c, "there is no decision with this id")
		return
	}
	if err != nil {
		h.internalError(c, "get decision", err)
		return
	}
	// GetDecision selects the same columns as ListDecisions.
	r := db.ListDecisionsRow(row)
	c.JSON(http.StatusOK, decisionResponse{Decision: view.NewDecision(r), Corrections: view.NewCorrections(r)})
}

// stats counts the decisions of every device, or of device_id.
func (h *Handler) stats(c *gin.Context) {
	deviceID, ok := queryID(c, "device_id")
	if !ok {
		return
	}
	ctx := c.Request.Context()
	decisions, err := h.q.DecisionStats(ctx, deviceID)
	if err != nil {
		h.internalError(c, "decision stats", err)
		return
	}
	calls, err := h.q.ModelCallStats(ctx, deviceID)
	if err != nil {
		h.internalError(c, "model call stats", err)
		return
	}
	c.JSON(http.StatusOK, newStatsView(decisions, calls))
}

// frame serves the JPEG the robot took for the decision. A frame never
// changes, so browsers may keep it.
func (h *Handler) frame(c *gin.Context) {
	id, ok := pathID(c)
	if !ok {
		return
	}
	jpeg, err := h.q.GetFrame(c.Request.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		httperr.Missing(c, "this decision has no frame")
		return
	}
	if err != nil {
		h.internalError(c, "get frame", err)
		return
	}
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Data(http.StatusOK, "image/jpeg", jpeg)
}

// queryID parses an optional UUID query parameter. On failure it answers
// 400 and returns false.
// queryLimit reads the page size, defaultPageSize when it isn't given.
func queryLimit(c *gin.Context) (int, bool) {
	s := c.Query("limit")
	if s == "" {
		return defaultPageSize, true
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > maxPageSize {
		httperr.BadRequest(c, "limit must be a number from 1 to 200")
		return 0, false
	}
	return n, true
}

// queryOneOf reads an optional query parameter that takes one of the
// allowed values.
func queryOneOf(c *gin.Context, name string, allowed map[string]bool) (*string, bool) {
	s := c.Query(name)
	if s == "" {
		return nil, true
	}
	if !allowed[s] {
		httperr.BadRequest(c, "unknown "+name+" "+strconv.Quote(s))
		return nil, false
	}
	return &s, true
}

func queryID(c *gin.Context, name string) (*uuid.UUID, bool) {
	s := c.Query(name)
	if s == "" {
		return nil, true
	}
	id, err := uuid.Parse(s)
	if err != nil {
		httperr.BadRequest(c, name+" is not a valid id")
		return nil, false
	}
	return &id, true
}
