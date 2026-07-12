package store

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/DSpeichert/netbootd/manifest"
	"github.com/rs/zerolog"
	yaml "gopkg.in/yaml.v2"
)

// ErrInvalidManifest wraps the validation errors PutManifest returns for
// client-supplied problems (missing ID/IPv4), so callers such as the HTTP API
// can distinguish a 400 Bad Request from a 500 backend failure.
var ErrInvalidManifest = errors.New("invalid manifest")

// Config controls store backend selection and options.
type Config struct {
	// Backend selects the implementation: "memory" (default), "disk" or "sqlite".
	Backend string
	// DiskPath is the directory used when Backend == "disk".
	DiskPath string
	// SQLitePath is the database file path used when Backend == "sqlite".
	SQLitePath string
	// RootPath is passed to manifest validation when a backend restores
	// manifests from files at startup.
	RootPath string
}

// Store is a pluggable manifest store. Every protocol server (DHCP/TFTP/HTTP/
// Syslog/API) depends on this interface rather than a concrete type.
type Store interface {
	// LoadFromDirectory parses every YAML manifest in a directory and upserts
	// it into the store. Used for the optional one-time startup import (-m).
	LoadFromDirectory(path string, rootPath string) error

	// PutManifest inserts or updates a manifest, keyed by ID.
	PutManifest(m manifest.Manifest) error

	// ForgetManifest removes a manifest by ID; it is a no-op if the ID is absent.
	ForgetManifest(id string) error

	// SetSuspended toggles a manifest's Suspended flag and persists the change.
	// It is a no-op if the ID is absent.
	SetSuspended(id string, suspended bool) error

	// Find returns a manifest by ID, or nil if not found.
	Find(id string) *manifest.Manifest

	// FindByIP returns a manifest by IPv4 address, or nil if not found.
	FindByIP(ip net.IP) *manifest.Manifest

	// FindByMAC returns a manifest by MAC address, or nil if not found.
	FindByMAC(mac net.HardwareAddr) *manifest.Manifest

	// GetAll returns every manifest keyed by ID.
	GetAll() map[string]*manifest.Manifest

	// Close releases backend resources (e.g. the sqlite handle).
	Close() error
}

// NewStore builds the Store implementation selected by cfg.Backend.
// An empty backend defaults to the in-memory store.
func NewStore(cfg Config) (Store, error) {
	switch cfg.Backend {
	case "", "memory":
		return NewMemoryStore(cfg)
	case "disk":
		return NewDiskStore(cfg)
	case "sqlite":
		return NewSQLiteStore(cfg)
	default:
		return nil, fmt.Errorf("unknown store backend %q (want memory, disk or sqlite)", cfg.Backend)
	}
}

// ipKey is the canonical map key for an IPv4/IPv6 address. Using a single
// helper keeps PutManifest, ForgetManifest and FindByIP in lock-step.
func ipKey(ip net.IP) string {
	if ip == nil {
		return ""
	}
	v16 := ip.To16()
	if v16 == nil {
		return ""
	}
	return string(v16)
}

// encodeManifest serializes a manifest for on-disk / in-db storage. YAML is
// used because manifest's custom types (IPWithNet, HardwareAddr) round-trip
// through YAML but not through encoding/json.
func encodeManifest(m manifest.Manifest) ([]byte, error) {
	return yaml.Marshal(m)
}

// decodeManifest reverses encodeManifest. It does not run Validate, since the
// bytes were produced by the store itself and rootPath is not available here.
func decodeManifest(b []byte) (manifest.Manifest, error) {
	var m manifest.Manifest
	err := yaml.Unmarshal(b, &m)
	return m, err
}

// loadFromDirectory reads every *.yml/*.yaml file in path, parses and validates
// it, and upserts it via put. It is shared by all backends' LoadFromDirectory.
func loadFromDirectory(put func(manifest.Manifest) error, path, rootPath string, logger zerolog.Logger) error {
	items, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, item := range items {
		if !item.Type().IsRegular() ||
			(!strings.HasSuffix(item.Name(), ".yml") && !strings.HasSuffix(item.Name(), ".yaml")) {
			continue
		}

		b, err := os.ReadFile(filepath.Join(path, item.Name()))
		if err != nil {
			logger.Error().Err(err).Str("file", item.Name()).Msg("cannot open manifest file")
			continue
		}
		m, err := manifest.ManifestFromYaml(b, rootPath)
		if err != nil {
			logger.Error().Err(err).Str("file", item.Name()).Msg("cannot parse YAML manifest")
			continue
		}
		if err := put(m); err != nil {
			logger.Error().Err(err).Str("file", item.Name()).Msg("cannot add manifest to store")
			continue
		}
		logger.Debug().Str("id", m.ID).Msg("loaded manifest from file")
	}
	return nil
}

// setSuspended is the shared SetSuspended implementation: read, mutate a copy,
// upsert. Routing through the store's own PutManifest means persistence is
// handled by whichever backend s is.
//
// It is important to copy *m before mutating: for the memory and disk
// backends, Find returns the same pointer stored in the internal index, which
// concurrent readers (Find/GetAll/FindByIP/FindByMAC) access without taking a
// write lock. Mutating it in place would race with those reads.
func setSuspended(s Store, id string, suspended bool) error {
	m := s.Find(id)
	if m == nil {
		return nil
	}
	updated := *m
	updated.Suspended = suspended
	return s.PutManifest(updated)
}

// validateManifest checks the minimal invariants required for a manifest to
// be stored. Both memoryStore and diskStore call this so that diskStore can
// validate before writing to disk (see diskStore.PutManifest).
func validateManifest(m manifest.Manifest) error {
	if m.IPv4.IP == nil {
		return fmt.Errorf("%w: no IPv4 address provided", ErrInvalidManifest)
	}
	if m.ID == "" {
		return fmt.Errorf("%w: ID cannot be null", ErrInvalidManifest)
	}
	return nil
}

var validID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// manifestFilename maps a manifest ID to a file inside dir, rejecting IDs that
// could escape the directory (path traversal) or that are otherwise unsafe as a
// filename component.
func manifestFilename(dir, id string) (string, error) {
	if id == "" || id == "." || id == ".." || !validID.MatchString(id) {
		return "", fmt.Errorf("manifest ID %q is not a valid filename component", id)
	}
	p := filepath.Join(dir, id+".yml")
	if filepath.Dir(p) != filepath.Clean(dir) {
		return "", fmt.Errorf("manifest ID %q escapes the store directory", id)
	}
	return p, nil
}
