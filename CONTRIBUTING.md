# Contributing to SQL Quest

Thank you for your interest in contributing! This document explains how to contribute to the project.

## Development Setup

### Prerequisites

- Go 1.25 or later
- Make (optional, for convenience)

### Getting Started

```bash
# Clone the repository
git clone https://github.com/bogusdeck/sqlsaga.git
cd sqlquest

# Download dependencies
go mod download

# Build
go build -o sqlquest ./cmd/sqlquest

# Run tests
go test ./...

# Run linter
go vet ./...

# Run the game
./sqlquest
```

## Project Structure

```
sqlquest/
├── cmd/sqlquest/          # CLI entrypoint
├── internal/
│   ├── database/          # SQLite persistence + Firebase stub
│   ├── game/              # Game engine, story, progress, scoring
│   ├── parser/            # SQL execution, validation, diffing
│   ├── parser/            # SQL execution, validation, diffing
│   ├── tui/               # Bubble Tea TUI components
│   │   └── components/    # Reusable TUI widgets
│   ├── stories/           # Embedded story JSON files
│   └── utils/             # Config, paths, device ID
├── stories/               # Source story JSON files (embedded at build)
├── .github/workflows/     # CI/CD pipelines
└── go.mod / go.sum        # Go module definition
```

## Adding a New Story

Stories are JSON files embedded at compile time via `//go:embed`. To add a new story:

1. Create a new JSON file in `internal/stories/stories/` (e.g., `mystery2.json`)
2. Follow the story schema (see below)
3. Rebuild - the story will be automatically available

### Story Schema

```json
{
  "id": "unique_story_id",
  "genre": "mystery|quest|sci-fi",
  "title": "Story Title",
  "description": "Brief description for the story selector",
  "chapters": [
    {
      "id": "chapter_1",
      "title": "Chapter Title",
      "narrative": "Story text setting the scene",
      "prerequisite": "previous_chapter_id",
      "challenges": [
        {
          "id": "c1_1",
          "type": "investigation|catalog|tracking",
          "narrative": "Context for this specific challenge",
          "schema": "CREATE TABLE ...;",
          "sample_data": "INSERT INTO ...;",
          "objective": "What the player should query",
          "solution": "SELECT ...;",
          "hints": ["Hint 1", "Hint 2", "Hint 3"],
          "xp": 100,
          "time_limit": 120,
          "validation": {
            "expected_columns": ["col1", "col2"],
            "expected_rows": [{"col1": "val1", "col2": "val2"}],
            "allow_order": true,
            "allow_extra_rows": false
          }
        }
      ]
    }
  ]
}
```

### Validation Rules

- `id`, `title`, `genre`, `description` required at story level
- At least one chapter required
- Each chapter needs `id`, `title`, `narrative`, and at least one challenge
- Each challenge needs:
  - `id`, `schema`, `sample_data`, `objective` (all required)
  - `validation.expected_columns` and `validation.expected_rows` (required)
  - `xp` > 0 (defaults to 100)
  - `time_limit` > 0 (defaults to 120 seconds)
  - `hints` array (can be empty)

### Tips for Good Stories

1. **Progressive difficulty**: Start with simple SELECT/WHERE, add ORDER BY/LIMIT, then GROUP BY, then JOINs, then CTEs/window functions
2. **Narrative integration**: Each challenge should advance the story
3. **Realistic data**: Use plausible sample data that tells a story
4. **Clear objectives**: Player should know exactly what to query
5. **Hints**: 2-3 progressive hints per challenge

## Code Style

- Follow standard Go formatting (`gofmt`)
- Run `go vet ./...` before committing
- Write tests for new functionality
- Keep functions small and focused
- Use meaningful variable names

## Testing

```bash
# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run tests for a specific package
go test -v ./internal/parser/...
```

## Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Run tests and linter (`go test ./... && go vet ./...`)
5. Commit your changes (`git commit -m 'Add amazing feature'`)
6. Push to the branch (`git push origin feature/amazing-feature`)
7. Open a Pull Request

## Reporting Issues

Use the GitHub issue tracker. Please include:

- Go version (`go version`)
- OS and architecture
- Steps to reproduce
- Expected vs actual behavior
- Any relevant logs or screenshots

## License

By contributing, you agree that your contributions will be licensed under the MIT License.