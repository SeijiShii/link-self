package roster

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/libp2p/go-libp2p/core/crypto"
)

// Golden cross-implementation vector: the same fixed seeds must yield the same
// DIDs, canonical bytes, and (deterministic Ed25519) signature in Go and in the
// TypeScript port (ts/linkself/test/roster.golden.test.ts). This locks the
// wire format so a roster signed by one implementation verifies in the other.
const (
	goldenUserDID = "did:key:z2DZjrAhKXqCQ2djr26Syq33DKt1YP5cUoAhxkiaeVUpBtX"
	goldenD1DID   = "did:key:z2DZ7WFjPGgZ5iteMKgdgLWu4AaSSADEdGRm6oBxB5NfFXH"
	goldenD2DID   = "did:key:z2DgPLAbK8rAzua3qFkqfS1Dnzd8BxxadcsVtghmB6AsPD2"
	goldenCanon   = `{"v":1,"userDID":"did:key:z2DZjrAhKXqCQ2djr26Syq33DKt1YP5cUoAhxkiaeVUpBtX","devices":[{"deviceDID":"did:key:z2DZ7WFjPGgZ5iteMKgdgLWu4AaSSADEdGRm6oBxB5NfFXH","label":"PC"},{"deviceDID":"did:key:z2DgPLAbK8rAzua3qFkqfS1Dnzd8BxxadcsVtghmB6AsPD2","label":"Phone"}]}`
	goldenSigHex  = "3b69a9020a9396d37e3b86ba5784b3c12a8dbd83a6445d3a073a388dd1792facf5ec15e339cb637847b2c511294c3f0c63c1dc38687601fe1e8cac357c24ee04"
)

func goldenIdentity(t *testing.T, seedByte byte) *did.Identity {
	t.Helper()
	priv, _, err := crypto.GenerateEd25519Key(bytes.NewReader(bytes.Repeat([]byte{seedByte}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	id, err := did.FromPrivKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestGoldenVector(t *testing.T) {
	user := goldenIdentity(t, 0x01)
	d1 := goldenIdentity(t, 0x02)
	d2 := goldenIdentity(t, 0x03)

	if user.DID != goldenUserDID {
		t.Fatalf("user DID = %s, want %s", user.DID, goldenUserDID)
	}
	if d1.DID != goldenD1DID || d2.DID != goldenD2DID {
		t.Fatalf("device DIDs = %s / %s", d1.DID, d2.DID)
	}

	r, err := Build(user, []DeviceEntry{
		{DeviceDID: d1.DID, Label: "PC"},
		{DeviceDID: d2.DID, Label: "Phone"},
	})
	if err != nil {
		t.Fatal(err)
	}
	cb, err := canonicalBytes(r.UserDID, r.Devices)
	if err != nil {
		t.Fatal(err)
	}
	if string(cb) != goldenCanon {
		t.Errorf("canonical bytes mismatch:\n got %s\nwant %s", cb, goldenCanon)
	}
	if got := hex.EncodeToString(r.Sig); got != goldenSigHex {
		t.Errorf("signature mismatch:\n got %s\nwant %s", got, goldenSigHex)
	}
	if !Verify(r) {
		t.Error("Verify() = false for golden roster")
	}
}
