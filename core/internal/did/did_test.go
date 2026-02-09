package did

import (
	"crypto/rand"
	"strings"
	"testing"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

func TestGenerate(t *testing.T) {
	id, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	if id.DID == "" {
		t.Fatal("DID should not be empty")
	}
	if !strings.HasPrefix(id.DID, Prefix) {
		t.Errorf("DID should start with %q, got %q", Prefix, id.DID)
	}
	// Same identity: DID should be reproducible from PubKey
	did2, err := PubKeyToDID(id.PubKey)
	if err != nil {
		t.Fatal(err)
	}
	if did2 != id.DID {
		t.Errorf("PubKeyToDID mismatch: %q vs %q", did2, id.DID)
	}
}

func TestParse(t *testing.T) {
	id, _ := Generate()
	raw, err := Parse(id.DID)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != 32 {
		t.Errorf("expected 32 raw bytes, got %d", len(raw))
	}
	pub, err := crypto.UnmarshalEd25519PublicKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	didBack, err := PubKeyToDID(pub)
	if err != nil {
		t.Fatal(err)
	}
	if didBack != id.DID {
		t.Errorf("roundtrip DID mismatch: %q vs %q", didBack, id.DID)
	}
}

func TestParseToPubKey(t *testing.T) {
	id, _ := Generate()
	pub, err := ParseToPubKey(id.DID)
	if err != nil {
		t.Fatal(err)
	}
	if !pub.Equals(id.PubKey) {
		t.Error("ParseToPubKey returned different key")
	}
}

func TestParseInvalid(t *testing.T) {
	_, err := Parse("not-a-did")
	if err != ErrInvalidDID {
		t.Errorf("expected ErrInvalidDID, got %v", err)
	}
	_, err = Parse("did:key:")
	if err != ErrInvalidDID {
		t.Errorf("expected ErrInvalidDID for empty, got %v", err)
	}
}

func TestDIDToPeerIDKey(t *testing.T) {
	k1 := DIDToPeerIDKey("did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK")
	k2 := DIDToPeerIDKey("did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK")
	if len(k1) != 32 {
		t.Errorf("key length should be 32, got %d", len(k1))
	}
	for i := range k1 {
		if k1[i] != k2[i] {
			t.Error("same DID must produce same key")
			break
		}
	}
}

func TestSamePrivateKeySameDID(t *testing.T) {
	priv, pub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id1, err := FromPrivKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	didFromPub, err := PubKeyToDID(pub)
	if err != nil {
		t.Fatal(err)
	}
	if id1.DID != didFromPub {
		t.Errorf("DID from Identity vs PubKey: %q vs %q", id1.DID, didFromPub)
	}
}

func TestDIDToPeerID(t *testing.T) {
	id, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	pid, err := DIDToPeerID(id.DID)
	if err != nil {
		t.Fatal(err)
	}
	expected, err := peer.IDFromPublicKey(id.PubKey)
	if err != nil {
		t.Fatal(err)
	}
	if pid != expected {
		t.Errorf("DIDToPeerID %q: got %s, want %s", id.DID, pid, expected)
	}
}

func TestDIDToPeerID_Invalid(t *testing.T) {
	_, err := DIDToPeerID("not-a-did")
	if err == nil {
		t.Fatal("expected error for invalid DID")
	}
	_, err = DIDToPeerID("")
	if err == nil {
		t.Fatal("expected error for empty DID")
	}
}
