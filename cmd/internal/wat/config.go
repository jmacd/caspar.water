package wat

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version      int    `yaml:"version"`
	Name         string `yaml:"name"`
	Year         int    `yaml:"year"`
	Journal      string `yaml:"journal"`
	WriteJournal string `yaml:"write_journal"`
	Sources      string `yaml:"sources"`
	root         string
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	file, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("open configuration: %w", err)
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode configuration: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return cfg, fmt.Errorf("configuration must contain exactly one YAML document")
	}
	if cfg.Version != 1 {
		return cfg, fmt.Errorf("unsupported configuration version %d (expected 1)", cfg.Version)
	}
	if strings.TrimSpace(cfg.Name) == "" || cfg.Year < 1 || cfg.Year > 9998 {
		return cfg, fmt.Errorf("name and a year between 1 and 9998 are required")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return cfg, err
	}
	cfg.root, err = filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return cfg, fmt.Errorf("resolve configuration directory: %w", err)
	}
	for _, item := range []struct {
		name string
		path *string
	}{
		{"journal", &cfg.Journal},
		{"write_journal", &cfg.WriteJournal},
		{"sources", &cfg.Sources},
	} {
		if strings.TrimSpace(*item.path) == "" || strings.ContainsAny(*item.path, "\r\n") {
			return cfg, fmt.Errorf("%s must be a nonempty local path", item.name)
		}
		resolved := *item.path
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(cfg.root, resolved)
		}
		resolved = filepath.Clean(resolved)
		if !within(cfg.root, resolved) {
			return cfg, fmt.Errorf("%s must be inside the configuration directory", item.name)
		}
		*item.path = resolved
	}
	return cfg, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && filepath.IsLocal(rel)
}
