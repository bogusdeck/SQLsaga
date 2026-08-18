package game

import (
	"context"
	"fmt"
	"sort"

	"github.com/bogusdeck/sqlsaga/internal/database"
)

// ProgressSummary summarises a user's run for the stats view.
type ProgressSummary struct {
	ChallengesCompleted int
	TotalXP             int
	TotalAttempts       int
	BestStreak          int
	CompletedIDs        []string
	RemainingIDs        []string
}

// LoadProgress reads everything we know about a user from the local store.
func LoadProgress(ctx context.Context, s Story, store *database.Store, userID string) (ProgressSummary, error) {
	stats, err := store.LoadStats(ctx, userID)
	if err != nil {
		return ProgressSummary{}, err
	}
	allIDs := collectAllChallengeIDs(s)
	completed := map[string]bool{}
	for _, ch := range s.Chapters {
		for _, c := range ch.Challenges {
			done, err := store.IsCompleted(ctx, userID, s.ID, c.ID)
			if err != nil {
				return ProgressSummary{}, err
			}
			if done {
				completed[c.ID] = true
			}
		}
	}
	var doneList, remaining []string
	for _, id := range allIDs {
		if completed[id] {
			doneList = append(doneList, id)
		} else {
			remaining = append(remaining, id)
		}
	}
	sort.Strings(doneList)
	sort.Strings(remaining)
	return ProgressSummary{
		ChallengesCompleted: stats.ChallengesCompleted,
		TotalXP:             stats.TotalXP,
		TotalAttempts:       stats.TotalAttempts,
		CompletedIDs:        doneList,
		RemainingIDs:        remaining,
	}, nil
}

func collectAllChallengeIDs(s Story) []string {
	var ids []string
	for _, ch := range s.Chapters {
		for _, c := range ch.Challenges {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// ResetProgress wipes all stored state for the current user.
func ResetProgress(ctx context.Context, store *database.Store, userID string) error {
	if err := store.ResetProgress(ctx, userID); err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	return nil
}
