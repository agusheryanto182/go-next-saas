package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadPasswordLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
		err   error
	}{
		{"line", "correct horse\nignored\n", "correct horse", nil},
		{"CRLF", "correct horse\r\n", "correct horse", nil},
		{"no newline", "correct horse", "correct horse", nil},
		{"spaces are kept", " pass word \n", " pass word ", nil},
		{"empty", "", "", nil},
		{"empty line", "\n", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := readPasswordLine(strings.NewReader(tt.input))
			require.ErrorIs(t, err, tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func openFile(t *testing.T, content string) *os.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "password")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	f, err := os.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestReadPasswordFromPipe(t *testing.T) {
	t.Parallel()
	var prompt strings.Builder

	got, err := readPassword(openFile(t, "correct horse\n"), &prompt)

	require.NoError(t, err)
	assert.Equal(t, "correct horse", got)
	assert.Empty(t, prompt.String(), "no prompt when stdin is not a terminal")
}

// The empty check is shared by the terminal and pipe paths, so pressing Enter at the prompt is
// refused the same way.
func TestReadPasswordRefusesEmpty(t *testing.T) {
	t.Parallel()

	for _, content := range []string{"", "\n", "\r\n"} {
		_, err := readPassword(openFile(t, content), io.Discard)
		require.ErrorIs(t, err, errPasswordEmpty, "%q", content)
	}
}
