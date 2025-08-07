package manifest

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

func ManifestFromJson(content []byte) (manifest Manifest, err error) {
	err = json.Unmarshal(content, &manifest)
	if err != nil {
		return manifest, err
	}

	return manifest, manifest.Validate()
}

func (m *Manifest) ToJson() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func ManifestFromYaml(content []byte) (manifest Manifest, err error) {
	err = yaml.Unmarshal(content, &manifest)
	if err != nil {
		return manifest, err
	}

	return manifest, manifest.Validate()
}

func (m Manifest) Validate() error {
	for _, mount := range m.Mounts {
		if mount.LocalDir != "" {
			if !filepath.IsAbs(mount.LocalDir) {
				return fmt.Errorf("localDir needs to be absolute path")
			}
		}
	}

	return nil
}

func (m *Manifest) ToYaml() ([]byte, error) {
	return yaml.Marshal(&m)
}
