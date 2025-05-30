package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

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

	log.Printf("Logger level set to: %s (Note: Using standard log, implement advanced logger if needed)", cfg.Logger.Level)

	milvusStorer, err := storage.NewMilvusStorer(&cfg.Milvus)
	if err != nil {
		log.Fatalf("Failed to initialize Milvus storer: %v", err)
	}
	defer milvusStorer.Close()

	cr := crawler.NewCrawler(&cfg.Crawler, milvusStorer)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal: %s. Shutting down...", sig)
		cancel()
	}()

	cr.Start(ctx)

	log.Println("Crawling engine finished or was interrupted.")
}
