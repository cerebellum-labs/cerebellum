package model

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeAPI answers chat completions with reply and status, and hands the
// request it got to check.
func fakeAPI(t *testing.T, status int, reply string) (*Client, *chatRequest) {
	t.Helper()
	var got chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer key" {
			t.Errorf("request to %s with %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Error(err)
		}
		w.WriteHeader(status)
		resp := map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": reply}}},
			"usage":   map[string]any{"prompt_tokens": 1000, "completion_tokens": 40},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(srv.Close)
	c := New(Config{BaseURL: srv.URL + "/v1/", APIKey: "key", Model: "m", PriceInputPerM: 0.3, PriceOutputPerM: 0.9}, srv.Client())
	return c, &got
}

var question = Question{
	Prompt:   "Is it a fruit?",
	Options:  []string{"basket", "discard"},
	Metadata: json.RawMessage(`{"detector":{"label":"unknown"}}`),
	Frame:    []byte{0xff, 0xd8, 0xff, 0xd9},
}

func TestAsk(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     int
		reply      string
		wantAction string
		wantErr    bool
	}{
		{"plain JSON", http.StatusOK, `{"action":"basket","confidence":0.9,"reason":"A kiwi."}`, "basket", false},
		{"in a code fence", http.StatusOK, "```json\n{\"action\":\"discard\",\"confidence\":0.8,\"reason\":\"A cup.\"}\n```", "discard", false},
		{"after reasoning", http.StatusOK, `<think>It looks {round}.</think>{"action":"basket","confidence":1,"reason":"An orange."}`, "basket", false},
		{"not an option", http.StatusOK, `{"action":"eat","confidence":0.9,"reason":"Tasty."}`, "", true},
		{"no JSON", http.StatusOK, "It is a fruit.", "", true},
		{"API error", http.StatusTooManyRequests, `{}`, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := fakeAPI(t, tt.status, tt.reply)
			call, err := c.Ask(t.Context(), question)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, want error %v", err, tt.wantErr)
			}
			if call.Action != tt.wantAction {
				t.Errorf("action = %q, want %q", call.Action, tt.wantAction)
			}
			if call.Model != "m" || call.Latency <= 0 {
				t.Errorf("call = %+v, want the model and a latency even on failure", call)
			}
			if tt.status == http.StatusOK && (call.InputTokens == nil || *call.InputTokens != 1000 || call.Output != tt.reply) {
				t.Errorf("call = %+v, want the usage and the raw reply", call)
			}
		})
	}
}

func TestAskCost(t *testing.T) {
	t.Parallel()
	c, _ := fakeAPI(t, http.StatusOK, `{"action":"basket","confidence":0.9,"reason":"x"}`)
	call, err := c.Ask(t.Context(), question)
	if err != nil {
		t.Fatal(err)
	}
	// 1000 × 0.3 / 1e6 + 40 × 0.9 / 1e6
	if want := 0.000336; call.CostUSD < want-1e-9 || call.CostUSD > want+1e-9 {
		t.Errorf("cost = %v, want %v", call.CostUSD, want)
	}
	if call.Confidence == nil || *call.Confidence != 0.9 {
		t.Errorf("confidence = %v, want 0.9", call.Confidence)
	}
}

func TestAskRequest(t *testing.T) {
	t.Parallel()
	c, got := fakeAPI(t, http.StatusOK, `{"action":"basket"}`)
	if _, err := c.Ask(t.Context(), question); err != nil {
		t.Fatal(err)
	}
	if got.Model != "m" || len(got.Messages) != 2 || got.Messages[0].Role != "system" {
		t.Fatalf("request = %+v", got)
	}
	if got.ChatTemplateKwargs["enable_thinking"] != false || got.MaxTokens != maxOutputTokens {
		t.Errorf("chat_template_kwargs = %v, max_tokens = %d, want reasoning off", got.ChatTemplateKwargs, got.MaxTokens)
	}
	if got.ResponseFormat == nil || got.ResponseFormat.Type != "json_object" {
		t.Errorf("response_format = %v, want json_object", got.ResponseFormat)
	}
	parts, err := json.Marshal(got.Messages[1].Content)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Is it a fruit?", "basket, discard", `\"label\":\"unknown\"`, "data:image/jpeg;base64,/9j/2Q=="} {
		if !strings.Contains(string(parts), want) {
			t.Errorf("user message %s lacks %s", parts, want)
		}
	}
}

func TestAskReasoningOn(t *testing.T) {
	t.Parallel()
	c, got := fakeAPI(t, http.StatusOK, `{"action":"basket"}`)
	c.cfg.Reasoning = true
	if _, err := c.Ask(t.Context(), question); err != nil {
		t.Fatal(err)
	}
	if got.ChatTemplateKwargs != nil || got.MaxTokens != maxReasoningTokens {
		t.Errorf("chat_template_kwargs = %v, max_tokens = %d, want reasoning on with room for it", got.ChatTemplateKwargs, got.MaxTokens)
	}
}

func TestAskNotConfigured(t *testing.T) {
	t.Parallel()
	c := New(Config{Model: "m"}, http.DefaultClient)
	if _, err := c.Ask(t.Context(), question); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("error = %v, want ErrNotConfigured", err)
	}
}
