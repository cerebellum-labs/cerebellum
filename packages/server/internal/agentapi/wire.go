package agentapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/view"
)

const (
	maxDecideItems = 5
	maxSyncItems   = 10
	maxOptions     = 32
	maxTextLen     = 200
	maxPromptLen   = 4 << 10
	maxMetadata    = 16 << 10
	maxFrame       = 1 << 20
	maxVersionLen  = 64
)

// decisionIn is a decision as the agent sends it: the request, its own
// decision and what the robot did. The server's fields come back in
// view.Decision.
type decisionIn struct {
	ID                  uuid.UUID       `json:"id"`
	Revision            int32           `json:"revision"`
	Task                string          `json:"task"`
	Options             []string        `json:"options"`
	ActionClass         string          `json:"action_class"`
	ProposedAction      *string         `json:"proposed_action"`
	Prompt              *string         `json:"prompt"`
	Metadata            json.RawMessage `json:"metadata"`
	LocalModel          *string         `json:"local_model"`
	LocalConfidence     *float32        `json:"local_confidence"`
	LocalLatencyMs      *int32          `json:"local_latency_ms"`
	NeedsReview         bool            `json:"needs_review"`
	DecidedAt           time.Time       `json:"decided_at"`
	Action              *string         `json:"action"`
	ExecutedAt          *time.Time      `json:"executed_at"`
	CancelledAt         *time.Time      `json:"cancelled_at"`
	CancelReason        *string         `json:"cancel_reason"`
	CorrectionAppliedAt *time.Time      `json:"correction_applied_at"`
}

// itemIn is a decision with the frame the robot took for it, base64.
type itemIn struct {
	Decision decisionIn `json:"decision"`
	Frame    *string    `json:"frame"`
}

type decideRequest struct {
	Items []itemIn `json:"items"`
}

type syncRequest struct {
	AgentVersion   string          `json:"agent_version"`
	DeviceMetadata json.RawMessage `json:"device_metadata"`
	Items          []itemIn        `json:"items"`
}

// failedItem is why one decision of a batch was not accepted; the others
// still were.
type failedItem struct {
	DecisionID uuid.UUID `json:"decision_id"`
	Error      string    `json:"error"`
	// Retry tells whether the same item may work later; when false, the
	// server will never accept it as it is.
	Retry bool `json:"retry"`
}

type decideResponse struct {
	// Decisions the server answered, in its version.
	Decisions []view.Decision `json:"decisions"`
	Errors    []failedItem    `json:"errors"`
}

type syncResponse struct {
	// The synced decisions, in the server's version.
	Decisions []view.Decision `json:"decisions"`
	// Every correction of the device not carried out yet.
	Corrections []view.Correction `json:"corrections"`
	Errors      []failedItem      `json:"errors"`
	ServerTime  view.Time         `json:"server_time"`
}

// validate checks a decision before it reaches the database, so the agent
// gets a reason it can read; the database checks the same rules again.
func (d *decisionIn) validate() error {
	if err := d.validateRequest(); err != nil {
		return err
	}
	return d.validateDecision()
}

// validateRequest checks what the robot asked: the id, the task and its
// options, the context.
func (d *decisionIn) validateRequest() error {
	switch {
	case d.ID.Version() != 7:
		return errors.New("id must be a UUIDv7")
	case d.Revision < 1:
		return errors.New("revision must be 1 or more")
	case d.Task == "" || len(d.Task) > maxTextLen:
		return errors.New("task must be 1 to 200 characters")
	case d.DecidedAt.IsZero():
		return errors.New("decided_at is required")
	case d.Prompt != nil && len(*d.Prompt) > maxPromptLen:
		return errors.New("prompt must be at most 4 KB")
	}
	if err := validOptions(d.Options); err != nil {
		return err
	}
	if d.ProposedAction != nil && !slices.Contains(d.Options, *d.ProposedAction) {
		return errors.New("proposed_action must be one of the options")
	}
	return validObject(d.Metadata, "metadata")
}

// validateDecision checks the decision against its class: who decides and
// what it needs for that.
func (d *decisionIn) validateDecision() error {
	hasPrompt := d.Prompt != nil && *d.Prompt != ""
	switch d.ActionClass {
	case "reflex", "reversible":
		if d.Action == nil || !slices.Contains(d.Options, *d.Action) {
			return fmt.Errorf("a %s decision needs an action from the options", d.ActionClass)
		}
	case "irreversible":
		if !hasPrompt {
			return errors.New("an irreversible decision needs the prompt for the cloud model")
		}
	default:
		return errors.New("action_class must be reflex, reversible or irreversible")
	}
	if d.NeedsReview && (d.ActionClass != "reversible" || !hasPrompt) {
		return errors.New("only a reversible decision with a prompt can need review")
	}
	return nil
}

func validOptions(options []string) error {
	if len(options) == 0 || len(options) > maxOptions {
		return errors.New("options must have 1 to 32 entries")
	}
	for i, o := range options {
		if o == "" || len(o) > maxTextLen {
			return errors.New("each option must be 1 to 200 characters")
		}
		if slices.Contains(options[:i], o) {
			return fmt.Errorf("option %q is listed twice", o)
		}
	}
	return nil
}

// validObject accepts an empty value or a JSON object of at most 16 KB.
func validObject(raw json.RawMessage, name string) error {
	if len(raw) == 0 {
		return nil
	}
	if len(raw) > maxMetadata {
		return fmt.Errorf("%s must be at most 16 KB", name)
	}
	if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || trimmed[0] != '{' {
		return fmt.Errorf("%s must be a JSON object", name)
	}
	return nil
}

// orEmptyObject gives the database {} for a missing object.
func orEmptyObject(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 || string(bytes.TrimSpace(raw)) == "null" {
		return json.RawMessage("{}")
	}
	return raw
}

// decodeFrame returns the JPEG of a base64 frame, or nil for none.
func decodeFrame(frame *string) ([]byte, error) {
	if frame == nil || *frame == "" {
		return nil, nil
	}
	jpeg, err := base64.StdEncoding.DecodeString(*frame)
	if err != nil {
		return nil, errors.New("frame is not valid base64")
	}
	if len(jpeg) > maxFrame {
		return nil, errors.New("frame must be at most 1 MB")
	}
	// Every JPEG starts with the SOI marker and the next marker's 0xFF.
	if len(jpeg) < 3 || jpeg[0] != 0xFF || jpeg[1] != 0xD8 || jpeg[2] != 0xFF {
		return nil, errors.New("frame is not a JPEG")
	}
	return jpeg, nil
}
