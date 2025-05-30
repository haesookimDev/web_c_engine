package crawler

import (
	"crypto/sha256"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// GenerateContentHash creates a SHA256 hash for the given content.
func GenerateContentHash(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// GetRandomUserAgent selects a random user agent from the provided list.
func GetRandomUserAgent(userAgents []string) string {
	if len(userAgents) == 0 {
		return "GoCrawler/1.0 (+http://example.com/bot)" // Default user agent
	}
	rand.New(rand.NewSource(time.Now().UnixNano()))
	return userAgents[rand.Intn(len(userAgents))]
}

// IsAdLink checks if a URL matches any of the ad link patterns.
func IsAdLink(link string, adPatterns []string) bool {
	for _, pattern := range adPatterns {
		match, _ := regexp.MatchString(pattern, link)
		if match {
			return true
		}
	}
	return false
}

// IsExcludedDomain checks if the link belongs to an excluded domain.
func IsExcludedDomain(linkURL *url.URL, excludedDomains []string) bool {
	for _, domain := range excludedDomains {
		if strings.HasSuffix(linkURL.Hostname(), domain) {
			return true
		}
	}
	return false
}

// ExtractMainContent attempts to extract the main textual content from HTML.
// This is a simplistic approach; more sophisticated libraries like go-readability might be better.
func ExtractMainContent(doc *goquery.Document, contentTags []string) string {
	var contentBuilder strings.Builder

	if len(contentTags) > 0 {
		for _, tagSelector := range contentTags {
			doc.Find(tagSelector).Each(func(i int, s *goquery.Selection) {
				contentBuilder.WriteString(s.Text())
				contentBuilder.WriteString("\n")
			})
		}
	}

	// Fallback or alternative: extract text from common semantic tags
	if contentBuilder.Len() == 0 {
		doc.Find("article, main, section, p, h1, h2, h3").Each(func(i int, s *goquery.Selection) {
			// Avoid script and style tags if they are nested within these
			s.Find("script, style, nav, footer, aside, .adsbygoogle").Remove()
			text := strings.TrimSpace(s.Text())
			if len(text) > 50 { // Heuristic: only consider somewhat substantial text blocks
				contentBuilder.WriteString(text)
				contentBuilder.WriteString("\n\n")
			}
		})
	}

	// Basic cleaning: remove excessive newlines and whitespace
	cleanedContent := regexp.MustCompile(`\s{2,}`).ReplaceAllString(contentBuilder.String(), " ")
	cleanedContent = regexp.MustCompile(`\n{3,}`).ReplaceAllString(cleanedContent, "\n\n")
	return strings.TrimSpace(cleanedContent)
}

// FetchPage fetches the content of a URL.
func FetchPage(targetURL string, userAgent string) (*http.Response, error) {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 { // Limit redirects
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5") // You might want to make this configurable or detect

	return client.Do(req)
}

// NormalizeURL resolves a relative URL against a base URL.
func NormalizeURL(base *url.URL, relativePath string) (string, error) {
	relURL, err := url.Parse(relativePath)
	if err != nil {
		return "", err
	}
	absURL := base.ResolveReference(relURL)
	return absURL.String(), nil
}
