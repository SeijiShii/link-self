package presence

import (
	"testing"
	"time"

	"github.com/SeijiShii/link-self/core/internal/did"
)

const window = 2 * time.Minute

func TestRegisterSignVerify(t *testing.T) {
	user, _ := did.Generate()
	dev, _ := did.Generate()
	now := time.Unix(1_700_000_000, 0)
	req, err := SignRegister(user, dev.DID, []string{"/ip4/1.1.1.1/tcp/1/ws"}, now.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyRegister(req, now, window) {
		t.Error("VerifyRegister = false, want true")
	}
}

func TestQuerySignVerify(t *testing.T) {
	user, _ := did.Generate()
	now := time.Unix(1_700_000_000, 0)
	req, _ := SignQuery(user, now.UnixMilli())
	if !VerifyQuery(req, now, window) {
		t.Error("VerifyQuery = false, want true")
	}
}

func TestRejectWrongUserDID(t *testing.T) {
	user, _ := did.Generate()
	impostor, _ := did.Generate()
	dev, _ := did.Generate()
	now := time.Unix(1_700_000_000, 0)
	req, _ := SignRegister(user, dev.DID, []string{"/a"}, now.UnixMilli())
	req.UserDID = impostor.DID // claim a different user, keep signature
	if VerifyRegister(req, now, window) {
		t.Error("VerifyRegister accepted an impostor userDID")
	}
}

func TestRejectTamperedAddrs(t *testing.T) {
	user, _ := did.Generate()
	dev, _ := did.Generate()
	now := time.Unix(1_700_000_000, 0)
	req, _ := SignRegister(user, dev.DID, []string{"/a"}, now.UnixMilli())
	req.Addrs = []string{"/evil"} // tamper after signing
	if VerifyRegister(req, now, window) {
		t.Error("VerifyRegister accepted tampered addrs")
	}
}

func TestRejectReplayOutsideWindow(t *testing.T) {
	user, _ := did.Generate()
	dev, _ := did.Generate()
	signAt := time.Unix(1_700_000_000, 0)
	req, _ := SignRegister(user, dev.DID, []string{"/a"}, signAt.UnixMilli())
	// 5 minutes later, outside the 2-minute window.
	if VerifyRegister(req, signAt.Add(5*time.Minute), window) {
		t.Error("VerifyRegister accepted a stale (replayed) request")
	}
}

func TestAddrOrderIndependentSignature(t *testing.T) {
	user, _ := did.Generate()
	dev, _ := did.Generate()
	now := time.Unix(1_700_000_000, 0)
	r1, _ := SignRegister(user, dev.DID, []string{"/a", "/b"}, now.UnixMilli())
	r2, _ := SignRegister(user, dev.DID, []string{"/b", "/a"}, now.UnixMilli())
	if string(r1.Sig) != string(r2.Sig) {
		t.Error("signature depends on addr order")
	}
}
