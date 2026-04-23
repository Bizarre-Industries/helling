// Package auth provides password hashing, JWT signing, and session primitives
// for hellingd. The argon2id parameters here follow ADR-030 and
// docs/standards/security.md.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2idParams are the Helling argon2id parameters. ADR-030 pins these at
// time=3, memory=64 MiB, parallelism=4, 32-byte output, 16-byte salt.
type Argon2idParams struct {
	TimeCost    uint32
	MemoryKiB   uint32
	Parallelism uint8
	SaltLen     uint32
	KeyLen      uint32
}

// DefaultArgon2idParams is the production parameter set.
var DefaultArgon2idParams = Argon2idParams{
	TimeCost:    3,
	MemoryKiB:   64 * 1024,
	Parallelism: 4,
	SaltLen:     16,
	KeyLen:      32,
}

// ErrPasswordMismatch indicates the candidate password does not match the hash.
var ErrPasswordMismatch = errors.New("auth: password mismatch")

// ErrMalformedHash indicates a PHC-formatted argon2id hash could not be parsed.
var ErrMalformedHash = errors.New("auth: malformed argon2id hash")

// HashPassword returns a PHC-formatted argon2id hash of the password:
//
//	$argon2id$v=19$m=<mem>,t=<time>,p=<para>$<saltB64>$<hashB64>
func HashPassword(password string, p Argon2idParams) (string, error) {
	if password == "" {
		return "", errors.New("auth: empty password")
	}
	salt := make([]byte, p.SaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: read salt: %w", err)
	}
	hash := argon2.IDKey([]byte(password), salt, p.TimeCost, p.MemoryKiB, p.Parallelism, p.KeyLen)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		p.MemoryKiB, p.TimeCost, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword returns nil when the candidate matches the PHC hash.
// Returns ErrPasswordMismatch on mismatch, ErrMalformedHash on parse errors.
func VerifyPassword(password, phc string) error {
	params, salt, hash, err := parseArgon2idPHC(phc)
	if err != nil {
		return err
	}
	keyLen := clampUint32(len(hash))
	candidate := argon2.IDKey([]byte(password), salt, params.TimeCost, params.MemoryKiB, params.Parallelism, keyLen)
	if subtle.ConstantTimeCompare(candidate, hash) != 1 {
		return ErrPasswordMismatch
	}
	return nil
}

// clampUint32 narrows a non-negative int to uint32, clamping at max. Argon2
// digest lengths never exceed a few hundred bytes in practice.
func clampUint32(n int) uint32 {
	if n < 0 {
		return 0
	}
	const maxU32 = int(^uint32(0))
	if n > maxU32 {
		return ^uint32(0)
	}
	return uint32(n)
}

func parseArgon2idPHC(phc string) (params Argon2idParams, salt []byte, hash []byte, err error) {
	parts := strings.Split(phc, "$")
	// Expected layout: ["", "argon2id", "v=19", "m=...,t=...,p=...", "<salt>", "<hash>"]
	if len(parts) != 6 || parts[1] != "argon2id" {
		return Argon2idParams{}, nil, nil, ErrMalformedHash
	}
	var version int
	if _, scanErr := fmt.Sscanf(parts[2], "v=%d", &version); scanErr != nil || version != argon2.Version {
		return Argon2idParams{}, nil, nil, ErrMalformedHash
	}
	var memory, time uint32
	var parallel uint8
	if _, scanErr := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &parallel); scanErr != nil {
		return Argon2idParams{}, nil, nil, ErrMalformedHash
	}
	salt, decodeErr := base64.RawStdEncoding.DecodeString(parts[4])
	if decodeErr != nil {
		return Argon2idParams{}, nil, nil, ErrMalformedHash
	}
	hash, decodeErr = base64.RawStdEncoding.DecodeString(parts[5])
	if decodeErr != nil {
		return Argon2idParams{}, nil, nil, ErrMalformedHash
	}
	return Argon2idParams{
		TimeCost:    time,
		MemoryKiB:   memory,
		Parallelism: parallel,
		SaltLen:     clampUint32(len(salt)),
		KeyLen:      clampUint32(len(hash)),
	}, salt, hash, nil
}
