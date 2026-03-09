package trending

import (
	"encoding/json"
	"os"
	"sort"
	"time"

	"github.com/alanisme/awesome-zed-extensions/internal/model"
	"github.com/alanisme/awesome-zed-extensions/internal/safefile"
)

const retentionDays = 90

// LoadHistory reads the history file from disk.
// Returns an empty history if the file does not exist.
func LoadHistory(path string) (*model.HistoryFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &model.HistoryFile{
				Extensions: make(map[string][]model.HistorySnapshot),
			}, nil
		}
		return nil, err
	}

	var h model.HistoryFile
	if err := json.Unmarshal(data, &h); err != nil {
		return &model.HistoryFile{
			Extensions: make(map[string][]model.HistorySnapshot),
		}, nil
	}

	if h.Extensions == nil {
		h.Extensions = make(map[string][]model.HistorySnapshot)
	}

	return &h, nil
}

// SaveHistory writes the current scan data to the history file.
// Deduplicates by date (only one snapshot per calendar day per extension)
// and prunes entries older than the retention period.
func SaveHistory(path string, extensions []model.Extension, history *model.HistoryFile) error {
	now := time.Now().UTC()
	today := now.Format("2006-01-02")
	cutoff := now.AddDate(0, 0, -retentionDays)

	for _, ext := range extensions {
		snapshots := history.Extensions[ext.ID]

		// Prune old entries and remove any existing entry for today.
		var pruned []model.HistorySnapshot
		for _, s := range snapshots {
			if s.ScannedAt.Before(cutoff) {
				continue
			}
			if s.ScannedAt.Format("2006-01-02") == today {
				continue
			}
			pruned = append(pruned, s)
		}

		// Append today's snapshot.
		pruned = append(pruned, model.HistorySnapshot{
			Stars:     ext.Stars,
			ScannedAt: now,
		})

		history.Extensions[ext.ID] = pruned
	}

	history.LastUpdated = now

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return safefile.WriteAtomic(path, data, 0644)
}

// ComputeTrending calculates the fastest-growing extensions over the past 7 days.
// Only uses snapshots strictly within the 7-day window; does not fall back to
// older data, which would produce misleading "weekly" growth numbers.
func ComputeTrending(extensions []model.Extension, history *model.HistoryFile) []model.TrendingExtension {
	weekAgo := time.Now().UTC().AddDate(0, 0, -7)

	var trending []model.TrendingExtension
	for _, ext := range extensions {
		snapshots := history.Extensions[ext.ID]
		if len(snapshots) == 0 {
			continue
		}

		// Find the oldest snapshot within the 7-day window.
		var oldestInWindow *model.HistorySnapshot
		for i := range snapshots {
			s := &snapshots[i]
			if s.ScannedAt.Before(weekAgo) {
				continue
			}
			if oldestInWindow == nil || s.ScannedAt.Before(oldestInWindow.ScannedAt) {
				oldestInWindow = s
			}
		}

		// No data within the window — skip (don't fall back to stale data).
		if oldestInWindow == nil {
			continue
		}

		growth := ext.Stars - oldestInWindow.Stars
		if growth > 0 {
			trending = append(trending, model.TrendingExtension{
				Extension: ext,
				Growth:    growth,
			})
		}
	}

	sort.Slice(trending, func(i, j int) bool {
		if trending[i].Growth != trending[j].Growth {
			return trending[i].Growth > trending[j].Growth
		}
		return trending[i].ID < trending[j].ID
	})

	if len(trending) > 20 {
		trending = trending[:20]
	}

	return trending
}

// FindRecentlyAdded returns extensions first seen in the history within the last 30 days.
func FindRecentlyAdded(extensions []model.Extension, history *model.HistoryFile) []model.Extension {
	monthAgo := time.Now().UTC().AddDate(0, 0, -30)

	var recent []model.Extension
	for _, ext := range extensions {
		snapshots := history.Extensions[ext.ID]

		// If no history at all, it's new.
		if len(snapshots) == 0 {
			recent = append(recent, ext)
			continue
		}

		// Find the earliest snapshot.
		earliest := snapshots[0].ScannedAt
		for _, s := range snapshots[1:] {
			if s.ScannedAt.Before(earliest) {
				earliest = s.ScannedAt
			}
		}

		if earliest.After(monthAgo) {
			recent = append(recent, ext)
		}
	}

	sort.Slice(recent, func(i, j int) bool {
		if !recent[i].CreatedAt.Equal(recent[j].CreatedAt) {
			return recent[i].CreatedAt.After(recent[j].CreatedAt)
		}
		return recent[i].ID < recent[j].ID
	})

	if len(recent) > 30 {
		recent = recent[:30]
	}

	return recent
}
