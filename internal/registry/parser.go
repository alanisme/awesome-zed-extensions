package registry

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/alanisme/awesome-zed-extensions/internal/model"
)

const (
	extensionsTomlURL = "https://raw.githubusercontent.com/zed-industries/extensions/main/extensions.toml"
	gitmodulesURL     = "https://raw.githubusercontent.com/zed-industries/extensions/main/.gitmodules"
	maxRetries        = 3
)

type extensionEntry struct {
	Submodule string `toml:"submodule"`
	Version   string `toml:"version"`
}

// FetchExtensions fetches and parses extensions.toml and .gitmodules from
// the zed-industries/extensions repository, returning a joined list.
func FetchExtensions(ctx context.Context, httpClient *http.Client) ([]model.Extension, error) {
	entries, err := fetchExtensionsToml(ctx, httpClient)
	if err != nil {
		return nil, fmt.Errorf("fetch extensions.toml: %w", err)
	}

	modules, err := fetchGitmodules(ctx, httpClient)
	if err != nil {
		return nil, fmt.Errorf("fetch .gitmodules: %w", err)
	}

	var extensions []model.Extension
	for id, entry := range entries {
		repoURL, ok := modules[entry.Submodule]
		if !ok {
			continue
		}

		owner, repo := parseGitHubURL(repoURL)
		if owner == "" || repo == "" {
			continue
		}

		extensions = append(extensions, model.Extension{
			ID:      id,
			Version: entry.Version,
			RepoURL: repoURL,
			Owner:   owner,
			Repo:    repo,
		})
	}

	return extensions, nil
}

func fetchWithRetry(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			log.Printf("  Retry %d/%d for %s (waiting %v)", attempt, maxRetries, url, backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode == http.StatusOK {
			return body, nil
		}

		lastErr = fmt.Errorf("unexpected status %d", resp.StatusCode)
		if resp.StatusCode >= 500 {
			continue
		}
		return nil, lastErr
	}

	return nil, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

func fetchExtensionsToml(ctx context.Context, client *http.Client) (map[string]extensionEntry, error) {
	body, err := fetchWithRetry(ctx, client, extensionsTomlURL)
	if err != nil {
		return nil, err
	}

	var entries map[string]extensionEntry
	if err := toml.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("parse TOML: %w", err)
	}

	return entries, nil
}

func fetchGitmodules(ctx context.Context, client *http.Client) (map[string]string, error) {
	body, err := fetchWithRetry(ctx, client, gitmodulesURL)
	if err != nil {
		return nil, err
	}

	return parseGitmodules(strings.NewReader(string(body)))
}

func parseGitmodules(r io.Reader) (map[string]string, error) {
	modules := make(map[string]string)
	scanner := bufio.NewScanner(r)

	var currentPath string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "path = ") {
			currentPath = strings.TrimPrefix(line, "path = ")
		} else if strings.HasPrefix(line, "url = ") && currentPath != "" {
			url := strings.TrimPrefix(line, "url = ")
			url = strings.TrimSuffix(url, ".git")
			modules[currentPath] = url
			currentPath = ""
		}
	}

	return modules, scanner.Err()
}

func parseGitHubURL(rawURL string) (owner, repo string) {
	rawURL = strings.TrimSuffix(rawURL, ".git")

	for _, prefix := range []string{
		"https://github.com/",
		"http://github.com/",
	} {
		if strings.HasPrefix(rawURL, prefix) {
			rawURL = strings.TrimPrefix(rawURL, prefix)
			parts := strings.SplitN(rawURL, "/", 3)
			if len(parts) >= 2 {
				return parts[0], parts[1]
			}
		}
	}

	return "", ""
}
