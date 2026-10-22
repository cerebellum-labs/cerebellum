package agentapi

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func validDecision() decisionIn {
	return decisionIn{
		ID:          uuid.Must(uuid.NewV7()),
		Revision:    1,
		Task:        "sort_item",
		Options:     []string{"basket", "discard"},
		ActionClass: "reversible",
		Action:      new("basket"),
		DecidedAt:   time.Now(),
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		change  func(d *decisionIn)
		wantErr string // part of the error; empty when valid
	}{
		{"valid", func(*decisionIn) {}, ""},
		{"irreversible with a prompt", func(d *decisionIn) { d.ActionClass, d.Action, d.Prompt = "irreversible", nil, new("Fruit?") }, ""},
		{"reflex", func(d *decisionIn) { d.ActionClass, d.Options, d.Action = "reflex", []string{"stop"}, new("stop") }, ""},
		{"UUIDv4", func(d *decisionIn) { d.ID = uuid.New() }, "UUIDv7"},
		{"revision 0", func(d *decisionIn) { d.Revision = 0 }, "revision"},
		{"no task", func(d *decisionIn) { d.Task = "" }, "task"},
		{"no time", func(d *decisionIn) { d.DecidedAt = time.Time{} }, "decided_at"},
		{"no options", func(d *decisionIn) { d.Options = nil }, "options"},
		{"an option twice", func(d *decisionIn) { d.Options = []string{"basket", "basket"} }, "twice"},
		{"empty option", func(d *decisionIn) { d.Options = []string{"basket", ""} }, "option"},
		{"proposal outside the options", func(d *decisionIn) { d.ProposedAction = new("eat") }, "proposed_action"},
		{"action outside the options", func(d *decisionIn) { d.Action = new("eat") }, "action from the options"},
		{"reversible without an action", func(d *decisionIn) { d.Action = nil }, "action from the options"},
		{"irreversible without a prompt", func(d *decisionIn) { d.ActionClass = "irreversible" }, "prompt"},
		{"unknown class", func(d *decisionIn) { d.ActionClass = "maybe" }, "action_class"},
		{"review without a prompt", func(d *decisionIn) { d.NeedsReview = true }, "review"},
		{"metadata not an object", func(d *decisionIn) { d.Metadata = json.RawMessage(`[1]`) }, "metadata"},
		{"metadata too big", func(d *decisionIn) {
			d.Metadata = json.RawMessage(`{"x":"` + strings.Repeat("a", maxMetadata) + `"}`)
		}, "16 KB"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			d := validDecision()
			tt.change(&d)
			err := d.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("error = %v, want none", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want one mentioning %q", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeFrame(t *testing.T) {
	t.Parallel()
	b64 := func(b []byte) *string { s := base64.StdEncoding.EncodeToString(b); return &s }

	tests := []struct {
		name    string
		frame   *string
		wantLen int
		wantErr bool
	}{
		{"none", nil, 0, false},
		{"empty", new(""), 0, false},
		{"JPEG", b64([]byte{0xff, 0xd8, 0xff, 0xd9}), 4, false},
		{"PNG", b64([]byte("\x89PNG\r\n")), 0, true},
		{"not base64", new("!!!"), 0, true},
		{"too big", b64(append([]byte{0xff, 0xd8, 0xff}, make([]byte, maxFrame)...)), 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := decodeFrame(tt.frame)
			if (err != nil) != tt.wantErr || len(got) != tt.wantLen {
				t.Errorf("got %d bytes, error %v; want %d bytes, error %v", len(got), err, tt.wantLen, tt.wantErr)
			}
		})
	}
}
