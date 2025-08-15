package manifest

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

func ManifestFromJson(content []byte, rootPath string) (manifest Manifest, err error) {
	err = json.Unmarshal(content, &manifest)
	if err != nil {
		return manifest, err
	}

	return manifest, manifest.Validate(rootPath)
}

func (m *Manifest) ToJson() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func ManifestFromYaml(content []byte, rootPath string) (manifest Manifest, err error) {
	err = yaml.Unmarshal(content, &manifest)
	if err != nil {
		return manifest, err
	}

	return manifest, manifest.Validate(rootPath)
}

func (m Manifest) Validate(rootPath string) error {
	for _, mount := range m.Mounts {
		if mount.LocalDir != "" {
			if !filepath.IsAbs(mount.LocalDir) && rootPath == "" {
				return fmt.Errorf("localDir needs to be absolute path when rootPath is not set")
			}
		}
	}

	return nil
}

func (m *Manifest) ToYaml() ([]byte, error) {
	return yaml.Marshal(&m)
}
