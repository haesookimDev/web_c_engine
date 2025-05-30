package embedder

import (
	"context"
	"fmt"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"crawlengine/config"
)

type TextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimension() int
}

type DummyEmbedder struct {
	dimension int
}

func NewDummyEmbedder(dimension int) *DummyEmbedder {
	if dimension <= 0 {
		log.Printf("Warning: Invalid dimension %d for DummyEmbedder, defaulting to 768.", dimension)
		dimension = 768
	}
	return &DummyEmbedder{dimension: dimension}
}

func (de *DummyEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		log.Println("Warning: Embedding empty text, returning zero vector.")
		return make([]float32, de.dimension), nil
	}

	vec := make([]float32, de.dimension)
	for i := 0; i < de.dimension; i++ {
		val := float32(len(text)+i) * 0.01
		vec[i] = float32(math.Sin(float64(val)))
	}
	return vec, nil
}

func (de *DummyEmbedder) Dimension() int {
	return de.dimension
}

type APIEmbedder struct {
	apiEndpoint string
	apiKey      string
	modelName   string
	dimension   int
	httpClient  *http.Client
}

func NewAPIEmbedder(cfg config.EmbedderConfig, dimension int) (*APIEmbedder, error) {
	if cfg.APIEndpoint == "" {
		return nil, fmt.Errorf("API endpoint is required for APIEmbedder")
	}
	return &APIEmbedder{
		apiEndpoint: cfg.APIEndpoint,
		apiKey:      cfg.APIKey,
		modelName:   cfg.ModelName,
		dimension:   dimension,
		httpClient:  &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (ae *APIEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	// requestBody, err := json.Marshal(map[string]string{"text": text, "model": ae.modelName})
	// ... http.NewRequestWithContext, set headers (Authorization if apiKey exists), client.Do ...
	// ... parse response, extract vector ...
	log.Printf("APIEmbedder: Embedding text (length %d) via %s", len(text), ae.apiEndpoint)

	if text == "" {
		return make([]float32, ae.dimension), nil
	}
	vec := make([]float32, ae.dimension)
	for i := 0; i < ae.dimension; i++ {
		vec[i] = float32(len(text)+i) * 0.01
	}
	return vec, fmt.Errorf("APIEmbedder.Embed not fully implemented")
}

func (ae *APIEmbedder) Dimension() int {
	return ae.dimension
}

func NewTextEmbedder(cfg *config.EmbedderConfig, milvusDimension int) (TextEmbedder, error) {
	log.Printf("Initializing embedder of type: '%s' with dimension: %d", cfg.Type, milvusDimension)
	switch strings.ToLower(cfg.Type) {
	case "dummy":
		return NewDummyEmbedder(milvusDimension), nil
	case "api":
		return NewAPIEmbedder(*cfg, milvusDimension)
	default:
		return nil, fmt.Errorf("unsupported embedder type: %s", cfg.Type)
	}
}
