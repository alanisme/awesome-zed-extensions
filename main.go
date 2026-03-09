package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/alanisme/awesome-zed-extensions/internal/category"
	gh "github.com/alanisme/awesome-zed-extensions/internal/github"
	"github.com/alanisme/awesome-zed-extensions/internal/model"
	"github.com/alanisme/awesome-zed-extensions/internal/registry"
	"github.com/alanisme/awesome-zed-extensions/internal/render"
	"github.com/alanisme/awesome-zed-extensions/internal/safefile"
	"github.com/alanisme/awesome-zed-extensions/internal/trending"
)

const (
	historyPath    = "data/history.json"
	cachePath      = "data/cache.json"
	extensionsPath = "data/extensions.json"
	overridesPath  = "data/overrides.json"
	readmePath     = "README.md"
	staleDays      = 365
)

// config holds tunable parameters, overridable via environment variables.
type config struct {
	MaxWorkers     int
	CacheTTLHours  int
	DedicatedStars int
}

func loadConfig() config {
	return config{
		MaxWorkers:     envInt("MAX_WORKERS", 10),
		CacheTTLHours:  envInt("CACHE_TTL_HOURS", 20),
		DedicatedStars: envInt("DEDICATED_STARS_THRESHOLD", 5000),
	}
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func loadOverrides(path string) model.Overrides {
	data, err := os.ReadFile(path)
	if err != nil {
		return model.Overrides{
			Category:  make(map[string]string),
			Dedicated: make(map[string]bool),
		}
	}
	var o model.Overrides
	if err := json.Unmarshal(data, &o); err != nil {
		log.Printf("WARN: Failed to parse overrides: %v", err)
		return model.Overrides{
			Category:  make(map[string]string),
			Dedicated: make(map[string]bool),
		}
	}
	if o.Category == nil {
		o.Category = make(map[string]string)
	}
	if o.Dedicated == nil {
		o.Dedicated = make(map[string]bool)
	}
	return o
}

func applyOverrides(extensions []model.Extension, overrides model.Overrides) {
	for i := range extensions {
		ext := &extensions[i]
		if cat, ok := overrides.Category[ext.ID]; ok {
			ext.Category = cat
		}
		if ded, ok := overrides.Dedicated[ext.ID]; ok {
			ext.Dedicated = ded
		}
	}
}

func main() {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		log.Fatal("GITHUB_TOKEN environment variable is required")
	}

	cfg := loadConfig()
	log.Printf("Config: workers=%d, cacheTTL=%dh, dedicatedThreshold=%d stars",
		cfg.MaxWorkers, cfg.CacheTTLHours, cfg.DedicatedStars)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Graceful shutdown on SIGINT/SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("Received %v, shutting down gracefully...", sig)
		cancel()
	}()

	log.Println("Fetching extension registry...")
	httpClient := &http.Client{Timeout: 30 * time.Second}
	extensions, err := registry.FetchExtensions(ctx, httpClient)
	if err != nil {
		log.Fatalf("Failed to fetch extensions: %v", err)
	}
	log.Printf("Found %d extensions in registry", len(extensions))

	log.Println("Loading history...")
	history, err := trending.LoadHistory(historyPath)
	if err != nil {
		log.Fatalf("Failed to load history: %v", err)
	}

	cacheTTL := time.Duration(cfg.CacheTTLHours) * time.Hour

	log.Println("Fetching GitHub metadata...")
	client := gh.NewClient(token)
	cache := gh.NewCache(cachePath, cacheTTL)
	total, valid := cache.Stats()
	log.Printf("Cache: %d entries, %d valid", total, valid)
	results, stats := fetchAllMetadata(ctx, client, cache, extensions, cfg)

	log.Printf("Fetch stats: %d total, %d cache hits, %d 304 revalidated, %d fresh API calls, %d skipped",
		len(results), stats.cacheHits, stats.notModified, stats.apiFetches, stats.skipped)

	// Safety threshold: abort if less than 50% of extensions were fetched.
	ratio := float64(len(results)) / float64(len(extensions))
	if len(extensions) > 0 && ratio < 0.5 {
		log.Fatalf("Only %.0f%% of extensions fetched — aborting to protect existing data.", ratio*100)
	}

	// Apply manual overrides for edge cases.
	overrides := loadOverrides(overridesPath)
	if len(overrides.Category)+len(overrides.Dedicated) > 0 {
		log.Printf("Applying %d category + %d dedicated overrides",
			len(overrides.Category), len(overrides.Dedicated))
		applyOverrides(results, overrides)
	}

	log.Println("Saving cache...")
	if err := cache.Save(); err != nil {
		log.Printf("WARN: Failed to save cache: %v", err)
	}

	log.Println("Computing trends...")
	trendingExts := trending.ComputeTrending(results, history)
	recentlyAdded := trending.FindRecentlyAdded(results, history)

	log.Println("Saving history...")
	if err := trending.SaveHistory(historyPath, results, history); err != nil {
		log.Fatalf("Failed to save history: %v", err)
	}

	log.Println("Generating charts...")
	data := buildRenderData(results, trendingExts, recentlyAdded)
	if err := render.GenerateCategoryChart("assets/categories.svg", data); err != nil {
		log.Fatalf("Failed to generate category chart: %v", err)
	}
	if err := render.GenerateTopChart("assets/top-extensions.svg", data.TopExtensions); err != nil {
		log.Fatalf("Failed to generate top chart: %v", err)
	}

	log.Println("Generating README...")
	if err := render.GenerateREADME(readmePath, data); err != nil {
		log.Fatalf("Failed to generate README: %v", err)
	}

	log.Println("Exporting extensions.json...")
	if err := exportJSON(extensionsPath, data); err != nil {
		log.Printf("WARN: Failed to export extensions.json: %v", err)
	}

	apiCalls, _ := client.Stats()
	log.Printf("Done! %d extensions, %d API calls.", data.TotalCount, apiCalls)
}

// exportJSON writes the aggregated extension data to a JSON file for reuse.
func exportJSON(path string, data model.RenderData) error {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return safefile.WriteAtomic(path, out, 0644)
}

// fetchStats tracks observability counters for the metadata fetch phase.
type fetchStats struct {
	cacheHits   int
	notModified int
	apiFetches  int
	skipped     int
}

func fetchAllMetadata(ctx context.Context, client *gh.Client, cache *gh.Cache, extensions []model.Extension, cfg config) ([]model.Extension, fetchStats) {
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		sem     = make(chan struct{}, cfg.MaxWorkers)
		results []model.Extension
		stats   fetchStats
	)

	for _, ext := range extensions {
		wg.Add(1)
		go func(ext model.Extension) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, source := processExtension(ctx, client, cache, ext, cfg)
			mu.Lock()
			if result != nil {
				results = append(results, *result)
			}
			switch source {
			case sourceFreshCache:
				stats.cacheHits++
			case sourceNotModified:
				stats.notModified++
			case sourceAPI:
				stats.apiFetches++
			default:
				stats.skipped++
			}
			mu.Unlock()
		}(ext)
	}

	wg.Wait()
	return results, stats
}

// fetchSource indicates how an extension's metadata was obtained.
type fetchSource int

const (
	sourceSkipped     fetchSource = iota
	sourceFreshCache
	sourceNotModified
	sourceAPI
)

func processExtension(ctx context.Context, client *gh.Client, cache *gh.Cache, ext model.Extension, cfg config) (*model.Extension, fetchSource) {
	// Skip extensions that point to the main Zed repository (built-in extensions).
	if ext.Owner == "zed-industries" && ext.Repo == "zed" {
		return nil, sourceSkipped
	}

	// Try fresh cache first (within TTL).
	if entry, ok := cache.Get(ext.Owner, ext.Repo); ok && entry.Info != nil {
		applyEntry(&ext, entry, cfg)
		return &ext, sourceFreshCache
	}

	// Try conditional request with stale cache entry's ETag.
	stale := cache.GetStale(ext.Owner, ext.Repo)
	repoETag := ""
	if stale != nil {
		repoETag = stale.RepoETag
	}

	// Fetch from GitHub API (with conditional request if we have an ETag).
	info, newRepoETag, err := client.GetRepoInfo(ctx, ext.Owner, ext.Repo, repoETag)
	if err != nil {
		log.Printf("  WARN: %s/%s: %v", ext.Owner, ext.Repo, err)
		return nil, sourceSkipped
	}

	// 304 Not Modified — reuse stale cache data, refresh TTL.
	if info == nil && stale != nil && stale.Info != nil && repoETag != "" {
		stale.RepoETag = newRepoETag
		cache.Set(ext.Owner, ext.Repo, stale)
		applyEntry(&ext, stale, cfg)
		return &ext, sourceNotModified
	}
	if info == nil {
		return nil, sourceSkipped
	}

	ext.Stars = info.Stars
	ext.Description = info.Description
	ext.CreatedAt = info.CreatedAt
	ext.PushedAt = info.PushedAt
	ext.Archived = info.Archived
	ext.Maintained = !info.Archived && time.Since(info.PushedAt) < staleDays*24*time.Hour
	if info.License != nil {
		ext.License = info.License.SPDX
	}
	ext.Name = formatName(ext.ID)

	// Fetch extension.toml with conditional request.
	tomlETag := ""
	if stale != nil {
		tomlETag = stale.TomlETag
	}
	extToml, newTomlETag, err := client.GetRawFile(ctx, ext.Owner, ext.Repo, info.DefaultBranch, "extension.toml", tomlETag)
	if err != nil {
		log.Printf("  WARN: %s: failed to fetch extension.toml: %v", ext.ID, err)
	}

	// 304 for extension.toml — reuse stale toml data.
	if extToml == nil && stale != nil && stale.ExtToml != nil && tomlETag != "" {
		extToml = stale.ExtToml
	}

	// Override name/description from extension.toml if available.
	if extToml != nil {
		parseName, parseDesc := parseExtensionToml(extToml)
		if parseName != "" {
			ext.Name = parseName
		}
		if parseDesc != "" && ext.Description == "" {
			ext.Description = parseDesc
		}
	}

	ext.Category = category.Classify(extToml, info.Topics)
	ext.Dedicated = isDedicatedZedRepo(info, ext.Owner, ext.Repo, cfg.DedicatedStars)

	// Save to cache with ETags.
	cache.Set(ext.Owner, ext.Repo, &gh.CacheEntry{
		Info:     info,
		ExtToml:  extToml,
		RepoETag: newRepoETag,
		TomlETag: newTomlETag,
	})

	return &ext, sourceAPI
}

func applyEntry(ext *model.Extension, entry *gh.CacheEntry, cfg config) {
	info := entry.Info
	ext.Stars = info.Stars
	ext.Description = info.Description
	ext.CreatedAt = info.CreatedAt
	ext.PushedAt = info.PushedAt
	ext.Archived = info.Archived
	ext.Maintained = !info.Archived && time.Since(info.PushedAt) < staleDays*24*time.Hour
	if info.License != nil {
		ext.License = info.License.SPDX
	}
	ext.Name = formatName(ext.ID)

	if entry.ExtToml != nil {
		parseName, parseDesc := parseExtensionToml(entry.ExtToml)
		if parseName != "" {
			ext.Name = parseName
		}
		if parseDesc != "" && ext.Description == "" {
			ext.Description = parseDesc
		}
	}

	ext.Category = category.Classify(entry.ExtToml, info.Topics)
	ext.Dedicated = isDedicatedZedRepo(info, ext.Owner, ext.Repo, cfg.DedicatedStars)
}

// extensionMeta holds the top-level fields we care about from extension.toml.
type extensionMeta struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
}

func parseExtensionToml(data []byte) (name, description string) {
	var meta extensionMeta
	if err := toml.Unmarshal(data, &meta); err != nil {
		return "", ""
	}
	return meta.Name, meta.Description
}

// isDedicatedZedRepo checks if a repo is specifically built for Zed.
// Large repos (>5000 stars) that don't mention "zed" in their name,
// description, or topics are likely general-purpose projects that
// happen to include a Zed extension.
func isDedicatedZedRepo(info *gh.RepoInfo, owner, repo string, threshold int) bool {
	// Repos from zed-extensions org are always dedicated.
	if owner == "zed-extensions" || owner == "zed-industries" {
		return true
	}

	// Small repos are likely dedicated Zed extensions.
	if info.Stars < threshold {
		return true
	}

	// Check if "zed" appears in repo name, description, or topics.
	lower := strings.ToLower(repo)
	if strings.Contains(lower, "zed") {
		return true
	}
	if strings.Contains(strings.ToLower(info.Description), "zed") {
		return true
	}
	for _, t := range info.Topics {
		if strings.Contains(strings.ToLower(t), "zed") {
			return true
		}
	}

	return false
}

func formatName(id string) string {
	words := strings.Split(id, "-")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func buildRenderData(
	all []model.Extension,
	trendingExts []model.TrendingExtension,
	recentlyAdded []model.Extension,
) model.RenderData {
	// Separate dedicated Zed extensions from non-dedicated (large parent projects).
	var dedicated []model.Extension
	for _, ext := range all {
		if ext.Dedicated {
			dedicated = append(dedicated, ext)
		}
	}

	// Sort dedicated extensions by stars descending, then by ID for stability.
	sort.Slice(dedicated, func(i, j int) bool {
		if dedicated[i].Stars != dedicated[j].Stars {
			return dedicated[i].Stars > dedicated[j].Stars
		}
		return dedicated[i].ID < dedicated[j].ID
	})

	// Top 50 from dedicated extensions only (excludes inflated star counts).
	top := dedicated
	if len(top) > 50 {
		top = top[:50]
	}

	// Split by category (dedicated only, already sorted by stars).
	var themes, languages, tools, others []model.Extension
	for _, ext := range dedicated {
		switch ext.Category {
		case "Theme":
			themes = append(themes, ext)
		case "Language":
			languages = append(languages, ext)
		case "Tool":
			tools = append(tools, ext)
		default:
			others = append(others, ext)
		}
	}

	return model.RenderData{
		TopExtensions: top,
		Trending:      trendingExts,
		RecentlyAdded: recentlyAdded,
		Themes:        themes,
		Languages:     languages,
		Tools:         tools,
		Others:        others,
		TotalCount:    len(dedicated),
		UpdatedAt:     time.Now().UTC().Format("2006-01-02"),
	}
}
