// Package database wraps the local SQLite store used to track progress.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store is the local progress / analytics store.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the local SQLite database at the given path.
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, errors.New("database path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir db dir: %w", err)
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // single writer at a time; WAL handles concurrent readers
	if err := db.PingContext(context.Background()); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database file lock.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS progress (
			user_id        TEXT NOT NULL,
			story_id       TEXT NOT NULL,
			chapter_id     TEXT NOT NULL,
			challenge_id   TEXT NOT NULL,
			completed      INTEGER NOT NULL DEFAULT 0,
			attempts       INTEGER NOT NULL DEFAULT 0,
			best_time      REAL    NOT NULL DEFAULT 0,
			score          INTEGER NOT NULL DEFAULT 0,
			timestamp      TEXT    NOT NULL,
			PRIMARY KEY (user_id, story_id, chapter_id, challenge_id)
		)`,
		`CREATE TABLE IF NOT EXISTS query_history (
			id             INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id        TEXT NOT NULL,
			challenge_id   TEXT NOT NULL,
			query          TEXT NOT NULL,
			execution_time REAL NOT NULL,
			success        INTEGER NOT NULL,
			timestamp      TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS achievements (
			user_id        TEXT NOT NULL,
			achievement_id TEXT NOT NULL,
			unlocked_at    TEXT NOT NULL,
			PRIMARY KEY (user_id, achievement_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_progress_user ON progress(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_history_user ON query_history(user_id, challenge_id)`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(context.Background(), q); err != nil {
			return fmt.Errorf("migrate %q: %w", firstLine(q), err)
		}
	}
	return nil
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}

// Progress is a single per-user-per-challenge record.
type Progress struct {
	UserID      string
	StoryID     string
	ChapterID   string
	ChallengeID string
	Completed   bool
	Attempts    int
	BestTime    float64
	Score       int
	Timestamp   time.Time
}

// SaveAttempt records (or updates) the latest attempt for a challenge.
func (s *Store) SaveAttempt(ctx context.Context, p Progress) error {
	if p.Timestamp.IsZero() {
		p.Timestamp = time.Now().UTC()
	}
	const q = `
INSERT INTO progress (user_id, story_id, chapter_id, challenge_id, completed, attempts, best_time, score, timestamp)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(user_id, story_id, chapter_id, challenge_id) DO UPDATE SET
  attempts = excluded.attempts,
  completed = CASE WHEN excluded.completed = 1 OR progress.completed = 1 THEN 1 ELSE 0 END,
  best_time = CASE WHEN excluded.best_time > 0 AND (progress.best_time = 0 OR excluded.best_time < progress.best_time)
                   THEN excluded.best_time ELSE progress.best_time END,
  score = MAX(progress.score, excluded.score),
  timestamp = excluded.timestamp
`
	_, err := s.db.ExecContext(ctx, q,
		p.UserID, p.StoryID, p.ChapterID, p.ChallengeID,
		boolToInt(p.Completed), p.Attempts, p.BestTime, p.Score, p.Timestamp.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("save attempt: %w", err)
	}
	return nil
}

// AppendHistory stores the raw query that was run.
func (s *Store) AppendHistory(ctx context.Context, userID, challengeID, query string, execTime float64, success bool) error {
	const q = `INSERT INTO query_history (user_id, challenge_id, query, execution_time, success, timestamp) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q, userID, challengeID, query, execTime, boolToInt(success), time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("append history: %w", err)
	}
	return nil
}

// UnlockAchievement grants an achievement (idempotent).
func (s *Store) UnlockAchievement(ctx context.Context, userID, achievementID string) error {
	const q = `INSERT OR IGNORE INTO achievements (user_id, achievement_id, unlocked_at) VALUES (?, ?, ?)`
	_, err := s.db.ExecContext(ctx, q, userID, achievementID, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("unlock achievement: %w", err)
	}
	return nil
}

// Stats aggregates a user's run so far.
type Stats struct {
	ChallengesCompleted int
	TotalXP             int
	TotalAttempts       int
	StreakBest          int
}

// LoadStats pulls aggregate stats for one user.
func (s *Store) LoadStats(ctx context.Context, userID string) (Stats, error) {
	var st Stats
	err := s.db.QueryRowContext(ctx, `
SELECT
  COALESCE(SUM(CASE WHEN completed = 1 THEN 1 ELSE 0 END), 0),
  COALESCE(SUM(CASE WHEN completed = 1 THEN score ELSE 0 END), 0),
  COALESCE(SUM(attempts), 0)
FROM progress WHERE user_id = ?`, userID).Scan(&st.ChallengesCompleted, &st.TotalXP, &st.TotalAttempts)
	if err != nil {
		return st, fmt.Errorf("load stats: %w", err)
	}
	return st, nil
}

// IsCompleted reports whether a challenge is marked complete for the user.
func (s *Store) IsCompleted(ctx context.Context, userID, storyID, challengeID string) (bool, error) {
	var done bool
	err := s.db.QueryRowContext(ctx, `
SELECT EXISTS(SELECT 1 FROM progress WHERE user_id = ? AND story_id = ? AND challenge_id = ? AND completed = 1)`,
		userID, storyID, challengeID).Scan(&done)
	if err != nil {
		return false, fmt.Errorf("is completed: %w", err)
	}
	return done, nil
}

// ResetProgress wipes all stored state for a user.
func (s *Store) ResetProgress(ctx context.Context, userID string) error {
	for _, q := range []string{
		`DELETE FROM progress WHERE user_id = ?`,
		`DELETE FROM query_history WHERE user_id = ?`,
		`DELETE FROM achievements WHERE user_id = ?`,
	} {
		if _, err := s.db.ExecContext(ctx, q, userID); err != nil {
			return fmt.Errorf("reset %q: %w", firstLine(q), err)
		}
	}
	return nil
}

// ExportProgress serializes every stored row for a user to a map for JSON output.
func (s *Store) ExportProgress(ctx context.Context, userID string) (map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT story_id, chapter_id, challenge_id, completed, attempts, best_time, score, timestamp
FROM progress WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("export: %w", err)
	}
	defer rows.Close()

	var progress []map[string]any
	for rows.Next() {
		var (
			storyID, chapterID, challengeID, ts string
			completed, attempts, score          int
			bestTime                            float64
		)
		if err := rows.Scan(&storyID, &chapterID, &challengeID, &completed, &attempts, &bestTime, &score, &ts); err != nil {
			return nil, err
		}
		progress = append(progress, map[string]any{
			"story_id":     storyID,
			"chapter_id":   chapterID,
			"challenge_id": challengeID,
			"completed":    completed == 1,
			"attempts":     attempts,
			"best_time":    bestTime,
			"score":        score,
			"timestamp":    ts,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return map[string]any{
		"user_id":  userID,
		"exported": time.Now().UTC().Format(time.RFC3339),
		"progress": progress,
	}, nil
}

// ImportProgress upserts a batch of progress rows (e.g. from an export file).
func (s *Store) ImportProgress(ctx context.Context, userID string, rows []map[string]any) error {
	for _, r := range rows {
		ts, _ := r["timestamp"].(string)
		t, _ := time.Parse(time.RFC3339, ts)
		completed, _ := r["completed"].(bool)
		attempts, _ := r["attempts"].(float64)
		bestTime, _ := r["best_time"].(float64)
		score, _ := r["score"].(float64)
		storyID, _ := r["story_id"].(string)
		chapterID, _ := r["chapter_id"].(string)
		challengeID, _ := r["challenge_id"].(string)
		err := s.SaveAttempt(ctx, Progress{
			UserID:      userID,
			StoryID:     storyID,
			ChapterID:   chapterID,
			ChallengeID: challengeID,
			Completed:   completed,
			Attempts:    int(attempts),
			BestTime:    bestTime,
			Score:       int(score),
			Timestamp:   t,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// QueryHistoryEntry represents a single query history record.
type QueryHistoryEntry struct {
	ID             int
	UserID         string
	ChallengeID    string
	Query          string
	ExecutionTime  float64
	Success        bool
	Timestamp      time.Time
}

// GetHistory returns the query history for a user and challenge.
func (s *Store) GetHistory(ctx context.Context, userID, challengeID string) ([]QueryHistoryEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, challenge_id, query, execution_time, success, timestamp
		FROM query_history
		WHERE user_id = ? AND challenge_id = ?
		ORDER BY timestamp DESC`, userID, challengeID)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}
	defer rows.Close()

	var history []QueryHistoryEntry
	for rows.Next() {
		var h QueryHistoryEntry
		var ts string
		if err := rows.Scan(&h.ID, &h.UserID, &h.ChallengeID, &h.Query, &h.ExecutionTime, &h.Success, &ts); err != nil {
			return nil, err
		}
		h.Timestamp, _ = time.Parse(time.RFC3339, ts)
		history = append(history, h)
	}
	return history, rows.Err()
}

// GetProgress returns the progress for a specific challenge.
func (s *Store) GetProgress(ctx context.Context, userID, storyID, chapterID, challengeID string) (*Progress, error) {
	var p Progress
	var completed int
	var ts string
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, story_id, chapter_id, challenge_id, completed, attempts, best_time, score, timestamp
		FROM progress
		WHERE user_id = ? AND story_id = ? AND chapter_id = ? AND challenge_id = ?`,
		userID, storyID, chapterID, challengeID).Scan(&p.UserID, &p.StoryID, &p.ChapterID, &p.ChallengeID, &completed, &p.Attempts, &p.BestTime, &p.Score, &ts)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get progress: %w", err)
	}
	p.Completed = completed == 1
	p.Timestamp, _ = time.Parse(time.RFC3339, ts)
	return &p, nil
}

// GetAchievements returns all unlocked achievement IDs for a user.
func (s *Store) GetAchievements(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT achievement_id FROM achievements WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("get achievements: %w", err)
	}
	defer rows.Close()

	var achievements []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		achievements = append(achievements, id)
	}
	return achievements, rows.Err()
}

// StoryProgressSummary summarizes a user's progress in a story.
type StoryProgressSummary struct {
	StoryID        string
	ChallengesDone int
	TotalXP        int
	LastPlayed     time.Time
}

// GetPendingGames returns a summary of stories the user has started.
func (s *Store) GetPendingGames(ctx context.Context, userID string) ([]StoryProgressSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT story_id, 
		       COALESCE(SUM(CASE WHEN completed = 1 THEN 1 ELSE 0 END), 0) as done,
		       COALESCE(SUM(score), 0) as xp,
		       MAX(timestamp) as last_played
		FROM progress 
		WHERE user_id = ?
		GROUP BY story_id
		ORDER BY last_played DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("get pending games: %w", err)
	}
	defer rows.Close()

	var pending []StoryProgressSummary
	for rows.Next() {
		var p StoryProgressSummary
		var ts string
		if err := rows.Scan(&p.StoryID, &p.ChallengesDone, &p.TotalXP, &ts); err != nil {
			return nil, err
		}
		p.LastPlayed, _ = time.Parse(time.RFC3339, ts)
		pending = append(pending, p)
	}
	return pending, rows.Err()
}
