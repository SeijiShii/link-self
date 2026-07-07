package linkself

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

	err := client.Connect(ctx, "did:example:test", "")
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

func TestClientDangerouslyDeleteAllData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	identityPath := filepath.Join(tempDir, "identity.json")

	client := NewClient()
	config := Config{
		IdentityPath: identityPath,
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}

	info, err := client.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Verify identity file exists
	if _, err := os.Stat(identityPath); os.IsNotExist(err) {
		t.Fatal("Identity file was not created")
	}

	did := info.DID
	if did == "" {
		t.Fatal("DID is empty")
	}

	// Delete all data
	if err := client.DangerouslyDeleteAllData(ctx); err != nil {
		t.Fatalf("DangerouslyDeleteAllData failed: %v", err)
	}

	// Verify identity file is deleted
	if _, err := os.Stat(identityPath); !os.IsNotExist(err) {
		t.Error("Identity file was not deleted")
	}

	// Verify node is stopped (GetMyDID returns empty)
	if did := client.GetMyDID(); did != "" {
		t.Errorf("GetMyDID returned %q after delete, expected empty", did)
	}

	// Verify can start again with new identity
	client2 := NewClient()
	info2, err := client2.Start(ctx, Config{
		IdentityPath: identityPath,
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	})
	if err != nil {
		t.Fatalf("Restart after delete failed: %v", err)
	}
	defer client2.Stop(ctx)

	if info2.DID == did {
		t.Error("New DID should differ from deleted one")
	}
}

func TestClientDangerouslyDeleteAllDataWithoutStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := NewClient()

	// Should not error even without starting
	if err := client.DangerouslyDeleteAllData(ctx); err != nil {
		t.Errorf("DangerouslyDeleteAllData without start should not error: %v", err)
	}
}

func TestGenerateTestDID(t *testing.T) {
	t.Run("returns did:test: prefix", func(t *testing.T) {
		did, err := GenerateTestDID()
		if err != nil {
			t.Fatalf("GenerateTestDID failed: %v", err)
		}
		if !strings.HasPrefix(did, "did:test:") {
			t.Errorf("expected did:test: prefix, got %q", did)
		}
	})

	t.Run("each call returns different DID", func(t *testing.T) {
		did1, err := GenerateTestDID()
		if err != nil {
			t.Fatalf("first call failed: %v", err)
		}
		did2, err := GenerateTestDID()
		if err != nil {
			t.Fatalf("second call failed: %v", err)
		}
		if did1 == did2 {
			t.Errorf("two calls returned the same DID: %q", did1)
		}
	})

	t.Run("is clearly distinguishable from real DID", func(t *testing.T) {
		did, err := GenerateTestDID()
		if err != nil {
			t.Fatalf("GenerateTestDID failed: %v", err)
		}
		if strings.HasPrefix(did, "did:key:") {
			t.Error("test DID must NOT have did:key: prefix")
		}
		if !IsTestDID(did) {
			t.Error("IsTestDID should return true for generated test DID")
		}
	})

	t.Run("real DID is not test DID", func(t *testing.T) {
		if IsTestDID("did:key:z6MkpTHR8VNs5fA2") {
			t.Error("IsTestDID should return false for real DID")
		}
		if IsTestDID("") {
			t.Error("IsTestDID should return false for empty string")
		}
	})
}

func TestInjectTestMessage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tempDir := t.TempDir()
	identityPath := filepath.Join(tempDir, "identity.json")

	client := NewClient()
	config := Config{
		IdentityPath: identityPath,
		ListenAddrs:  []string{"/ip4/127.0.0.1/tcp/0"},
	}

	_, err := client.Start(ctx, config)
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer client.Stop(ctx)

	t.Run("handler receives injected message", func(t *testing.T) {
		received := make(chan struct{}, 1)
		var gotDID string
		var gotPayload []byte

		client.SetOnMessage(func(peerDID string, payload []byte) {
			gotDID = peerDID
			gotPayload = payload
			received <- struct{}{}
		})

		testDID, _ := GenerateTestDID()
		err := client.InjectTestMessage(ctx, testDID, []byte("hello from test"))
		if err != nil {
			t.Fatalf("InjectTestMessage failed: %v", err)
		}

		select {
		case <-received:
		case <-time.After(2 * time.Second):
			t.Fatal("handler was not called")
		}

		if gotDID != testDID {
			t.Errorf("peerDID = %q, want %q", gotDID, testDID)
		}
		if string(gotPayload) != "hello from test" {
			t.Errorf("payload = %q, want %q", gotPayload, "hello from test")
		}
	})

	t.Run("rejects non-test DID", func(t *testing.T) {
		err := client.InjectTestMessage(ctx, "did:key:z6MkpTHR8VNs5fA2", []byte("bad"))
		if err == nil {
			t.Error("expected error when injecting with real DID")
		}
	})

	t.Run("no panic when handler is nil", func(t *testing.T) {
		client.SetOnMessage(nil)
		testDID, _ := GenerateTestDID()
		err := client.InjectTestMessage(ctx, testDID, []byte("no handler"))
		if err != nil {
			t.Errorf("expected no error with nil handler, got: %v", err)
		}
	})
}

func TestInjectTestMessageWithoutStart(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := NewClient()
	testDID, _ := GenerateTestDID()
	err := client.InjectTestMessage(ctx, testDID, []byte("test"))
	if err == nil {
		t.Error("expected error when node not started")
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

func TestClientStartWithRelayConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tempDir := t.TempDir()

	// Relay-serving node (always-on node role).
	relayClient := NewClient()
	relayInfo, err := relayClient.Start(ctx, Config{
		IdentityPath:       filepath.Join(tempDir, "relay.json"),
		ListenAddrs:        []string{"/ip4/127.0.0.1/tcp/0/ws"},
		EnableRelayService: true,
		ForceReachability:  "public",
	})
	if err != nil {
		t.Fatalf("Start relay node: %v", err)
	}
	defer relayClient.Stop(ctx)

	// Leaf node using the relay to stay reachable.
	leafClient := NewClient()
	_, err = leafClient.Start(ctx, Config{
		IdentityPath:      filepath.Join(tempDir, "leaf.json"),
		ListenAddrs:       []string{"/ip4/127.0.0.1/tcp/0"},
		CircuitRelays:     []string{relayInfo.ListenAddr},
		ForceReachability: "private",
	})
	if err != nil {
		t.Fatalf("Start leaf node with CircuitRelays: %v", err)
	}
	defer leafClient.Stop(ctx)
}

func TestClientStartRejectsBadRelayConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tempDir := t.TempDir()

	client := NewClient()
	_, err := client.Start(ctx, Config{
		IdentityPath:  filepath.Join(tempDir, "id.json"),
		CircuitRelays: []string{"not-a-multiaddr"},
	})
	if err == nil {
		client.Stop(ctx)
		t.Fatal("Start accepted an invalid CircuitRelays entry")
	}

	client2 := NewClient()
	_, err = client2.Start(ctx, Config{
		IdentityPath:      filepath.Join(tempDir, "id2.json"),
		ForceReachability: "sometimes",
	})
	if err == nil {
		client2.Stop(ctx)
		t.Fatal("Start accepted an invalid ForceReachability value")
	}
}
