-- name: NotifyDashboard :exec
-- NotifyDashboard tells every server instance, through LISTEN dashboard,
-- that a row changed. Call it in the transaction that makes the change:
-- the notification goes out on commit and never for a rollback, and
-- identical ones in one transaction arrive once.
SELECT pg_notify('dashboard', json_build_object('table', sqlc.arg(table_name)::text, 'id', sqlc.arg(id)::uuid)::text);
