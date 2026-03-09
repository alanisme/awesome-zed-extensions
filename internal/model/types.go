package model

import "time"

// Extension holds metadata for a single Zed extension.
type Extension struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	RepoURL     string    `json:"repo_url"`
	Owner       string    `json:"owner"`
	Repo        string    `json:"repo"`
	Stars       int       `json:"stars"`
	Category    string    `json:"category"`
	Dedicated   bool      `json:"dedicated"`
	Archived    bool      `json:"archived,omitempty"`
	License     string    `json:"license,omitempty"`
	Maintained  bool      `json:"maintained"`
	Authors     []string  `json:"authors,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	PushedAt    time.Time `json:"pushed_at"`
}

// Overrides holds manual corrections for extensions that automated
// rules cannot classify correctly.
type Overrides struct {
	Category  map[string]string `json:"category"`
	Dedicated map[string]bool   `json:"dedicated"`
}

// HistorySnapshot stores a point-in-time record for an extension.
type HistorySnapshot struct {
	Stars     int       `json:"stars"`
	ScannedAt time.Time `json:"scanned_at"`
}

// HistoryFile is the persistent storage format for trend tracking.
type HistoryFile struct {
	LastUpdated time.Time                      `json:"last_updated"`
	Extensions  map[string][]HistorySnapshot   `json:"extensions"`
}

// TrendingExtension pairs an extension with its recent star growth.
type TrendingExtension struct {
	Extension
	Growth int `json:"growth"`
}

// RenderData holds all data needed to generate the README.
type RenderData struct {
	TopExtensions  []Extension
	Trending       []TrendingExtension
	RecentlyAdded  []Extension
	Themes         []Extension
	Languages      []Extension
	Tools          []Extension
	Others         []Extension
	TotalCount     int
	UpdatedAt      string
}
