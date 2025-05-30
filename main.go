package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time" // For timeout context if needed

	"crawlengine/config"
	"crawlengine/crawler"
	"crawlengine/storage"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.LoadConfig("config/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Logger level set to: %s", cfg.Logger.Level)

	// Context for Milvus initialization (e.g., with a timeout)
	initCtx, initCancel := context.WithTimeout(context.Background(), 30*time.Second) // 30-second timeout for Milvus setup
	defer initCancel()

	milvusStorer, err := storage.NewMilvusStorer(initCtx, &cfg.Milvus) // Pass context
	if err != nil {
		log.Fatalf("Failed to initialize Milvus storer: %v", err)
	}
	defer milvusStorer.Close()

	cr := crawler.NewCrawler(&cfg.Crawler, milvusStorer)

	// Main context for the crawler itself
	crawlerCtx, crawlerCancel := context.WithCancel(context.Background())
	defer crawlerCancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %s. Shutting down...", sig)
		crawlerCancel() // Signal crawler workers to stop
	}()

	cr.Start(crawlerCtx)

	log.Println("Crawling engine finished or was interrupted.")
}
