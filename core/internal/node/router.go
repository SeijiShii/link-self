package node

import (
	"github.com/SeijiShii/link-self/core/internal/envelope"
)

// MessageRouter dispatches incoming messages by envelope type.
type MessageRouter struct {
	OnDeviceSync  func(peerDID string, payload []byte)
	OnGroupShare  func(peerDID string, payload []byte)
	OnSubAnnounce func(peerDID string, payload []byte)
	OnMessage     func(peerDID string, payload []byte) // plain / legacy
}

// dispatch routes a raw incoming message to the appropriate handler.
func (r *MessageRouter) dispatch(peerDID string, data []byte) {
	typ, payload, _ := envelope.Unwrap(data)
	switch typ {
	case envelope.TypeDeviceSync:
		if r.OnDeviceSync != nil {
			r.OnDeviceSync(peerDID, payload)
		}
	case envelope.TypeGroupShare:
		if r.OnGroupShare != nil {
			r.OnGroupShare(peerDID, payload)
		}
	case envelope.TypeSubAnnounce:
		if r.OnSubAnnounce != nil {
			r.OnSubAnnounce(peerDID, payload)
		}
	default:
		if r.OnMessage != nil {
			r.OnMessage(peerDID, payload)
		}
	}
}
