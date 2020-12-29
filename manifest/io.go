package manifest

import (
	"encoding/json"
	"gopkg.in/yaml.v2"
)

func ManifestFromJson(content []byte) (manifest Manifest, err error) {
	err = json.Unmarshal(content, &manifest)
	return
}

func (m *Manifest) ToJson() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func ManifestFromYaml(content []byte) (manifest Manifest, err error) {
	err = yaml.Unmarshal(content, &manifest)
	return
}

func (m *Manifest) ToYaml() ([]byte, error) {
	return yaml.Marshal(&m)
}
