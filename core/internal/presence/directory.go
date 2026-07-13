// Package presence implements the device-presence directory that an always-on
// node (relay / bootstrap) exposes so that a user's devices can find each
// other's current reachable address — the discovery piece of two-layer
// multi-device sync (browser leaves cannot accept inbound connections, so they
// publish their Circuit-Relay address here and look up their siblings).
//
// A device REGISTERs "user U's device D is reachable at addrs A" (with a TTL,
// refreshed while online) and QUERYs "which devices of user U are online".
// Registration/lookup authorization (a device must prove it belongs to user U,
// via a user-key signature) is layered on top by the protocol handler; this
// file is the in-memory directory data structure only.
package presence

import (
	"sync"
	"time"
)

// Location is a device's current reachable address set.
type Location struct {
	DeviceDID string
	Addrs     []string
}

type entry struct {
	addrs     []string
	expiresAt time.Time
}

// Directory is a concurrency-safe, TTL-expiring map of userDID → devices.
type Directory struct {
	mu     sync.Mutex
	byUser map[string]map[string]entry // userDID -> deviceDID -> entry
}

// NewDirectory creates an empty directory.
func NewDirectory() *Directory {
	return &Directory{byUser: make(map[string]map[string]entry)}
}

// Register records (or refreshes) a device's reachable address set for a user,
// expiring at now+ttl. Re-registering the same device replaces its entry.
func (d *Directory) Register(userDID, deviceDID string, addrs []string, now time.Time, ttl time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()
	devices := d.byUser[userDID]
	if devices == nil {
		devices = make(map[string]entry)
		d.byUser[userDID] = devices
	}
	cp := append([]string(nil), addrs...)
	devices[deviceDID] = entry{addrs: cp, expiresAt: now.Add(ttl)}
}

// Query returns the user's currently-reachable devices (entries not expired at
// now). Expired entries are ignored (and lazily dropped).
func (d *Directory) Query(userDID string, now time.Time) []Location {
	d.mu.Lock()
	defer d.mu.Unlock()
	devices := d.byUser[userDID]
	if devices == nil {
		return nil
	}
	out := make([]Location, 0, len(devices))
	for deviceDID, e := range devices {
		if now.Before(e.expiresAt) {
			out = append(out, Location{
				DeviceDID: deviceDID,
				Addrs:     append([]string(nil), e.addrs...),
			})
		} else {
			delete(devices, deviceDID)
		}
	}
	if len(devices) == 0 {
		delete(d.byUser, userDID)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Unregister removes a device (e.g. on graceful shutdown).
func (d *Directory) Unregister(userDID, deviceDID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if devices := d.byUser[userDID]; devices != nil {
		delete(devices, deviceDID)
		if len(devices) == 0 {
			delete(d.byUser, userDID)
		}
	}
}

// Prune drops all entries that have expired at now.
func (d *Directory) Prune(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for userDID, devices := range d.byUser {
		for deviceDID, e := range devices {
			if !now.Before(e.expiresAt) {
				delete(devices, deviceDID)
			}
		}
		if len(devices) == 0 {
			delete(d.byUser, userDID)
		}
	}
}
