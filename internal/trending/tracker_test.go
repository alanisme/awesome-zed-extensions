package trending

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alanisme/awesome-zed-extensions/internal/model"
)

func TestSaveHistory_DeduplicatesSameDay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	history := &model.HistoryFile{
		Extensions: make(map[string][]model.HistorySnapshot),
	}

	exts := []model.Extension{
		{ID: "test-ext", Stars: 100},
	}

	// Save twice on the same day.
	if err := SaveHistory(path, exts, history); err != nil {
		t.Fatal(err)
	}

	// Reload and save again with different stars.
	history2, err := LoadHistory(path)
	if err != nil {
		t.Fatal(err)
	}

	exts[0].Stars = 110
	if err := SaveHistory(path, exts, history2); err != nil {
		t.Fatal(err)
	}

	// Reload and verify only one entry for today.
	history3, err := LoadHistory(path)
	if err != nil {
		t.Fatal(err)
	}

	snapshots := history3.Extensions["test-ext"]
	today := time.Now().UTC().Format("2006-01-02")
	count := 0
	for _, s := range snapshots {
		if s.ScannedAt.Format("2006-01-02") == today {
			count++
		}
	}

	if count != 1 {
		t.Errorf("expected 1 snapshot for today, got %d", count)
	}

	// Should have the latest stars value.
	last := snapshots[len(snapshots)-1]
	if last.Stars != 110 {
		t.Errorf("expected stars=110, got %d", last.Stars)
	}
}

func TestSaveHistory_PrunesOldEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	old := time.Now().UTC().AddDate(0, 0, -100)
	history := &model.HistoryFile{
		Extensions: map[string][]model.HistorySnapshot{
			"test-ext": {
				{Stars: 50, ScannedAt: old},
			},
		},
	}

	exts := []model.Extension{
		{ID: "test-ext", Stars: 100},
	}

	if err := SaveHistory(path, exts, history); err != nil {
		t.Fatal(err)
	}

	history2, err := LoadHistory(path)
	if err != nil {
		t.Fatal(err)
	}

	// Old entry should be pruned, only today's remains.
	snapshots := history2.Extensions["test-ext"]
	if len(snapshots) != 1 {
		t.Errorf("expected 1 snapshot after pruning, got %d", len(snapshots))
	}
}

func TestComputeTrending_StrictWeekWindow(t *testing.T) {
	now := time.Now().UTC()
	threeDaysAgo := now.AddDate(0, 0, -3)
	tenDaysAgo := now.AddDate(0, 0, -10)

	history := &model.HistoryFile{
		Extensions: map[string][]model.HistorySnapshot{
			"growing": {
				{Stars: 80, ScannedAt: threeDaysAgo},
			},
			"old-only": {
				{Stars: 50, ScannedAt: tenDaysAgo},
			},
		},
	}

	exts := []model.Extension{
		{ID: "growing", Stars: 100},
		{ID: "old-only", Stars: 200},
	}

	trending := ComputeTrending(exts, history)

	// "growing" should appear with growth=20.
	if len(trending) != 1 {
		t.Fatalf("expected 1 trending, got %d", len(trending))
	}
	if trending[0].ID != "growing" {
		t.Errorf("expected 'growing', got %s", trending[0].ID)
	}
	if trending[0].Growth != 20 {
		t.Errorf("expected growth=20, got %d", trending[0].Growth)
	}
}

func TestComputeTrending_NoHistory(t *testing.T) {
	history := &model.HistoryFile{
		Extensions: make(map[string][]model.HistorySnapshot),
	}
	exts := []model.Extension{
		{ID: "new-ext", Stars: 50},
	}

	trending := ComputeTrending(exts, history)
	if len(trending) != 0 {
		t.Errorf("expected 0 trending for new ext, got %d", len(trending))
	}
}

func TestLoadHistory_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.json")
	h, err := LoadHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if h.Extensions == nil {
		t.Error("expected initialized map")
	}
}

func TestLoadHistory_CorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, []byte("not json{{{"), 0644)

	h, err := LoadHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if h.Extensions == nil {
		t.Error("expected initialized map on corrupt file")
	}
}

func TestFindRecentlyAdded(t *testing.T) {
	now := time.Now().UTC()

	history := &model.HistoryFile{
		Extensions: map[string][]model.HistorySnapshot{
			"old-ext": {
				{Stars: 50, ScannedAt: now.AddDate(0, 0, -60)},
			},
			"new-ext": {
				{Stars: 10, ScannedAt: now.AddDate(0, 0, -5)},
			},
		},
	}

	exts := []model.Extension{
		{ID: "old-ext", Stars: 100},
		{ID: "new-ext", Stars: 20},
		{ID: "brand-new", Stars: 5},
	}

	recent := FindRecentlyAdded(exts, history)

	ids := make(map[string]bool)
	for _, r := range recent {
		ids[r.ID] = true
	}

	if !ids["new-ext"] {
		t.Error("expected new-ext in recently added")
	}
	if !ids["brand-new"] {
		t.Error("expected brand-new in recently added")
	}
	if ids["old-ext"] {
		t.Error("old-ext should not be in recently added")
	}
}
