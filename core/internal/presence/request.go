package presence

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/SeijiShii/link-self/core/internal/did"
)

// Authorized presence requests. A device proves it belongs to user U by signing
// the request with the USER key (which every paired device holds). The node
// verifies against the user DID before registering or answering a query — this
// stops outsiders from registering fake devices under a user or enumerating a
// user's devices. A timestamp within an acceptance window guards against replay.
//
// Canonical bytes use the same compact-JSON approach as roster.go so a TS
// client (which signs) and this Go node (which verifies) can interoperate.

// RegisterRequest publishes a device's reachable addresses for its user.
type RegisterRequest struct {
	UserDID   string   `json:"userDID"`
	DeviceDID string   `json:"deviceDID"`
	Addrs     []string `json:"addrs"`
	TS        int64    `json:"ts"` // unix milliseconds
	Sig       []byte   `json:"sig"`
}

// QueryRequest asks for the user's currently-online devices.
type QueryRequest struct {
	UserDID string `json:"userDID"`
	TS      int64  `json:"ts"` // unix milliseconds
	Sig     []byte `json:"sig"`
}

func sortedCopy(a []string) []string {
	c := append([]string(nil), a...)
	sort.Strings(c)
	return c
}

func registerCanonical(userDID, deviceDID string, addrs []string, ts int64) ([]byte, error) {
	return json.Marshal(struct {
		V         int      `json:"v"`
		Op        string   `json:"op"`
		UserDID   string   `json:"userDID"`
		DeviceDID string   `json:"deviceDID"`
		Addrs     []string `json:"addrs"`
		TS        int64    `json:"ts"`
	}{1, "register", userDID, deviceDID, sortedCopy(addrs), ts})
}

func queryCanonical(userDID string, ts int64) ([]byte, error) {
	return json.Marshal(struct {
		V       int    `json:"v"`
		Op      string `json:"op"`
		UserDID string `json:"userDID"`
		TS      int64  `json:"ts"`
	}{1, "query", userDID, ts})
}

// SignRegister builds a register request signed by the user key.
func SignRegister(user *did.Identity, deviceDID string, addrs []string, ts int64) (*RegisterRequest, error) {
	cb, err := registerCanonical(user.DID, deviceDID, addrs, ts)
	if err != nil {
		return nil, err
	}
	sig, err := user.PrivKey.Sign(cb)
	if err != nil {
		return nil, err
	}
	return &RegisterRequest{
		UserDID:   user.DID,
		DeviceDID: deviceDID,
		Addrs:     addrs,
		TS:        ts,
		Sig:       sig,
	}, nil
}

// SignQuery builds a query request signed by the user key.
func SignQuery(user *did.Identity, ts int64) (*QueryRequest, error) {
	cb, err := queryCanonical(user.DID, ts)
	if err != nil {
		return nil, err
	}
	sig, err := user.PrivKey.Sign(cb)
	if err != nil {
		return nil, err
	}
	return &QueryRequest{UserDID: user.DID, TS: ts, Sig: sig}, nil
}

func withinWindow(ts int64, now time.Time, window time.Duration) bool {
	delta := now.Sub(time.UnixMilli(ts))
	if delta < 0 {
		delta = -delta
	}
	return delta <= window
}

// VerifyRegister checks the signature (against the claimed user DID) and that
// the timestamp is within the acceptance window of now.
func VerifyRegister(req *RegisterRequest, now time.Time, window time.Duration) bool {
	if !withinWindow(req.TS, now, window) {
		return false
	}
	cb, err := registerCanonical(req.UserDID, req.DeviceDID, req.Addrs, req.TS)
	if err != nil {
		return false
	}
	return verifySig(req.UserDID, cb, req.Sig)
}

// VerifyQuery checks the signature and timestamp window for a query request.
func VerifyQuery(req *QueryRequest, now time.Time, window time.Duration) bool {
	if !withinWindow(req.TS, now, window) {
		return false
	}
	cb, err := queryCanonical(req.UserDID, req.TS)
	if err != nil {
		return false
	}
	return verifySig(req.UserDID, cb, req.Sig)
}

func verifySig(userDID string, msg, sig []byte) bool {
	pub, err := did.ParseToPubKey(userDID)
	if err != nil {
		return false
	}
	ok, err := pub.Verify(msg, sig)
	return err == nil && ok
}
