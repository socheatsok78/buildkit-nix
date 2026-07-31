package dockershim

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Manifest struct {
	Config   string   `json:"Config"`
	RepoTags []string `json:"RepoTags"`
	Layers   []string `json:"Layers"`
}

func UnmarshalManifest(data []byte) (*Manifest, error) {
	var m []Manifest

	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}
	if len(m) == 0 {
		return nil, errors.New("manifest is empty")
	}

	return &m[0], nil
}
