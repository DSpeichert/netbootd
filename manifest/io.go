package manifest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

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
	for i, mount := range m.Mounts {
		if mount.BaseDir != "" {
			if !filepath.IsAbs(mount.BaseDir) {
				return fmt.Errorf("BaseDir needs to be absolute path")
			}
			if !strings.HasSuffix(mount.BaseDir, "/") {
				mount.BaseDir = mount.BaseDir + "/"
			}

			m.Mounts[i] = mount
		}
	}

	return nil
}

func (m *Manifest) ToYaml() ([]byte, error) {
	return yaml.Marshal(&m)
}
