// Package envelope provides a thin message type wrapper for multiplexing
// DeviceSync, GroupShare, and plain messages over the same transport.
package envelope

import "encoding/json"

// Type identifies the kind of payload inside an envelope.
type Type string

const (
	TypeDeviceSync  Type = "devicesync"
	TypeGroupShare  Type = "groupshare"
	TypeSubAnnounce Type = "sub_announce"
	TypeMessage     Type = "message"
)

// envelope is the JSON wire format. Payload is []byte so Go's json package
// automatically handles base64 encoding, allowing arbitrary binary payloads.
type envelope struct {
	Type    Type   `json:"type"`
	Payload []byte `json:"payload"`
}

// Wrap creates an envelope with the given type and payload, returning JSON bytes.
func Wrap(t Type, payload []byte) ([]byte, error) {
	return json.Marshal(envelope{Type: t, Payload: payload})
}

// Unwrap extracts the type and payload from an envelope.
// If the data is not a valid envelope, returns TypeMessage and the original data.
func Unwrap(data []byte) (Type, []byte, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return TypeMessage, data, nil
	}
	if env.Type == "" {
		return TypeMessage, data, nil
	}
	return env.Type, env.Payload, nil
}
