package node

import (
	"sync"
	"testing"

	"github.com/SeijiShii/link-self/core/internal/envelope"
)

func TestRouter_DeviceSync(t *testing.T) {
	var mu sync.Mutex
	var gotDID string
	var gotPayload []byte

	router := &MessageRouter{
		OnDeviceSync: func(peerDID string, payload []byte) {
			mu.Lock()
			defer mu.Unlock()
			gotDID = peerDID
			gotPayload = payload
		},
	}

	inner := []byte(`{"table":"contacts"}`)
	wrapped, _ := envelope.Wrap(envelope.TypeDeviceSync, inner)
	router.dispatch("did:key:alice", wrapped)

	mu.Lock()
	defer mu.Unlock()
	if gotDID != "did:key:alice" {
		t.Errorf("peerDID = %q, want did:key:alice", gotDID)
	}
	if string(gotPayload) != string(inner) {
		t.Errorf("payload = %s, want %s", gotPayload, inner)
	}
}

func TestRouter_GroupShare(t *testing.T) {
	var gotPayload []byte

	router := &MessageRouter{
		OnGroupShare: func(peerDID string, payload []byte) {
			gotPayload = payload
		},
	}

	inner := []byte(`{"channel":"notes"}`)
	wrapped, _ := envelope.Wrap(envelope.TypeGroupShare, inner)
	router.dispatch("did:key:bob", wrapped)

	if string(gotPayload) != string(inner) {
		t.Errorf("payload = %s, want %s", gotPayload, inner)
	}
}

func TestRouter_PlainMessage(t *testing.T) {
	var gotPayload []byte

	router := &MessageRouter{
		OnMessage: func(peerDID string, payload []byte) {
			gotPayload = payload
		},
	}

	inner := []byte(`hello`)
	wrapped, _ := envelope.Wrap(envelope.TypeMessage, inner)
	router.dispatch("did:key:carol", wrapped)

	if string(gotPayload) != string(inner) {
		t.Errorf("payload = %s, want %s", gotPayload, inner)
	}
}

func TestRouter_LegacyNonEnvelope(t *testing.T) {
	var gotPayload []byte

	router := &MessageRouter{
		OnMessage: func(peerDID string, payload []byte) {
			gotPayload = payload
		},
	}

	raw := []byte("plain text, not an envelope")
	router.dispatch("did:key:dave", raw)

	if string(gotPayload) != string(raw) {
		t.Errorf("payload = %s, want original %s", gotPayload, raw)
	}
}

func TestRouter_NilHandler_NoPanic(t *testing.T) {
	router := &MessageRouter{} // all handlers nil

	wrapped, _ := envelope.Wrap(envelope.TypeDeviceSync, []byte("data"))
	router.dispatch("did:key:eve", wrapped) // should not panic

	router.dispatch("did:key:frank", []byte("legacy")) // should not panic
}

func TestRouter_OnlyDeviceSyncSet_GroupShareIgnored(t *testing.T) {
	called := false
	router := &MessageRouter{
		OnDeviceSync: func(peerDID string, payload []byte) {
			called = true
		},
		// OnGroupShare is nil
	}

	wrapped, _ := envelope.Wrap(envelope.TypeGroupShare, []byte("data"))
	router.dispatch("did:key:grace", wrapped) // GroupShare with no handler

	if called {
		t.Error("OnDeviceSync should not be called for GroupShare messages")
	}
}
