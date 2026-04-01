package pairing

import (
	"time"

	"github.com/SeijiShii/link-self/core/internal/did"
)

// PairingSession manages a single pairing attempt on the existing device side.
// The existing device creates a session, shares the token (e.g. via QR code),
// and the new device calls Complete with the secret to receive the user key.
type PairingSession struct {
	Token      *Token
	userKey    []byte // exported user private key
}

// NewPairingSession creates a pairing session for transferring the user key.
// ttl is the time window during which the new device can complete pairing.
func NewPairingSession(userID *did.UserIdentity, ttl time.Duration) (*PairingSession, error) {
	tok, err := CreateToken(ttl)
	if err != nil {
		return nil, err
	}
	keyData, err := userID.ExportPrivKey()
	if err != nil {
		return nil, err
	}
	return &PairingSession{
		Token:   tok,
		userKey: keyData,
	}, nil
}

// Complete validates the secret and returns the exported user private key.
// The new device should call did.ImportUserIdentity(key) to reconstruct the identity.
func (s *PairingSession) Complete(secret string) (userPrivKey []byte, err error) {
	if err := ValidateToken(secret, s.Token); err != nil {
		return nil, err
	}
	return s.userKey, nil
}
