package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

// RepoInfo holds metadata fetched from the GitHub API.
type RepoInfo struct {
	Stars         int       `json:"stargazers_count"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	PushedAt      time.Time `json:"pushed_at"`
	Topics        []string  `json:"topics"`
	Archived      bool      `json:"archived"`
	Fork          bool      `json:"fork"`
	DefaultBranch string    `json:"default_branch"`
	License       *struct {
		SPDX string `json:"spdx_id"`
	} `json:"license"`
}

// Client is a rate-limited GitHub API client with retry and rate-limit awareness.
type Client struct {
	http       *http.Client
	token      string
	limiter    *rate.Limiter
	retries    int
	apiCalls   atomic.Int64
	cacheHits  atomic.Int64
}

// NewClient creates a Client with optimized HTTP transport, rate limiting,
// and exponential backoff retry.
func NewClient(token string) *Client {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
	}

	return &Client{
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		token:   token,
		limiter: rate.NewLimiter(rate.Every(125*time.Millisecond), 2),
		retries: 3,
	}
}

// Stats returns API call statistics.
func (c *Client) Stats() (apiCalls, cacheHits int64) {
	return c.apiCalls.Load(), c.cacheHits.Load()
}

// FetchResult holds the result of a conditional fetch.
type FetchResult struct {
	Body       []byte
	StatusCode int
	ETag       string
	NotModified bool
}

// GetRepoInfo fetches repository metadata from the GitHub API.
// If etag is provided, sends If-None-Match for conditional request.
// Returns nil without error if the repo is not found (404).
func (c *Client) GetRepoInfo(ctx context.Context, owner, repo, etag string) (*RepoInfo, string, error) {
	if !isValidName(owner) || !isValidName(repo) {
		return nil, "", fmt.Errorf("invalid owner/repo: %s/%s", owner, repo)
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	return c.getRepoInfoURL(ctx, url, etag)
}

func (c *Client) getRepoInfoURL(ctx context.Context, url, etag string) (*RepoInfo, string, error) {
	result, err := c.doWithRetry(ctx, url, etag)
	if err != nil {
		return nil, "", err
	}
	if result.NotModified {
		return nil, result.ETag, nil
	}
	if result.StatusCode == http.StatusNotFound {
		return nil, "", nil
	}
	if result.Body == nil {
		return nil, "", fmt.Errorf("GitHub API %s: status %d", url, result.StatusCode)
	}

	var info RepoInfo
	if err := json.Unmarshal(result.Body, &info); err != nil {
		return nil, "", fmt.Errorf("decode repo info: %w", err)
	}

	return &info, result.ETag, nil
}

// GetRawFile fetches a raw file from a GitHub repository using the given branch.
// If etag is provided, sends If-None-Match for conditional request.
// Returns nil without error if not found.
func (c *Client) GetRawFile(ctx context.Context, owner, repo, branch, path, etag string) ([]byte, string, error) {
	if !isValidName(owner) || !isValidName(repo) {
		return nil, "", fmt.Errorf("invalid owner/repo: %s/%s", owner, repo)
	}
	if branch == "" {
		branch = "main"
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, branch, path)
	return c.getRawFileURL(ctx, url, etag)
}

func (c *Client) getRawFileURL(ctx context.Context, url, etag string) ([]byte, string, error) {
	result, err := c.doWithRetry(ctx, url, etag)
	if err != nil {
		return nil, "", err
	}
	if result.NotModified {
		return nil, result.ETag, nil
	}
	if result.StatusCode == http.StatusOK && result.Body != nil {
		return result.Body, result.ETag, nil
	}

	return nil, "", nil
}

// doWithRetry performs an HTTP GET with exponential backoff retry.
// Supports conditional requests via ETag (If-None-Match / 304).
// Handles rate limiting (403/429) by respecting Retry-After and X-RateLimit-Reset headers.
func (c *Client) doWithRetry(ctx context.Context, url, etag string) (*FetchResult, error) {
	var lastErr error

	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		if err := c.limiter.Wait(ctx); err != nil {
			return nil, err
		}

		c.apiCalls.Add(1)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		c.setHeaders(req)
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue // Retry on network errors.
		}

		respETag := resp.Header.Get("ETag")

		// Handle 304 Not Modified — cache is still valid.
		if resp.StatusCode == http.StatusNotModified {
			resp.Body.Close()
			return &FetchResult{
				StatusCode:  304,
				ETag:        respETag,
				NotModified: true,
			}, nil
		}

		// Limit response body to 10MB to prevent OOM.
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}

		switch {
		case resp.StatusCode == http.StatusNotFound:
			return &FetchResult{StatusCode: http.StatusNotFound}, nil

		case resp.StatusCode == http.StatusOK:
			return &FetchResult{
				Body:       body,
				StatusCode: http.StatusOK,
				ETag:       respETag,
			}, nil

		case resp.StatusCode == 429 || resp.StatusCode == 403:
			waitDur := c.parseRateLimitWait(resp)
			log.Printf("  Rate limited on %s, waiting %v", url, waitDur)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitDur):
			}
			lastErr = fmt.Errorf("rate limited: status %d", resp.StatusCode)
			continue

		case resp.StatusCode >= 500:
			lastErr = fmt.Errorf("server error: status %d", resp.StatusCode)
			continue

		default:
			return &FetchResult{StatusCode: resp.StatusCode}, nil
		}
	}

	return nil, fmt.Errorf("after %d retries: %w", c.retries, lastErr)
}

// parseRateLimitWait extracts wait duration from GitHub rate limit headers.
func (c *Client) parseRateLimitWait(resp *http.Response) time.Duration {
	// Check Retry-After header first.
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			return time.Duration(secs) * time.Second
		}
	}

	// Check X-RateLimit-Reset header.
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if ts, err := strconv.ParseInt(reset, 10, 64); err == nil {
			waitUntil := time.Unix(ts, 0)
			dur := time.Until(waitUntil)
			if dur > 0 && dur < 15*time.Minute {
				return dur
			}
		}
	}

	return 30 * time.Second
}

func (c *Client) setHeaders(req *http.Request) {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "awesome-zed-extensions")
}

// isValidName checks that a GitHub owner or repo name contains only
// safe characters (alphanumeric, hyphen, underscore, dot).
func isValidName(name string) bool {
	if name == "" || len(name) > 100 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}
