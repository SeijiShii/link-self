package envelope

import (
	"bytes"
	"testing"
)

func TestWrap_DeviceSync(t *testing.T) {
	payload := []byte(`{"table":"contacts","record_id":"alice"}`)
	data, err := Wrap(TypeDeviceSync, payload)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	typ, got, err := Unwrap(data)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if typ != TypeDeviceSync {
		t.Errorf("type = %q, want %q", typ, TypeDeviceSync)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload = %s, want %s", got, payload)
	}
}

func TestWrap_GroupShare(t *testing.T) {
	payload := []byte(`{"channel":"notes","id":"note1"}`)
	data, err := Wrap(TypeGroupShare, payload)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	typ, _, err := Unwrap(data)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if typ != TypeGroupShare {
		t.Errorf("type = %q, want %q", typ, TypeGroupShare)
	}
}

func TestWrap_Message(t *testing.T) {
	payload := []byte("hello world")
	data, err := Wrap(TypeMessage, payload)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	typ, got, err := Unwrap(data)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if typ != TypeMessage {
		t.Errorf("type = %q, want %q", typ, TypeMessage)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload = %s, want %s", got, payload)
	}
}

func TestUnwrap_InvalidJSON_FallbackToMessage(t *testing.T) {
	raw := []byte("not json at all")
	typ, payload, err := Unwrap(raw)
	if err != nil {
		t.Fatalf("Unwrap should not error on invalid JSON: %v", err)
	}
	if typ != TypeMessage {
		t.Errorf("type = %q, want %q (fallback)", typ, TypeMessage)
	}
	if !bytes.Equal(payload, raw) {
		t.Errorf("payload = %s, want original %s", payload, raw)
	}
}

func TestUnwrap_MissingTypeField_FallbackToMessage(t *testing.T) {
	raw := []byte(`{"data":"something"}`)
	typ, payload, err := Unwrap(raw)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if typ != TypeMessage {
		t.Errorf("type = %q, want %q (fallback for missing type)", typ, TypeMessage)
	}
	if !bytes.Equal(payload, raw) {
		t.Errorf("payload = %s, want original %s", payload, raw)
	}
}

func TestRoundTrip_PreservesPayload(t *testing.T) {
	original := []byte(`{"complex":{"nested":"data"},"array":[1,2,3]}`)
	wrapped, err := Wrap(TypeGroupShare, original)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}

	typ, unwrapped, err := Unwrap(wrapped)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if typ != TypeGroupShare {
		t.Errorf("type = %q, want %q", typ, TypeGroupShare)
	}
	if !bytes.Equal(unwrapped, original) {
		t.Errorf("payload = %s, want %s", unwrapped, original)
	}
}

func TestWrap_EmptyPayload(t *testing.T) {
	data, err := Wrap(TypeDeviceSync, []byte{})
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	typ, payload, err := Unwrap(data)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if typ != TypeDeviceSync {
		t.Errorf("type = %q, want %q", typ, TypeDeviceSync)
	}
	if len(payload) != 0 {
		t.Errorf("payload should be empty, got %s", payload)
	}
}

func TestWrap_NilPayload(t *testing.T) {
	data, err := Wrap(TypeMessage, nil)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	typ, _, err := Unwrap(data)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if typ != TypeMessage {
		t.Errorf("type = %q, want %q", typ, TypeMessage)
	}
}

func TestWrap_BinaryPayload(t *testing.T) {
	payload := []byte{0x00, 0xFF, 0x80, 0x01}
	data, err := Wrap(TypeDeviceSync, payload)
	if err != nil {
		t.Fatalf("Wrap: %v", err)
	}
	typ, got, err := Unwrap(data)
	if err != nil {
		t.Fatalf("Unwrap: %v", err)
	}
	if typ != TypeDeviceSync {
		t.Errorf("type = %q, want %q", typ, TypeDeviceSync)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("payload mismatch")
	}
}
