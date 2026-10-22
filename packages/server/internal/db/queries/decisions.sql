-- name: ListDecisions :many
-- ListDecisions pages through the journal, newest first: the next page
-- starts before the last id of this one. Every filter is optional.
-- GetDecision selects the same columns, so their rows convert into each
-- other: keep the two in step.
SELECT sqlc.embed(d),
       dev.key   AS device_key,
       dev.name  AS device_name,
       EXISTS (SELECT 1 FROM frames f WHERE f.decision_id = d.id) AS has_frame,
       mc.model         AS call_model,
       mc.confidence    AS call_confidence,
       mc.reason        AS call_reason,
       mc.latency_ms    AS call_latency_ms,
       mc.cost_usd      AS call_cost_usd,
       mc.input_tokens  AS call_input_tokens,
       mc.output_tokens AS call_output_tokens,
       mc.created_at    AS answered_at,
       rc.model         AS review_model,
       rc.reason        AS review_reason
FROM decisions d
JOIN devices dev ON dev.id = d.device_id
LEFT JOIN model_calls mc ON mc.id = d.model_call_id
LEFT JOIN model_calls rc ON rc.id = d.review_call_id
WHERE (sqlc.narg(device_id)::uuid IS NULL OR d.device_id = sqlc.narg(device_id)::uuid)
  AND (sqlc.narg(status)::text IS NULL OR d.status = sqlc.narg(status)::text)
  AND (sqlc.narg(before)::uuid IS NULL OR d.id < sqlc.narg(before)::uuid)
ORDER BY d.id DESC
LIMIT sqlc.arg(row_limit);

-- name: GetDecision :one
SELECT sqlc.embed(d),
       dev.key   AS device_key,
       dev.name  AS device_name,
       EXISTS (SELECT 1 FROM frames f WHERE f.decision_id = d.id) AS has_frame,
       mc.model         AS call_model,
       mc.confidence    AS call_confidence,
       mc.reason        AS call_reason,
       mc.latency_ms    AS call_latency_ms,
       mc.cost_usd      AS call_cost_usd,
       mc.input_tokens  AS call_input_tokens,
       mc.output_tokens AS call_output_tokens,
       mc.created_at    AS answered_at,
       rc.model         AS review_model,
       rc.reason        AS review_reason
FROM decisions d
JOIN devices dev ON dev.id = d.device_id
LEFT JOIN model_calls mc ON mc.id = d.model_call_id
LEFT JOIN model_calls rc ON rc.id = d.review_call_id
WHERE d.id = $1;

-- name: GetFrame :one
SELECT jpeg
FROM frames
WHERE decision_id = $1;

-- name: DecisionStats :one
-- DecisionStats counts decisions, all or one device's, in one pass.
SELECT count(*) AS total,
       count(*) FILTER (WHERE source = 'device') AS source_device,
       count(*) FILTER (WHERE source = 'server') AS source_server,
       count(*) FILTER (WHERE status = 'decided_on_device')  AS status_decided_on_device,
       count(*) FILTER (WHERE status = 'waiting_for_server') AS status_waiting_for_server,
       count(*) FILTER (WHERE status = 'decided_on_server')  AS status_decided_on_server,
       count(*) FILTER (WHERE status = 'executed')           AS status_executed,
       count(*) FILTER (WHERE status = 'corrected')          AS status_corrected,
       count(*) FILTER (WHERE status = 'cancelled')          AS status_cancelled,
       count(*) FILTER (WHERE action_class = 'reflex')       AS class_reflex,
       count(*) FILTER (WHERE action_class = 'reversible')   AS class_reversible,
       count(*) FILTER (WHERE action_class = 'irreversible') AS class_irreversible,
       count(*) FILTER (WHERE corrected_action IS NOT NULL)  AS corrections
FROM decisions
WHERE sqlc.narg(device_id)::uuid IS NULL OR device_id = sqlc.narg(device_id)::uuid;

-- name: ModelCallStats :one
-- ModelCallStats sums the cost of every model call, failed and review ones
-- included, and averages how long the server took to decide; the average
-- means nothing while decide_calls is 0.
SELECT coalesce(sum(mc.cost_usd), 0)::numeric AS cost_usd,
       count(*) FILTER (WHERE mc.purpose = 'decide' AND mc.error IS NULL) AS decide_calls,
       coalesce(avg(mc.latency_ms) FILTER (WHERE mc.purpose = 'decide' AND mc.error IS NULL), 0)::float8 AS avg_decide_latency_ms
FROM model_calls mc
JOIN decisions d ON d.id = mc.decision_id
WHERE sqlc.narg(device_id)::uuid IS NULL OR d.device_id = sqlc.narg(device_id)::uuid;
