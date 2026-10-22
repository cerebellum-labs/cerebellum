// Package view turns database rows into the JSON the APIs send: the
// dashboard and the agents see a device, a decision and a correction the
// same way.
package view

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
)

// OnlineWindow is how recently a device must have been heard from to count
// as online: three of its 5-second sync rounds.
const OnlineWindow = 15 * time.Second

// Time marshals as ISO 8601 in UTC with milliseconds, like
// "2026-10-21T18:09:00.123Z": strings in this form sort by time.
type Time time.Time

// MarshalJSON implements json.Marshaler.
func (t Time) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(t).UTC().Format("2006-01-02T15:04:05.000Z"))
}

// TimePtr converts a nullable timestamp.
func TimePtr(t *time.Time) *Time {
	if t == nil {
		return nil
	}
	v := Time(*t)
	return &v
}

// Device is a device as the dashboard shows it; its token hash stays out.
type Device struct {
	ID           uuid.UUID       `json:"id"`
	Key          string          `json:"key"`
	Name         string          `json:"name"`
	Online       bool            `json:"online"`
	LastSeenAt   *Time           `json:"last_seen_at"`
	AgentVersion *string         `json:"agent_version"`
	Metadata     json.RawMessage `json:"metadata"`
	CreatedAt    Time            `json:"created_at"`
}

// NewDevice returns the view of d, online if it was heard from shortly
// before now.
func NewDevice(d db.Device, now time.Time) Device {
	return Device{
		ID:           d.ID,
		Key:          d.Key,
		Name:         d.Name,
		Online:       d.LastSeenAt != nil && now.Sub(*d.LastSeenAt) < OnlineWindow,
		LastSeenAt:   TimePtr(d.LastSeenAt),
		AgentVersion: d.AgentVersion,
		Metadata:     d.Metadata,
		CreatedAt:    Time(d.CreatedAt),
	}
}

// Decision is a decision with the model call it came from (for a server
// decision) and its device.
type Decision struct {
	ID              uuid.UUID       `json:"id"`
	DeviceID        uuid.UUID       `json:"device_id"`
	DeviceKey       string          `json:"device_key"`
	DeviceName      string          `json:"device_name"`
	Revision        int32           `json:"revision"`
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
	DecidedAt    Time     `json:"decided_at"`
	ReceivedAt   Time     `json:"received_at"`
	AnsweredAt   *Time    `json:"answered_at"`
	ExecutedAt   *Time    `json:"executed_at"`
	CancelledAt  *Time    `json:"cancelled_at"`
	ReviewedAt   *Time    `json:"reviewed_at"`
	// When the robot carried out the correction, if there is one.
	CorrectionAppliedAt *Time `json:"correction_applied_at"`
}

// NewDecision returns the view of a row of db.ListDecisions; db.GetDecision
// rows convert to it.
func NewDecision(r db.ListDecisionsRow) Decision {
	d := r.Decision
	v := Decision{
		ID:                  d.ID,
		DeviceID:            d.DeviceID,
		DeviceKey:           r.DeviceKey,
		DeviceName:          r.DeviceName,
		Revision:            d.Revision,
		Task:                d.Task,
		Options:             d.Options,
		ActionClass:         d.ActionClass,
		Status:              d.Status,
		Source:              d.Source,
		NeedsReview:         d.NeedsReview,
		Metadata:            d.Metadata,
		ProposedAction:      d.ProposedAction,
		Action:              d.Action,
		CorrectedAction:     d.CorrectedAction,
		Prompt:              d.Prompt,
		LocalModel:          d.LocalModel,
		LocalConfidence:     d.LocalConfidence,
		CancelReason:        d.CancelReason,
		HasFrame:            r.HasFrame,
		DecidedAt:           Time(d.DecidedAt),
		ReceivedAt:          Time(d.ReceivedAt),
		ExecutedAt:          TimePtr(d.ExecutedAt),
		CancelledAt:         TimePtr(d.CancelledAt),
		ReviewedAt:          TimePtr(d.ReviewedAt),
		CorrectionAppliedAt: TimePtr(d.CorrectionAppliedAt),
	}
	if d.ModelCallID != nil {
		v.Model = r.CallModel
		v.Confidence = r.CallConfidence
		v.Reason = r.CallReason
		v.LatencyMs = r.CallLatencyMs
		v.CostUsd = r.CallCostUsd
		v.InputTokens = r.CallInputTokens
		v.OutputTokens = r.CallOutputTokens
		v.AnsweredAt = TimePtr(r.AnsweredAt)
	} else {
		v.Model = d.LocalModel
		v.Confidence = d.LocalConfidence
	}
	return v
}

// Correction is a review that disagreed with the device. A decision has at
// most one, so it shares the decision's id.
type Correction struct {
	ID         uuid.UUID `json:"id"`
	DecisionID uuid.UUID `json:"decision_id"`
	Action     string    `json:"action"`
	Reason     string    `json:"reason"`
	Model      string    `json:"model"`
	CreatedAt  Time      `json:"created_at"`
	AppliedAt  *Time     `json:"applied_at"`
}

// NewCorrections returns the decision's correction, if it has one.
func NewCorrections(r db.ListDecisionsRow) []Correction {
	d := r.Decision
	if d.CorrectedAction == nil || d.ReviewedAt == nil {
		return []Correction{}
	}
	return []Correction{NewCorrection(d.ID, *d.CorrectedAction, r.ReviewReason, r.ReviewModel, *d.ReviewedAt, d.CorrectionAppliedAt)}
}

// NewCorrection builds a correction from the decision's review columns and
// its review call.
func NewCorrection(decisionID uuid.UUID, action string, reason, model *string, reviewedAt time.Time, appliedAt *time.Time) Correction {
	return Correction{
		ID:         decisionID,
		DecisionID: decisionID,
		Action:     action,
		Reason:     deref(reason),
		Model:      deref(model),
		CreatedAt:  Time(reviewedAt),
		AppliedAt:  TimePtr(appliedAt),
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
