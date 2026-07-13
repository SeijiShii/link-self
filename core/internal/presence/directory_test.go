package presence

import (
	"sort"
	"testing"
	"time"
)

func deviceDIDs(locs []Location) []string {
	out := make([]string, len(locs))
	for i, l := range locs {
		out[i] = l.DeviceDID
	}
	sort.Strings(out)
	return out
}

func TestRegisterAndQuery(t *testing.T) {
	d := NewDirectory()
	t0 := time.Unix(1000, 0)
	ttl := time.Minute

	d.Register("user-1", "dev-a", []string{"/ip4/1.1.1.1/tcp/1/ws"}, t0, ttl)
	d.Register("user-1", "dev-b", []string{"/ip4/2.2.2.2/tcp/2/ws"}, t0, ttl)
	d.Register("user-2", "dev-c", []string{"/ip4/3.3.3.3/tcp/3/ws"}, t0, ttl)

	got := d.Query("user-1", t0.Add(time.Second))
	if want := []string{"dev-a", "dev-b"}; !equal(deviceDIDs(got), want) {
		t.Errorf("Query(user-1) = %v, want %v", deviceDIDs(got), want)
	}
	if u2 := d.Query("user-2", t0.Add(time.Second)); len(u2) != 1 || u2[0].DeviceDID != "dev-c" {
		t.Errorf("Query(user-2) = %v", u2)
	}
	if u3 := d.Query("user-3", t0); u3 != nil {
		t.Errorf("Query(unknown) = %v, want nil", u3)
	}
}

func TestQueryReturnsAddrs(t *testing.T) {
	d := NewDirectory()
	t0 := time.Unix(1000, 0)
	d.Register("u", "dev", []string{"/a", "/b"}, t0, time.Minute)
	got := d.Query("u", t0)
	if len(got) != 1 || len(got[0].Addrs) != 2 || got[0].Addrs[0] != "/a" {
		t.Errorf("addrs = %v", got)
	}
}

func TestReRegisterReplaces(t *testing.T) {
	d := NewDirectory()
	t0 := time.Unix(1000, 0)
	d.Register("u", "dev", []string{"/old"}, t0, time.Minute)
	d.Register("u", "dev", []string{"/new"}, t0, time.Minute)
	got := d.Query("u", t0)
	if len(got) != 1 || got[0].Addrs[0] != "/new" {
		t.Errorf("re-register did not replace: %v", got)
	}
}

func TestExpiry(t *testing.T) {
	d := NewDirectory()
	t0 := time.Unix(1000, 0)
	d.Register("u", "dev", []string{"/a"}, t0, time.Minute)

	if got := d.Query("u", t0.Add(30*time.Second)); len(got) != 1 {
		t.Errorf("should still be live: %v", got)
	}
	// At/after expiry the device is gone.
	if got := d.Query("u", t0.Add(time.Minute)); got != nil {
		t.Errorf("expired entry returned: %v", got)
	}
}

func TestUnregister(t *testing.T) {
	d := NewDirectory()
	t0 := time.Unix(1000, 0)
	d.Register("u", "dev-a", []string{"/a"}, t0, time.Minute)
	d.Register("u", "dev-b", []string{"/b"}, t0, time.Minute)
	d.Unregister("u", "dev-a")
	got := d.Query("u", t0)
	if len(got) != 1 || got[0].DeviceDID != "dev-b" {
		t.Errorf("after unregister = %v", got)
	}
}

func TestPrune(t *testing.T) {
	d := NewDirectory()
	t0 := time.Unix(1000, 0)
	d.Register("u", "short", []string{"/a"}, t0, time.Minute)
	d.Register("u", "long", []string{"/b"}, t0, time.Hour)
	d.Prune(t0.Add(2 * time.Minute)) // drops "short"
	got := d.Query("u", t0.Add(2*time.Minute))
	if len(got) != 1 || got[0].DeviceDID != "long" {
		t.Errorf("after prune = %v", got)
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
