package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Handle          string   `yaml:"handle"`
	PDS             string   `yaml:"pds"`
	ContentDir      string   `yaml:"content_dir"`
	SiteURL         string   `yaml:"site_url"`
	SiteName        string   `yaml:"site_name"`
	SiteDescription string   `yaml:"site_description,omitempty"`
	PublicationRkey string   `yaml:"publication_rkey,omitempty"`
	IncludeDrafts   bool     `yaml:"include_drafts,omitempty"`
	ExcludeDirs     []string `yaml:"exclude_dirs,omitempty"`
	ExcludeFiles    []string `yaml:"exclude_files,omitempty"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	if cfg.PDS == "" {
		cfg.PDS = "https://bsky.social"
	}
	if cfg.ContentDir == "" {
		cfg.ContentDir = "content"
	}
	return cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
