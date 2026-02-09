// Package dht wraps libp2p Kademlia DHT to provide DID-based Provide/Find for LinkSelf.
// DHT key design: namespace "/linkself/did/" + base32(sha256(did)). Value: serialized peer.AddrInfo (JSON or binary).
package dht

import (
	"context"
	"encoding/base32"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SeijiShii/link-self/core/internal/did"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	record "github.com/libp2p/go-libp2p-record"
)

const (
	// Namespace for LinkSelf DID records in the DHT.
	Namespace = "linkself"
	// KeyPrefix is the full key prefix for DID records: /linkself/did/
	KeyPrefix = "/" + Namespace + "/did/"
)

// linkselfValidator validates and selects values for keys under /linkself/did/.
// Value must be valid JSON-marshalled peer.AddrInfo.
type linkselfValidator struct{}

func (linkselfValidator) Validate(key string, value []byte) error {
	if !strings.HasPrefix(key, KeyPrefix) {
		return fmt.Errorf("invalid key prefix for linkself: %q", key)
	}
	var info peer.AddrInfo
	if err := json.Unmarshal(value, &info); err != nil {
		return fmt.Errorf("invalid AddrInfo value: %w", err)
	}
	if err := info.ID.Validate(); err != nil {
		return err
	}
	return nil
}

func (linkselfValidator) Select(key string, values [][]byte) (int, error) {
	if len(values) == 0 {
		return -1, fmt.Errorf("no values to select")
	}
	// Prefer first valid value (caller may pass [new, old]; we prefer index 0).
	for i, v := range values {
		var info peer.AddrInfo
		if json.Unmarshal(v, &info) == nil && info.ID.Validate() == nil {
			return i, nil
		}
	}
	return 0, nil
}

// LinkselfValidator returns a record.Validator that includes only the linkself namespace.
// Use with dht.Option: dht.Validator(LinkselfValidator()) for a DHT that only stores linkself records.
func LinkselfValidator() record.Validator {
	return record.NamespacedValidator{
		"linkself": LinkselfValidatorNamespace(),
	}
}

// LinkselfValidatorNamespace returns the validator for the "linkself" namespace only.
// Use with dht.NamespacedValidator("linkself", LinkselfValidatorNamespace()) to add to default validators.
func LinkselfValidatorNamespace() record.Validator {
	return linkselfValidator{}
}

// DIDKey returns the DHT record key for the given DID (used with PutValue/GetValue).
func DIDKey(didStr string) string {
	k := did.DIDToPeerIDKey(didStr)
	enc := base32.StdEncoding.WithPadding(base32.NoPadding)
	return KeyPrefix + enc.EncodeToString(k)
}


// PutDID stores the peer AddrInfo for the given DID in the DHT.
func PutDID(ctx context.Context, r routing.ValueStore, didStr string, info peer.AddrInfo) error {
	if didStr == "" {
		return fmt.Errorf("DID must not be empty")
	}
	if err := info.ID.Validate(); err != nil {
		return fmt.Errorf("invalid AddrInfo: %w", err)
	}
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	key := DIDKey(didStr)
	return r.PutValue(ctx, key, data)
}

// FindDID looks up the peer AddrInfo for the given DID in the DHT.
func FindDID(ctx context.Context, r routing.ValueStore, didStr string) (peer.AddrInfo, error) {
	if didStr == "" {
		return peer.AddrInfo{}, fmt.Errorf("DID must not be empty")
	}
	key := DIDKey(didStr)
	val, err := r.GetValue(ctx, key)
	if err != nil {
		return peer.AddrInfo{}, err
	}
	var info peer.AddrInfo
	if err := json.Unmarshal(val, &info); err != nil {
		return peer.AddrInfo{}, fmt.Errorf("decode AddrInfo: %w", err)
	}
	return info, nil
}
