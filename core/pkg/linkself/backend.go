package linkself

import (
	"github.com/SeijiShii/link-self/core/internal/devicesync"
	"github.com/SeijiShii/link-self/core/internal/group"
	"github.com/SeijiShii/link-self/core/internal/groupshare"
	"github.com/SeijiShii/link-self/core/internal/storage/sqlite"
)

// StorageBackend is an opaque type that bundles all storage implementations
// used by a LinkSelf client. The unexported marker method prevents external
// packages from implementing this interface directly, keeping the storage
// internals hidden from application code.
type StorageBackend interface {
	// storages returns all storage implementations needed by client.Start.
	// The method is unexported so only this package can satisfy the interface.
	storages() backendStorages
	// close releases any resources held by the backend (e.g. SQLite connection).
	close() error
}

// backendStorages holds all five storage implementations wired into a client.
type backendStorages struct {
	deviceStorage     devicesync.DeviceStorage
	sharedStorage     groupshare.SharedStorage
	subscriptionStore groupshare.SubscriptionStore
	groupStore        group.Store
}

// --- Memory backend ---

// memoryBackend implements StorageBackend using in-memory stores.
type memoryBackend struct{}

// MemoryBackend returns a StorageBackend that stores all data in memory.
// Data is lost when the client stops. This is the default when Config.StorageBackend is nil.
func MemoryBackend() StorageBackend {
	return &memoryBackend{}
}

func (m *memoryBackend) storages() backendStorages {
	return backendStorages{
		deviceStorage:     devicesync.NewMemStorage(),
		sharedStorage:     groupshare.NewMemSharedStorage(),
		subscriptionStore: groupshare.NewMemSubscriptionStore(),
		groupStore:        group.NewMemStore(),
	}
}

func (m *memoryBackend) close() error {
	return nil
}

// --- SQLite backend ---

// sqliteBackend implements StorageBackend using a SQLite3 database file.
type sqliteBackend struct {
	path string
	db   *sqlite.DB
}

// SQLiteBackend returns a StorageBackend that persists all data in a SQLite3
// database at the given path. Use ":memory:" for an in-process SQLite database
// that is discarded after close. The database file is opened lazily when
// client.Start() is called and closed when client.Stop() is called.
func SQLiteBackend(path string) StorageBackend {
	return &sqliteBackend{path: path}
}

func (s *sqliteBackend) storages() backendStorages {
	return backendStorages{
		deviceStorage:     s.db.DeviceStorage(),
		sharedStorage:     s.db.SharedStorage(),
		subscriptionStore: s.db.SubscriptionStore(),
		groupStore:        s.db.GroupStore(),
	}
}

func (s *sqliteBackend) close() error {
	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
}

// open opens the SQLite database. Called from client.Start().
func (s *sqliteBackend) open() error {
	db, err := sqlite.Open(s.path)
	if err != nil {
		return err
	}
	s.db = db
	return nil
}
