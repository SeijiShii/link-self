package dataroot

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDefaultRoot_ReturnsNonEmpty(t *testing.T) {
	root, err := DefaultRoot()
	if err != nil {
		t.Fatalf("DefaultRoot: %v", err)
	}
	if root == "" {
		t.Fatal("DefaultRoot returned empty string")
	}
	t.Logf("DefaultRoot = %s", root)
}

func TestDefaultRoot_EnvOverride(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom-linkself")
	t.Setenv("LINKSELF_DATA_DIR", custom)

	root, err := DefaultRoot()
	if err != nil {
		t.Fatalf("DefaultRoot: %v", err)
	}
	if root != custom {
		t.Errorf("DefaultRoot = %q, want %q", root, custom)
	}
}

func TestDefaultRoot_PlatformPath(t *testing.T) {
	// Clear env override
	t.Setenv("LINKSELF_DATA_DIR", "")

	root, err := DefaultRoot()
	if err != nil {
		t.Fatalf("DefaultRoot: %v", err)
	}

	switch runtime.GOOS {
	case "linux":
		if !strings.Contains(root, "link-self") {
			t.Errorf("Linux root should contain 'link-self': %s", root)
		}
	case "darwin":
		if !strings.Contains(root, "LinkSelf") {
			t.Errorf("macOS root should contain 'LinkSelf': %s", root)
		}
	case "windows":
		if !strings.Contains(root, "LinkSelf") {
			t.Errorf("Windows root should contain 'LinkSelf': %s", root)
		}
	}
}

func TestEncodeDID(t *testing.T) {
	did := "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"
	encoded := EncodeDID(did)

	if encoded == "" {
		t.Fatal("EncodeDID returned empty string")
	}
	// Must be filesystem-safe: no colons, no slashes
	if strings.ContainsAny(encoded, ":/\\") {
		t.Errorf("EncodeDID contains unsafe chars: %q", encoded)
	}

	// Same input should give same output
	if EncodeDID(did) != encoded {
		t.Error("EncodeDID is not deterministic")
	}
}

func TestDIDDir(t *testing.T) {
	root := "/data/linkself"
	did := "did:key:z6MkhaXgBZDvotDkL5257faiztiGiC2QtKLGpbnnEGta2doK"
	dir := DIDDir(root, did)

	if !strings.HasPrefix(dir, root) {
		t.Errorf("DIDDir should start with root: %s", dir)
	}
}

func TestSuiteDir(t *testing.T) {
	root := "/data/linkself"
	did := "did:key:z6Mk..."
	dir := SuiteDir(root, did, "jp.example.my-suite")

	if !strings.Contains(dir, "suites") {
		t.Errorf("SuiteDir should contain 'suites': %s", dir)
	}
	if !strings.Contains(dir, "jp.example.my-suite") {
		t.Errorf("SuiteDir should contain suite ID: %s", dir)
	}
}

func TestNetworkDir(t *testing.T) {
	root := "/data/linkself"
	did := "did:key:z6Mk..."
	dir := NetworkDir(root, did, "my-suite", "instance-123")

	if !strings.Contains(dir, "networks") {
		t.Errorf("NetworkDir should contain 'networks': %s", dir)
	}
	if !strings.Contains(dir, "instance-123") {
		t.Errorf("NetworkDir should contain instance ID: %s", dir)
	}
}

func TestListDIDs_Empty(t *testing.T) {
	root := t.TempDir()
	dids, err := ListDIDs(root)
	if err != nil {
		t.Fatalf("ListDIDs: %v", err)
	}
	if len(dids) != 0 {
		t.Errorf("ListDIDs on empty dir = %v, want empty", dids)
	}
}

func TestListDIDs_WithDIDs(t *testing.T) {
	root := t.TempDir()

	// Create two DID directories
	did1 := "did:key:z6MkAAAA"
	did2 := "did:key:z6MkBBBB"
	os.MkdirAll(DIDDir(root, did1), 0700)
	os.MkdirAll(DIDDir(root, did2), 0700)

	dids, err := ListDIDs(root)
	if err != nil {
		t.Fatalf("ListDIDs: %v", err)
	}
	if len(dids) != 2 {
		t.Errorf("ListDIDs = %d entries, want 2", len(dids))
	}
}
