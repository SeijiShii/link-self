package pairing

import (
	"testing"
	"time"
)

func TestCreateToken_HasExpiry(t *testing.T) {
	tok, err := CreateToken(5 * time.Minute)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	if tok.Secret == "" {
		t.Fatal("token secret should not be empty")
	}
	if tok.ExpiresAt.IsZero() {
		t.Fatal("token should have an expiry time")
	}
	if tok.ExpiresAt.Before(time.Now()) {
		t.Fatal("token should not already be expired")
	}
}

func TestValidateToken_Valid(t *testing.T) {
	tok, _ := CreateToken(5 * time.Minute)
	err := ValidateToken(tok.Secret, tok)
	if err != nil {
		t.Fatalf("ValidateToken should succeed for valid token: %v", err)
	}
}

func TestValidateToken_Expired(t *testing.T) {
	tok, _ := CreateToken(1 * time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	err := ValidateToken(tok.Secret, tok)
	if err == nil {
		t.Fatal("ValidateToken should fail for expired token")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	tok, _ := CreateToken(5 * time.Minute)
	err := ValidateToken("wrong-secret", tok)
	if err == nil {
		t.Fatal("ValidateToken should fail for wrong secret")
	}
}
