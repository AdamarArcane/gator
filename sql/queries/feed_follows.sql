-- name: CreateFeedFollow :many
WITH inserted_feed_follow AS (
    INSERT INTO feed_follows (id, user_id, feed_id, created_at, updated_at)
    VALUES ($1, $2, $3, NOW(), NOW())
    RETURNING *
)
SELECT
    iff.*,
    u.name AS user_name,
    f.name AS feed_name
FROM inserted_feed_follow iff
JOIN users u ON u.id = iff.user_id
JOIN feeds f ON f.id = iff.feed_id;

-- name: GetFeedFollowsForUser :many
SELECT f.name, f.url
FROM feed_follows ff
JOIN feeds f ON ff.feed_id = f.id
WHERE ff.user_id = $1;