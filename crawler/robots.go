package crawler

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"

	"github.com/temoto/robotstxt"
)

var (
	robotsCache = make(map[string]*robotstxt.RobotsData)
	cacheMutex  = &sync.RWMutex{}
)

// GetRobotsData fetches and parses robots.txt for a given base URL.
// It uses a simple in-memory cache.
func GetRobotsData(baseURL *url.URL, userAgent string) (*robotstxt.RobotsData, error) {
	cacheMutex.RLock()
	data, found := robotsCache[baseURL.Host]
	cacheMutex.RUnlock()

	if found {
		return data, nil
	}

	robotsURL := baseURL.Scheme + "://" + baseURL.Host + "/robots.txt"
	log.Printf("Fetching robots.txt from: %s for agent: %s", robotsURL, userAgent)

	resp, err := FetchPage(robotsURL, userAgent) // Use our FetchPage to respect UA
	if err != nil {
		log.Printf("Error fetching robots.txt for %s: %v. Assuming allow all.", baseURL.Host, err)
		return robotstxt.FromStatusAndBytes(http.StatusOK, []byte("User-agent: *\nAllow: /"))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("robots.txt for %s returned status %d. Assuming allow all for this specific error.", baseURL.Host, resp.StatusCode)
		return robotstxt.FromStatusAndBytes(http.StatusOK, []byte("User-agent: *\nAllow: /"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading robots.txt body for %s: %v. Assuming allow all.", baseURL.Host, err)
		return robotstxt.FromStatusAndBytes(http.StatusOK, []byte("User-agent: *\nAllow: /"))
	}

	robotsData, err := robotstxt.FromBytes(body)
	if err != nil {
		log.Printf("Error parsing robots.txt for %s: %v. Assuming allow all.", baseURL.Host, err)
		return robotstxt.FromStatusAndBytes(http.StatusOK, []byte("User-agent: *\nAllow: /"))
	}

	cacheMutex.Lock()
	robotsCache[baseURL.Host] = robotsData
	cacheMutex.Unlock()

	return robotsData, nil
}

// IsAllowedByRobots checks if crawling a path is allowed by robots.txt.
func IsAllowedByRobots(targetURL *url.URL, userAgent string) bool {
	robotsData, err := GetRobotsData(targetURL, userAgent)
	if err != nil {
		log.Printf("Cannot determine robots.txt for %s, disallowing path %s: %v", targetURL.Host, targetURL.Path, err)
		return false
	}
	return robotsData.TestAgent(targetURL.Path, userAgent)
}
