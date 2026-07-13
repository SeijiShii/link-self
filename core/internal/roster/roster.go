// Package roster implements the device roster: the abstraction that unifies a
// user's devices under the two-layer identity model (data-sync-decisions §4.2
// P3). Each device has its own device DID (its libp2p transport key, so
// peerID ≡ deviceDID) while the user DID is the account visible to the network.
// A roster lists the user's device DIDs and is signed by the USER key, so any
// holder of the user DID can verify which devices belong to that user.
//
// Wire-compatible with the TypeScript port (ts/linkself/src/roster.ts): the
// canonical bytes signed are the compact JSON {v,userDID,devices:[{deviceDID,
// label}]} with devices sorted by deviceDID, and Marshal produces
// {userDID,devices,sig} with sig base64-encoded.
package roster

import (
	"encoding/json"
	"sort"

	"github.com/SeijiShii/link-self/core/internal/did"
)

// DeviceEntry is one device in a user's roster.
type DeviceEntry struct {
	// DeviceDID is the device's own did:key (its libp2p transport identity).
	DeviceDID string `json:"deviceDID"`
	// Label is a human-readable name (e.g. "PC"); empty string if unset.
	Label string `json:"label"`
}

// SignedRoster is a user's device list, signed by the user key.
type SignedRoster struct {
	// UserDID is the account DID — the signer, visible to the network.
	UserDID string `json:"userDID"`
	// Devices are all device DIDs belonging to this user.
	Devices []DeviceEntry `json:"devices"`
	// Sig is the Ed25519 signature by the user key over the canonical encoding.
	// json.Marshal encodes it as base64 (matching the TS wire format).
	Sig []byte `json:"sig"`
}

// canonicalBytes is the deterministic encoding signed by the user key. Devices
// are sorted by deviceDID so the same roster always yields the same bytes.
func canonicalBytes(userDID string, devices []DeviceEntry) ([]byte, error) {
	sorted := append([]DeviceEntry(nil), devices...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].DeviceDID < sorted[j].DeviceDID
	})
	return json.Marshal(struct {
		V       int           `json:"v"`
		UserDID string        `json:"userDID"`
		Devices []DeviceEntry `json:"devices"`
	}{V: 1, UserDID: userDID, Devices: sorted})
}

// Build builds and signs a roster with the user identity. Devices are
// de-duplicated by deviceDID (last wins).
func Build(user *did.Identity, devices []DeviceEntry) (*SignedRoster, error) {
	deduped := dedupeByDeviceDID(devices)
	b, err := canonicalBytes(user.DID, deduped)
	if err != nil {
		return nil, err
	}
	sig, err := user.PrivKey.Sign(b)
	if err != nil {
		return nil, err
	}
	return &SignedRoster{UserDID: user.DID, Devices: deduped, Sig: sig}, nil
}

// Verify checks the roster signature against its own UserDID's public key.
func Verify(r *SignedRoster) bool {
	pub, err := did.ParseToPubKey(r.UserDID)
	if err != nil {
		return false
	}
	b, err := canonicalBytes(r.UserDID, r.Devices)
	if err != nil {
		return false
	}
	ok, err := pub.Verify(b, r.Sig)
	return err == nil && ok
}

// HasDevice reports whether a device DID is a member of the roster.
func HasDevice(r *SignedRoster, deviceDID string) bool {
	for _, d := range r.Devices {
		if d.DeviceDID == deviceDID {
			return true
		}
	}
	return false
}

// DeviceDIDs returns the device DIDs in the roster.
func DeviceDIDs(r *SignedRoster) []string {
	out := make([]string, len(r.Devices))
	for i, d := range r.Devices {
		out[i] = d.DeviceDID
	}
	return out
}

// WithDevice returns a new signed roster with entry added or its label updated.
func WithDevice(user *did.Identity, devices []DeviceEntry, entry DeviceEntry) (*SignedRoster, error) {
	next := make([]DeviceEntry, 0, len(devices)+1)
	for _, d := range devices {
		if d.DeviceDID != entry.DeviceDID {
			next = append(next, d)
		}
	}
	next = append(next, entry)
	return Build(user, next)
}

// WithoutDevice returns a new signed roster with the given device removed.
func WithoutDevice(user *did.Identity, devices []DeviceEntry, deviceDID string) (*SignedRoster, error) {
	next := make([]DeviceEntry, 0, len(devices))
	for _, d := range devices {
		if d.DeviceDID != deviceDID {
			next = append(next, d)
		}
	}
	return Build(user, next)
}

// Marshal serializes a roster to JSON (sig base64), for storage or transfer.
func Marshal(r *SignedRoster) ([]byte, error) {
	return json.Marshal(r)
}

// Unmarshal parses a roster from JSON produced by Marshal.
func Unmarshal(data []byte) (*SignedRoster, error) {
	var r SignedRoster
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func dedupeByDeviceDID(devices []DeviceEntry) []DeviceEntry {
	seen := make(map[string]int, len(devices)) // deviceDID -> index in out
	out := make([]DeviceEntry, 0, len(devices))
	for _, d := range devices {
		if i, ok := seen[d.DeviceDID]; ok {
			out[i] = d // last wins
			continue
		}
		seen[d.DeviceDID] = len(out)
		out = append(out, d)
	}
	return out
}
