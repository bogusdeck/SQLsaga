# SQL Saga

An interactive terminal game that teaches SQL through a narrative mystery story.

## Features

- **Story-driven learning**: Progress through a mystery narrative by writing SQL queries
- **Real SQLite execution**: Queries run against an in-memory SQLite database with the challenge's schema and sample data
- **Instant feedback**: Results compared against expected output with detailed diffs
- **Hints system**: Reveal progressive hints when stuck (Ctrl+H)
- **Progress tracking**: Local SQLite database tracks XP, streaks, attempts, and completion
- **Cross-platform TUI**: Built with Bubble Tea (Charmbracelet) - runs on Linux, macOS, Windows

## Installation

```bash
# From source
git clone https://github.com/bogusdeck/sqlsaga.git
cd sqlsaga
go build -o sqlsaga ./cmd/sqlquest

# Or install directly (when released)
go install github.com/bogusdeck/sqlsaga/cmd/sqlquest@latest
```

## Quick Start

```bash
# Start the game
./sqlquest

# Start a specific story
./sqlquest -story mystery

# Jump to a specific chapter
./sqlquest -chapter chapter_1

# View your stats
./sqlquest -stats

# Reset progress
./sqlquest -reset
```

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| Ctrl+S | Submit query |
| Ctrl+R | Reset editor |
| Ctrl+H | Reveal next hint |
| Ctrl+N | Next challenge |
| Ctrl+P | Previous challenge |
| Ctrl+E | Toggle EXPLAIN QUERY PLAN |
| Ctrl+G | Show stats overlay |
| ? | Toggle help overlay |
| Q / Ctrl+C | Quit |

## Game Structure

- **Stories**: Top-level narrative arcs (e.g., "The Mystery of the Missing Artifact")
- **Chapters**: Group challenges with shared narrative context
- **Challenges**: Individual SQL puzzles with:
  - Narrative context
  - Database schema
  - Sample data
  - Objective (what to query)
  - Expected output for validation
  - Hints (progressive)
  - XP reward

## Current Content

- **Mystery**: "The Mystery of the Missing Artifact" (1 chapter, 3 challenges)
  - Chapter 1: The Crime Scene
    - Challenge 1: Time-range filtering (WHERE + BETWEEN)
    - Challenge 2: Sorting and limiting (ORDER BY + LIMIT)
    - Challenge 3: Grouping and aggregation (GROUP BY + COUNT)

## CLI Flags

```
-story string       story id or genre (default "mystery")
-chapter string     chapter id to jump to
-challenge string   challenge id to jump to
-reset              wipe local progress before starting
-stats              print current stats and exit
-leaderboard        print cloud leaderboard (stubbed offline)
-export string      export progress to JSON file
-import string      import progress from JSON file
-run                force TUI to start even if other flags set
-validate string    validate a SQL file against current challenge
```

## Development

### Running Tests

```bash
go test ./...
```

### Project Structure

```
cmd/sqlquest/           # CLI entrypoint
internal/
  game/                 # Game engine, story, progress, scoring
  parser/               # SQL execution, validation, diffing
  database/             # Local SQLite persistence + Firebase stub
  tui/                  # Bubble Tea TUI components
  stories/              # Embedded story JSON files
  utils/                # Config, paths
stories/                # Source story JSON files (embedded at build)
```

### Adding Stories

Stories are JSON files in `internal/stories/stories/` (embedded via `//go:embed`). See `stories/mystery/story.json` for the format.

Required fields:
- `id`, `genre`, `title`, `description`
- `chapters[]` with `id`, `title`, `narrative`, `challenges[]`
- Each challenge needs: `id`, `schema`, `sample_data`, `objective`, `validation.expected_columns`, `validation.expected_rows`

## Configuration

Config stored at `~/.sqlquest/config.yaml`:

```yaml
theme: "dark"
autocomplete: true
sync_enabled: false
device_id: "device-xxx"
editor:
  tab_width: 2
  show_line_numbers: true
  auto_indent: true
story_preferences:
  - "mystery"
```

Progress database at `~/.sqlquest/sqlquest.db`.

## License

MIT