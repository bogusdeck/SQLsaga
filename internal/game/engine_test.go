package game

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bogusdeck/sqlsaga/internal/database"
)

func newTestStore(t *testing.T) *database.Store {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	store, err := database.Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { os.Remove(dbPath); store.Close() })
	return store
}

func testStory() Story {
	return Story{
		ID:    "test",
		Title: "Test Story",
		Chapters: []Chapter{
			{ID: "ch1", Title: "Chapter 1", Challenges: []Challenge{
				{ID: "c1", Schema: "CREATE TABLE t (id INT PRIMARY KEY, name TEXT);", SampleData: "INSERT INTO t (id, name) VALUES (1, 'a');", Objective: "Select all", Validation: Validation{ExpectedColumns: []string{"id", "name"}, ExpectedRows: []map[string]any{{"id": 1, "name": "a"}}, AllowOrder: true}},
				{ID: "c2", Schema: "CREATE TABLE t (id INT PRIMARY KEY, name TEXT);", SampleData: "INSERT INTO t (id, name) VALUES (1, 'a'), (2, 'b');", Objective: "Select all", Validation: Validation{ExpectedColumns: []string{"id", "name"}, ExpectedRows: []map[string]any{{"id": 1, "name": "a"}, {"id": 2, "name": "b"}}, AllowOrder: true}},
			}},
			{ID: "ch2", Title: "Chapter 2", Challenges: []Challenge{
				{ID: "c3", Schema: "CREATE TABLE t (id INT PRIMARY KEY, val INT);", SampleData: "INSERT INTO t (id, val) VALUES (1, 10);", Objective: "Select all", Validation: Validation{ExpectedColumns: []string{"id", "val"}, ExpectedRows: []map[string]any{{"id": 1, "val": 10}}, AllowOrder: true}},
			}},
		},
	}
}

func TestEngine_NewEngine(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	if engine.Story.ID != "test" {
		t.Errorf("expected story ID 'test', got %q", engine.Story.ID)
	}
	if engine.ChapterIndex != 0 {
		t.Errorf("expected chapter index 0, got %d", engine.ChapterIndex)
	}
	if engine.ChallengeIndex != 0 {
		t.Errorf("expected challenge index 0, got %d", engine.ChallengeIndex)
	}
	if engine.TotalXP != 0 {
		t.Errorf("expected 0 XP, got %d", engine.TotalXP)
	}
	if engine.Streak != 0 {
		t.Errorf("expected 0 streak, got %d", engine.Streak)
	}
}

func TestEngine_CurrentChapter(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	ch := engine.CurrentChapter()
	if ch == nil || ch.ID != "ch1" {
		t.Fatalf("expected chapter ch1, got %v", ch)
	}

	engine.ChapterIndex = 1
	ch = engine.CurrentChapter()
	if ch == nil || ch.ID != "ch2" {
		t.Fatalf("expected chapter ch2, got %v", ch)
	}

	engine.ChapterIndex = 5
	if engine.CurrentChapter() != nil {
		t.Error("expected nil for out of bounds chapter")
	}
}

func TestEngine_CurrentChallenge(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	c := engine.CurrentChallenge()
	if c == nil || c.ID != "c1" {
		t.Fatalf("expected challenge c1, got %v", c)
	}

	engine.ChallengeIndex = 1
	c = engine.CurrentChallenge()
	if c == nil || c.ID != "c2" {
		t.Fatalf("expected challenge c2, got %v", c)
	}

	engine.ChallengeIndex = 5
	if engine.CurrentChallenge() != nil {
		t.Error("expected nil for out of bounds challenge")
	}
}

func TestEngine_JumpToChapter(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	if err := engine.JumpToChapter("ch2"); err != nil {
		t.Fatalf("JumpToChapter failed: %v", err)
	}
	if engine.ChapterIndex != 1 {
		t.Errorf("expected chapter index 1, got %d", engine.ChapterIndex)
	}
	if engine.ChallengeIndex != 0 {
		t.Errorf("expected challenge index 0, got %d", engine.ChallengeIndex)
	}

	if err := engine.JumpToChapter("nonexistent"); err == nil {
		t.Error("expected error for nonexistent chapter")
	}
}

func TestEngine_JumpToChallengeAbsolute(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	if err := engine.JumpToChallengeAbsolute(1); err != nil {
		t.Fatalf("JumpToChallengeAbsolute failed: %v", err)
	}
	if engine.ChapterIndex != 0 || engine.ChallengeIndex != 1 {
		t.Errorf("expected chapter 0, challenge 1, got chapter %d, challenge %d", engine.ChapterIndex, engine.ChallengeIndex)
	}

	if err := engine.JumpToChallengeAbsolute(2); err != nil {
		t.Fatalf("JumpToChallengeAbsolute failed: %v", err)
	}
	if engine.ChapterIndex != 1 || engine.ChallengeIndex != 0 {
		t.Errorf("expected chapter 1, challenge 0, got chapter %d, challenge %d", engine.ChapterIndex, engine.ChallengeIndex)
	}

	if err := engine.JumpToChallengeAbsolute(10); err == nil {
		t.Error("expected error for out of bounds challenge")
	}
}

func TestEngine_NextPrevChallenge(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	if !engine.NextChallenge() {
		t.Error("expected to advance to c2")
	}
	if engine.CurrentChallenge().ID != "c2" {
		t.Errorf("expected c2, got %s", engine.CurrentChallenge().ID)
	}

	if !engine.NextChallenge() {
		t.Error("expected to advance to c3")
	}
	if engine.CurrentChallenge().ID != "c3" {
		t.Errorf("expected c3, got %s", engine.CurrentChallenge().ID)
	}

	if engine.NextChallenge() {
		t.Error("expected false at end")
	}

	if !engine.PrevChallenge() {
		t.Error("expected to go back to c2")
	}
	if engine.CurrentChallenge().ID != "c2" {
		t.Errorf("expected c2, got %s", engine.CurrentChallenge().ID)
	}

	if !engine.PrevChallenge() {
		t.Error("expected to go back to c1")
	}
	if engine.CurrentChallenge().ID != "c1" {
		t.Errorf("expected c1, got %s", engine.CurrentChallenge().ID)
	}

	if engine.PrevChallenge() {
		t.Error("expected false at start")
	}
}

func TestEngine_RevealNextHint(t *testing.T) {
	story := Story{
		ID:    "test",
		Title: "Test",
		Chapters: []Chapter{
			{ID: "ch1", Title: "Chapter 1", Challenges: []Challenge{
				{ID: "c1", Schema: "CREATE TABLE t (id INT);", SampleData: "", Objective: "test", Validation: Validation{ExpectedColumns: []string{"id"}}, Hints: []string{"hint1", "hint2"}},
			}},
		},
	}
	store := newTestStore(t)
	engine := NewEngine(story, store, database.NewLocalStub(), "user1")

	h1 := engine.RevealNextHint()
	if h1 != "hint1" {
		t.Errorf("expected 'hint1', got %q", h1)
	}
	if engine.HintsRevealed() != 1 {
		t.Errorf("expected 1 hint revealed, got %d", engine.HintsRevealed())
	}

	h2 := engine.RevealNextHint()
	if h2 != "hint2" {
		t.Errorf("expected 'hint2', got %q", h2)
	}
	if engine.HintsRevealed() != 2 {
		t.Errorf("expected 2 hints revealed, got %d", engine.HintsRevealed())
	}

	h3 := engine.RevealNextHint()
	if h3 != "" {
		t.Errorf("expected empty string, got %q", h3)
	}
}

func TestEngine_ScoreFor(t *testing.T) {
	c := Challenge{ID: "c1", XP: 100, TimeLimit: 120}

	// Base XP + time bonus: 120 limit - 60 elapsed = 60 remaining, /2 = 30 bonus
	if scoreFor(c, 0, 60) != 130 {
		t.Errorf("base XP + time bonus: expected 130, got %d", scoreFor(c, 0, 60))
	}

	// Fast completion: 120 limit - 30 elapsed = 90 remaining, /2 = 45 bonus
	if scoreFor(c, 0, 30) != 145 {
		t.Errorf("time bonus: expected 145, got %d", scoreFor(c, 0, 30))
	}

	// With 2 failed attempts: 100 + 30 bonus - 50 penalty = 80
	if scoreFor(c, 2, 60) != 80 {
		t.Errorf("penalty: expected 80, got %d", scoreFor(c, 2, 60))
	}

	c2 := Challenge{XP: 10, TimeLimit: 10}
	if scoreFor(c2, 10, 5) < 0 {
		t.Errorf("expected min 0, got negative")
	}

	c3 := Challenge{XP: 0}
	if scoreFor(c3, 0, 60) != 100 {
		t.Errorf("expected default 100, got %d", scoreFor(c3, 0, 60))
	}
}

func TestEngine_Submit_CorrectQuery(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	sub := engine.Submit(context.Background(), "SELECT * FROM t")
	if sub.Err != nil {
		t.Fatalf("unexpected error: %v", sub.Err)
	}
	if !sub.Matched {
		t.Errorf("expected match, got diff: %s", sub.Diff.String())
	}
	if sub.XPEarned <= 0 {
		t.Errorf("expected positive XP, got %d", sub.XPEarned)
	}
	if engine.TotalXP != sub.XPEarned {
		t.Errorf("expected total XP %d, got %d", sub.XPEarned, engine.TotalXP)
	}
	if engine.Streak != 1 {
		t.Errorf("expected streak 1, got %d", engine.Streak)
	}
	if !engine.CompletedSet["c1"] {
		t.Error("expected challenge marked complete")
	}
}

func TestEngine_Submit_IncorrectQuery(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	sub := engine.Submit(context.Background(), "SELECT id FROM t")
	if sub.Err != nil {
		t.Fatalf("unexpected error: %v", sub.Err)
	}
	if sub.Matched {
		t.Error("expected mismatch")
	}
	if engine.Streak != 0 {
		t.Errorf("expected streak 0 after failure, got %d", engine.Streak)
	}
	if engine.FailedAttempts != 1 {
		t.Errorf("expected 1 failed attempt, got %d", engine.FailedAttempts)
	}
}

func TestEngine_Submit_SyntaxError(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	sub := engine.Submit(context.Background(), "SELEC * FROM t")
	if sub.Err == nil {
		t.Fatal("expected syntax error")
	}
	if sub.Matched {
		t.Error("expected no match on syntax error")
	}
	if engine.Streak != 0 {
		t.Errorf("expected streak 0 after error, got %d", engine.Streak)
	}
}

func TestEngine_Submit_MultipleAttempts(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	sub1 := engine.Submit(context.Background(), "SELECT id FROM t")
	if sub1.Matched {
		t.Fatal("expected first attempt to fail")
	}

	sub2 := engine.Submit(context.Background(), "SELECT * FROM t")
	if !sub2.Matched {
		t.Fatalf("expected second attempt to succeed: %s", sub2.Diff.String())
	}

	if sub2.XPEarned >= 100 {
		t.Errorf("expected XP penalty, got %d", sub2.XPEarned)
	}
	if engine.Attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", engine.Attempts)
	}
	if engine.FailedAttempts != 1 {
		t.Errorf("expected 1 failed attempt, got %d", engine.FailedAttempts)
	}
}

func TestEngine_ChallengePosition(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	// Position is within current chapter (ChallengeIndex + 1)
	pos, total := engine.ChallengePosition()
	if pos != 1 {
		t.Errorf("expected position 1, got %d", pos)
	}
	if total != 3 {
		t.Errorf("expected 3 total challenges, got %d", total)
	}

	engine.NextChallenge() // c2 in chapter 1
	pos, total = engine.ChallengePosition()
	if pos != 2 {
		t.Errorf("expected position 2, got %d", pos)
	}

	engine.NextChallenge() // c3 in chapter 2 (ChallengeIndex resets to 0)
	pos, total = engine.ChallengePosition()
	if pos != 1 { // position within chapter 2
		t.Errorf("expected position 1 (within chapter), got %d", pos)
	}
	if total != 3 {
		t.Errorf("expected 3 total challenges, got %d", total)
	}
}

func TestEngine_ResetChallenge(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	engine.Submit(context.Background(), "SELECT * FROM t")
	if engine.Attempts == 0 {
		t.Fatal("expected attempts > 0")
	}

	engine.NextChallenge()
	if engine.Attempts != 0 {
		t.Errorf("expected attempts reset to 0, got %d", engine.Attempts)
	}
	if engine.FailedAttempts != 0 {
		t.Errorf("expected failed attempts reset to 0, got %d", engine.FailedAttempts)
	}
	if engine.HintsRevealed() != 0 {
		t.Errorf("expected hints reset to 0, got %d", engine.HintsRevealed())
	}
}

func TestEngine_Submit_RecordsAttempt(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	engine.Submit(context.Background(), "SELECT * FROM t")

	history, err := store.GetHistory(context.Background(), "user1", "c1")
	if err != nil {
		t.Fatalf("get history: %v", err)
	}
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
	if !history[0].Success {
		t.Error("expected success=true")
	}
}

func TestEngine_Submit_UpdatesProgress(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	engine.Submit(context.Background(), "SELECT * FROM t")

	progress, err := store.GetProgress(context.Background(), "user1", "test", "ch1", "c1")
	if err != nil {
		t.Fatalf("get progress: %v", err)
	}
	if progress == nil {
		t.Fatal("expected progress record")
	}
	if !progress.Completed {
		t.Error("expected completed=true")
	}
	if progress.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", progress.Attempts)
	}
	if progress.Score <= 0 {
		t.Errorf("expected positive score, got %d", progress.Score)
	}
	if progress.BestTime <= 0 {
		t.Errorf("expected positive best_time, got %f", progress.BestTime)
	}
}

func TestEngine_Submit_FailedAttemptRecordsProgress(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	engine.Submit(context.Background(), "SELECT id FROM t")

	progress, err := store.GetProgress(context.Background(), "user1", "test", "ch1", "c1")
	if err != nil {
		t.Fatalf("get progress: %v", err)
	}
	if progress == nil {
		t.Fatal("expected progress record")
	}
	if progress.Completed {
		t.Error("expected completed=false")
	}
	if progress.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", progress.Attempts)
	}
	if progress.Score != 0 {
		t.Errorf("expected score 0, got %d", progress.Score)
	}
}

func TestEngine_StreakIncrements(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	for i := 0; i < 3; i++ {
		sub := engine.Submit(context.Background(), "SELECT * FROM t")
		if !sub.Matched {
			t.Fatalf("attempt %d should pass", i+1)
		}
		if engine.Streak != i+1 {
			t.Errorf("expected streak %d, got %d", i+1, engine.Streak)
		}
		engine.NextChallenge()
	}
}

func TestEngine_StreakResetsOnFailure(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	engine.Submit(context.Background(), "SELECT * FROM t")
	if engine.Streak != 1 {
		t.Fatalf("expected streak 1, got %d", engine.Streak)
	}

	engine.Submit(context.Background(), "SELECT id FROM t")
	if engine.Streak != 0 {
		t.Errorf("expected streak 0 after failure, got %d", engine.Streak)
	}
}

func TestEngine_TimeBonus(t *testing.T) {
	story := Story{
		ID:    "test",
		Title: "Test",
		Chapters: []Chapter{
			{ID: "ch1", Title: "Chapter 1", Challenges: []Challenge{
				{ID: "c1", Schema: "CREATE TABLE t (id INT);", SampleData: "INSERT INTO t VALUES (1);", Objective: "test", Validation: Validation{ExpectedColumns: []string{"id"}, ExpectedRows: []map[string]any{{"id": 1}}, AllowOrder: true}, XP: 100, TimeLimit: 60},
			}},
		},
	}
	store := newTestStore(t)
	engine := NewEngine(story, store, database.NewLocalStub(), "user1")

	engine.StartedAt = time.Now().Add(-10 * time.Second)
	sub := engine.Submit(context.Background(), "SELECT * FROM t")
	if sub.XPEarned <= 100 {
		t.Errorf("expected time bonus, got XP %d", sub.XPEarned)
	}
}

func TestLoadAllStories(t *testing.T) {
	stories, err := LoadAllStories()
	if err != nil {
		t.Fatalf("LoadAllStories failed: %v", err)
	}
	if len(stories) == 0 {
		t.Fatal("expected at least one story")
	}
	for _, s := range stories {
		if s.Story.ID == "" {
			t.Error("story ID empty")
		}
		if s.Story.Title == "" {
			t.Error("story title empty")
		}
		if len(s.Story.Chapters) == 0 {
			t.Errorf("story %s has no chapters", s.Story.ID)
		}
	}
}

func TestLoadStory_ByID(t *testing.T) {
	s, err := LoadStory("mystery_artifact")
	if err != nil {
		t.Fatalf("LoadStory failed: %v", err)
	}
	if s.Story.ID != "mystery_artifact" {
		t.Errorf("expected ID mystery_artifact, got %s", s.Story.ID)
	}
}

func TestLoadStory_ByGenre(t *testing.T) {
	s, err := LoadStory("mystery")
	if err != nil {
		t.Fatalf("LoadStory failed: %v", err)
	}
	if s.Story.Genre != "mystery" {
		t.Errorf("expected genre mystery, got %s", s.Story.Genre)
	}
}

func TestLoadStory_NotFound(t *testing.T) {
	_, err := LoadStory("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent story")
	}
}

func TestValidateStory(t *testing.T) {
	s := Story{
		Title: "Test",
		Chapters: []Chapter{{
			ID: "ch1",
			Challenges: []Challenge{{
				ID:          "c1",
				Schema:      "CREATE TABLE t (id INT);",
				SampleData:  "INSERT INTO t VALUES (1);",
				Objective:   "test",
				Validation:  Validation{ExpectedColumns: []string{"id"}},
			}},
		}},
	}
	if err := validateStory(&s); err == nil {
		t.Error("expected error for missing story ID")
	}

	s = Story{
		ID: "test",
		Chapters: []Chapter{{
			ID: "ch1",
			Challenges: []Challenge{{
				ID:          "c1",
				Schema:      "CREATE TABLE t (id INT);",
				SampleData:  "INSERT INTO t VALUES (1);",
				Objective:   "test",
				Validation:  Validation{ExpectedColumns: []string{"id"}},
			}},
		}},
	}
	if err := validateStory(&s); err == nil {
		t.Error("expected error for missing story title")
	}

	s = Story{ID: "test", Title: "Test"}
	if err := validateStory(&s); err == nil {
		t.Error("expected error for no chapters")
	}

	s = Story{
		ID:    "test",
		Title: "Test",
		Chapters: []Chapter{{
			Challenges: []Challenge{{
				ID:          "c1",
				Schema:      "CREATE TABLE t (id INT);",
				SampleData:  "INSERT INTO t VALUES (1);",
				Objective:   "test",
				Validation:  Validation{ExpectedColumns: []string{"id"}},
			}},
		}},
	}
	if err := validateStory(&s); err == nil {
		t.Error("expected error for missing chapter ID")
	}

	s = Story{
		ID:    "test",
		Title: "Test",
		Chapters: []Chapter{{
			ID: "ch1",
			Challenges: []Challenge{{
				ID:         "c1",
				SampleData: "INSERT INTO t VALUES (1);",
				Objective:  "test",
				Validation: Validation{ExpectedColumns: []string{"id"}},
			}},
		}},
	}
	if err := validateStory(&s); err == nil {
		t.Error("expected error for missing schema")
	}

	s = Story{
		ID:    "test",
		Title: "Test",
		Chapters: []Chapter{{
			ID: "ch1",
			Challenges: []Challenge{{
				ID:         "c1",
				Schema:     "CREATE TABLE t (id INT);",
				Objective:  "test",
				Validation: Validation{ExpectedColumns: []string{"id"}},
			}},
		}},
	}
	if err := validateStory(&s); err == nil {
		t.Error("expected error for missing sample_data")
	}

	s = Story{
		ID:    "test",
		Title: "Test",
		Chapters: []Chapter{{
			ID: "ch1",
			Challenges: []Challenge{{
				ID:          "c1",
				Schema:      "CREATE TABLE t (id INT);",
				SampleData:  "INSERT INTO t VALUES (1);",
				Validation:  Validation{ExpectedColumns: []string{"id"}},
			}},
		}},
	}
	if err := validateStory(&s); err == nil {
		t.Error("expected error for missing objective")
	}

	s = Story{
		ID:    "test",
		Title: "Test",
		Chapters: []Chapter{{
			ID: "ch1",
			Challenges: []Challenge{{
				ID:         "c1",
				Schema:     "CREATE TABLE t (id INT);",
				SampleData: "INSERT INTO t VALUES (1);",
				Objective:  "test",
				Validation: Validation{},
			}},
		}},
	}
	if err := validateStory(&s); err == nil {
		t.Error("expected error for missing expected_columns")
	}

	s = Story{
		ID:    "test",
		Title: "Test",
		Chapters: []Chapter{{
			ID: "ch1",
			Challenges: []Challenge{{
				ID:          "c1",
				Schema:      "CREATE TABLE t (id INT);",
				SampleData:  "INSERT INTO t VALUES (1);",
				Objective:   "test",
				XP:          100,
				TimeLimit:   60,
				Validation:  Validation{ExpectedColumns: []string{"id"}},
			}},
		}},
	}
	if err := validateStory(&s); err != nil {
		t.Errorf("unexpected error for valid story: %v", err)
	}
}
func TestEngine_SetStory(t *testing.T) {
	store := newTestStore(t)
	engine := NewEngine(testStory(), store, database.NewLocalStub(), "user1")

	// Mutate state to prove SetStory resets the per-story run fields.
	engine.ChapterIndex = 1
	engine.ChallengeIndex = 1
	engine.Attempts = 3
	engine.FailedAttempts = 2
	engine.TotalXP = 250
	engine.Streak = 4
	engine.hintsRevealed = 2
	engine.CompletedSet["c2"] = true

	newStory := Story{
		ID:    "replacement",
		Title: "Replacement",
		Chapters: []Chapter{
			{ID: "a", Title: "A", Challenges: []Challenge{
				{ID: "x1", Schema: "CREATE TABLE t(id INT);", SampleData: "", Objective: "x", Validation: Validation{ExpectedColumns: []string{"id"}}},
			}},
		},
	}
	engine.SetStory(newStory)

	if engine.Story.ID != "replacement" {
		t.Errorf("expected story id 'replacement', got %q", engine.Story.ID)
	}
	if engine.ChapterIndex != 0 || engine.ChallengeIndex != 0 {
		t.Errorf("expected cursor reset to 0/0, got %d/%d", engine.ChapterIndex, engine.ChallengeIndex)
	}
	if engine.Attempts != 0 || engine.FailedAttempts != 0 || engine.hintsRevealed != 0 {
		t.Errorf("expected attempts/hints reset, got attempts=%d failed=%d hints=%d", engine.Attempts, engine.FailedAttempts, engine.hintsRevealed)
	}
	if len(engine.CompletedSet) != 0 {
		t.Errorf("expected CompletedSet cleared, got %d entries", len(engine.CompletedSet))
	}
	if engine.LastSubmission != nil {
		t.Errorf("expected LastSubmission nil, got %+v", engine.LastSubmission)
	}
	// Cross-story session state is preserved.
	if engine.TotalXP != 250 {
		t.Errorf("expected TotalXP preserved (250), got %d", engine.TotalXP)
	}
	if engine.Streak != 4 {
		t.Errorf("expected Streak preserved (4), got %d", engine.Streak)
	}

	// New story's first challenge is reachable.
	c := engine.CurrentChallenge()
	if c == nil || c.ID != "x1" {
		t.Errorf("expected current challenge x1, got %v", c)
	}
}
