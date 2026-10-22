-- name: UpsertDecision :one
-- UpsertDecision writes what the agent owns. A new decision is inserted
-- whole, except the action of an irreversible one, which only the server
-- sets. An existing one takes only the agent's later facts (execution,
-- cancellation, applying a correction), and only from its own device and
-- with a higher revision: a late retry can't roll it back, and no device
-- can touch another's. No row comes back when nothing was written.
INSERT INTO decisions (
    id, device_id, revision, task, options, action_class, proposed_action, prompt,
    metadata, local_model, local_confidence, local_latency_ms, needs_review, decided_at, action,
    executed_at, cancelled_at, cancel_reason, correction_applied_at
)
VALUES (
    sqlc.arg(id), sqlc.arg(device_id), sqlc.arg(revision), sqlc.arg(task), sqlc.arg(options),
    sqlc.arg(action_class), sqlc.narg(proposed_action), sqlc.narg(prompt), sqlc.arg(metadata),
    sqlc.narg(local_model), sqlc.narg(local_confidence), sqlc.narg(local_latency_ms),
    sqlc.arg(needs_review), sqlc.arg(decided_at),
    CASE WHEN sqlc.arg(action_class)::text = 'irreversible' THEN NULL ELSE sqlc.narg(action)::text END,
    sqlc.narg(executed_at), sqlc.narg(cancelled_at), sqlc.narg(cancel_reason),
    -- A new decision has no correction to apply. The row proposed here is
    -- checked even when it conflicts, so the value goes in by the update.
    NULL
)
ON CONFLICT (id) DO UPDATE
SET revision              = excluded.revision,
    executed_at           = excluded.executed_at,
    cancelled_at          = excluded.cancelled_at,
    cancel_reason         = excluded.cancel_reason,
    correction_applied_at = sqlc.narg(correction_applied_at)::timestamptz
WHERE decisions.device_id = excluded.device_id
  AND excluded.revision > decisions.revision
RETURNING id;

-- name: DecisionOwner :one
-- DecisionOwner tells which device a decision belongs to.
SELECT device_id
FROM decisions
WHERE id = $1;

-- name: InsertFrame :exec
-- InsertFrame keeps the first frame of a decision; a frame never changes.
INSERT INTO frames (decision_id, jpeg)
VALUES ($1, $2)
ON CONFLICT (decision_id) DO NOTHING;

-- name: InsertModelCall :exec
INSERT INTO model_calls (
    id, decision_id, purpose, model, action, confidence, reason, output, error,
    latency_ms, input_tokens, output_tokens, cost_usd
)
VALUES (
    sqlc.arg(id), sqlc.arg(decision_id), sqlc.arg(purpose), sqlc.arg(model), sqlc.narg(action),
    sqlc.narg(confidence), sqlc.narg(reason), sqlc.narg(output), sqlc.narg(error),
    sqlc.arg(latency_ms), sqlc.narg(input_tokens), sqlc.narg(output_tokens), sqlc.arg(cost_usd)
);

-- name: SetServerDecision :execrows
-- SetServerDecision records the server's answer to an irreversible
-- decision. Only the first answer counts.
UPDATE decisions
SET action = $2,
    model_call_id = $3
WHERE id = $1
  AND action_class = 'irreversible'
  AND model_call_id IS NULL;

-- name: TouchDevice :exec
UPDATE devices
SET last_seen_at = now()
WHERE id = $1;

-- name: UpdateDeviceInfo :exec
-- UpdateDeviceInfo records what the agent reports about itself on sync.
UPDATE devices
SET last_seen_at  = now(),
    agent_version = $2,
    metadata      = $3
WHERE id = $1;

-- name: ListPendingCorrections :many
-- ListPendingCorrections returns the corrections the device hasn't carried
-- out, oldest first.
SELECT d.id, d.corrected_action, d.reviewed_at, d.correction_applied_at,
       rc.model  AS review_model,
       rc.reason AS review_reason
FROM decisions d
LEFT JOIN model_calls rc ON rc.id = d.review_call_id
WHERE d.device_id = $1
  AND d.corrected_action IS NOT NULL
  AND d.correction_applied_at IS NULL
ORDER BY d.id;
