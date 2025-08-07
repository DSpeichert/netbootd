package store

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/DSpeichert/netbootd/manifest"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Config struct {
	PersistenceDirectory string
}

type Store struct {
	config Config

	// mapping Manifest ID to Manifest
	manifests map[string]*manifest.Manifest

	// mapping IP Address to Manifest
	// IP is normalized string(ip.To16)
	ip map[string]*manifest.Manifest

	// mapping Mac Address to Manifest
	mac map[string]*manifest.Manifest

	logger zerolog.Logger

	mutex sync.RWMutex

	// sort of global config
	GlobalHints struct {
		HttpPort int
		ApiPort  int
	}
}

func NewStore(cfg Config) (*Store, error) {
	store := Store{
		config:    cfg,
		manifests: make(map[string]*manifest.Manifest),
		ip:        make(map[string]*manifest.Manifest),
		mac:       make(map[string]*manifest.Manifest),
		logger:    log.With().Str("module", "store").Logger(),
	}

	return &store, nil
}

func (s *Store) LoadFromDirectory(path string) (err error) {
	items, err := os.ReadDir(path)
	for _, item := range items {
		if !item.Type().IsRegular() ||
			(!strings.HasSuffix(item.Name(), ".yml") && !strings.HasSuffix(item.Name(), ".yaml")) {
			continue
		}

		b, err := os.ReadFile(filepath.Join(path, item.Name()))
		if err != nil {
			s.logger.Error().
				Err(err).
				Msg("cannot open file")
			continue
		}
		m, err := manifest.ManifestFromYaml(b)
		if err != nil {
			s.logger.Error().
				Err(err).
				Msg("cannot parse YAML manifest")
			continue
		}
		err = s.PutManifest(m)
		if err != nil {
			s.logger.Error().
				Err(err).
				Msg("cannot add manifest to store")
			continue
		}

		if s.logger.Debug().Enabled() {
			s.logger.Debug().
				Interface("manifest", m).
				Msg("Loaded manifest from file")
		}

	}
	return
}

func (s *Store) PutManifest(m manifest.Manifest) error {
	if m.IPv4.IP == nil {
		return errors.New("no IPv4 address provided")
	}

	if m.ID == "" {
		return errors.New("ID cannot be null")
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.manifests[m.ID] = &m
	s.ip[string(m.IPv4.IP.To16())] = &m
	for _, mac := range m.MAC {
		s.mac[mac.String()] = &m
	}

	if s.config.PersistenceDirectory != "" {
		return s.putPersistentManifest(m)
	}

	return nil
}

func (s *Store) ForgetManifest(id string) error {
	s.mutex.RLock()
	m, ok := s.manifests[id]
	s.mutex.RUnlock()
	if !ok {
		return nil
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.manifests, m.ID)
	delete(s.ip, string(m.IPv4.IP.To16()))
	for _, mac := range m.MAC {
		delete(s.mac, mac.String())
	}

	if s.config.PersistenceDirectory != "" {
		return s.forgetPersistentManifest(id)
	}

	return nil
}

func (s *Store) Find(id string) *manifest.Manifest {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.manifests[id]
}

func (s *Store) FindByIP(ip net.IP) *manifest.Manifest {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.ip[string(ip.To16())]
}

func (s *Store) FindByMAC(mac net.HardwareAddr) *manifest.Manifest {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.mac[mac.String()]
}

func (s *Store) GetAll() map[string]*manifest.Manifest {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.manifests
}
