package dashboard

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
)

// onlineWindow is how recently a device must have been heard from to count
// as online: three of its 5-second sync rounds.
const onlineWindow = 15 * time.Second

// isoTime marshals as ISO 8601 in UTC with milliseconds, like
// "2026-10-21T18:09:00.123Z": strings in this form sort by time.
type isoTime time.Time

func (t isoTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).UTC().Format("2006-01-02T15:04:05.000Z"))
}

func isoPtr(t *time.Time) *isoTime {
	if t == nil {
		return nil
	}
	v := isoTime(*t)
	return &v
}

type deviceView struct {
	ID           uuid.UUID       `json:"id"`
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Online       bool            `json:"online"`
	LastSeenAt   *isoTime        `json:"last_seen_at"`
	AgentVersion *string         `json:"agent_version"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    isoTime         `json:"created_at"`
}

func newDeviceView(d db.Device, now time.Time) deviceView {
	return deviceView{
		ID:           d.ID,
		Key:          d.Key,
		Name:         d.Name,
		Online:       d.LastSeenAt != nil && now.Sub(*d.LastSeenAt) < onlineWindow,
		LastSeenAt:   isoPtr(d.LastSeenAt),
		AgentVersion: d.AgentVersion,
		Metadata:     d.Metadata,
		CreatedAt:    isoTime(d.CreatedAt),
	}
}

// decisionView is a decision as the dashboard shows it: the decision's own
// fields, the model call it came from (for server decisions) and its
// device.
type decisionView struct {
	ID              uuid.UUID       `json:"id"`
	DeviceID        uuid.UUID       `json:"device_id"`
	DeviceKey       string          `json:"device_key"`
	DeviceName      string          `json:"device_name"`
	Task            string          `json:"task"`
	Options         []string        `json:"options"`
	ActionClass     string          `json:"action_class"`
	Status          string          `json:"status"`
	Source          *string         `json:"source"`
	NeedsReview     bool            `json:"needs_review"`
	Metadata        json.RawMessage `json:"metadata"`
	ProposedAction  *string         `json:"proposed_action"`
	Action          *string         `json:"action"`
	CorrectedAction *string         `json:"corrected_action"`
	Prompt          *string         `json:"prompt"`
	LocalModel      *string         `json:"local_model"`
	LocalConfidence *float32        `json:"local_confidence"`
	// The model behind the final action: the local one for a device
	// decision, the cloud one for a server decision.
	Model        *string  `json:"model"`
	Confidence   *float32 `json:"confidence"`
	Reason       *string  `json:"reason"`
	LatencyMs    *int32   `json:"latency_ms"`
	CostUsd      *float64 `json:"cost_usd"`
	InputTokens  *int32   `json:"input_tokens"`
	OutputTokens *int32   `json:"output_tokens"`
	CancelReason *string  `json:"cancel_reason"`
	HasFrame     bool     `json:"has_frame"`
	DecidedAt    isoTime  `json:"decided_at"`
	ReceivedAt   isoTime  `json:"received_at"`
	AnsweredAt   *isoTime `json:"answered_at"`
	ExecutedAt   *isoTime `json:"executed_at"`
	CancelledAt  *isoTime `json:"cancelled_at"`
	ReviewedAt   *isoTime `json:"reviewed_at"`
}

// correctionView is the dashboard's correction: the review's answer when it
// disagreed with the device. A decision has at most one, so it shares the
// decision's id.
type correctionView struct {
	ID         uuid.UUID `json:"id"`
	DecisionID uuid.UUID `json:"decision_id"`
	Action     string    `json:"action"`
	Reason     string    `json:"reason"`
	Model      string    `json:"model"`
	CreatedAt  isoTime   `json:"created_at"`
	AppliedAt  *isoTime  `json:"applied_at"`
}

func newDecisionView(r db.ListDecisionsRow) decisionView {
	d := r.Decision
	v := decisionView{
		ID:              d.ID,
		DeviceID:        d.DeviceID,
		DeviceKey:       r.DeviceKey,
		DeviceName:      r.DeviceName,
		Task:            d.Task,
		Options:         d.Options,
		ActionClass:     d.ActionClass,
		Status:          d.Status,
		Source:          d.Source,
		NeedsReview:     d.NeedsReview,
		Metadata:        d.Metadata,
		ProposedAction:  d.ProposedAction,
		Action:          d.Action,
		CorrectedAction: d.CorrectedAction,
		Prompt:          d.Prompt,
		LocalModel:      d.LocalModel,
		LocalConfidence: d.LocalConfidence,
		CancelReason:    d.CancelReason,
		HasFrame:        r.HasFrame,
		DecidedAt:       isoTime(d.DecidedAt),
		ReceivedAt:      isoTime(d.ReceivedAt),
		ExecutedAt:      isoPtr(d.ExecutedAt),
		CancelledAt:     isoPtr(d.CancelledAt),
		ReviewedAt:      isoPtr(d.ReviewedAt),
	}
	if d.ModelCallID != nil {
		v.Model = r.CallModel
		v.Confidence = r.CallConfidence
		v.Reason = r.CallReason
		v.LatencyMs = r.CallLatencyMs
		v.CostUsd = r.CallCostUsd
		v.InputTokens = r.CallInputTokens
		v.OutputTokens = r.CallOutputTokens
		v.AnsweredAt = isoPtr(r.AnsweredAt)
	} else {
		v.Model = d.LocalModel
		v.Confidence = d.LocalConfidence
	}
	return v
}

// newCorrections returns the decision's correction, if it has one.
func newCorrections(r db.ListDecisionsRow) []correctionView {
	d := r.Decision
	if d.CorrectedAction == nil || d.ReviewedAt == nil {
		return []correctionView{}
	}
	return []correctionView{{
		ID:         d.ID,
		DecisionID: d.ID,
		Action:     *d.CorrectedAction,
		Reason:     deref(r.ReviewReason),
		Model:      deref(r.ReviewModel),
		CreatedAt:  isoTime(*d.ReviewedAt),
		AppliedAt:  isoPtr(d.CorrectionAppliedAt),
	}}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

type statsView struct {
	Total        int64               `json:"total"`
	BySource     map[string]int64    `json:"by_source"`
	ByStatus     map[string]int64    `json:"by_status"`
	ByClass      map[string]int64    `json:"by_class"`
	Corrections  int64               `json:"corrections"`
	CostUsd      float64             `json:"cost_usd"`
	AvgLatencyMs map[string]*float64 `json:"avg_latency_ms"`
}

func newStatsView(d db.DecisionStatsRow, m db.ModelCallStatsRow) statsView {
	v := statsView{
		Total:    d.Total,
		BySource: map[string]int64{"device": d.SourceDevice, "server": d.SourceServer},
		ByStatus: map[string]int64{
			"decided_on_device":  d.StatusDecidedOnDevice,
			"waiting_for_server": d.StatusWaitingForServer,
			"decided_on_server":  d.StatusDecidedOnServer,
			"executed":           d.StatusExecuted,
			"corrected":          d.StatusCorrected,
			"cancelled":          d.StatusCancelled,
		},
		ByClass: map[string]int64{
			"reflex":       d.ClassReflex,
			"reversible":   d.ClassReversible,
			"irreversible": d.ClassIrreversible,
		},
		Corrections: d.Corrections,
		CostUsd:     m.CostUsd,
		// The device's own latency isn't reported to the server.
		AvgLatencyMs: map[string]*float64{"device": nil, "server": nil},
	}
	if m.DecideCalls > 0 {
		avg := m.AvgDecideLatencyMs
		v.AvgLatencyMs["server"] = &avg
	}
	return v
}
