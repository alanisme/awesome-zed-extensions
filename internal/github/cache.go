package github

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/alanisme/awesome-zed-extensions/internal/safefile"
)

const cacheVersion = 2

// CacheEntry holds a cached API response with a timestamp.
type CacheEntry struct {
	Info        *RepoInfo `json:"info"`
	ExtToml     []byte    `json:"ext_toml,omitempty"`
	FetchedAt   time.Time `json:"fetched_at"`
	RepoETag    string    `json:"repo_etag,omitempty"`
	TomlETag    string    `json:"toml_etag,omitempty"`
}

// cacheFile is the on-disk format with a schema version.
type cacheFile struct {
	Version int                    `json:"version"`
	Entries map[string]*CacheEntry `json:"entries"`
}

// Cache provides thread-safe, file-backed caching for GitHub API responses.
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	path    string
	ttl     time.Duration
	dirty   bool
}

// NewCache creates or loads a cache from the given file path.
func NewCache(path string, ttl time.Duration) *Cache {
	c := &Cache{
		entries: make(map[string]*CacheEntry),
		path:    path,
		ttl:     ttl,
	}
	c.load()
	return c
}

func cacheKey(owner, repo string) string {
	h := md5.Sum([]byte(fmt.Sprintf("%s/%s", owner, repo)))
	return fmt.Sprintf("%x", h)
}

// Get retrieves a cached entry if it exists and is not expired.
func (c *Cache) Get(owner, repo string) (*CacheEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := cacheKey(owner, repo)
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}

	if time.Since(entry.FetchedAt) > c.ttl {
		return nil, false
	}

	return entry, true
}

// Set stores a cache entry.
func (c *Cache) Set(owner, repo string, entry *CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := cacheKey(owner, repo)
	entry.FetchedAt = time.Now().UTC()
	c.entries[key] = entry
	c.dirty = true
}

// Save writes the cache to disk if there were changes.
func (c *Cache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.dirty {
		return nil
	}

	cf := cacheFile{
		Version: cacheVersion,
		Entries: c.entries,
	}
	data, err := json.Marshal(cf)
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}

	if err := safefile.WriteAtomic(c.path, data, 0644); err != nil {
		return err
	}

	c.dirty = false
	return nil
}

// GetStale retrieves a cached entry even if expired, for use with conditional requests.
func (c *Cache) GetStale(owner, repo string) *CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := cacheKey(owner, repo)
	entry, ok := c.entries[key]
	if !ok {
		return nil
	}
	return entry
}

// Stats returns cache hit statistics.
func (c *Cache) Stats() (total, valid int) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total = len(c.entries)
	for _, e := range c.entries {
		if time.Since(e.FetchedAt) <= c.ttl {
			valid++
		}
	}
	return
}

func (c *Cache) load() {
	data, err := os.ReadFile(c.path)
	if err != nil {
		return
	}

	// Try versioned format first.
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err == nil && cf.Version > 0 {
		if cf.Version != cacheVersion {
			// Incompatible version — start fresh.
			return
		}
		if cf.Entries != nil {
			c.entries = cf.Entries
		}
		return
	}

	// Fall back to legacy format (flat map without version).
	var entries map[string]*CacheEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}
	c.entries = entries
	c.dirty = true // Will re-save in new format.
}
