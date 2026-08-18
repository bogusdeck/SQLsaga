// SQL Quest is an interactive terminal game that teaches SQL through a
// narrative mystery. This is the CLI entrypoint.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/bogusdeck/sqlquest/internal/database"
	"github.com/bogusdeck/sqlquest/internal/game"
	"github.com/bogusdeck/sqlquest/internal/parser"
	"github.com/bogusdeck/sqlquest/internal/tui"
	"github.com/bogusdeck/sqlquest/internal/utils"
)

// Build-time variables (set via -ldflags)
var (
	version   = "dev"
	buildDate = "unknown"
	gitCommit = "unknown"
)

var (
	flagStory      = flag.String("story", "mystery", "story id or genre (e.g. mystery, quest)")
	flagChapter    = flag.String("chapter", "", "chapter id to jump to (optional)")
	flagChallenge  = flag.String("challenge", "", "challenge id to jump to (optional, within current chapter)")
	flagReset      = flag.Bool("reset", false, "wipe local progress before starting")
	flagStats      = flag.Bool("stats", false, "print current stats and exit")
	flagLeader     = flag.Bool("leaderboard", false, "print the cloud leaderboard and exit (stubbed offline)")
	flagExport     = flag.String("export", "", "export progress to a JSON file and exit")
	flagImport     = flag.String("import", "", "import progress from a JSON file and exit")
	flagDevRun     = flag.Bool("run", false, "force the TUI to start even if other flags are set")
	flagValidate   = flag.String("validate", "", "validate a SQL file against a challenge and exit (path to .sql)")
	flagVersion    = flag.Bool("version", false, "print version and exit")
)

func main() {
	flag.Usage = usage
	flag.Parse()

	if *flagVersion {
		fmt.Printf("sqlquest version %s\n", version)
		fmt.Printf("  build date: %s\n", buildDate)
		fmt.Printf("  git commit: %s\n", gitCommit)
		return
	}

	cfg, err := utils.Load()
	if err != nil {
		fail("config", err)
	}

	dbPath, err := utils.DBPath()
	if err != nil {
		fail("db path", err)
	}
	store, err := database.Open(dbPath)
	if err != nil {
		fail("open db", err)
	}
	defer store.Close()

	if *flagReset && !*flagStats && *flagExport == "" && *flagImport == "" && *flagValidate == "" {
		if err := store.ResetProgress(context.Background(), cfg.DeviceID); err != nil {
			fail("reset", err)
		}
		fmt.Println("Progress reset for", cfg.DeviceID)
	}

	if *flagStats {
		printStats(context.Background(), store, cfg.DeviceID)
		return
	}
	if *flagExport != "" {
		if err := exportProgress(*flagExport, store, cfg.DeviceID); err != nil {
			fail("export", err)
		}
		return
	}
	if *flagImport != "" {
		if err := importProgress(*flagImport, store, cfg.DeviceID); err != nil {
			fail("import", err)
		}
		fmt.Println("Imported progress from", *flagImport)
		return
	}
	if *flagLeader {
		fb := database.NewLocalStub()
		board, _ := fb.GetLeaderboard(context.Background(), 10)
		fmt.Println("Leaderboard is offline-only in this build.")
		for i, e := range board {
			fmt.Printf("%2d. %s — %d XP\n", i+1, e.DisplayName, e.TotalXP)
		}
		return
	}
	if *flagValidate != "" {
		if err := runValidate(*flagValidate, *flagStory, *flagChapter, store, cfg.DeviceID); err != nil {
			fail("validate", err)
		}
		return
	}

	// TUI mode.
	story, err := game.LoadStory(*flagStory)
	if err != nil {
		fail("load story", err)
	}
	fb := database.NewLocalStub()
	fb.SetEnabled(cfg.SyncEnabled)

	engine := game.NewEngine(story.Story, store, fb, cfg.DeviceID)
	if *flagChapter != "" {
		if err := engine.JumpToChapter(*flagChapter); err != nil {
			fail("jump to chapter", err)
		}
	} else if *flagChallenge != "" {
		idx, err := challengeIndexByID(story.Story, *flagChallenge)
		if err != nil {
			fail("jump to challenge", err)
		}
		if err := engine.JumpToChallengeAbsolute(idx); err != nil {
			fail("jump to challenge", err)
		}
	} else {
		// Auto-resume to the first incomplete challenge
		engine.ResumeStory(context.Background(), story.Story)
	}

	

	app := tui.NewApp(engine, cfg)
	app.EnsureCurrentChallenge()
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fail("tui", err)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, "sqlquest — interactive SQL learning game\n\n")
	fmt.Fprintf(os.Stderr, "Usage:\n  sqlquest [flags]\n\nFlags:\n")
	flag.PrintDefaults()
}

func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "sqlquest: %s: %v\n", what, err)
	os.Exit(1)
}

func printStats(ctx context.Context, store *database.Store, userID string) {
	st, err := store.LoadStats(ctx, userID)
	if err != nil {
		fail("stats", err)
	}
	fmt.Printf("Device:      %s\n", userID)
	fmt.Printf("Completed:   %d challenges\n", st.ChallengesCompleted)
	fmt.Printf("Total XP:    %d\n", st.TotalXP)
	fmt.Printf("Attempts:    %d\n", st.TotalAttempts)
}

func exportProgress(path string, store *database.Store, userID string) error {
	ctx := context.Background()
	data, err := store.ExportProgress(ctx, userID)
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func importProgress(path string, store *database.Store, userID string) error {
	ctx := context.Background()
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var payload struct {
		UserID   string                   `json:"user_id"`
		Progress []map[string]any         `json:"progress"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	return store.ImportProgress(ctx, userID, payload.Progress)
}

func challengeIndexByID(s game.Story, id string) (int, error) {
	idx := 0
	for _, ch := range s.Chapters {
		for _, c := range ch.Challenges {
			if c.ID == id {
				return idx, nil
			}
			idx++
		}
	}
	return 0, fmt.Errorf("challenge %q not found", id)
}

func runValidate(sqlPath, storyID, chapterID string, store *database.Store, userID string) error {
	_ = context.Background()
	story, err := game.LoadStory(storyID)
	if err != nil {
		return err
	}
	engine := game.NewEngine(story.Story, store, database.NewLocalStub(), userID)
	if chapterID != "" {
		if err := engine.JumpToChapter(chapterID); err != nil {
			return err
		}
	}
	challenge := engine.CurrentChallenge()
	if challenge == nil {
		return errors.New("no challenge to validate")
	}
	f, err := os.Open(sqlPath)
	if err != nil {
		return err
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		return err
	}
	query := strings.TrimSpace(string(raw))
	if query == "" {
		return errors.New("validation file is empty")
	}
	rr := parser.Run("", challenge.Schema, challenge.SampleData, query, parser.DefaultQueryTimeout)
	if rr.Err != nil {
		return fmt.Errorf("query error: %w", rr.Err)
	}
	diff := parser.CompareResults(rr.Result, challenge.Validation)
	if diff.Matched {
		fmt.Println("✓ Query matches expected output.")
		fmt.Printf("  exec: %.2f ms  rows: %d\n", rr.ExecMillis, len(rr.Result.Rows))
		fmt.Printf("  XP: %s\n", strconv.Itoa(challenge.XP))
		return nil
	}
	fmt.Println("✗ Query does not match expected output.")
	fmt.Println(diff.String())
	os.Exit(2)
	return nil
}
