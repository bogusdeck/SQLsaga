package database

import (
	"context"
	"os"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { os.Remove(dbPath); store.Close() })
	return store
}

func TestStore_OpenAndClose(t *testing.T) {
	store := newTestStore(t)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if err := store.Close(); err != nil {
		t.Errorf("close failed: %v", err)
	}
}

func TestStore_SaveAttempt(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	p := Progress{
		UserID:      "user1",
		StoryID:     "story1",
		ChapterID:   "ch1",
		ChallengeID: "c1",
		Completed:   true,
		Attempts:    3,
		BestTime:    45.5,
		Score:       100,
		Timestamp:   time.Now().UTC(),
	}

	if err := store.SaveAttempt(ctx, p); err != nil {
		t.Fatalf("SaveAttempt failed: %v", err)
	}

	// Verify by reading stats
	stats, err := store.LoadStats(ctx, "user1")
	if err != nil {
		t.Fatalf("LoadStats failed: %v", err)
	}
	if stats.ChallengesCompleted != 1 {
		t.Errorf("expected 1 completed, got %d", stats.ChallengesCompleted)
	}
	if stats.TotalXP != 100 {
		t.Errorf("expected 100 XP, got %d", stats.TotalXP)
	}
	if stats.TotalAttempts != 3 {
		t.Errorf("expected 3 attempts, got %d", stats.TotalAttempts)
	}
}

func TestStore_SaveAttempt_UpdatesExisting(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// First attempt - failed
	p1 := Progress{
		UserID:      "user1",
		StoryID:     "story1",
		ChapterID:   "ch1",
		ChallengeID: "c1",
		Completed:   false,
		Attempts:    1,
		BestTime:    0,
		Score:       0,
		Timestamp:   time.Now().UTC(),
	}
	if err := store.SaveAttempt(ctx, p1); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Second attempt - success
	p2 := Progress{
		UserID:      "user1",
		StoryID:     "story1",
		ChapterID:   "ch1",
		ChallengeID: "c1",
		Completed:   true,
		Attempts:    2,
		BestTime:    30.0,
		Score:       100,
		Timestamp:   time.Now().UTC(),
	}
	if err := store.SaveAttempt(ctx, p2); err != nil {
		t.Fatalf("second save: %v", err)
	}

	stats, err := store.LoadStats(ctx, "user1")
	if err != nil {
		t.Fatalf("LoadStats: %v", err)
	}
	if stats.ChallengesCompleted != 1 {
		t.Errorf("expected 1 completed, got %d", stats.ChallengesCompleted)
	}
	if stats.TotalAttempts != 2 {
		t.Errorf("expected 2 total attempts, got %d", stats.TotalAttempts)
	}
}

func TestStore_AppendHistory(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.AppendHistory(ctx, "user1", "c1", "SELECT * FROM t", 15.5, true); err != nil {
		t.Fatalf("AppendHistory failed: %v", err)
	}

	history, err := store.GetHistory(ctx, "user1", "c1")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
	if history[0].Query != "SELECT * FROM t" {
		t.Errorf("expected query 'SELECT * FROM t', got %q", history[0].Query)
	}
	if !history[0].Success {
		t.Error("expected success=true")
	}
}

func TestStore_GetHistory(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	store.AppendHistory(ctx, "user1", "c1", "SELECT 1", 10.0, true)
	store.AppendHistory(ctx, "user1", "c1", "SELECT 2", 20.0, false)
	store.AppendHistory(ctx, "user2", "c1", "SELECT 3", 30.0, true)

	history, err := store.GetHistory(ctx, "user1", "c1")
	if err != nil {
		t.Fatalf("GetHistory failed: %v", err)
	}
	if len(history) != 2 {
		t.Errorf("expected 2 entries for user1, got %d", len(history))
	}
}

func TestStore_UnlockAchievement(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	if err := store.UnlockAchievement(ctx, "user1", "first_steps"); err != nil {
		t.Fatalf("UnlockAchievement failed: %v", err)
	}

	// Idempotent - second unlock should not error
	if err := store.UnlockAchievement(ctx, "user1", "first_steps"); err != nil {
		t.Errorf("second unlock failed: %v", err)
	}

	achievements, err := store.GetAchievements(ctx, "user1")
	if err != nil {
		t.Fatalf("GetAchievements failed: %v", err)
	}
	found := false
	for _, a := range achievements {
		if a == "first_steps" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'first_steps' achievement")
	}
}

func TestStore_GetAchievements(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	store.UnlockAchievement(ctx, "user1", "first_steps")
	store.UnlockAchievement(ctx, "user1", "query_master")

	achievements, err := store.GetAchievements(ctx, "user1")
	if err != nil {
		t.Fatalf("GetAchievements failed: %v", err)
	}
	if len(achievements) != 2 {
		t.Errorf("expected 2 achievements, got %d", len(achievements))
	}
}

func TestStore_IsCompleted(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Not completed initially
	done, err := store.IsCompleted(ctx, "user1", "story1", "c1")
	if err != nil {
		t.Fatalf("IsCompleted failed: %v", err)
	}
	if done {
		t.Error("expected not completed initially")
	}

	// Mark completed
	p := Progress{
		UserID:      "user1",
		StoryID:     "story1",
		ChapterID:   "ch1",
		ChallengeID: "c1",
		Completed:   true,
		Attempts:    1,
		BestTime:    10.0,
		Score:       100,
		Timestamp:   time.Now().UTC(),
	}
	if err := store.SaveAttempt(ctx, p); err != nil {
		t.Fatalf("SaveAttempt: %v", err)
	}

	done, err = store.IsCompleted(ctx, "user1", "story1", "c1")
	if err != nil {
		t.Fatalf("IsCompleted failed: %v", err)
	}
	if !done {
		t.Error("expected completed=true after save")
	}
}

func TestStore_ResetProgress(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Add some data
	store.SaveAttempt(ctx, Progress{
		UserID:      "user1",
		StoryID:     "story1",
		ChapterID:   "ch1",
		ChallengeID: "c1",
		Completed:   true,
		Attempts:    1,
		BestTime:    10.0,
		Score:       100,
		Timestamp:   time.Now().UTC(),
	})
	store.AppendHistory(ctx, "user1", "c1", "SELECT 1", 10.0, true)
	store.UnlockAchievement(ctx, "user1", "first_steps")

	// Reset
	if err := store.ResetProgress(ctx, "user1"); err != nil {
		t.Fatalf("ResetProgress failed: %v", err)
	}

	// Verify all cleared
	stats, _ := store.LoadStats(ctx, "user1")
	if stats.ChallengesCompleted != 0 || stats.TotalXP != 0 || stats.TotalAttempts != 0 {
		t.Errorf("stats not cleared: %+v", stats)
	}

	history, _ := store.GetHistory(ctx, "user1", "c1")
	if len(history) != 0 {
		t.Errorf("history not cleared: %d entries", len(history))
	}

	achievements, err := store.GetAchievements(ctx, "user1")
	if err != nil {
		t.Fatalf("GetAchievements failed: %v", err)
	}
	if len(achievements) != 0 {
		t.Errorf("achievements not cleared: %d entries", len(achievements))
	}
}

func TestStore_ExportImportProgress(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Create some progress
	store.SaveAttempt(ctx, Progress{
		UserID:      "user1",
		StoryID:     "story1",
		ChapterID:   "ch1",
		ChallengeID: "c1",
		Completed:   true,
		Attempts:    2,
		BestTime:    30.0,
		Score:       100,
		Timestamp:   time.Now().UTC(),
	})

	// Export
	exported, err := store.ExportProgress(ctx, "user1")
	if err != nil {
		t.Fatalf("ExportProgress failed: %v", err)
	}
	if exported["user_id"] != "user1" {
		t.Errorf("expected user_id=user1, got %v", exported["user_id"])
	}
	progress := exported["progress"].([]map[string]any)
	if len(progress) != 1 {
		t.Errorf("expected 1 progress entry, got %d", len(progress))
	}

	// Import to new user
	store2 := newTestStore(t)
	if err := store2.ImportProgress(ctx, "user2", progress); err != nil {
		t.Fatalf("ImportProgress failed: %v", err)
	}

	stats, _ := store2.LoadStats(ctx, "user2")
	if stats.ChallengesCompleted != 1 {
		t.Errorf("expected 1 completed after import, got %d", stats.ChallengesCompleted)
	}
}

func TestStore_LoadStats_EmptyUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	stats, err := store.LoadStats(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("LoadStats failed: %v", err)
	}
	if stats.ChallengesCompleted != 0 || stats.TotalXP != 0 || stats.TotalAttempts != 0 {
		t.Errorf("expected zero stats for new user, got %+v", stats)
	}
}