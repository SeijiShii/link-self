// Package pairing implements a simple device pairing protocol for LinkSelf.
// A time-limited token is used to securely transfer the user key from an
// existing device to a new device.
package pairing

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrTokenExpired = errors.New("pairing token expired")
	ErrTokenInvalid = errors.New("pairing token invalid")
)

// Token represents a time-limited pairing token.
type Token struct {
	Secret    string    // random secret the new device must present
	ExpiresAt time.Time // when this token becomes invalid
}

// CreateToken generates a new pairing token valid for the given duration.
func CreateToken(ttl time.Duration) (*Token, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, err
	}
	return &Token{
		Secret:    hex.EncodeToString(b),
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

// ValidateToken checks that the provided secret matches and the token has not expired.
func ValidateToken(secret string, tok *Token) error {
	if time.Now().After(tok.ExpiresAt) {
		return ErrTokenExpired
	}
	if secret != tok.Secret {
		return ErrTokenInvalid
	}
	return nil
}
