package config

import (
	"sort"

	toml "github.com/pelletier/go-toml/v2"
)

// Project represents a tracked project with its remote path.
type Project struct {
	Path string `toml:"path"`
}

// ProjectsConfig holds the set of tracked projects.
type ProjectsConfig struct {
	Projects map[string]Project `toml:"projects"`
}

// ParseProjectsConfig parses TOML data into a ProjectsConfig.
func ParseProjectsConfig(data []byte) (*ProjectsConfig, error) {
	var cfg ProjectsConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// SerializeProjectsConfig serializes a ProjectsConfig to TOML bytes.
func SerializeProjectsConfig(cfg *ProjectsConfig) ([]byte, error) {
	return toml.Marshal(cfg)
}

// SortedProjectKeys returns the project keys in alphabetical order.
func (c *ProjectsConfig) SortedProjectKeys() []string {
	keys := make([]string, 0, len(c.Projects))
	for key := range c.Projects {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
