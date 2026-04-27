package redpen

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/valyala/fasthttp"
)

// GhClient is an authenticated GitHub API client backed by fasthttp.
type GhClient struct {
	Token  string
	client *fasthttp.Client
}

// NewGhClient creates a client with sane timeouts.
func NewGhClient(token string) *GhClient {
	return &GhClient{
		Token: token,
		client: &fasthttp.Client{
			ReadTimeout:         90 * time.Second,
			WriteTimeout:        30 * time.Second,
			MaxConnsPerHost:     16,
			MaxIdleConnDuration: 30 * time.Second,
		},
	}
}

const (
	apiBase    = "https://api.github.com"
	maxRetries = 5
)

// get performs a GET with retry-on-rate-limit. Returns body bytes and Link header.
func (c *GhClient) get(url string) (body []byte, linkHeader string, err error) {
	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(url)
	req.Header.SetMethod(fasthttp.MethodGet)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		resp.Reset()
		if doErr := c.client.Do(req, resp); doErr != nil {
			return nil, "", fmt.Errorf("GET %s: %w", url, doErr)
		}

		sc := resp.StatusCode()
		// copy before resp is reused on next iteration
		b := make([]byte, len(resp.Body()))
		copy(b, resp.Body())
		link := string(resp.Header.Peek("Link"))

		switch sc {
		case fasthttp.StatusOK:
			return b, link, nil
		case fasthttp.StatusTooManyRequests, fasthttp.StatusForbidden:
			wait := rateLimitBackoff(resp)
			log.Warn().
				Int("attempt", attempt+1).
				Int("max", maxRetries).
				Str("wait", wait.Round(time.Second).String()).
				Str("url", url).
				Msg("rate limited, waiting")
			time.Sleep(wait)
		default:
			return nil, "", fmt.Errorf("HTTP %d %s: %.200s", sc, url, b)
		}
	}
	return nil, "", fmt.Errorf("exceeded %d retries: %s", maxRetries, url)
}

func rateLimitBackoff(resp *fasthttp.Response) time.Duration {
	if ra := string(resp.Header.Peek("Retry-After")); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			return time.Duration(secs) * time.Second
		}
	}
	if reset := string(resp.Header.Peek("X-RateLimit-Reset")); reset != "" {
		if ts, err := strconv.ParseInt(reset, 10, 64); err == nil {
			if until := time.Until(time.Unix(ts, 0)); until > 0 {
				return until + 2*time.Second
			}
		}
	}
	return 60 * time.Second
}

var nextLinkRe = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

func nextURL(linkHeader string) string {
	if m := nextLinkRe.FindStringSubmatch(linkHeader); len(m) == 2 {
		return m[1]
	}
	return ""
}

// FetchAll paginates through all pages of a GitHub API endpoint returning a JSON array.
func FetchAll[T any](c *GhClient, initialURL string) ([]T, error) {
	var all []T
	url := initialURL
	for url != "" {
		b, link, err := c.get(url)
		if err != nil {
			return nil, err
		}
		var page []T
		if err := json.Unmarshal(b, &page); err != nil {
			return nil, fmt.Errorf("unmarshal %s: %w (body: %.200s)", url, err, b)
		}
		all = append(all, page...)
		url = nextURL(link)
		if url != "" {
			time.Sleep(80 * time.Millisecond)
		}
	}
	return all, nil
}

// RateLimitResource holds per-resource GitHub rate limit counters.
type RateLimitResource struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	Reset     int64 `json:"reset"`
	Used      int   `json:"used"`
}

// RateLimitInfo holds rate limit info for all resource types.
type RateLimitInfo struct {
	Core   RateLimitResource `json:"core"`
	Search RateLimitResource `json:"search"`
}

type rateLimitResponse struct {
	Resources RateLimitInfo `json:"resources"`
}

// FetchRateLimit returns the current GitHub API rate limit status.
// This call does not consume any rate limit budget.
func FetchRateLimit(c *GhClient) (RateLimitInfo, error) {
	b, _, err := c.get(apiBase + "/rate_limit")
	if err != nil {
		return RateLimitInfo{}, fmt.Errorf("rate limit: %w", err)
	}
	var r rateLimitResponse
	if err := json.Unmarshal(b, &r); err != nil {
		return RateLimitInfo{}, fmt.Errorf("parse rate limit: %w", err)
	}
	return r.Resources, nil
}
