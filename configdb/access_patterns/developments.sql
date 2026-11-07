-- name: ListDevelopments :many
SELECT name, starting_label, attention_label, standards_reviewer_app, specification_reviewer_app
FROM development
ORDER BY name;

-- name: CreateDevelopment :exec
INSERT INTO development (
    name,
    starting_label,
    attention_label,
    standards_reviewer_app,
    specification_reviewer_app
) VALUES (?, ?, ?, ?, ?);

-- name: ListDevelopmentStartingLabels :many
SELECT starting_label FROM development ORDER BY starting_label;

-- name: ListDevelopmentReviewerApps :many
SELECT app_id FROM (
    SELECT standards_reviewer_app AS app_id FROM development
    UNION
    SELECT specification_reviewer_app AS app_id FROM development
)
ORDER BY app_id;

-- name: ListDevelopmentChecks :many
SELECT name FROM development_check WHERE development_name = ? ORDER BY name;

-- name: CreateDevelopmentCheck :exec
INSERT INTO development_check (development_name, name) VALUES (?, ?);
