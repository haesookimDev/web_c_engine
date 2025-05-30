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
	Host                  string `yaml:"host"`
	Port                  string `yaml:"port"`
	CollectionName        string `yaml:"collection_name"`
	EmbeddingDimension    int    `yaml:"embedding_dimension"`
	MaxLengthURL          int    `yaml:"max_length_url"`
	MaxLengthHTML         int    `yaml:"max_length_html"`
	MaxLengthContent      int    `yaml:"max_length_content"`
	MaxLengthTitle        int    `yaml:"max_length_title"`
	MaxLengthMetaDesc     int    `yaml:"max_length_meta_desc"`
	MaxLengthCanonicalURL int    `yaml:"max_length_canonical_url"`
	MaxLengthLanguage     int    `yaml:"max_length_language"`
	MaxLengthHeadings     int    `yaml:"max_length_headings"`
	IndexType             string `yaml:"index_type"`
	MetricType            string `yaml:"metric_type"`
	Nlist                 int    `yaml:"nlist"`
}

type LoggerConfig struct {
	Level string `yaml:"level"`
}

type EmbedderConfig struct {
	Type        string `yaml:"type"`
	APIEndpoint string `yaml:"api_endpoint,omitempty"`
	APIKey      string `yaml:"api_key,omitempty"`
	ModelName   string `yaml:"model_name,omitempty"`
}

type Config struct {
	Crawler  CrawlerConfig  `yaml:"crawler"`
	Milvus   MilvusConfig   `yaml:"milvus"`
	Logger   LoggerConfig   `yaml:"logger"`
	Embedder EmbedderConfig `yaml:"embedder"`
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

	if cfg.Milvus.EmbeddingDimension == 0 {
		cfg.Milvus.EmbeddingDimension = 768
	}
	if cfg.Embedder.Type == "" {
		cfg.Embedder.Type = "dummy"
	}

	return cfg, nil
}
