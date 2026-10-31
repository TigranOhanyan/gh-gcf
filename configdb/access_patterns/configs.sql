-- name: GetAppliedMigrationVersions :many
SELECT version_id FROM goose_db_version WHERE is_applied = 1;
