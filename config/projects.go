package config

import (
	"sort"

	toml "github.com/pelletier/go-toml/v2"
)

// Project represents a tracked project with its remote path.
type Project struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

// ProjectsConfig holds the set of tracked projects.
type ProjectsConfig struct {
	Projects []Project `toml:"projects"`
}

// oldProjectsConfig is the legacy map-based format
type oldProjectsConfig struct {
	Projects map[string]struct {
		Path string `toml:"path"`
	} `toml:"projects"`
}

// ParseProjectsConfig parses TOML data, migrating from old format if needed.
func ParseProjectsConfig(data []byte) (*ProjectsConfig, error) {
	// Try new array format first
	var cfg ProjectsConfig
	if err := toml.Unmarshal(data, &cfg); err == nil && len(cfg.Projects) > 0 {
		return &cfg, nil
	}

	// Try old map format
	var oldCfg oldProjectsConfig
	if err := toml.Unmarshal(data, &oldCfg); err != nil {
		return nil, err
	}

	// Migrate: convert map to slice, sorted alphabetically
	if len(oldCfg.Projects) > 0 {
		names := make([]string, 0, len(oldCfg.Projects))
		for name := range oldCfg.Projects {
			names = append(names, name)
		}
		sort.Strings(names)

		cfg.Projects = make([]Project, len(names))
		for i, name := range names {
			cfg.Projects[i] = Project{
				Name: name,
				Path: oldCfg.Projects[name].Path,
			}
		}
		return &cfg, nil
	}

	cfg.Projects = []Project{}
	return &cfg, nil
}

// SerializeProjectsConfig serializes a ProjectsConfig to TOML bytes.
func SerializeProjectsConfig(cfg *ProjectsConfig) ([]byte, error) {
	return toml.Marshal(cfg)
}
