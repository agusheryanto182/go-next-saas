package password

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cheap keeps the tests fast; New's production parameters are covered by TestProductionRoundTrip.
func cheap() params { return params{memory: 64, time: 1, threads: 1} }

func TestProductionRoundTrip(t *testing.T) {
	t.Parallel()
	h := New()
	ctx := context.Background()

	encoded, err := h.Hash(ctx, "correct horse battery staple")
	require.NoError(t, err)
	assert.Regexp(t, `^\$argon2id\$v=19\$m=19456,t=2,p=1\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{43}$`, encoded)

	ok, err := h.Verify(ctx, encoded, "correct horse battery staple")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.False(t, h.NeedsRehash(encoded))
	assert.Equal(t, runtime.GOMAXPROCS(0), cap(h.slots))
}

func TestVerify(t *testing.T) {
	t.Parallel()
	h := newHasher(cheap(), 2)
	ctx := context.Background()
	encoded, err := h.Hash(ctx, "s3cret-password")
	require.NoError(t, err)

	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{name: "same password", password: "s3cret-password", want: true},
		{name: "wrong password", password: "s3cret-passwore"},
		{name: "prefix of password", password: "s3cret"},
		{name: "empty password", password: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ok, err := h.Verify(ctx, encoded, tt.password)
			require.NoError(t, err)
			assert.Equal(t, tt.want, ok)
		})
	}
}

func TestHashUsesFreshSalt(t *testing.T) {
	t.Parallel()
	h := newHasher(cheap(), 2)
	ctx := context.Background()

	first, err := h.Hash(ctx, "same password")
	require.NoError(t, err)
	second, err := h.Hash(ctx, "same password")
	require.NoError(t, err)
	assert.NotEqual(t, first, second)

	for _, encoded := range []string{first, second} {
		ok, err := h.Verify(ctx, encoded, "same password")
		require.NoError(t, err)
		assert.True(t, ok)
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	t.Parallel()
	h := newHasher(cheap(), 2)
	const (
		salt = "c29tZXNhbHRzb21lc2FsdA"                  // 16 bytes
		key  = "a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2V5a2U" // 29 bytes
	)
	valid := "$argon2id$v=19$m=64,t=1,p=1$" + salt + "$" + key
	_, _, _, err := decode(valid)
	require.NoError(t, err, "the base case must be well-formed")

	tests := []struct {
		name    string
		encoded string
	}{
		{name: "empty", encoded: ""},
		{name: "plain text", encoded: "s3cret-password"},
		{name: "truncated after variant", encoded: "$argon2id$"},
		{name: "truncated after version", encoded: "$argon2id$v=19$"},
		{name: "truncated after parameters", encoded: "$argon2id$v=19$m=64,t=1,p=1$" + salt},
		{name: "truncated key", encoded: valid[:len(valid)-20]},
		{name: "empty key", encoded: "$argon2id$v=19$m=64,t=1,p=1$" + salt + "$"},
		{name: "empty salt", encoded: "$argon2id$v=19$m=64,t=1,p=1$$" + key},
		{name: "short salt", encoded: "$argon2id$v=19$m=64,t=1,p=1$c2FsdA$" + key},
		{name: "extra segment", encoded: valid + "$extra"},
		{name: "missing leading dollar", encoded: valid[1:]},
		{name: "argon2i", encoded: "$argon2i$v=19$m=64,t=1,p=1$" + salt + "$" + key},
		{name: "argon2d", encoded: "$argon2d$v=19$m=64,t=1,p=1$" + salt + "$" + key},
		{name: "bcrypt", encoded: "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"},
		{name: "old version", encoded: "$argon2id$v=16$m=64,t=1,p=1$" + salt + "$" + key},
		{name: "missing version", encoded: "$argon2id$m=64,t=1,p=1$" + salt + "$" + key},
		{name: "parameters out of order", encoded: "$argon2id$v=19$t=1,m=64,p=1$" + salt + "$" + key},
		{name: "missing parameter", encoded: "$argon2id$v=19$m=64,t=1$" + salt + "$" + key},
		{name: "extra parameter", encoded: "$argon2id$v=19$m=64,t=1,p=1,k=1$" + salt + "$" + key},
		{name: "non-numeric memory", encoded: "$argon2id$v=19$m=x,t=1,p=1$" + salt + "$" + key},
		{name: "negative time", encoded: "$argon2id$v=19$m=64,t=-1,p=1$" + salt + "$" + key},
		{name: "signed memory", encoded: "$argon2id$v=19$m=+64,t=1,p=1$" + salt + "$" + key},
		{name: "leading zero", encoded: "$argon2id$v=19$m=064,t=1,p=1$" + salt + "$" + key},
		{name: "zero time", encoded: "$argon2id$v=19$m=64,t=0,p=1$" + salt + "$" + key},
		{name: "zero threads", encoded: "$argon2id$v=19$m=64,t=1,p=0$" + salt + "$" + key},
		{name: "threads over 255", encoded: "$argon2id$v=19$m=4096,t=1,p=256$" + salt + "$" + key},
		{name: "memory below 8 per thread", encoded: "$argon2id$v=19$m=15,t=1,p=2$" + salt + "$" + key},
		{name: "memory over limit", encoded: "$argon2id$v=19$m=4294967295,t=1,p=1$" + salt + "$" + key},
		{name: "time over limit", encoded: "$argon2id$v=19$m=64,t=4294967295,p=1$" + salt + "$" + key},
		{name: "memory just over limit", encoded: "$argon2id$v=19$m=262145,t=1,p=1$" + salt + "$" + key},
		{name: "time just over limit", encoded: "$argon2id$v=19$m=64,t=17,p=1$" + salt + "$" + key},
		{name: "memory overflows uint32", encoded: "$argon2id$v=19$m=4294967296,t=1,p=1$" + salt + "$" + key},
		{name: "invalid base64 salt", encoded: "$argon2id$v=19$m=64,t=1,p=1$!!!!!!!!!!!!!!!!!!!!!!$" + key},
		{name: "padded base64 key", encoded: "$argon2id$v=19$m=64,t=1,p=1$" + salt + "$" + key + "="},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var ok bool
			var err error
			require.NotPanics(t, func() { ok, err = h.Verify(context.Background(), tt.encoded, "s3cret-password") })
			require.ErrorIs(t, err, ErrInvalidHash)
			assert.False(t, ok)
			assert.True(t, h.NeedsRehash(tt.encoded))
		})
	}
}

func TestNeedsRehash(t *testing.T) {
	t.Parallel()
	h := New()
	ctx := context.Background()
	current, err := h.Hash(ctx, "s3cret-password")
	require.NoError(t, err)

	other := func(p params) string {
		encoded, err := newHasher(p, 1).Hash(ctx, "s3cret-password")
		require.NoError(t, err)
		return encoded
	}
	const salt16, key32 = "AAAAAAAAAAAAAAAAAAAAAA", "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	tests := []struct {
		name    string
		encoded string
		want    bool
	}{
		{name: "current parameters", encoded: current},
		{name: "current parameters, made elsewhere", encoded: "$argon2id$v=19$m=19456,t=2,p=1$" + salt16 + "$" + key32},
		{name: "less memory", encoded: other(params{memory: 4096, time: 2, threads: 1}), want: true},
		{name: "fewer passes", encoded: other(params{memory: 19456, time: 1, threads: 1}), want: true},
		{name: "more threads", encoded: other(params{memory: 19456, time: 2, threads: 2}), want: true},
		{name: "shorter salt", encoded: "$argon2id$v=19$m=19456,t=2,p=1$AAAAAAAAAAA$" + key32, want: true},
		{name: "shorter key", encoded: "$argon2id$v=19$m=19456,t=2,p=1$" + salt16 + "$AAAAAAAAAAAAAAAAAAAAAA", want: true},
		{name: "malformed", encoded: "not a hash", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, h.NeedsRehash(tt.encoded))
		})
	}
}

func TestVerifyAcceptsOlderParameters(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	old, err := newHasher(params{memory: 4096, time: 1, threads: 2}, 1).Hash(ctx, "s3cret-password")
	require.NoError(t, err)

	ok, err := newHasher(cheap(), 1).Verify(ctx, old, "s3cret-password")
	require.NoError(t, err)
	assert.True(t, ok, "Verify uses the parameters the hash records")
}

func TestVerifyDummy(t *testing.T) {
	t.Parallel()
	h := newHasher(cheap(), 1)

	_, _, _, err := decode(h.dummy)
	require.NoError(t, err, "the dummy hash is well-formed, so VerifyDummy does a full derivation")
	assert.False(t, h.NeedsRehash(h.dummy), "the dummy hash has the current parameters")

	for _, pw := range []string{"", "s3cret-password"} {
		ok, err := h.Verify(context.Background(), h.dummy, pw)
		require.NoError(t, err)
		assert.False(t, ok)
		require.NoError(t, h.VerifyDummy(context.Background(), pw))
	}
}

// TestConcurrencyLimit holds every slot, so the callers that arrive must wait: cancelled waiters
// return context.Canceled without hashing, and the others go ahead once slots are released.
func TestConcurrencyLimit(t *testing.T) {
	t.Parallel()
	const slots, waiters, cancelled = 2, 6, 3
	h := newHasher(cheap(), slots)
	encoded, err := h.Hash(context.Background(), "s3cret-password")
	require.NoError(t, err)

	for range slots {
		h.slots <- struct{}{}
	}

	calls := []struct {
		name string
		call func(ctx context.Context) error
	}{
		{name: "Hash", call: func(ctx context.Context) error { _, err := h.Hash(ctx, "s3cret-password"); return err }},
		{name: "Verify", call: func(ctx context.Context) error { _, err := h.Verify(ctx, encoded, "s3cret-password"); return err }},
		{name: "VerifyDummy", call: func(ctx context.Context) error { return h.VerifyDummy(ctx, "s3cret-password") }},
	}

	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan error, waiters)
	var started sync.WaitGroup
	for i := range waiters {
		callCtx := context.Background()
		if i < cancelled {
			callCtx = ctx
		}
		started.Add(1)
		go func() {
			started.Done()
			results <- calls[i%len(calls)].call(callCtx)
		}()
	}
	started.Wait()

	select {
	case err := <-results:
		t.Fatalf("a caller finished while every slot was taken: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	cancel()
	for range cancelled {
		select {
		case err := <-results:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(5 * time.Second):
			t.Fatal("a cancelled waiter did not return")
		}
	}

	select {
	case err := <-results:
		t.Fatalf("an uncancelled caller finished while every slot was taken: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	for range slots {
		<-h.slots
	}
	for range waiters - cancelled {
		select {
		case err := <-results:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("a waiter did not get a slot after the slots were released")
		}
	}
	assert.Empty(t, h.slots, "every caller released its slot")
}

func TestCancelledContextDoesNotHash(t *testing.T) {
	t.Parallel()
	h := newHasher(cheap(), 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := h.Hash(ctx, "s3cret-password")
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, h.VerifyDummy(ctx, "s3cret-password"), context.Canceled)
}

// BenchmarkHash measures one hash with the production parameters (m=19 MiB, t=2, p=1). On an
// Intel Core i5-10300H it takes about 25 ms and allocates about 19 MiB per hash; sign-in latency and
// the GOMAXPROCS limit follow from it. Run with `go test -bench . -benchmem ./pkg/password`.
func BenchmarkHash(b *testing.B) {
	h := New()
	ctx := context.Background()
	for b.Loop() {
		if _, err := h.Hash(ctx, "correct horse battery staple"); err != nil {
			b.Fatal(err)
		}
	}
}
