package model

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/cerebellum-labs/cerebellum/packages/server/internal/db"
)

// Purposes of a call, as model_calls.purpose keeps them.
const (
	PurposeDecide = "decide"
	PurposeReview = "review"
)

// Record keeps a call of the decision in model_calls, failed ones too:
// callErr is the error Ask returned with it. It returns the call's id.
func Record(ctx context.Context, q *db.Queries, decisionID uuid.UUID, purpose string, call Call, callErr error) (uuid.UUID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("new model call id: %w", err)
	}
	params := db.InsertModelCallParams{
		ID:           id,
		DecisionID:   decisionID,
		Purpose:      purpose,
		Model:        call.Model,
		Confidence:   call.Confidence,
		Output:       nonEmpty(call.Output),
		LatencyMs:    int32(min(call.Latency.Milliseconds(), 1<<31-1)), //nolint:gosec // capped to int32 above
		InputTokens:  call.InputTokens,
		OutputTokens: call.OutputTokens,
		CostUsd:      call.CostUSD,
	}
	if callErr == nil {
		params.Action = &call.Action
		params.Reason = nonEmpty(call.Reason)
	} else {
		msg := callErr.Error()
		params.Error = &msg
	}
	if err := q.InsertModelCall(ctx, params); err != nil {
		return uuid.UUID{}, fmt.Errorf("record model call: %w", err)
	}
	return id, nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
