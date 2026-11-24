package persistent

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver used by goose
	"github.com/pressly/goose/v3"

	"github.com/agusheryanto182/go-next-saas/api/migrations"
)

// Migrate applies every pending migration. It must connect as the owner role (MIGRATION_DATABASE_URL),
// which owns the schema; the runtime role cannot run DDL.
func Migrate(ctx context.Context, url string, log *slog.Logger) error {
	return withProvider(url, migrations.FS, func(p *goose.Provider) error {
		results, err := p.Up(ctx)
		for _, r := range results {
			log.InfoContext(ctx, "migration applied", slog.Int64("version", r.Source.Version), slog.String("file", r.Source.Path))
		}
		if err != nil {
			return fmt.Errorf("migrate up: %w", err)
		}
		return nil
	})
}

func withProvider(url string, fsys fs.FS, fn func(*goose.Provider) error) error {
	db, err := sql.Open("pgx", url)
	if err != nil {
		return fmt.Errorf("open migration connection: %w", err)
	}
	defer db.Close()

	p, err := goose.NewProvider(goose.DialectPostgres, db, fsys)
	if err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	return fn(p)
}
