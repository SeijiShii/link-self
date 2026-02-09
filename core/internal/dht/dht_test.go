package dht

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
)

func TestDIDKey(t *testing.T) {
	k1 := DIDKey("did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK")
	k2 := DIDKey("did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK")
	if !strings.HasPrefix(k1, KeyPrefix) {
		t.Errorf("DIDKey should start with %q, got %q", KeyPrefix, k1)
	}
	if k1 != k2 {
		t.Error("same DID must produce same key")
	}
	// Different DIDs produce different keys
	k3 := DIDKey("did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doX")
	if k1 == k3 {
		t.Error("different DIDs should produce different keys")
	}
}

func TestLinkselfValidatorNamespace_Validate(t *testing.T) {
	v := LinkselfValidatorNamespace()
	// Valid: key under KeyPrefix, value is valid peer.AddrInfo JSON
	id, _ := did.Generate()
	pid, _ := did.DIDToPeerID(id.DID)
	info := peer.AddrInfo{ID: pid, Addrs: nil}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	key := DIDKey(id.DID)
	if err := v.Validate(key, data); err != nil {
		t.Errorf("valid key/value should pass: %v", err)
	}
	// Invalid key prefix
	if err := v.Validate("/other/ns/key", data); err == nil {
		t.Error("invalid key prefix should fail")
	}
	// Invalid value (not valid AddrInfo JSON)
	if err := v.Validate(key, []byte("not json")); err == nil {
		t.Error("invalid value should fail")
	}
}

func TestLinkselfValidatorNamespace_Select(t *testing.T) {
	v := LinkselfValidatorNamespace()
	id, _ := did.Generate()
	pid, _ := did.DIDToPeerID(id.DID)
	info := peer.AddrInfo{ID: pid, Addrs: nil}
	data, _ := json.Marshal(info)
	key := DIDKey(id.DID)
	idx, err := v.Select(key, [][]byte{data})
	if err != nil {
		t.Fatal(err)
	}
	if idx != 0 {
		t.Errorf("Select: got index %d, want 0", idx)
	}
	// Empty values
	_, err = v.Select(key, nil)
	if err == nil {
		t.Error("Select with no values should fail")
	}
}

func TestPutDID_EmptyDID(t *testing.T) {
	store := &mockValueStore{values: make(map[string][]byte)}
	ctx := context.Background()
	id, _ := did.Generate()
	pid, _ := did.DIDToPeerID(id.DID)
	info := peer.AddrInfo{ID: pid}
	err := PutDID(ctx, store, "", info)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("PutDID with empty DID should fail: %v", err)
	}
}

func TestFindDID_EmptyDID(t *testing.T) {
	store := &mockValueStore{values: make(map[string][]byte)}
	ctx := context.Background()
	_, err := FindDID(ctx, store, "")
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("FindDID with empty DID should fail: %v", err)
	}
}

func TestPutDID_FindDID_Roundtrip(t *testing.T) {
	// Use in-memory mock ValueStore
	store := &mockValueStore{values: make(map[string][]byte)}
	ctx := context.Background()
	id, _ := did.Generate()
	pid, _ := did.DIDToPeerID(id.DID)
	info := peer.AddrInfo{ID: pid, Addrs: nil}
	if err := PutDID(ctx, store, id.DID, info); err != nil {
		t.Fatal(err)
	}
	got, err := FindDID(ctx, store, id.DID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != info.ID {
		t.Errorf("FindDID: got ID %s, want %s", got.ID, info.ID)
	}
}

type mockValueStore struct {
	values map[string][]byte
}

func (m *mockValueStore) PutValue(ctx context.Context, key string, value []byte, opts ...routing.Option) error {
	m.values[key] = value
	return nil
}

func (m *mockValueStore) GetValue(ctx context.Context, key string, opts ...routing.Option) ([]byte, error) {
	v, ok := m.values[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return v, nil
}

func (m *mockValueStore) SearchValue(ctx context.Context, key string, opts ...routing.Option) (<-chan []byte, error) {
	ch := make(chan []byte, 1)
	if v, ok := m.values[key]; ok {
		ch <- v
	}
	close(ch)
	return ch, nil
}
