// Package dataroot provides platform-specific data directory resolution
// for LinkSelf's storage hierarchy:
//
//	<root>/<encoded-DID>/suites/<suite-id>/networks/<instance-id>/
package dataroot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultRoot returns the LinkSelf data root directory.
// If LINKSELF_DATA_DIR is set (and non-empty), that value is returned.
// Otherwise, a platform-specific default is used:
//   - Linux: $XDG_DATA_HOME/link-self (default ~/.local/share/link-self)
//   - macOS: ~/Library/Application Support/LinkSelf
//   - Windows: %LOCALAPPDATA%\LinkSelf
func DefaultRoot() (string, error) {
	if env := os.Getenv("LINKSELF_DATA_DIR"); env != "" {
		return env, nil
	}

	switch runtime.GOOS {
	case "linux":
		xdg := os.Getenv("XDG_DATA_HOME")
		if xdg == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("get home dir: %w", err)
			}
			xdg = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(xdg, "link-self"), nil

	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home dir: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "LinkSelf"), nil

	case "windows":
		local := os.Getenv("LOCALAPPDATA")
		if local == "" {
			return "", fmt.Errorf("LOCALAPPDATA not set")
		}
		return filepath.Join(local, "LinkSelf"), nil

	default:
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home dir: %w", err)
		}
		return filepath.Join(home, ".linkself"), nil
	}
}

// EncodeDID converts a DID string to a filesystem-safe directory name.
// Uses the first 16 bytes of SHA-256 in hex (32 chars) for brevity.
func EncodeDID(did string) string {
	h := sha256.Sum256([]byte(did))
	return hex.EncodeToString(h[:16])
}

// DIDDir returns the directory for a specific DID under root.
func DIDDir(root, did string) string {
	return filepath.Join(root, EncodeDID(did))
}

// SuiteDir returns the directory for a suite under a DID space.
func SuiteDir(root, did, suiteID string) string {
	return filepath.Join(DIDDir(root, did), "suites", suiteID)
}

// NetworkDir returns the directory for a network instance.
func NetworkDir(root, did, suiteID, instanceID string) string {
	return filepath.Join(SuiteDir(root, did, suiteID), "networks", instanceID)
}

// ListDIDs returns DID strings for all DID directories under root.
// It looks for directories that were created by EncodeDID (hex-encoded hash).
// Returns the encoded directory names (not the original DIDs, since the hash is one-way).
// Returns an empty slice if root doesn't exist or is empty.
func ListDIDs(root string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var dids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// EncodeDID produces 32-char hex strings
		if len(name) == 32 && isHex(name) {
			dids = append(dids, name)
		}
	}
	return dids, nil
}

func isHex(s string) bool {
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return false
		}
	}
	return true
}
