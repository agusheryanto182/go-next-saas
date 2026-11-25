// Package migrations embeds the SQL migrations so the API binary can apply them (`app migrate`).
// Files follow goose's format: NNNNN_name.sql with `-- +goose Up` and `-- +goose Down` sections.
package migrations

import "embed"

// FS holds every migration file.
//
//go:embed *.sql
var FS embed.FS
