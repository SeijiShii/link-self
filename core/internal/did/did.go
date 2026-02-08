// Package did implements did:key generation, parsing, and verification for LinkSelf.
// It uses Ed25519 keys compatible with libp2p (same key type as libp2p default).
package did

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/multiformats/go-multibase"
)

const (
	// Prefix is the did:key method prefix.
	Prefix = "did:key:"
	// MulticodecEd25519Pub is the multicodec identifier for Ed25519 public key (did:key / multiformats).
	MulticodecEd25519Pub = 0xed
)

var (
	// ErrInvalidDID is returned when the DID string format is invalid.
	ErrInvalidDID = errors.New("invalid did:key format")
	// ErrUnsupportedKeyType is returned when the key type is not Ed25519.
	ErrUnsupportedKeyType = errors.New("only Ed25519 did:key is supported")
)

// Identity holds a key pair and the derived DID string.
// The private key never leaves the process; only the DID (from public key) is shared.
type Identity struct {
	PrivKey crypto.PrivKey
	PubKey  crypto.PubKey
	DID     string
}

// Generate creates a new Ed25519 identity and its did:key.
func Generate() (*Identity, error) {
	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}
	did, err := PubKeyToDID(pub)
	if err != nil {
		return nil, err
	}
	return &Identity{PrivKey: priv, PubKey: pub, DID: did}, nil
}

// FromPrivKey builds an Identity from an existing libp2p private key (must be Ed25519).
func FromPrivKey(priv crypto.PrivKey) (*Identity, error) {
	pub := priv.GetPublic()
	did, err := PubKeyToDID(pub)
	if err != nil {
		return nil, err
	}
	return &Identity{PrivKey: priv, PubKey: pub, DID: did}, nil
}

// PubKeyToDID encodes a libp2p Ed25519 public key as did:key (multibase base58btc + multicodec 0xed + raw 32 bytes).
func PubKeyToDID(pub crypto.PubKey) (string, error) {
	raw, err := pub.Raw()
	if err != nil {
		return "", err
	}
	// We only support Ed25519 (32-byte raw).
	if len(raw) != 32 {
		return "", ErrUnsupportedKeyType
	}
	payload := append([]byte{MulticodecEd25519Pub}, raw...)
	encoded, err := multibase.Encode(multibase.Base58BTC, payload)
	if err != nil {
		return "", fmt.Errorf("multibase encode: %w", err)
	}
	return Prefix + encoded, nil
}

// Parse parses a did:key string and returns the multibase payload (multicodec + raw public key bytes).
// Caller can use the raw bytes with crypto.UnmarshalEd25519PublicKey to get a libp2p PubKey.
func Parse(did string) (rawPublicKey []byte, err error) {
	if !strings.HasPrefix(did, Prefix) {
		return nil, ErrInvalidDID
	}
	mb := strings.TrimPrefix(did, Prefix)
	if mb == "" {
		return nil, ErrInvalidDID
	}
	_, data, err := multibase.Decode(mb)
	if err != nil {
		return nil, fmt.Errorf("multibase decode: %w", err)
	}
	if len(data) < 2 {
		return nil, ErrInvalidDID
	}
	if data[0] != MulticodecEd25519Pub {
		return nil, ErrUnsupportedKeyType
	}
	rawPublicKey = data[1:]
	if len(rawPublicKey) != 32 {
		return nil, ErrInvalidDID
	}
	return rawPublicKey, nil
}

// ParseToPubKey parses a did:key string and returns a libp2p PubKey (Ed25519).
func ParseToPubKey(did string) (crypto.PubKey, error) {
	raw, err := Parse(did)
	if err != nil {
		return nil, err
	}
	return crypto.UnmarshalEd25519PublicKey(raw)
}

// DIDToPeerIDKey returns a stable byte key for DHT (SHA256 of DID string).
// Used as the DHT key for Provide/Find so that DID lookup is deterministic.
func DIDToPeerIDKey(did string) []byte {
	h := sha256.Sum256([]byte(did))
	return h[:]
}
