<div align="center">
  <table>
    <tr>
      <td>
        <pre>
███████╗ ██████╗ ██╗     ███████╗ █████╗  ██████╗  █████╗ 
██╔════╝██╔═══██╗██║     ██╔════╝██╔══██╗██╔════╝ ██╔══██╗
███████╗██║   ██║██║     ███████╗███████║██║  ███╗███████║
╚════██║██║▄▄ ██║██║     ╚════██║██╔══██║██║   ██║██╔══██║
███████║╚██████╔╝███████╗███████║██║  ██║╚██████╔╝██║  ██║
╚══════╝ ╚═▀▀═╝ ╚══════╝╚══════╝╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝
        </pre>
      </td>
      <td>
        <img width="200" src="https://github.com/user-attachments/assets/aed8f76e-fe22-45d4-abb7-d8d3d50bfe90">
      </td>
    </tr>
  </table>
</div>



An interactive terminal game that teaches SQL through a narrative mystery story.

<img width="1470" height="956" alt="Screenshot 2026-08-19 at 3 14 32 PM" src="https://github.com/user-attachments/assets/3c3973d7-4d9b-4af8-9bfe-651c97f75809" />


## Features

- **Story-driven learning**: Progress through a mystery narrative by writing SQL queries
- **Real SQLite execution**: Queries run against an in-memory SQLite database with the challenge's schema and sample data
- **Instant feedback**: Results compared against expected output with detailed diffs
- **Hints system**: Reveal progressive hints when stuck (Ctrl+H)
- **Progress tracking**: Local SQLite database tracks XP, streaks, attempts, and completion
- **Cross-platform TUI**: Built with Bubble Tea (Charmbracelet) - runs on Linux, macOS, Windows

## Installation

### Homebrew (macOS / Linux)
```bash
brew install bogusdeck/sqlsaga/sqlsaga
```

### APT (Debian / Ubuntu) - via GitHub Pages
```bash
# Add the repository
echo "deb [trusted=yes] https://bogusdeck.github.io/sqlsaga stable main" | sudo tee /etc/apt/sources.list.d/sqlsaga.list

# Update and install
sudo apt update && sudo apt install sqlsaga
```

### Manual .deb (Debian / Ubuntu)
```bash
wget https://github.com/bogusdeck/sqlsaga/releases/latest/download/sqlsaga_linux_amd64.deb
sudo dpkg -i sqlsaga_linux_amd64.deb
# Or with apt (handles dependencies)
sudo apt install ./sqlsaga_linux_amd64.deb
```

### dnf / yum (Fedora / RHEL / CentOS)
```bash
wget https://github.com/bogusdeck/sqlsaga/releases/latest/download/sqlsaga_linux_amd64.rpm
sudo dnf install sqlsaga_linux_amd64.rpm
```

### apk (Alpine Linux)
```bash
wget https://github.com/bogusdeck/sqlsaga/releases/latest/download/sqlsaga_linux_amd64.apk
apk add --allow-untrusted sqlsaga_linux_amd64.apk
```

### Manual (all platforms)
Download from [releases](https://github.com/bogusdeck/sqlsaga/releases):
- **macOS**: `sqlsaga_darwin_amd64.tar.gz` (Intel) or `sqlsaga_darwin_arm64.tar.gz` (Apple Silicon)
- **Linux**: `sqlsaga_linux_amd64.tar.gz` (x86_64) or `sqlsaga_linux_arm64.tar.gz` (ARM64)
- **Windows**: `sqlsaga_windows_amd64.zip` or `sqlsaga_windows_arm64.zip`

```bash
# Example Linux/macOS
tar -xzf sqlsaga_*.tar.gz
sudo mv sqlsaga /usr/local/bin/
```

### From source
```bash
git clone https://github.com/bogusdeck/sqlsaga.git
cd sqlsaga
go build -o sqlsaga ./cmd/sqlsaga
```

### Go install
```bash
go install github.com/bogusdeck/sqlsaga/cmd/sqlsaga@latest
```

## Quick Start

```bash
# Start the game
sqlsaga

# Start a specific story
sqlsaga -story mystery

# Jump to a specific chapter
sqlsaga -chapter chapter_1

# View your stats
sqlsaga -stats

# Reset progress
sqlsaga -reset
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
-challenge string   challenge id to jump to (optional, within current chapter)
-reset              wipe local progress before starting
-stats              print current stats and exit
-leaderboard        print cloud leaderboard (stubbed offline)
-export string      export progress to JSON file
-import string      import progress from JSON file
-run                force TUI to start even if other flags set
-submit string      submit a story JSON file for local use
-install string     install a story JSON file locally permanently
-validate string    validate a SQL file against current challenge
-version            print version and exit
```

## Development

### Running Tests

```bash
go test ./...
```

### Project Structure

```
cmd/sqlsaga/           # CLI entrypoint
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

Config stored at `~/.sqlsaga/config.yaml`:

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

Progress database at `~/.sqlsaga/sqlsaga.db`.

## License

MIT
