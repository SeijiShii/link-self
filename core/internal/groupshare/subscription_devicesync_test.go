package groupshare_test

import (
	"testing"

	"github.com/SeijiShii/link-self/core/internal/devicesync"
	"github.com/SeijiShii/link-self/core/internal/groupshare"
)

func TestDeviceSyncSubscriptionStore_SetAndGet(t *testing.T) {
	engine := &devicesync.ReplicationEngine{
		Storage: devicesync.NewMemStorage(),
		SelfDID: "did:key:zAlice",
	}

	store := groupshare.NewDeviceSyncSubscriptionStore(engine)

	// Initially nil
	topics, err := store.GetSubscription("did:key:zAlice", "docs")
	if err != nil {
		t.Fatal(err)
	}
	if topics != nil {
		t.Fatalf("expected nil, got %v", topics)
	}

	// Set subscription
	if err := store.SetSubscription("did:key:zAlice", "docs", []string{"001", "002"}); err != nil {
		t.Fatal(err)
	}

	topics, err = store.GetSubscription("did:key:zAlice", "docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(topics) != 2 || topics[0] != "001" || topics[1] != "002" {
		t.Fatalf("got %v, want [001 002]", topics)
	}
}

func TestDeviceSyncSubscriptionStore_Overwrite(t *testing.T) {
	engine := &devicesync.ReplicationEngine{
		Storage: devicesync.NewMemStorage(),
		SelfDID: "did:key:zAlice",
	}

	store := groupshare.NewDeviceSyncSubscriptionStore(engine)

	store.SetSubscription("did:key:zAlice", "docs", []string{"001"})
	store.SetSubscription("did:key:zAlice", "docs", []string{"002", "003"})

	topics, _ := store.GetSubscription("did:key:zAlice", "docs")
	if len(topics) != 2 || topics[0] != "002" {
		t.Fatalf("overwrite failed, got %v", topics)
	}
}

func TestDeviceSyncSubscriptionStore_Unsubscribe(t *testing.T) {
	engine := &devicesync.ReplicationEngine{
		Storage: devicesync.NewMemStorage(),
		SelfDID: "did:key:zAlice",
	}

	store := groupshare.NewDeviceSyncSubscriptionStore(engine)

	store.SetSubscription("did:key:zAlice", "docs", []string{"001"})
	store.SetSubscription("did:key:zAlice", "docs", []string{})

	topics, _ := store.GetSubscription("did:key:zAlice", "docs")
	if topics == nil {
		t.Fatal("empty slice should be stored, not nil")
	}
	if len(topics) != 0 {
		t.Fatalf("expected empty, got %v", topics)
	}
}

func TestDeviceSyncSubscriptionStore_GetAllSubscriptions(t *testing.T) {
	engine := &devicesync.ReplicationEngine{
		Storage: devicesync.NewMemStorage(),
		SelfDID: "did:key:zAlice",
	}

	store := groupshare.NewDeviceSyncSubscriptionStore(engine)

	store.SetSubscription("did:key:zAlice", "docs", []string{"001"})
	store.SetSubscription("did:key:zAlice", "chat", []string{"*"})

	all, err := store.GetAllSubscriptions("did:key:zAlice")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 channels, got %d", len(all))
	}
	if len(all["docs"]) != 1 || all["docs"][0] != "001" {
		t.Fatalf("docs = %v", all["docs"])
	}
	if len(all["chat"]) != 1 || all["chat"][0] != "*" {
		t.Fatalf("chat = %v", all["chat"])
	}
}

func TestDeviceSyncSubscriptionStore_MultiDID(t *testing.T) {
	engine := &devicesync.ReplicationEngine{
		Storage: devicesync.NewMemStorage(),
		SelfDID: "did:key:zAlice",
	}

	store := groupshare.NewDeviceSyncSubscriptionStore(engine)

	store.SetSubscription("did:key:zAlice", "docs", []string{"001"})
	store.SetSubscription("did:key:zBob", "docs", []string{"002"})

	alice, _ := store.GetSubscription("did:key:zAlice", "docs")
	bob, _ := store.GetSubscription("did:key:zBob", "docs")

	if len(alice) != 1 || alice[0] != "001" {
		t.Fatalf("Alice = %v", alice)
	}
	if len(bob) != 1 || bob[0] != "002" {
		t.Fatalf("Bob = %v", bob)
	}
}

func TestDeviceSyncSubscriptionStore_ReplicationSync(t *testing.T) {
	// Two engines sharing the same storage simulate DeviceSync replication.
	// When Device A sets a subscription, Device B should see it after replication.
	storage := devicesync.NewMemStorage()
	engineA := &devicesync.ReplicationEngine{
		Storage: storage,
		SelfDID: "did:key:zAlice",
	}

	storeA := groupshare.NewDeviceSyncSubscriptionStore(engineA)
	storeB := groupshare.NewDeviceSyncSubscriptionStore(engineA) // same engine = same storage

	storeA.SetSubscription("did:key:zAlice", "docs", []string{"001", "002"})

	// storeB reads from the same storage — should see the subscription.
	topics, _ := storeB.GetSubscription("did:key:zAlice", "docs")
	if len(topics) != 2 {
		t.Fatalf("replication failed, storeB got %v", topics)
	}
}
