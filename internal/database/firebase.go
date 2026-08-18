// Package database: Firebase sync is stubbed for v1. The interface below lets
// the rest of the app depend on a stable contract; the real Firestore client
// can be wired in later by satisfying FirebaseClient.
package database

import (
	"context"
	"sync"
)

// FirebaseClient is the cloud-sync surface area. Implementations should be
// safe for concurrent use.
type FirebaseClient interface {
	SyncProgress(ctx context.Context, userID string, payload map[string]any) error
	GetLeaderboard(ctx context.Context, limit int) ([]LeaderboardEntry, error)
	GetStories(ctx context.Context) (map[string][]byte, error)
	SubmitAnalytics(ctx context.Context, event AnalyticsEvent) error
}

// LeaderboardEntry is a single row from a remote leaderboard.
type LeaderboardEntry struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	TotalXP     int    `json:"total_xp"`
	Rank        int    `json:"rank"`
}

// AnalyticsEvent is a generic event payload sent to the cloud.
type AnalyticsEvent struct {
	Name      string         `json:"name"`
	UserID    string         `json:"user_id"`
	Timestamp string         `json:"timestamp"`
	Props     map[string]any `json:"props"`
}

// LocalStub is a no-op FirebaseClient that lets the CLI run fully offline.
type LocalStub struct {
	mu      sync.Mutex
	events  []AnalyticsEvent
	enabled bool
}

// NewLocalStub returns a no-op Firebase client.
func NewLocalStub() *LocalStub { return &LocalStub{} }

// Enabled reports whether the stub (or a real client behind the same surface)
// has cloud sync turned on.
func (l *LocalStub) Enabled() bool {
	if l == nil {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.enabled
}

// SetEnabled lets the user toggle cloud sync from the config file.
func (l *LocalStub) SetEnabled(v bool) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.enabled = v
	l.mu.Unlock()
}

// SyncProgress is a no-op when the stub is disabled.
func (l *LocalStub) SyncProgress(_ context.Context, _ string, _ map[string]any) error { return nil }

// GetLeaderboard returns an empty list in offline mode.
func (l *LocalStub) GetLeaderboard(_ context.Context, _ int) ([]LeaderboardEntry, error) {
	return []LeaderboardEntry{}, nil
}

// GetStories returns an empty map; the embedded stories win.
func (l *LocalStub) GetStories(_ context.Context) (map[string][]byte, error) {
	return map[string][]byte{}, nil
}

// SubmitAnalytics records the event in memory only.
func (l *LocalStub) SubmitAnalytics(_ context.Context, e AnalyticsEvent) error {
	l.mu.Lock()
	l.events = append(l.events, e)
	l.mu.Unlock()
	return nil
}

// RecentEvents returns a copy of the buffered events (used by tests).
func (l *LocalStub) RecentEvents() []AnalyticsEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]AnalyticsEvent, len(l.events))
	copy(out, l.events)
	return out
}
