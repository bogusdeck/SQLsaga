// Package game holds the SQL Quest story engine, scoring, and progression.
package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/bogusdeck/sqlsaga/internal/parser"
	"github.com/bogusdeck/sqlsaga/internal/stories"
	"github.com/bogusdeck/sqlsaga/internal/utils"
)

// Validation is re-exported from the parser package to keep story files simple.
type Validation = parser.Validation

// Challenge is one solvable puzzle inside a chapter.
type Challenge struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Narrative  string            `json:"narrative"`
	Schema     string            `json:"schema"`
	SampleData string            `json:"sample_data"`
	Objective  string            `json:"objective"`
	Solution   string            `json:"solution"`
	Hints      []string          `json:"hints"`
	XP         int               `json:"xp"`
	TimeLimit  int               `json:"time_limit"`
	Validation parser.Validation `json:"validation"`
}

// Chapter groups challenges with shared narrative context.
type Chapter struct {
	ID           string      `json:"id"`
	Title        string      `json:"title"`
	Narrative    string      `json:"narrative"`
	Challenges   []Challenge `json:"challenges"`
	Prerequisite string      `json:"prerequisite"`
}

// Story is a top-level narrative arc (e.g. a mystery).
type Story struct {
	ID          string    `json:"id"`
	Genre       string    `json:"genre"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Chapters    []Chapter `json:"chapters"`
}

// LoadedStory wraps a Story with a human-friendly label used in the HUD.
type LoadedStory struct {
	Story    Story
	FilePath string
}

// ErrNoStories is returned when no story files can be found.
var ErrNoStories = errors.New("no stories found in embedded bundle")

// LoadAllStories walks the embedded story filesystem and parses every story
// JSON file. It also loads locally submitted stories. The returned slice is sorted by story id.
func LoadAllStories() ([]LoadedStory, error) {
	var out []LoadedStory

	// 1. Load embedded stories
	err := fs.WalkDir(stories.FS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		data, err := fs.ReadFile(stories.FS, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var s Story
		if err := json.Unmarshal(data, &s); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if err := ValidateStory(&s); err != nil {
			return fmt.Errorf("validate %s: %w", path, err)
		}
		out = append(out, LoadedStory{Story: s, FilePath: path})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 2. Load local stories
	if localDir, err := utils.StoriesDir(); err == nil {
		localEntries, err := os.ReadDir(localDir)
		if err == nil {
			for _, d := range localEntries {
				if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
					continue
				}
				path := filepath.Join(localDir, d.Name())
				data, err := os.ReadFile(path)
				if err != nil {
					continue // Skip on error
				}
				var s Story
				if err := json.Unmarshal(data, &s); err != nil {
					continue
				}
				if err := ValidateStory(&s); err != nil {
					continue
				}
				out = append(out, LoadedStory{Story: s, FilePath: path})
			}
		}
	}

	if len(out) == 0 {
		return nil, ErrNoStories
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Story.ID < out[j].Story.ID })
	return out, nil
}

// LoadStory finds a story by id (or genre alias).
func LoadStory(id string) (*LoadedStory, error) {
	all, err := LoadAllStories()
	if err != nil {
		return nil, err
	}
	id = strings.ToLower(strings.TrimSpace(id))
	for i := range all {
		if all[i].Story.ID == id || all[i].Story.Genre == id {
			return &all[i], nil
		}
	}
	return nil, fmt.Errorf("story %q not found", id)
}

func ValidateStory(s *Story) error {
	if s.ID == "" {
		return errors.New("story id is required")
	}
	if s.Title == "" {
		return errors.New("story title is required")
	}
	if len(s.Chapters) == 0 {
		return errors.New("story has no chapters")
	}
	for ci, ch := range s.Chapters {
		if ch.ID == "" {
			return fmt.Errorf("chapter %d: id is required", ci)
		}
		if len(ch.Challenges) == 0 {
			return fmt.Errorf("chapter %s: no challenges", ch.ID)
		}
		for ki, c := range ch.Challenges {
			if c.ID == "" {
				return fmt.Errorf("chapter %s challenge %d: id is required", ch.ID, ki)
			}
			if c.Schema == "" {
				return fmt.Errorf("chapter %s challenge %s: schema is required", ch.ID, c.ID)
			}
			if c.SampleData == "" {
				return fmt.Errorf("chapter %s challenge %s: sample_data is required", ch.ID, c.ID)
			}
			if c.Objective == "" {
				return fmt.Errorf("chapter %s challenge %s: objective is required", ch.ID, c.ID)
			}
			if c.XP <= 0 {
				c.XP = 100
			}
			if c.TimeLimit <= 0 {
				c.TimeLimit = 120
			}
			if len(c.Validation.ExpectedColumns) == 0 {
				return fmt.Errorf("chapter %s challenge %s: expected_columns required", ch.ID, c.ID)
			}
			s.Chapters[ci].Challenges[ki] = c
		}
	}
	return nil
}

// DefaultTimeLimit is the soft deadline when a challenge omits one.
const DefaultTimeLimit = 120 * time.Second
