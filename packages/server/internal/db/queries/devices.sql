-- name: GetDeviceByTokenHash :one
-- GetDeviceByTokenHash finds the device a token belongs to. A deleted
-- device's token no longer works.
SELECT id, key, name
FROM devices
WHERE token_hash = $1
  AND deleted_at IS NULL;
