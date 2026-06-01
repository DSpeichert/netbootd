package store

import (
	"os"
	"path/filepath"

	"github.com/DSpeichert/netbootd/manifest"
)

func (s *Store) putPersistentManifest(m *manifest.Manifest) error {
	path := filepath.Join(s.config.PersistenceDirectory, filepath.Base(m.ID) + ".yml")

	b, err := m.ToYaml()
	if err != nil {
		return err
	}

	err = os.WriteFile(path, b, 0644)
	if err != nil {
		return err
	}

	s.paths[m.ID] = path

	return nil
}

func (s *Store) forgetPersistentManifest(id string) error {
	path := s.paths[id]
	delete(s.paths, id)

	err := os.Remove(path)
	if err != nil {
		s.logger.Err(err).Str("path", path).Msg("Failed to remove manifest file.")
	}

	return nil
}
