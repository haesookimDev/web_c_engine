package storage

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"crawlengine/config"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
)

type WebDocument struct {
	HashID               string    `json:"hash_id"`
	URL                  string    `json:"url"`
	HTMLSource           string    `json:"html_source"`
	MainContent          string    `json:"main_content"`
	Title                string    `json:"title"`
	MetaDescription      string    `json:"meta_description"`
	CanonicalURL         string    `json:"canonical_url"`
	Language             string    `json:"language"`
	PublicationTimestamp int64     `json:"publication_timestamp"`
	HeadingsText         string    `json:"headings_text"`
	CrawledAt            time.Time `json:"crawled_at"`
	ContentVector        []float32 `json:"content_vector"`
}

type MilvusStorer struct {
	milvusClient client.Client
	cfg          *config.MilvusConfig
}

func NewMilvusStorer(ctx context.Context, cfg *config.MilvusConfig) (*MilvusStorer, error) {
	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	log.Printf("Connecting to Milvus at %s", addr)

	cli, err := client.NewClient(ctx, client.Config{Address: addr})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Milvus: %w", err)
	}
	log.Printf("Successfully connected to Milvus at %s", addr)

	storer := &MilvusStorer{
		milvusClient: cli,
		cfg:          cfg,
	}

	// Ensure collection exists
	if err := storer.ensureCollection(ctx); err != nil {
		cli.Close()
		return nil, fmt.Errorf("failed to ensure Milvus collection: %w", err)
	}

	return storer, nil
}

func (ms *MilvusStorer) ensureCollection(ctx context.Context) error {
	exists, err := ms.milvusClient.HasCollection(ctx, ms.cfg.CollectionName)
	if err != nil {
		return fmt.Errorf("failed to check for collection %s: %w", ms.cfg.CollectionName, err)
	}

	if exists {
		log.Printf("Collection '%s' already exists.", ms.cfg.CollectionName)
		return nil
	}

	log.Printf("Collection '%s' does not exist. Creating...", ms.cfg.CollectionName)

	schema := &entity.Schema{
		CollectionName: ms.cfg.CollectionName,
		Description:    "Web documents crawled for AI search engine",
		AutoID:         false,
		Fields: []*entity.Field{
			entity.NewField().WithName("hash_id").WithDataType(entity.FieldTypeVarChar).WithIsPrimaryKey(true).WithMaxLength(64),
			entity.NewField().WithName("url").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(ms.cfg.MaxLengthURL)),
			entity.NewField().WithName("html_source").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(ms.cfg.MaxLengthHTML)),
			entity.NewField().WithName("main_content").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(ms.cfg.MaxLengthContent)),
			entity.NewField().WithName("title").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(ms.cfg.MaxLengthTitle)),
			entity.NewField().WithName("meta_description").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(ms.cfg.MaxLengthMetaDesc)),
			entity.NewField().WithName("canonical_url").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(ms.cfg.MaxLengthCanonicalURL)),
			entity.NewField().WithName("language").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(ms.cfg.MaxLengthLanguage)),
			entity.NewField().WithName("publication_timestamp").WithDataType(entity.FieldTypeInt64), // Stores Unix timestamp
			entity.NewField().WithName("headings_text").WithDataType(entity.FieldTypeVarChar).WithMaxLength(int64(ms.cfg.MaxLengthHeadings)),
			entity.NewField().WithName("crawled_at").WithDataType(entity.FieldTypeInt64), // Stores Unix timestamp
			entity.NewField().WithName("content_vector").WithDataType(entity.FieldTypeFloatVector).WithDim(int64(ms.cfg.EmbeddingDimension)),
		},
	}

	err = ms.milvusClient.CreateCollection(ctx, schema, entity.DefaultShardNumber) // entity.DefaultShardNumber or specify
	if err != nil {
		return fmt.Errorf("failed to create collection %s: %w", ms.cfg.CollectionName, err)
	}
	log.Printf("Collection '%s' created successfully.", ms.cfg.CollectionName)

	log.Printf("Creating index for field 'content_vector' in collection '%s'...", ms.cfg.CollectionName)
	var idx entity.Index // Declare idx as the interface type entity.Index

	metricType := entity.L2
	if strings.ToUpper(ms.cfg.MetricType) == "IP" {
		metricType = entity.IP
	} else if strings.ToUpper(ms.cfg.MetricType) != "L2" {
		log.Printf("Warning: Invalid MetricType '%s' in config, defaulting to L2.", ms.cfg.MetricType)
	}

	if strings.ToUpper(ms.cfg.IndexType) == "IVF_FLAT" {
		idx, err = entity.NewIndexIvfFlat(metricType, ms.cfg.Nlist)
		if err != nil {
			return fmt.Errorf("failed to create IVF_FLAT index parameters: %w", err)
		}
	} else if strings.ToUpper(ms.cfg.IndexType) == "HNSW" {
		// M: typically 4-64. Higher M = more accurate but slower & more memory.
		// efConstruction: typically 100-500. Higher = better graph but slower build.
		hnswM := 16
		hnswEfConstruction := 200
		idx, _ = entity.NewIndexHNSW(metricType, hnswM, hnswEfConstruction)
		log.Printf("Using HNSW index with M=%d, efConstruction=%d", hnswM, hnswEfConstruction)
	} else {
		log.Printf("Unsupported index type '%s' in config, defaulting to IVF_FLAT with L2 and nlist=%d", ms.cfg.IndexType, ms.cfg.Nlist)
		idx, _ = entity.NewIndexIvfFlat(entity.L2, ms.cfg.Nlist) // Defaulting
	}

	err = ms.milvusClient.CreateIndex(ctx, ms.cfg.CollectionName, "content_vector", idx, false) // sync=false (async)
	if err != nil {
		return fmt.Errorf("failed to create index for collection %s on field 'content_vector': %w", ms.cfg.CollectionName, err)
	}
	log.Printf("Index for 'content_vector' on collection '%s' creation request sent.", ms.cfg.CollectionName)

	err = ms.milvusClient.LoadCollection(ctx, ms.cfg.CollectionName, false)
	if err != nil {
		return fmt.Errorf("failed to load collection %s: %w", ms.cfg.CollectionName, err)
	}
	log.Printf("Collection '%s' loaded.", ms.cfg.CollectionName)

	return nil
}
func (ms *MilvusStorer) StoreDocument(ctx context.Context, doc *WebDocument) error {
	if doc == nil {
		return fmt.Errorf("cannot store nil document")
	}
	log.Printf("Attempting to store document for URL: %s with ID: %s", doc.URL, doc.HashID)

	if len(doc.ContentVector) != 0 && len(doc.ContentVector) != ms.cfg.EmbeddingDimension {
		return fmt.Errorf("document ID %s has content vector with dimension %d, but collection expects %d",
			doc.HashID, len(doc.ContentVector), ms.cfg.EmbeddingDimension)
	}

	currentContentVector := doc.ContentVector
	if len(currentContentVector) == 0 {
		log.Printf("Warning: Document ID %s has no content vector. Inserting a zero vector as placeholder.", doc.HashID)
		currentContentVector = make([]float32, ms.cfg.EmbeddingDimension)
	}

	hashIDs := []string{doc.HashID}
	urls := []string{doc.URL}
	htmlSources := []string{doc.HTMLSource}
	mainContents := []string{doc.MainContent}
	titles := []string{doc.Title}
	metaDescriptions := []string{doc.MetaDescription}
	canonicalURLs := []string{doc.CanonicalURL}
	languages := []string{doc.Language}
	publicationTimestamps := []int64{doc.PublicationTimestamp}
	headingsTexts := []string{doc.HeadingsText}
	crawledAts := []int64{doc.CrawledAt.Unix()}
	contentVectors := [][]float32{currentContentVector}

	colHashID := entity.NewColumnVarChar("hash_id", hashIDs)
	colURL := entity.NewColumnVarChar("url", urls)
	colHTMLSource := entity.NewColumnVarChar("html_source", htmlSources)
	colMainContent := entity.NewColumnVarChar("main_content", mainContents)
	colTitle := entity.NewColumnVarChar("title", titles)
	colMetaDescription := entity.NewColumnVarChar("meta_description", metaDescriptions)
	colCanonicalURL := entity.NewColumnVarChar("canonical_url", canonicalURLs)
	colLanguage := entity.NewColumnVarChar("language", languages)
	colPublicationTimestamp := entity.NewColumnInt64("publication_timestamp", publicationTimestamps)
	colHeadingsText := entity.NewColumnVarChar("headings_text", headingsTexts)
	colCrawledAt := entity.NewColumnInt64("crawled_at", crawledAts)
	colContentVector := entity.NewColumnFloatVector("content_vector", ms.cfg.EmbeddingDimension, contentVectors)

	_, err := ms.milvusClient.Insert(
		ctx,
		ms.cfg.CollectionName,
		"",
		colHashID,
		colURL,
		colHTMLSource,
		colMainContent,
		colTitle,
		colMetaDescription,
		colCanonicalURL,
		colLanguage,
		colPublicationTimestamp,
		colHeadingsText,
		colCrawledAt,
		colContentVector,
	)

	if err != nil {
		return fmt.Errorf("failed to insert document into Milvus (URL: %s, ID: %s): %w", doc.URL, doc.HashID, err)
	}

	log.Printf("Successfully inserted document ID: %s for URL: %s into Milvus collection '%s'", doc.HashID, doc.URL, ms.cfg.CollectionName)

	err = ms.milvusClient.Flush(ctx, ms.cfg.CollectionName, false)
	if err != nil {
		log.Printf("Warning: Failed to flush collection %s: %v", ms.cfg.CollectionName, err)
	} else {
		log.Printf("Collection %s flushed.", ms.cfg.CollectionName)
	}

	return nil
}

// Close closes the Milvus client connection.
func (ms *MilvusStorer) Close() {
	if ms.milvusClient != nil {
		err := ms.milvusClient.Close()
		if err != nil {
			log.Printf("Error closing Milvus client connection: %v", err)
			return
		}
		log.Println("Milvus client connection closed.")
	}
}
