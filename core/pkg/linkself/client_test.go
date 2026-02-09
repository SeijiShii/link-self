package linkself

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
}

func TestClientStartStop(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use temp directory for identity
	tempDir := t.TempDir()
	identityPath := filepath.Join(tempDir, "identity.json")

	client := NewClient()

	// Test Start
	config := Config{
		IdentityPath: identityPath,
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}

	info, err := client.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if info == nil {
		t.Fatal("Start returned nil NodeInfo")
	}

	if info.DID == "" {
		t.Error("DID is empty")
	}

	t.Logf("Started node with DID: %s", info.DID)

	// Test GetMyDID
	did := client.GetMyDID()
	if did == "" {
		t.Error("GetMyDID returned empty string")
	}
	if did != info.DID {
		t.Errorf("GetMyDID returned %q, expected %q", did, info.DID)
	}

	// Test Stop
	if err := client.Stop(ctx); err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Verify identity file was created
	if _, err := os.Stat(identityPath); os.IsNotExist(err) {
		t.Error("Identity file was not created")
	}
}

func TestClientStartWithExistingIdentity(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	identityPath := filepath.Join(tempDir, "identity.json")

	// First start - creates identity
	client1 := NewClient()
	config1 := Config{
		IdentityPath: identityPath,
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}

	info1, err := client1.Start(ctx, config1)
	if err != nil {
		t.Fatalf("First Start failed: %v", err)
	}
	did1 := info1.DID
	client1.Stop(ctx)

	// Second start - should load existing identity
	client2 := NewClient()
	config2 := Config{
		IdentityPath: identityPath,
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}

	info2, err := client2.Start(ctx, config2)
	if err != nil {
		t.Fatalf("Second Start failed: %v", err)
	}
	defer client2.Stop(ctx)

	if info2.DID != did1 {
		t.Errorf("DID mismatch: got %q, expected %q", info2.DID, did1)
	}
}

func TestClientSendMessageWithoutStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := NewClient()

	err := client.SendMessage(ctx, "did:example:test", "test message")
	if err == nil {
		t.Error("Expected error when sending message without starting")
	}
}

func TestClientConnectWithoutStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := NewClient()

	err := client.Connect(ctx, "did:example:test")
	if err == nil {
		t.Error("Expected error when connecting without starting")
	}
}

func TestClientSetOnMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	identityPath := filepath.Join(tempDir, "identity.json")

	client := NewClient()

	// Set message handler before start (should not panic)
	messageReceived := false
	client.SetOnMessage(func(peerDID string, payload []byte) {
		messageReceived = true
	})

	config := Config{
		IdentityPath: identityPath,
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}

	_, err := client.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer client.Stop(ctx)

	// Set message handler after start
	client.SetOnMessage(func(peerDID string, payload []byte) {
		messageReceived = true
	})

	// Handler should be set (we can't easily test if it's actually called without a real peer)
	if !messageReceived {
		// This is expected - we're just testing that SetOnMessage doesn't panic
		t.Log("SetOnMessage works correctly")
	}
}

func TestClientDefaultIdentityPath(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := NewClient()

	// Test with empty IdentityPath (should use default)
	config := Config{
		ListenAddrs: []string{"/ip4/127.0.0.1/tcp/0"},
	}

	info, err := client.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start with default identity path failed: %v", err)
	}
	defer client.Stop(ctx)

	if info.DID == "" {
		t.Error("DID is empty")
	}
}

func TestClientDefaultListenAddr(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	identityPath := filepath.Join(tempDir, "identity.json")

	client := NewClient()

	// Test with empty ListenAddrs (should use default)
	config := Config{
		IdentityPath: identityPath,
	}

	info, err := client.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start with default listen addr failed: %v", err)
	}
	defer client.Stop(ctx)

	if info.DID == "" {
		t.Error("DID is empty")
	}
	if info.ListenAddr == "" {
		t.Error("ListenAddr is empty")
	}
}
