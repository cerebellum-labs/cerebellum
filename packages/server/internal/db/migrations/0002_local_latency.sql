-- How long the robot's local model took to decide, as the agent measured
-- it. Written once with the request, like local_confidence. Null for
-- decisions from agents that don't report it.

-- +goose Up

-- Nullable without a default: no table rewrite.
ALTER TABLE decisions ADD COLUMN local_latency_ms integer CHECK (local_latency_ms >= 0);

-- +goose Down

ALTER TABLE decisions DROP COLUMN local_latency_ms;
