//go:build integration

package persistent

import (
	"log/slog"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"github.com/agusheryanto182/go-next-saas/api/internal/repo/persistent/pgtest"
	"github.com/agusheryanto182/go-next-saas/api/migrations"
)

// Internal test: goose's provider is not part of the package API.
func TestMigrationsUpDownUp(t *testing.T) {
	t.Parallel()
	db := pgtest.Start(t).OwnerURL
	ctx := t.Context()
	log := slog.New(slog.DiscardHandler)

	require.NoError(t, Migrate(ctx, db, log), "up")
	require.NoError(t, withProvider(db, migrations.FS, func(p *goose.Provider) error {
		_, err := p.DownTo(ctx, 0)
		return err
	}), "down to 0")
	require.NoError(t, Migrate(ctx, db, log), "up again")
}
