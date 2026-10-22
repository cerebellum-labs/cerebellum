// Package model asks the cloud model for decisions through an
// OpenAI-compatible chat completions API, Nebius Token Factory by default.
package model

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"
)

const (
	systemPrompt = "You make decisions for a robot. Use the photo if there is one and the context. " +
		"Answer the question. Reply with JSON only: " +
		`{"action": one of the options, "confidence": number from 0 to 1, "reason": one short sentence}.`
	// Token caps, far above what was measured, since an answer cut off by
	// the cap is lost; only the tokens actually made are paid for. Without
	// reasoning an answer is a JSON object of about 40 tokens.
	maxOutputTokens = 1000
	// With reasoning on, the answer followed 300–500 tokens of thinking on
	// simple frames. At about 230 tokens a second, the cap is half a minute
	// of generation and 0.0072 USD at Nemotron's price.
	maxReasoningTokens = 8000
	// How much of an error response is kept for the log.
	maxErrorBody = 2 << 10
)

// ErrNotConfigured is returned when no API key is set.
var ErrNotConfigured = errors.New("the cloud model is not configured: NEBIUS_API_KEY is not set")

// ErrInvalidAnswer wraps an answer that is not JSON or names no option.
var ErrInvalidAnswer = errors.New("the model's answer is not usable")

// Config is where the model is and what it costs.
type Config struct {
	BaseURL         string
	APIKey          string
	Model           string
	PriceInputPerM  float64
	PriceOutputPerM float64
	// Reasoning lets a reasoning model think before it answers. Measured on
	// Nemotron 3.5 Super VL: about 1 s and 40 output tokens without it,
	// about 2 s and 300–500 with it, and more doubt about the robot's own
	// detector. Off, the request asks the chat template to skip it.
	Reasoning bool
}

// Client asks the model. It is safe for concurrent use.
type Client struct {
	cfg  Config
	http *http.Client
}

// New returns a client sending requests through httpClient.
func New(cfg Config, httpClient *http.Client) *Client {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Client{cfg: cfg, http: httpClient}
}

// Configured reports whether the client has an API key to call with.
func (c *Client) Configured() bool { return c.cfg.APIKey != "" }

// Question is what the model decides on.
type Question struct {
	Prompt  string
	Options []string
	// Metadata is the robot's own context, passed as JSON; may be empty.
	Metadata json.RawMessage
	// Frame is the robot's JPEG; nil without a camera.
	Frame []byte
}

// Call is one request to the model, worth keeping whether it worked or not.
type Call struct {
	Model string
	// The parsed answer; Action is empty when the answer was not usable.
	Action     string
	Confidence *float32
	Reason     string
	// Output is the model's reply as is.
	Output       string
	Latency      time.Duration
	InputTokens  *int32
	OutputTokens *int32
	CostUSD      float64
}

// Ask sends q and returns the call. The call is filled as far as it got
// even when err is not nil, so a failed call can be recorded too.
func (c *Client) Ask(ctx context.Context, q Question) (Call, error) {
	call := Call{Model: c.cfg.Model}
	if !c.Configured() {
		return call, ErrNotConfigured
	}
	completion, err := c.complete(ctx, q, &call)
	if err != nil {
		return call, err
	}
	return call, c.read(&call, completion, q.Options)
}

// complete sends the request and decodes the response, timing the call.
func (c *Client) complete(ctx context.Context, q Question, call *Call) (chatResponse, error) {
	body, err := json.Marshal(c.request(q))
	if err != nil {
		return chatResponse{}, fmt.Errorf("encode model request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return chatResponse{}, fmt.Errorf("build model request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	defer func() { call.Latency = time.Since(start) }()
	resp, err := c.http.Do(req)
	if err != nil {
		return chatResponse{}, fmt.Errorf("call the model: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return chatResponse{}, fmt.Errorf("read the model's response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return chatResponse{}, fmt.Errorf("the model API answered %d: %s", resp.StatusCode, truncate(data, maxErrorBody))
	}
	var completion chatResponse
	if err := json.Unmarshal(data, &completion); err != nil {
		return chatResponse{}, fmt.Errorf("decode the model's response: %w", err)
	}
	return completion, nil
}

// read fills the call from the response: usage and cost, the reply, and
// the answer if it names one of the options.
func (c *Client) read(call *Call, completion chatResponse, options []string) error {
	if completion.Usage != nil {
		in, out := completion.Usage.PromptTokens, completion.Usage.CompletionTokens
		call.InputTokens, call.OutputTokens = &in, &out
		call.CostUSD = float64(in)*c.cfg.PriceInputPerM/1e6 + float64(out)*c.cfg.PriceOutputPerM/1e6
	}
	if len(completion.Choices) == 0 {
		return fmt.Errorf("%w: no choices in the response", ErrInvalidAnswer)
	}
	choice := completion.Choices[0]
	call.Output = choice.Message.Content
	if call.Output == "" && choice.FinishReason == "length" {
		return fmt.Errorf("%w: the token limit ran out before the answer, likely spent on reasoning", ErrInvalidAnswer)
	}

	ans, err := parseAnswer(call.Output)
	if err != nil {
		return err
	}
	if !slices.Contains(options, ans.Action) {
		return fmt.Errorf("%w: %q is not one of the options", ErrInvalidAnswer, ans.Action)
	}
	call.Action = ans.Action
	call.Reason = ans.Reason
	if ans.Confidence != nil && *ans.Confidence >= 0 && *ans.Confidence <= 1 {
		confidence := float32(*ans.Confidence)
		call.Confidence = &confidence
	}
	return nil
}

func (c *Client) request(q Question) chatRequest {
	var text strings.Builder
	text.WriteString(q.Prompt)
	text.WriteString("\nOptions: ")
	text.WriteString(strings.Join(q.Options, ", "))
	if len(q.Metadata) > 0 && string(q.Metadata) != "{}" {
		text.WriteString("\nContext from the robot (JSON): ")
		text.Write(q.Metadata)
	}
	content := []contentPart{{Type: "text", Text: text.String()}}
	if q.Frame != nil {
		url := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(q.Frame)
		content = append(content, contentPart{Type: "image_url", ImageURL: &imageURL{URL: url}})
	}
	req := chatRequest{
		Model: c.cfg.Model,
		Messages: []message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		},
		MaxTokens:   maxOutputTokens,
		Temperature: 0,
		// Without it the reply can break JSON, like a reason with no
		// opening quote after the model has reasoned.
		ResponseFormat: &responseFormat{Type: "json_object"},
	}
	if c.cfg.Reasoning {
		req.MaxTokens = maxReasoningTokens
	} else {
		req.ChatTemplateKwargs = map[string]any{"enable_thinking": false}
	}
	return req
}

type answer struct {
	Action     string   `json:"action"`
	Confidence *float64 `json:"confidence"`
	Reason     string   `json:"reason"`
}

// parseAnswer finds the JSON object in a reply. Models often wrap it in a
// code fence or put their reasoning first, so it takes the text between
// the first { after any <think> block and the last }.
func parseAnswer(output string) (answer, error) {
	text := output
	if _, after, ok := strings.Cut(text, "</think>"); ok {
		text = after
	}
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return answer{}, fmt.Errorf("%w: no JSON object in the reply", ErrInvalidAnswer)
	}
	var a answer
	if err := json.Unmarshal([]byte(text[start:end+1]), &a); err != nil {
		return answer{}, fmt.Errorf("%w: %w", ErrInvalidAnswer, err)
	}
	return a, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []message       `json:"messages"`
	MaxTokens      int             `json:"max_tokens"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
	// Passed to the model's chat template by vLLM-based APIs like Token
	// Factory.
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type message struct {
	Role string `json:"role"`
	// A string for the system prompt, parts for the user's text and image.
	Content any `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int32 `json:"prompt_tokens"`
		CompletionTokens int32 `json:"completion_tokens"`
	} `json:"usage"`
}
