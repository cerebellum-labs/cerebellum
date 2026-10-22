-- name: ListReviewQueue :many
-- ListReviewQueue returns the oldest decisions waiting for a review, with
-- how many review calls have failed on each so far.
SELECT d.id, d.action, d.options, d.prompt, d.metadata,
       (SELECT count(*)
        FROM model_calls mc
        WHERE mc.decision_id = d.id
          AND mc.purpose = 'review'
          AND mc.error IS NOT NULL) AS failed_reviews
FROM decisions d
WHERE d.needs_review
  AND d.reviewed_at IS NULL
ORDER BY d.id
LIMIT $1;

-- name: FinishReview :execrows
-- FinishReview closes a review: with the call that answered it and the
-- corrected action when the answer disagreed, or with neither when it gave
-- up. A review is finished once.
UPDATE decisions
SET reviewed_at      = now(),
    review_call_id   = sqlc.narg(review_call_id),
    corrected_action = sqlc.narg(corrected_action)
WHERE id = sqlc.arg(id)
  AND reviewed_at IS NULL;
