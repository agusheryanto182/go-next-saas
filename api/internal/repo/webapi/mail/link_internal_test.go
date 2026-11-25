package mail

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetLink(t *testing.T) {
	t.Parallel()
	for _, token := range []string{"tok_Ab-9", "a+b/c&d=e f#g"} {
		link := resetLink("https://app.example.test", token)
		require.True(t, strings.HasPrefix(link, "https://app.example.test/reset-password#token="), link)

		u, err := url.Parse(link)
		require.NoError(t, err)
		assert.Empty(t, u.RawQuery, "the token never goes in the query")
		params, err := url.ParseQuery(u.EscapedFragment())
		require.NoError(t, err)
		assert.Equal(t, token, params.Get("token"), "the page reads back the exact token")
	}
}
