package store

import (
	"errors"
	"net"
	"os"
	"sync"

	"github.com/DSpeichert/netbootd/manifest"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// diskStore keeps the same in-memory indexes as the memory backend and mirrors
// every mutation to a directory of per-manifest YAML files, so API-managed
// manifests survive a restart. On startup it restores whatever files already
// exist in the directory.
type diskStore struct {
	mem    *memoryStore
	dir    string
	logger zerolog.Logger
	mu     sync.Mutex // serializes file writes/removes
}

// NewDiskStore returns a disk-backed Store rooted at cfg.DiskPath. The
// directory is created if missing; existing manifests in it are restored.
func NewDiskStore(cfg Config) (Store, error) {
	if cfg.DiskPath == "" {
		return nil, errors.New("DiskPath is required for disk backend")
	}
	if err := os.MkdirAll(cfg.DiskPath, 0o755); err != nil {
		return nil, err
	}

	s := &diskStore{
		mem:    newMemoryStore(),
		dir:    cfg.DiskPath,
		logger: log.With().Str("module", "store").Str("backend", "disk").Logger(),
	}

	// Restore previously persisted manifests. Errors on individual files are
	// logged by loadFromDirectory and skipped.
	if err := loadFromDirectory(s.mem.PutManifest, s.dir, cfg.RootPath, s.logger); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *diskStore) LoadFromDirectory(path, rootPath string) error {
	return loadFromDirectory(s.PutManifest, path, rootPath, s.logger)
}

func (s *diskStore) PutManifest(m manifest.Manifest) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.mem.PutManifest(m); err != nil {
		return err
	}
	return s.writeFile(m)
}

func (s *diskStore) ForgetManifest(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mem.Find(id) == nil {
		return nil
	}
	if err := s.mem.ForgetManifest(id); err != nil {
		return err
	}
	// A failure to remove the file must not fail the forget: the manifest is
	// already gone from the working set.
	if path, err := manifestFilename(s.dir, id); err != nil {
		s.logger.Error().Err(err).Str("id", id).Msg("cannot resolve manifest file for removal")
	} else if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		s.logger.Error().Err(err).Str("path", path).Msg("failed to remove manifest file")
	}
	return nil
}

func (s *diskStore) SetSuspended(id string, suspended bool) error {
	return setSuspended(s, id, suspended)
}

// writeFile persists a manifest as YAML. Caller holds s.mu.
func (s *diskStore) writeFile(m manifest.Manifest) error {
	path, err := manifestFilename(s.dir, m.ID)
	if err != nil {
		return err
	}
	b, err := encodeManifest(m)
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

func (s *diskStore) Find(id string) *manifest.Manifest               { return s.mem.Find(id) }
func (s *diskStore) FindByIP(ip net.IP) *manifest.Manifest           { return s.mem.FindByIP(ip) }
func (s *diskStore) FindByMAC(m net.HardwareAddr) *manifest.Manifest { return s.mem.FindByMAC(m) }
func (s *diskStore) GetAll() map[string]*manifest.Manifest           { return s.mem.GetAll() }
func (s *diskStore) Close() error                                    { return nil }
