package redpen

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"time"
)

type GhClient struct {
	Token      string
	HTTPClient *http.Client
}

func NewGhClient(token string) *GhClient {
	return &GhClient{
		Token:      token,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

const apiBase = "https://api.github.com"

func (c *GhClient) Get(url string) ([]byte, http.Header, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	for attempt := 0; attempt < 5; attempt++ {
		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("HTTP request failed: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, nil, fmt.Errorf("reading response body: %w", readErr)
		}

		switch resp.StatusCode {
		case http.StatusOK:
			return body, resp.Header, nil
		case http.StatusTooManyRequests, http.StatusForbidden:
			wait := 60 * time.Second
			if ra := resp.Header.Get("Retry-After"); ra != "" {
				if secs, err := strconv.Atoi(ra); err == nil {
					wait = time.Duration(secs) * time.Second
				}
			}
			// Also check X-RateLimit-Reset
			if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
				if ts, err := strconv.ParseInt(reset, 10, 64); err == nil {
					resetTime := time.Unix(ts, 0)
					until := time.Until(resetTime)
					if until > 0 {
						wait = until + 2*time.Second
					}
				}
			}
			log.Printf("  Rate limited (attempt %d/5), waiting %v...", attempt+1, wait.Round(time.Second))
			time.Sleep(wait)
			continue
		default:
			return nil, nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, url, string(body))
		}
	}
	return nil, nil, fmt.Errorf("exceeded max retries for %s", url)
}

// nextLinkRe extracts the "next" page URL from a Link header.
var nextLinkRe = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

func nextURL(linkHeader string) string {
	if m := nextLinkRe.FindStringSubmatch(linkHeader); len(m) == 2 {
		return m[1]
	}
	return ""
}

// FetchAll paginates through all pages of a GitHub API endpoint.
func FetchAll[T any](c *GhClient, initialURL string) ([]T, error) {
	var all []T
	url := initialURL
	for url != "" {
		body, headers, err := c.Get(url)
		if err != nil {
			return nil, err
		}
		var page []T
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("unmarshal %s: %w\nbody: %.200s", url, err, string(body))
		}
		all = append(all, page...)
		url = nextURL(headers.Get("Link"))
		if url != "" {
			time.Sleep(80 * time.Millisecond) // be polite to the API
		}
	}
	return all, nil
}
