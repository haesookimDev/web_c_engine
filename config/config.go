package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type CrawlerConfig struct {
	SeedURLs        []string `yaml:"seed_urls"`
	MaxDepth        int      `yaml:"max_depth"`
	DelayMs         int64    `yaml:"delay_ms"`
	MaxConcurrency  int      `yaml:"max_concurrency"`
	UserAgents      []string `yaml:"user_agents"`
	AdLinkPatterns  []string `yaml:"ad_link_patterns"`
	ContentTags     []string `yaml:"content_tags"`
	ExcludedDomains []string `yaml:"excluded_domains"`
}

type MilvusConfig struct {
	Host           string `yaml:"host"`
	Port           string `yaml:"port"`
	CollectionName string `yaml:"collection_name"`
}

type LoggerConfig struct {
	Level string `yaml:"level"`
}

type Config struct {
	Crawler CrawlerConfig `yaml:"crawler"`
	Milvus  MilvusConfig  `yaml:"milvus"`
	Logger  LoggerConfig  `yaml:"logger"`
}

// LoadConfig loads configuration from the given path.
func LoadConfig(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	err = yaml.Unmarshal(data, cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
