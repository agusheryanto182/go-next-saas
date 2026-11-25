// Package password hashes and verifies passwords with argon2id (ADR 0011).
//
// Hashes are PHC strings, $argon2id$v=19$m=19456,t=2,p=1$<salt>$<key>, with salt and key in
// base64 without padding, so stored hashes record their own parameters and stay verifiable when the
// parameters change. A Hasher runs at most GOMAXPROCS derivations at once: each one holds about
// 19 MiB, and without a limit a burst of sign-in attempts could exhaust a small server's memory.
package password

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

// ErrInvalidHash is returned for an encoded hash that is not a well-formed argon2id PHC string.
var ErrInvalidHash = errors.New("password: invalid argon2id hash")

const (
	_saltLen = 16
	_keyLen  = 32

	// Parsing limits. They are far above anything this package writes and only stop a corrupt
	// stored hash from making a verification allocate or loop without bound.
	_minSaltLen = 8
	_minKeyLen  = 16
	_maxMemory  = 1 << 18 // KiB, 256 MiB
	_maxTime    = 16
)

// params are the argon2id cost parameters. memory is in KiB.
type params struct {
	memory  uint32
	time    uint32
	threads uint8
}

// current are OWASP's minimum recommended parameters for argon2id (ADR 0011).
func current() params { return params{memory: 19456, time: 2, threads: 1} }

// Hasher hashes and verifies passwords. Its methods are safe for concurrent use.
type Hasher struct {
	params params
	slots  chan struct{}
	// dummy is a well-formed hash with the current parameters that no password matches; VerifyDummy
	// verifies against it.
	dummy string
}

// New returns a Hasher with the current parameters that runs at most GOMAXPROCS derivations at once.
func New() *Hasher {
	return newHasher(current(), runtime.GOMAXPROCS(0))
}

func newHasher(p params, slots int) *Hasher {
	return &Hasher{
		params: p,
		slots:  make(chan struct{}, slots),
		dummy:  encode(p, make([]byte, _saltLen), make([]byte, _keyLen)),
	}
}

// Hash returns the PHC-encoded argon2id hash of password with a fresh random salt. It waits for a
// free slot and returns ctx.Err() if ctx ends first.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	salt := make([]byte, _saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("password - Hash - rand.Read: %w", err)
	}
	key, err := h.derive(ctx, h.params, salt, password, _keyLen)
	if err != nil {
		return "", err
	}
	return encode(h.params, salt, key), nil
}

// Verify reports whether password matches encoded, using the parameters encoded records. A malformed
// encoded returns ErrInvalidHash; a mismatch returns false and no error. It waits for a free slot and
// returns ctx.Err() if ctx ends first.
func (h *Hasher) Verify(ctx context.Context, encoded, password string) (bool, error) {
	p, salt, want, err := decode(encoded)
	if err != nil {
		return false, err
	}
	got, err := h.derive(ctx, p, salt, password, uint32(len(want))) //nolint:gosec // decode bounds len(want) by the encoded string, far below MaxUint32
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// VerifyDummy does the work of verifying password against a hash with the current parameters and
// discards the result. Callers run it for unknown emails and accounts that cannot sign in, so the
// response time does not reveal whether an account exists. It only fails when ctx ends while waiting.
func (h *Hasher) VerifyDummy(ctx context.Context, password string) error {
	_, err := h.Verify(ctx, h.dummy, password)
	return err
}

// NeedsRehash reports whether encoded was made with other parameters, salt or key length than Hash
// uses now, so a successful sign-in should store a new hash. A malformed encoded needs one too.
func (h *Hasher) NeedsRehash(encoded string) bool {
	p, salt, key, err := decode(encoded)
	if err != nil {
		return true
	}
	return p != h.params || len(salt) != _saltLen || len(key) != _keyLen
}

// derive runs argon2id once a slot is free. The slot bounds how many derivations, and so how much
// memory, are in use at once.
func (h *Hasher) derive(ctx context.Context, p params, salt []byte, password string, keyLen uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case h.slots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-h.slots }()
	return argon2.IDKey([]byte(password), salt, p.time, p.memory, p.threads, keyLen), nil
}

func encode(p params, salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.time, p.threads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// decode parses a PHC argon2id string strictly: the version and parameters must appear exactly as
// encode writes them, within the parsing limits.
func decode(encoded string) (params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return params{}, nil, nil, fmt.Errorf("%w: not an argon2id PHC string", ErrInvalidHash)
	}
	if parts[2] != "v="+strconv.Itoa(argon2.Version) {
		return params{}, nil, nil, fmt.Errorf("%w: unsupported version", ErrInvalidHash)
	}
	p, err := decodeParams(parts[3])
	if err != nil {
		return params{}, nil, nil, err
	}
	salt, err := base64.RawStdEncoding.Strict().DecodeString(parts[4])
	if err != nil || len(salt) < _minSaltLen {
		return params{}, nil, nil, fmt.Errorf("%w: salt", ErrInvalidHash)
	}
	key, err := base64.RawStdEncoding.Strict().DecodeString(parts[5])
	if err != nil || len(key) < _minKeyLen {
		return params{}, nil, nil, fmt.Errorf("%w: key", ErrInvalidHash)
	}
	return p, salt, key, nil
}

func decodeParams(s string) (params, error) {
	fields := strings.Split(s, ",")
	if len(fields) != 3 {
		return params{}, fmt.Errorf("%w: parameters", ErrInvalidHash)
	}
	memory, errM := parseField(fields[0], "m=", _maxMemory)
	time, errT := parseField(fields[1], "t=", _maxTime)
	threads, errP := parseField(fields[2], "p=", 255)
	if errM != nil || errT != nil || errP != nil || memory < 8*threads {
		return params{}, fmt.Errorf("%w: parameters", ErrInvalidHash)
	}
	return params{memory: memory, time: time, threads: uint8(threads)}, nil //nolint:gosec // parseField bounds threads to 255
}

// parseField parses "<prefix><n>" with 1 <= n <= limit, in canonical decimal form.
func parseField(field, prefix string, limit uint32) (uint32, error) {
	digits, ok := strings.CutPrefix(field, prefix)
	if !ok {
		return 0, ErrInvalidHash
	}
	n, err := strconv.ParseUint(digits, 10, 32)
	if err != nil || n < 1 || n > uint64(limit) || strconv.FormatUint(n, 10) != digits {
		return 0, ErrInvalidHash
	}
	return uint32(n), nil
}
