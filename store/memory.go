package store

import (
	"errors"
	"net"
	"sync"

	"github.com/DSpeichert/netbootd/manifest"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// memoryStore is the default backend: manifests live only in RAM and are lost
// on restart. It is also embedded (by composition) by the disk backend, which
// adds file persistence on top of the same indexes.
type memoryStore struct {
	// mapping Manifest ID to Manifest
	manifests map[string]*manifest.Manifest
	// mapping IP address (ipKey) to Manifest
	ip map[string]*manifest.Manifest
	// mapping MAC address to Manifest
	mac map[string]*manifest.Manifest

	logger zerolog.Logger
	mu     sync.RWMutex
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		manifests: make(map[string]*manifest.Manifest),
		ip:        make(map[string]*manifest.Manifest),
		mac:       make(map[string]*manifest.Manifest),
		logger:    log.With().Str("module", "store").Str("backend", "memory").Logger(),
	}
}

// NewMemoryStore returns an in-memory Store.
func NewMemoryStore(cfg Config) (Store, error) {
	return newMemoryStore(), nil
}

func (s *memoryStore) LoadFromDirectory(path, rootPath string) error {
	return loadFromDirectory(s.PutManifest, path, rootPath, s.logger)
}

func (s *memoryStore) PutManifest(m manifest.Manifest) error {
	if m.IPv4.IP == nil {
		return errors.New("no IPv4 address provided")
	}
	if m.ID == "" {
		return errors.New("ID cannot be null")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a copy of the struct so a later mutation of the caller's Manifest
	// value does not reach into the store. Note this is a shallow copy: the
	// stored manifest still shares slice/map backing arrays with the caller,
	// which is acceptable because callers construct a fresh Manifest per Put.
	stored := m

	// Drop any stale index entries from a previous version of this ID before
	// re-indexing, so a changed IP/MAC does not leave a dangling lookup.
	if old, ok := s.manifests[m.ID]; ok {
		s.unindex(old)
	}

	s.manifests[stored.ID] = &stored
	s.ip[ipKey(stored.IPv4.IP)] = &stored
	for _, mac := range stored.MAC {
		s.mac[mac.String()] = &stored
	}
	return nil
}

func (s *memoryStore) ForgetManifest(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.manifests[id]
	if !ok {
		return nil
	}
	delete(s.manifests, id)
	s.unindex(m)
	return nil
}

// unindex removes a manifest's IP and MAC lookup entries. Caller holds s.mu.
func (s *memoryStore) unindex(m *manifest.Manifest) {
	delete(s.ip, ipKey(m.IPv4.IP))
	for _, mac := range m.MAC {
		delete(s.mac, mac.String())
	}
}

func (s *memoryStore) SetSuspended(id string, suspended bool) error {
	return setSuspended(s, id, suspended)
}

func (s *memoryStore) Find(id string) *manifest.Manifest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.manifests[id]
}

func (s *memoryStore) FindByIP(ip net.IP) *manifest.Manifest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ip[ipKey(ip)]
}

func (s *memoryStore) FindByMAC(mac net.HardwareAddr) *manifest.Manifest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.mac[mac.String()]
}

func (s *memoryStore) GetAll() map[string]*manifest.Manifest {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]*manifest.Manifest, len(s.manifests))
	for k, v := range s.manifests {
		out[k] = v
	}
	return out
}

func (s *memoryStore) Close() error { return nil }
