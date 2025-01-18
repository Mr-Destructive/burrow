-- name: GetUser :one
SELECT id, name, password, is_admin FROM authors WHERE username = ?;

-- name: CreatePost :one
INSERT INTO posts (title, slug, body, metadata, author_id) 
VALUES (?, ?, ?, ?, ?) RETURNING *;

-- name: CreateAuthor :one
INSERT INTO authors (username, name, password, is_admin)
VALUES (?, ?, ?, ?) RETURNING id;

