-- name: CreateUser :one
INSERT INTO users (id, email, full_name, phone, password_hash)
VALUES (@id, @email, @full_name, sqlc.narg('phone'), sqlc.narg('password_hash'))
RETURNING *;

-- name: CreateOrganization :one
INSERT INTO organizations (id, name, slug)
VALUES (@id, @name, @slug)
RETURNING *;

-- name: CreateMembership :one
INSERT INTO memberships (organization_id, user_id, role)
VALUES (@organization_id, @user_id, @role)
RETURNING *;

-- name: ListOrganizationMembers :many
-- Members of the organization in scope, newest first.
SELECT m.role, m.status, m.created_at AS joined_at, u.id AS user_id, u.email, u.full_name, u.phone
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.organization_id = @organization_id
ORDER BY m.created_at DESC, u.id DESC;

-- name: ListUserOrganizations :many
-- Organizations the user in scope belongs to, by name.
SELECT o.id, o.name, o.slug, m.role, m.status
FROM memberships m
JOIN organizations o ON o.id = m.organization_id
WHERE m.user_id = @user_id
ORDER BY o.name;

-- name: GetOrganizationBySlug :one
SELECT * FROM organizations WHERE slug = @slug;
