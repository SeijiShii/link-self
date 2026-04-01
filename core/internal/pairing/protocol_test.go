package pairing

import (
	"testing"
	"time"

	"github.com/SeijiShii/link-self/core/internal/did"
)

func TestPairing_FullFlow(t *testing.T) {
	// Existing device creates a user identity and a pairing session.
	userID, err := did.GenerateUserIdentity()
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewPairingSession(userID, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	// New device presents the token secret to complete pairing.
	receivedKey, err := session.Complete(session.Token.Secret)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	// Verify the received key produces the same user DID.
	imported, err := did.ImportUserIdentity(receivedKey)
	if err != nil {
		t.Fatalf("ImportUserIdentity: %v", err)
	}
	if imported.DID != userID.DID {
		t.Errorf("DID mismatch: got %q, want %q", imported.DID, userID.DID)
	}
}

func TestPairing_ExpiredToken(t *testing.T) {
	userID, _ := did.GenerateUserIdentity()
	session, _ := NewPairingSession(userID, 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	_, err := session.Complete(session.Token.Secret)
	if err == nil {
		t.Fatal("should fail with expired token")
	}
}

func TestPairing_WrongToken(t *testing.T) {
	userID, _ := did.GenerateUserIdentity()
	session, _ := NewPairingSession(userID, 5*time.Minute)

	_, err := session.Complete("wrong-secret")
	if err == nil {
		t.Fatal("should fail with wrong secret")
	}
}
