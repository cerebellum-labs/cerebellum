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
)

const (
	defaultPageSize = 50
	maxPageSize     = 200
)

var statuses = map[string]bool{
	"decided_on_device":  true,
	"waiting_for_server": true,
	"decided_on_server":  true,
	"executed":           true,
	"corrected":          true,
	"cancelled":          true,
}

type decisionPage struct {
	Decisions []decisionView `json:"decisions"`
	// NextBefore is the cursor of the next page, empty after the last one.
	NextBefore *uuid.UUID `json:"next_before"`
}

// listDecisions serves a page of the journal, newest first. Query:
// device_id, status, before (the next_before of the previous page) and
// limit, all optional.
func (h *Handler) listDecisions(c *gin.Context) {
	var params db.ListDecisionsParams
	var ok bool
	if params.DeviceID, ok = queryID(c, "device_id"); !ok {
		return
	}
	if params.Before, ok = queryID(c, "before"); !ok {
		return
	}
	if s := c.Query("status"); s != "" {
		if !statuses[s] {
			httperr.BadRequest(c, "unknown status "+strconv.Quote(s))
			return
		}
		params.Status = &s
	}
	limit := defaultPageSize
	if s := c.Query("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > maxPageSize {
			httperr.BadRequest(c, "limit must be a number from 1 to 200")
			return
		}
		limit = n
	}
	// One row more than the page tells whether another page follows.
	params.RowLimit = int32(limit + 1)

	rows, err := h.q.ListDecisions(c.Request.Context(), params)
	if err != nil {
		h.internalError(c, "list decisions", err)
		return
	}
	page := decisionPage{Decisions: make([]decisionView, 0, min(len(rows), limit))}
	for i, r := range rows {
		if i == limit {
			next := page.Decisions[limit-1].ID
			page.NextBefore = &next
			break
		}
		page.Decisions = append(page.Decisions, newDecisionView(r))
	}
	c.JSON(http.StatusOK, page)
}

type decisionResponse struct {
	Decision    decisionView     `json:"decision"`
	Corrections []correctionView `json:"corrections"`
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
	c.JSON(http.StatusOK, decisionResponse{Decision: newDecisionView(r), Corrections: newCorrections(r)})
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
