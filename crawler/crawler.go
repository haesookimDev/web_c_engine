package crawler

import (
	"context"
	"io"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"crawlengine/config"
	"crawlengine/storage"

	"github.com/PuerkitoBio/goquery"
)

type CrawlTask struct {
	URL   string
	Depth int
}

type Crawler struct {
	Config      *config.CrawlerConfig
	Storer      *storage.MilvusStorer
	httpClient  HTTPClient // Could be a more sophisticated client interface
	visited     map[string]bool
	visitedLock sync.Mutex
	taskQueue   chan CrawlTask
	wg          sync.WaitGroup
	adPatterns  []*regexp.Regexp
}

// HTTPClient interface for fetching pages, allowing for mocks or advanced clients.
type HTTPClient interface {
	Get(url string, userAgent string) (*goquery.Document, string, error)
}

type DefaultHTTPClient struct{}

// Get fetches a page and returns a goquery Document and the raw HTML string.
func (c *DefaultHTTPClient) Get(targetURL string, userAgent string) (*goquery.Document, string, error) {
	resp, err := FetchPage(targetURL, userAgent)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("Non-200 status for %s: %d", targetURL, resp.StatusCode)
		return nil, "", err // Or a custom error type
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	htmlString := string(bodyBytes)

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlString))
	if err != nil {
		return nil, htmlString, err
	}
	return doc, htmlString, nil
}

// NewCrawler initializes a new Crawler.
func NewCrawler(cfg *config.CrawlerConfig, storer *storage.MilvusStorer) *Crawler {
	compiledAdPatterns := make([]*regexp.Regexp, len(cfg.AdLinkPatterns))
	for i, pattern := range cfg.AdLinkPatterns {
		compiledAdPatterns[i] = regexp.MustCompile(pattern) // Compile patterns once
	}

	return &Crawler{
		Config:     cfg,
		Storer:     storer,
		httpClient: &DefaultHTTPClient{},
		visited:    make(map[string]bool),
		taskQueue:  make(chan CrawlTask, cfg.MaxConcurrency*10), // Buffered channel
		adPatterns: compiledAdPatterns,
	}
}

// Start begins the crawling process.
func (c *Crawler) Start(ctx context.Context) {
	log.Println("Crawler starting...")

	for i := 0; i < c.Config.MaxConcurrency; i++ {
		c.wg.Add(1)
		go c.worker(ctx, i)
	}

	for _, seedURL := range c.Config.SeedURLs {
		c.taskQueue <- CrawlTask{URL: seedURL, Depth: 0}
		c.markVisited(seedURL)
	}

	// Wait for initial tasks to be queued, then consider closing queue if no more seeds
	// Or, more robustly, manage queue closing when all workers are idle and queue is empty.
	// For simplicity, we'll let workers run until context is done or queue naturally empties
	// and no new tasks are generated for a while. A more sophisticated shutdown is needed for long-running crawlers.

	c.wg.Wait()
	close(c.taskQueue) // Close queue only after all producers (initial seeds, link extractors) are done.
	// This basic example closes it after initial seeds. A better approach is needed.
	log.Println("Crawler finished all tasks.")
}

func (c *Crawler) worker(ctx context.Context, id int) {
	defer c.wg.Done()
	log.Printf("Worker %d started", id)
	for {
		select {
		case task, ok := <-c.taskQueue:
			if !ok {
				log.Printf("Worker %d: Task queue closed, exiting.", id)
				return // Queue closed
			}
			if task.Depth > c.Config.MaxDepth {
				log.Printf("Worker %d: Max depth %d reached for %s, skipping.", id, c.Config.MaxDepth, task.URL)
				continue
			}
			c.crawlPage(ctx, task)
			time.Sleep(time.Duration(c.Config.DelayMs) * time.Millisecond) // Respect delay
		case <-ctx.Done():
			log.Printf("Worker %d: Context cancelled, exiting.", id)
			return
		}
	}
}

func (c *Crawler) markVisited(url string) {
	c.visitedLock.Lock()
	defer c.visitedLock.Unlock()
	c.visited[url] = true
}

func (c *Crawler) hasVisited(url string) bool {
	c.visitedLock.Lock()
	defer c.visitedLock.Unlock()
	_, found := c.visited[url]
	return found
}

func (c *Crawler) crawlPage(ctx context.Context, task CrawlTask) {
	log.Printf("Crawling [Depth %d]: %s", task.Depth, task.URL)

	parsedURL, err := url.Parse(task.URL)
	if err != nil {
		log.Printf("Error parsing URL %s: %v", task.URL, err)
		return
	}

	currentUA := GetRandomUserAgent(c.Config.UserAgents)
	if !IsAllowedByRobots(parsedURL, currentUA) {
		log.Printf("Crawling disallowed by robots.txt for %s using agent %s", task.URL, currentUA)
		return
	}

	doc, htmlString, err := c.httpClient.Get(task.URL, currentUA)
	if err != nil {
		log.Printf("Error fetching %s: %v", task.URL, err)
		return
	}

	mainContent := ExtractMainContent(doc, c.Config.ContentTags)
	if mainContent == "" {
		log.Printf("Could not extract main content from %s", task.URL)
		// Decide if you still want to store pages with no extracted "main" content
	}

	// ID Generation: Using main content hash is generally better for semantic uniqueness.
	contentHash := GenerateContentHash(mainContent) // Or htmlString if preferred

	title := doc.Find("title").First().Text()
	metaDescription, _ := doc.Find("meta[name='description']").Attr("content")
	// You can add more meta tag extractions here (OpenGraph, etc.)

	webDoc := &storage.WebDocument{
		HashID:          contentHash,
		URL:             task.URL,
		HTMLSource:      htmlString,
		MainContent:     mainContent,
		Title:           strings.TrimSpace(title),
		MetaDescription: strings.TrimSpace(metaDescription),
		CrawledAt:       time.Now().UTC(),
	}

	if err := c.Storer.StoreDocument(ctx, webDoc); err != nil {
		log.Printf("Error storing document for %s (ID: %s): %v", task.URL, contentHash, err)
	} else {
		log.Printf("Successfully processed and sent to store: %s (ID: %s)", task.URL, contentHash)
	}

	if task.Depth < c.Config.MaxDepth {
		c.extractAndQueueLinks(doc, parsedURL, task.Depth+1)
	}
}

func (c *Crawler) extractAndQueueLinks(doc *goquery.Document, baseURL *url.URL, nextDepth int) {
	doc.Find("a[href]").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if !exists || strings.HasPrefix(href, "#") || strings.HasPrefix(strings.ToLower(href), "javascript:") {
			return
		}

		absURLString, err := NormalizeURL(baseURL, href)
		if err != nil {
			log.Printf("Error normalizing URL %s (base %s): %v", href, baseURL.String(), err)
			return
		}

		linkURL, err := url.Parse(absURLString)
		if err != nil {
			log.Printf("Error parsing absolute URL %s: %v", absURLString, err)
			return
		}

		// Only crawl links within the same domain (or subdomains if configured)
		if linkURL.Hostname() != baseURL.Hostname() {
			// log.Printf("Skipping external link: %s", absURLString)
			return
		}

		if IsExcludedDomain(linkURL, c.Config.ExcludedDomains) {
			log.Printf("Skipping excluded domain link: %s", absURLString)
			return
		}

		// Check for ad links using compiled regex
		isAd := false
		for _, pattern := range c.adPatterns {
			if pattern.MatchString(absURLString) {
				isAd = true
				break
			}
		}
		if isAd {
			log.Printf("Skipping ad link: %s", absURLString)
			return
		}

		if !c.hasVisited(absURLString) {
			c.markVisited(absURLString)
			log.Printf("Queueing new link: %s (Depth: %d)", absURLString, nextDepth)
			// Non-blocking send or check context
			select {
			case c.taskQueue <- CrawlTask{URL: absURLString, Depth: nextDepth}:
			default:
				log.Printf("Task queue full or blocked. Dropping link: %s", absURLString)
			}
		}
	})
}
