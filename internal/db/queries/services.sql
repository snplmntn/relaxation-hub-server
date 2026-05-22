-- name: CreateService :one
INSERT INTO services (
    name,
    description,
    base_price,
    duration_minutes,
    category,
    is_active,
    preview_image_url,
    therapist_commission
) VALUES (
    sqlc.arg(name),
    sqlc.arg(description),
    sqlc.arg(base_price),
    sqlc.arg(duration_minutes),
    sqlc.arg(category),
    sqlc.arg(is_active),
    sqlc.arg(preview_image_url),
    sqlc.arg(therapist_commission)
)
RETURNING *;

-- name: GetServiceByID :one
SELECT *
FROM services
WHERE service_id = sqlc.arg(service_id)::bigint
  AND deleted_at IS NULL;

-- name: GetServicesByIDs :many
SELECT *
FROM services
WHERE service_id = ANY(sqlc.arg(service_ids)::bigint[])
  AND deleted_at IS NULL;

-- name: ListActiveServices :many
SELECT *
FROM services
WHERE deleted_at IS NULL
  AND is_active = TRUE
ORDER BY name ASC;

-- name: ListRecentServicesByUser :many
SELECT sqlc.embed(s)
FROM services s
INNER JOIN (
    SELECT service_id, MAX(created_at) AS last_booked
    FROM bookings
    WHERE client_id = sqlc.arg(client_id)::bigint
    GROUP BY service_id
) latest_b ON s.service_id = latest_b.service_id
WHERE s.deleted_at IS NULL
  AND latest_b.last_booked > NOW() - INTERVAL '30 days'
ORDER BY latest_b.last_booked DESC
LIMIT 3;

-- name: ListPopularServices :many
SELECT sqlc.embed(s)
FROM services s
INNER JOIN bookings b ON b.service_id = s.service_id
WHERE s.deleted_at IS NULL
  AND s.is_active = TRUE
  AND b.status = 'completed'
  AND b.created_at > NOW() - INTERVAL '30 days'
GROUP BY
    s.service_id,
    s.name,
    s.description,
    s.base_price,
    s.duration_minutes,
    s.category,
    s.is_active,
    s.preview_image_url,
    s.therapist_commission,
    s.deleted_at,
    s.created_at
ORDER BY COUNT(b.booking_id) DESC
LIMIT 3;

-- name: ListUnavailableServices :many
SELECT *
FROM services
WHERE is_active = FALSE
  AND deleted_at IS NULL
ORDER BY name ASC
LIMIT 3;

-- name: SoftDeleteService :execrows
UPDATE services
SET deleted_at = CURRENT_TIMESTAMP
WHERE service_id = sqlc.arg(service_id)::bigint
  AND deleted_at IS NULL;
