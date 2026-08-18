package game

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bogusdeck/sqlsaga/internal/database"
	"github.com/bogusdeck/sqlsaga/internal/parser"
)

// Engine is the live state of a single playthrough.
type Engine struct {
	Story         Story
	ChapterIndex  int
	ChallengeIndex int

	// current challenge state
	Attempts        int
	FailedAttempts  int
	Streak          int
	TotalXP         int
	StartedAt       time.Time
	LastSubmission  *Submission
	CompletedSet    map[string]bool // challenge id -> completed
	hintsRevealed   int
	firebase        database.FirebaseClient
	store           *database.Store
	userID          string
}

// Submission captures the result of a single user query attempt.
type Submission struct {
	Query     string
	Result    parser.RunResult
	Diff      parser.Diff
	Matched   bool
	XPEarned  int
	TimeTaken time.Duration
	Timestamp time.Time
	// Raw error from execution (nil on success)
	Err error
}

// NewEngine wires the engine against a loaded story and the persistence store.
func NewEngine(s Story, store *database.Store, fb database.FirebaseClient, userID string) *Engine {
	return &Engine{
		Story:         s,
		ChapterIndex:  0,
		ChallengeIndex: 0,
		StartedAt:     time.Now(),
		CompletedSet:  map[string]bool{},
		store:         store,
		firebase:      fb,
		userID:        userID,
	}
}

// SetStory swaps the active story and resets the per-story run state
// (chapter/challenge cursor, attempts, hints, timer). XP, streak, and the
// user ID are preserved so the player keeps their earned progress across
// stories.
func (e *Engine) SetStory(s Story) {
	e.Story = s
	e.ChapterIndex = 0
	e.ChallengeIndex = 0
	e.Attempts = 0
	e.FailedAttempts = 0
	e.hintsRevealed = 0
	e.StartedAt = time.Now()
	e.LastSubmission = nil
	e.CompletedSet = map[string]bool{}
}

// CurrentChapter returns a pointer to the active chapter.
func (e *Engine) CurrentChapter() *Chapter {
	if e.ChapterIndex < 0 || e.ChapterIndex >= len(e.Story.Chapters) {
		return nil
	}
	return &e.Story.Chapters[e.ChapterIndex]
}

// CurrentChallenge returns a pointer to the active challenge.
func (e *Engine) CurrentChallenge() *Challenge {
	ch := e.CurrentChapter()
	if ch == nil {
		return nil
	}
	if e.ChallengeIndex < 0 || e.ChallengeIndex >= len(ch.Challenges) {
		return nil
	}
	return &ch.Challenges[e.ChallengeIndex]
}

// GetFullStorySchemaAndData returns all deduplicated schemas and sample data across the entire story.
func (e *Engine) GetFullStorySchemaAndData() (string, string) {
	tablesCreated := make(map[string]bool)
	dataSet := make(map[string]bool)
	
	var schemas []string
	var datas []string
	
	tableNameRegex := regexp.MustCompile(`(?i)\bCREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([a-zA-Z0-9_]+)`)
	insertRegex := regexp.MustCompile(`(?i)\bINSERT\s+INTO\s+`)
	
	for _, ch := range e.Story.Chapters {
		for _, c := range ch.Challenges {
			schemaRaw := strings.TrimSpace(c.Schema)
			if schemaRaw != "" {
				for _, stmt := range strings.Split(schemaRaw, ";") {
					stmt = strings.TrimSpace(stmt)
					if stmt == "" {
						continue
					}
					matches := tableNameRegex.FindStringSubmatch(stmt)
					if len(matches) > 1 {
						tableName := strings.ToLower(matches[1])
						if !tablesCreated[tableName] {
							tablesCreated[tableName] = true
							schemas = append(schemas, stmt)
						}
					} else {
						schemas = append(schemas, stmt)
					}
				}
			}
			
			d := strings.TrimSpace(c.SampleData)
			if d != "" && !dataSet[d] {
				dataSet[d] = true
				d = insertRegex.ReplaceAllString(d, "INSERT IGNORE INTO ")
				datas = append(datas, d)
			}
		}
	}
	
	schemaRes := ""
	if len(schemas) > 0 {
		schemaRes = strings.Join(schemas, ";\n") + ";"
	}
	return schemaRes, strings.Join(datas, "\n")
}

// ChallengePosition returns "n of m" indices for the current challenge.
func (e *Engine) ChallengePosition() (int, int) {
	abs := 0
	for i := 0; i < e.ChapterIndex; i++ {
		abs += len(e.Story.Chapters[i].Challenges)
	}
	abs += e.ChallengeIndex + 1
	return abs, totalChallenges(e.Story)
}

func totalChallenges(s Story) int {
	n := 0
	for _, c := range s.Chapters {
		n += len(c.Challenges)
	}
	return n
}

// JumpToChapter moves to the first challenge of a named chapter.
func (e *Engine) JumpToChapter(id string) error {
	for i, ch := range e.Story.Chapters {
		if ch.ID == id {
			e.ChapterIndex = i
			e.ChallengeIndex = 0
			e.StartedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("chapter %q not found", id)
}

// JumpToChallengeAbsolute jumps to a 0-based challenge index across chapters.
func (e *Engine) JumpToChallengeAbsolute(idx int) error {
	total := totalChallenges(e.Story)
	if idx < 0 || idx >= total {
		return fmt.Errorf("challenge %d out of range (0..%d)", idx, total-1)
	}
	remaining := idx
	for ci := range e.Story.Chapters {
		if remaining < len(e.Story.Chapters[ci].Challenges) {
			e.ChapterIndex = ci
			e.ChallengeIndex = remaining
			e.StartedAt = time.Now()
			return nil
		}
		remaining -= len(e.Story.Chapters[ci].Challenges)
	}
	return fmt.Errorf("challenge %d not found", idx)
}

// NextChallenge advances the cursor. Returns false if already at the end.
func (e *Engine) NextChallenge() bool {
	ch := e.CurrentChapter()
	if ch == nil {
		return false
	}
	if e.ChallengeIndex+1 < len(ch.Challenges) {
		e.ChallengeIndex++
		e.resetChallenge()
		return true
	}
	if e.ChapterIndex+1 < len(e.Story.Chapters) {
		e.ChapterIndex++
		e.ChallengeIndex = 0
		e.resetChallenge()
		return true
	}
	return false
}

// PrevChallenge rewinds the cursor. Returns false if at the start.
func (e *Engine) PrevChallenge() bool {
	if e.ChallengeIndex > 0 {
		e.ChallengeIndex--
		e.resetChallenge()
		return true
	}
	if e.ChapterIndex > 0 {
		e.ChapterIndex--
		e.ChallengeIndex = len(e.Story.Chapters[e.ChapterIndex].Challenges) - 1
		e.resetChallenge()
		return true
	}
	return false
}

func (e *Engine) resetChallenge() {
	e.Attempts = 0
	e.FailedAttempts = 0
	e.hintsRevealed = 0
	e.StartedAt = time.Now()
	e.LastSubmission = nil
}

// Reset restarts the game from the beginning.
func (e *Engine) Reset() {
	e.ChapterIndex = 0
	e.ChallengeIndex = 0
	e.TotalXP = 0
	e.Streak = 0
	e.Attempts = 0
	e.FailedAttempts = 0
	e.hintsRevealed = 0
	e.CompletedSet = make(map[string]bool)
	e.StartedAt = time.Now()
	e.LastSubmission = nil
}

func (e *Engine) RevealNextHint() string {
	c := e.CurrentChallenge()
	if c == nil {
		return ""
	}
	if e.hintsRevealed >= len(c.Hints) {
		return ""
	}
	h := c.Hints[e.hintsRevealed]
	e.hintsRevealed++
	return h
}

// HintsRevealed returns how many hints the user has already seen.
func (e *Engine) HintsRevealed() int { return e.hintsRevealed }

// Submit runs the user's query, persists the attempt, and returns a Submission.
func (e *Engine) Submit(ctx context.Context, query string) Submission {
	c := e.CurrentChallenge()
	if c == nil {
		return Submission{Err: fmt.Errorf("no active challenge")}
	}
	now := time.Now()
	elapsed := now.Sub(e.StartedAt)
	e.Attempts++

	trimmed := strings.TrimSpace(query)
	schemaStr, dataStr := e.GetFullStorySchemaAndData()
	rr := parser.Run("", schemaStr, dataStr, trimmed, parser.DefaultQueryTimeout)
	sub := Submission{
		Query:     trimmed,
		Result:    rr,
		TimeTaken: elapsed,
		Timestamp: now,
	}
	if rr.Err != nil {
		e.FailedAttempts++
		e.Streak = 0
		sub.Err = rr.Err
		_ = e.store.AppendHistory(ctx, e.userID, c.ID, trimmed, rr.ExecMillis, false)
		_ = e.store.SaveAttempt(ctx, database.Progress{
			UserID:      e.userID,
			StoryID:     e.Story.ID,
			ChapterID:   e.CurrentChapter().ID,
			ChallengeID: c.ID,
			Completed:   false,
			Attempts:    e.Attempts,
			BestTime:    0,
			Score:       0,
			Timestamp:   now,
		})
		e.LastSubmission = &sub
		return sub
	}

	diff := parser.CompareResults(rr.Result, c.Validation)
	sub.Diff = diff
	sub.Matched = diff.Matched

	if diff.Matched {
		xp := scoreFor(*c, e.FailedAttempts, int(elapsed.Seconds()))
		sub.XPEarned = xp
		e.TotalXP += xp
		e.Streak++
		e.CompletedSet[c.ID] = true
		_ = e.store.AppendHistory(ctx, e.userID, c.ID, trimmed, rr.ExecMillis, true)
		_ = e.store.SaveAttempt(ctx, database.Progress{
			UserID:      e.userID,
			StoryID:     e.Story.ID,
			ChapterID:   e.CurrentChapter().ID,
			ChallengeID: c.ID,
			Completed:   true,
			Attempts:    e.Attempts,
			BestTime:    elapsed.Seconds(),
			Score:       xp,
			Timestamp:   now,
		})
		// Achievement unlocks.
		e.unlockMilestones(ctx)
	} else {
		e.FailedAttempts++
		e.Streak = 0
		_ = e.store.AppendHistory(ctx, e.userID, c.ID, trimmed, rr.ExecMillis, false)
		_ = e.store.SaveAttempt(ctx, database.Progress{
			UserID:      e.userID,
			StoryID:     e.Story.ID,
			ChapterID:   e.CurrentChapter().ID,
			ChallengeID: c.ID,
			Completed:   false,
			Attempts:    e.Attempts,
			BestTime:    0,
			Score:       0,
			Timestamp:   now,
		})
	}

	e.LastSubmission = &sub
	_ = e.firebase.SubmitAnalytics(ctx, database.AnalyticsEvent{
		Name:      "challenge_attempt",
		UserID:    e.userID,
		Timestamp: now.UTC().Format(time.RFC3339),
		Props: map[string]any{
			"story_id":     e.Story.ID,
			"challenge_id": c.ID,
			"matched":      sub.Matched,
			"exec_ms":      rr.ExecMillis,
			"attempts":     e.Attempts,
		},
	})
	return sub
}

func (e *Engine) unlockMilestones(ctx context.Context) {
	completed := len(e.CompletedSet)
	switch {
	case completed == 1:
		_ = e.store.UnlockAchievement(ctx, e.userID, "first_steps")
	case e.Streak >= 10:
		_ = e.store.UnlockAchievement(ctx, e.userID, "query_master")
	}
}

// GetAchievements returns the list of unlocked achievement IDs for the current user.
func (e *Engine) GetAchievements(ctx context.Context) []string {
	achievements, err := e.store.GetAchievements(ctx, e.userID)
	if err != nil {
		return []string{}
	}
	return achievements
}

// GetLeaderboard fetches the global leaderboard from Firebase.
func (e *Engine) GetLeaderboard(ctx context.Context, limit int) ([]database.LeaderboardEntry, error) {
	return e.firebase.GetLeaderboard(ctx, limit)
}

// GetPendingGames fetches a summary of all started stories.
func (e *Engine) GetPendingGames(ctx context.Context) ([]database.StoryProgressSummary, error) {
	return e.store.GetPendingGames(ctx, e.userID)
}

// ResumeStory sets the active story and skips to the first incomplete challenge.
func (e *Engine) ResumeStory(ctx context.Context, s Story) error {
	e.SetStory(s)
	for i, ch := range s.Chapters {
		for j, c := range ch.Challenges {
			done, _ := e.store.IsCompleted(ctx, e.userID, s.ID, c.ID)
			if !done {
				e.ChapterIndex = i
				e.ChallengeIndex = j
				e.StartedAt = time.Now()
				return nil
			}
			e.CompletedSet[c.ID] = true
		}
	}
	e.ChapterIndex = len(s.Chapters) - 1
	if e.ChapterIndex >= 0 {
		e.ChallengeIndex = len(s.Chapters[e.ChapterIndex].Challenges) - 1
	} else {
		e.ChapterIndex = 0
		e.ChallengeIndex = 0
	}
	e.StartedAt = time.Now()
	return nil
}

func scoreFor(c Challenge, failedAttempts, elapsedSeconds int) int {
	base := c.XP
	if base <= 0 {
		base = 100
	}
	bonus := 0
	if c.TimeLimit > 0 {
		remaining := c.TimeLimit - elapsedSeconds
		if remaining > 0 {
			bonus = remaining / 2
		}
	}
	penalty := 25 * failedAttempts
	xp := base + bonus - penalty
	if xp < 0 {
		xp = 0
	}
	return xp
}
