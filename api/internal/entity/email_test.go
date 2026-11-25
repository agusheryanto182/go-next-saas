package entity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/agusheryanto182/go-next-saas/api/internal/entity"
)

func TestEmailKindValid(t *testing.T) {
	t.Parallel()
	assert.True(t, entity.EmailPasswordReset.Valid())
	assert.True(t, entity.EmailPasswordChanged.Valid())
	assert.False(t, entity.EmailKind("invitation").Valid())
	assert.False(t, entity.EmailKind("").Valid())
}

func TestEmailDataValues(t *testing.T) {
	t.Parallel()
	assert.Empty(t, entity.EmailData{}.Values())
	assert.Equal(t, []string{"Alice"}, entity.EmailData{FullName: "Alice"}.Values())
	assert.Equal(t, []string{"Alice", "tok"}, entity.EmailData{FullName: "Alice", Token: "tok"}.Values())
}
