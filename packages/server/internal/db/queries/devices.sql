-- name: GetDeviceByTokenHash :one
-- GetDeviceByTokenHash finds the device a token belongs to. A deleted
-- device's token no longer works.
SELECT id, key, name
FROM devices
WHERE token_hash = $1
  AND deleted_at IS NULL;

-- The device queries below return the whole row, so they share the Device
-- model; token_hash never leaves the server.

-- name: ListDevices :many
SELECT *
FROM devices
WHERE deleted_at IS NULL
ORDER BY created_at, id;

-- name: CreateDevice :one
INSERT INTO devices (key, name, token_hash)
VALUES ($1, $2, $3)
RETURNING *;

-- name: RotateDeviceToken :one
-- RotateDeviceToken replaces the token hash: the old token stops working at
-- once.
UPDATE devices
SET token_hash = $2
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: DeleteDevice :one
-- DeleteDevice marks the device deleted: its token stops working, its
-- journal stays.
UPDATE devices
SET deleted_at = now()
WHERE id = $1
  AND deleted_at IS NULL
RETURNING *;

-- name: GetDevice :one
SELECT *
FROM devices
WHERE id = $1;
