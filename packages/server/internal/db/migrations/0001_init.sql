-- Initial schema, PostgreSQL 18.
--
-- Every fact has one owner. The agent owns what happened on the device
-- (the request, its own decision, execution, cancellation, applying a
-- correction); the server owns model calls and reviews. Status and source
-- are derived from those facts, so neither side can overwrite the other's.

-- +goose Up

CREATE TABLE devices (
    id            uuid        PRIMARY KEY DEFAULT uuidv7(),
    key           text        NOT NULL CHECK (key ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(key) <= 64),
    name          text        NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    -- SHA-256 of the device token; the token itself is never stored.
    token_hash    bytea       NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    metadata      jsonb       NOT NULL DEFAULT '{}'
                              CHECK (jsonb_typeof(metadata) = 'object' AND octet_length(metadata::text) <= 16384),
    agent_version text,
    last_seen_at  timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    deleted_at    timestamptz
);

-- A deleted device frees its key for a new one.
CREATE UNIQUE INDEX devices_key_idx ON devices (key) WHERE deleted_at IS NULL;

CREATE TABLE decisions (
    -- Made by the agent, so offline records and retries never duplicate.
    -- UUIDv7 sorts by creation time and serves as the journal cursor.
    id              uuid        PRIMARY KEY CHECK (uuid_extract_version(id) = 7),
    -- Taken from the token, never from the request body.
    device_id       uuid        NOT NULL REFERENCES devices (id),
    -- The agent bumps it on every change; an update applies only with a
    -- higher revision, so a late retry can't roll a decision back.
    revision        integer     NOT NULL CHECK (revision > 0),

    -- The request, written by the agent once: updates never set these.
    task            text        NOT NULL,
    -- A reflex has its own action as the only option.
    options         text[]      NOT NULL
                                CHECK (cardinality(options) BETWEEN 1 AND 32 AND array_position(options, NULL) IS NULL),
    action_class    text        NOT NULL CHECK (action_class IN ('reflex', 'reversible', 'irreversible')),
    proposed_action text        CHECK (proposed_action = ANY (options)),
    -- The question for the cloud model: for the decision or for the review.
    prompt          text,
    metadata        jsonb       NOT NULL DEFAULT '{}'
                                CHECK (jsonb_typeof(metadata) = 'object' AND octet_length(metadata::text) <= 16384),
    local_model     text,
    local_confidence real       CHECK (local_confidence BETWEEN 0 AND 1),
    needs_review    boolean     NOT NULL DEFAULT false,
    -- Device clock: may be off on a device without a real-time clock.
    decided_at      timestamptz NOT NULL,
    -- Server clock: when the server first got the decision.
    received_at     timestamptz NOT NULL DEFAULT now(),

    -- The decision. The agent sets it for reflex and reversible, the server
    -- for irreversible, together with the model call it came from.
    action          text        CHECK (action = ANY (options)),
    model_call_id   uuid,

    -- Agent: what the robot did.
    executed_at     timestamptz,
    cancelled_at    timestamptz,
    cancel_reason   text,

    -- Server: the review of an uncertain decision. reviewed_at without a call
    -- means the review gave up after failed calls (see model_calls). A
    -- decision has at most one correction, so it lives here: corrected_action
    -- is the review's answer when it disagrees, with its reason in the call.
    reviewed_at           timestamptz,
    review_call_id        uuid,
    corrected_action      text,
    -- Agent: when the robot carried the correction out.
    correction_applied_at timestamptz,

    source          text GENERATED ALWAYS AS (
        CASE
            WHEN action IS NULL THEN NULL
            WHEN action_class = 'irreversible' THEN 'server'
            ELSE 'device'
        END) STORED,
    -- Not indexed: an index on it would turn every status change into a
    -- non-HOT update, and the journal filters it while walking id order.
    status          text NOT NULL GENERATED ALWAYS AS (
        CASE
            WHEN cancelled_at IS NOT NULL THEN 'cancelled'
            WHEN corrected_action IS NOT NULL THEN 'corrected'
            WHEN executed_at IS NOT NULL THEN 'executed'
            WHEN action IS NULL THEN 'waiting_for_server'
            WHEN action_class = 'irreversible' THEN 'decided_on_server'
            ELSE 'decided_on_device'
        END) STORED,

    -- The device decides reflexes and reversible actions on the spot; a
    -- reflex is reported once already done.
    CHECK (action_class = 'irreversible' OR action IS NOT NULL),
    CHECK (action_class <> 'reflex' OR executed_at IS NOT NULL),
    -- A server decision always points to its model call, a device one never.
    CHECK (CASE action_class
               WHEN 'irreversible' THEN (action IS NULL) = (model_call_id IS NULL)
               ELSE model_call_id IS NULL
           END),
    -- The server and the review both ask the model this prompt.
    CHECK (prompt IS NOT NULL OR (action_class <> 'irreversible' AND NOT needs_review)),
    -- Only an irreversible decision waits, so only it can be cancelled.
    CHECK (cancelled_at IS NULL OR action_class = 'irreversible'),
    CHECK ((cancelled_at IS NULL) = (cancel_reason IS NULL)),
    CHECK (cancelled_at IS NULL OR executed_at IS NULL),
    -- Only uncertain reversible decisions are reviewed and corrected, and a
    -- correction is another of the options.
    CHECK (NOT needs_review OR action_class = 'reversible'),
    CHECK (reviewed_at IS NULL OR needs_review),
    CHECK (review_call_id IS NULL OR reviewed_at IS NOT NULL),
    CHECK (corrected_action IS NULL
           OR (review_call_id IS NOT NULL AND corrected_action = ANY (options) AND corrected_action <> action)),
    CHECK (correction_applied_at IS NULL OR corrected_action IS NOT NULL)
);

-- One device's journal, newest first. The journal of all devices walks the
-- primary key backwards.
CREATE INDEX decisions_device_idx ON decisions (device_id, id DESC);

-- The review queue, oldest first.
CREATE INDEX decisions_review_queue_idx ON decisions (id)
    WHERE needs_review AND reviewed_at IS NULL;

-- Corrections a device hasn't carried out, returned on every sync.
CREATE INDEX decisions_pending_correction_idx ON decisions (device_id)
    WHERE corrected_action IS NOT NULL AND correction_applied_at IS NULL;

-- One row per call to the cloud model: deciding an irreversible action or
-- reviewing a reversible one. Retries and failed calls are kept, so the
-- cost is the sum of all calls, and failed reviews can be counted.
CREATE TABLE model_calls (
    id            uuid        PRIMARY KEY DEFAULT uuidv7(),
    decision_id   uuid        NOT NULL REFERENCES decisions (id) ON DELETE CASCADE,
    purpose       text        NOT NULL CHECK (purpose IN ('decide', 'review')),
    model         text        NOT NULL,
    -- The parsed answer; empty when the call failed or the reply wasn't valid.
    action        text,
    confidence    real        CHECK (confidence BETWEEN 0 AND 1),
    reason        text,
    -- The model's reply as is, and what went wrong with the call if anything.
    output        text,
    error         text,
    latency_ms    integer     NOT NULL CHECK (latency_ms >= 0),
    input_tokens  integer     CHECK (input_tokens >= 0),
    output_tokens integer     CHECK (output_tokens >= 0),
    -- At the price when the call was made. Unbounded numeric: a single
    -- token can cost less than a hundred-millionth of a dollar.
    cost_usd      numeric     NOT NULL DEFAULT 0 CHECK (cost_usd >= 0),
    created_at    timestamptz NOT NULL DEFAULT now(),

    -- Finds a decision's calls, and lets a decision require calls of its own.
    UNIQUE (decision_id, id)
);

ALTER TABLE decisions
    ADD FOREIGN KEY (id, model_call_id) REFERENCES model_calls (decision_id, id),
    ADD FOREIGN KEY (id, review_call_id) REFERENCES model_calls (decision_id, id);

-- What the robot saw. Its size and time are in the JPEG and the decision.
CREATE TABLE frames (
    decision_id uuid  PRIMARY KEY REFERENCES decisions (id) ON DELETE CASCADE,
    -- A 640×480 frame is 30–60 KB; the limit leaves room for larger ones.
    jpeg        bytea NOT NULL CHECK (octet_length(jpeg) <= 1048576)
);

-- JPEG is already compressed: store it out of line without trying again.
ALTER TABLE frames ALTER COLUMN jpeg SET STORAGE EXTERNAL;

-- +goose Down

-- decisions and model_calls reference each other, so they go in one statement.
DROP TABLE frames, model_calls, decisions, devices;
