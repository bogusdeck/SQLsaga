# Story Authoring Guide

This guide explains how to create new stories and challenges for SQL Saga.

## Quick Start

1. Copy `stories/mystery/story.json` to a new file in `internal/stories/stories/`
2. Edit the JSON following the schema below
3. Run `go build` - your story will be automatically embedded and available

## JSON Schema Reference

### Story Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique identifier (e.g., `mystery_artifact`) |
| `genre` | string | Yes | Category: `mystery`, `quest`, `sci-fi`, `horror`, etc. |
| `title` | string | Yes | Display name |
| `description` | string | Yes | Short blurb for story selector |
| `chapters` | array | Yes | At least one chapter |

### Chapter Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique within story (e.g., `chapter_1`) |
| `title` | string | Yes | Display name |
| `narrative` | string | Yes | Story text shown to player |
| `prerequisite` | string | No | Chapter ID that must be completed first |
| `challenges` | array | Yes | At least one challenge |

### Challenge Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | Yes | Unique within story (e.g., `c1_1`) |
| `type` | string | Yes | Theme tag: `investigation`, `catalog`, `tracking`, etc. |
| `narrative` | string | Yes | Flavor text for this challenge |
| `schema` | string | Yes | SQLite CREATE TABLE statements |
| `sample_data` | string | Yes | SQLite INSERT statements |
| `objective` | string | Yes | What the player must query |
| `solution` | string | Yes | Example correct query |
| `hints` | array | No | Progressive hints (2-3 recommended) |
| `xp` | integer | No | Base XP reward (default: 100) |
| `time_limit` | integer | No | Seconds for time bonus (default: 120) |
| `validation` | object | Yes | Expected output specification |

### Validation Object

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `expected_columns` | array | Yes | Column names in order |
| `expected_rows` | array | Yes | Expected row objects |
| `allow_order` | boolean | No | Ignore row order (default: true) |
| `allow_extra_rows` | boolean | No | Allow more rows than expected (default: false) |

## Example: Minimal Challenge

```json
{
  "id": "c1_1",
  "type": "investigation",
  "narrative": "Find all users who logged in today.",
  "schema": "CREATE TABLE logins (id INTEGER PRIMARY KEY, user TEXT, login_time TEXT);",
  "sample_data": "INSERT INTO logins VALUES (1, 'alice', '2024-01-15 10:00:00'), (2, 'bob', '2024-01-15 14:30:00'), (3, 'charlie', '2024-01-14 09:00:00');",
  "objective": "Select all users who logged in on 2024-01-15.",
  "solution": "SELECT user FROM logins WHERE login_time LIKE '2024-01-15%';",
  "hints": [
    "Use LIKE to match date prefixes",
    "The date format is YYYY-MM-DD HH:MM:SS"
  ],
  "xp": 100,
  "time_limit": 120,
  "validation": {
    "expected_columns": ["user"],
    "expected_rows": [
      {"user": "alice"},
      {"user": "bob"}
    ],
    "allow_order": true,
    "allow_extra_rows": false
  }
}
```

## Supported SQLite Features

The game uses modernc.org/sqlite (pure Go SQLite). Supported:

- SELECT, WHERE, ORDER BY, LIMIT, OFFSET
- JOIN (INNER, LEFT, RIGHT, CROSS)
- GROUP BY, HAVING
- Aggregate functions: COUNT, SUM, AVG, MIN, MAX
- Subqueries, CTEs (WITH clause)
- Window functions: ROW_NUMBER, RANK, LAG, LEAD
- Date/time functions: date(), datetime(), strftime()
- CASE expressions
- IN, BETWEEN, LIKE, GLOB
- UNION, INTERSECT, EXCEPT

Not supported (read-only mode):
- INSERT, UPDATE, DELETE
- CREATE, DROP, ALTER
- PRAGMA, VACUUM, ATTACH

## Testing Your Story

```bash
# Validate story loads
go build -o sqlsaga ./cmd/sqlsaga
./sqlsaga -story your_story_id -stats

# Test a specific challenge
echo "YOUR QUERY HERE" | ./sqlsaga -validate /dev/stdin -story your_story_id -chapter chapter_1

# Play through
./sqlsaga -story your_story_id
```

## Difficulty Progression Guidelines

| Chapter | Concepts | XP Range | Time Limit |
|---------|----------|----------|------------|
| 1 | SELECT, WHERE, basic filters | 100-150 | 120-180s |
| 2 | ORDER BY, LIMIT, basic JOINs | 150-200 | 180-240s |
| 3 | GROUP BY, aggregates, HAVING | 200-250 | 240-300s |
| 4 | Subqueries, CTEs, window functions | 250-350 | 300-400s |
| 5 | Complex multi-table, optimization | 350-500 | 400-600s |

## Common Patterns

### Time Range Queries
```sql
-- Between dates
WHERE visit_time BETWEEN '2024-01-15 14:00:00' AND '2024-01-15 15:00:00'

-- Date prefix
WHERE login_time LIKE '2024-01-15%'
```

### Join Pattern
```sql
SELECT o.order_id, c.name
FROM orders o
JOIN customers c ON o.customer_id = c.id
WHERE o.status = 'shipped'
```

### Aggregation Pattern
```sql
SELECT category, COUNT(*) AS cnt, AVG(price) AS avg_price
FROM products
GROUP BY category
HAVING COUNT(*) > 5
ORDER BY cnt DESC
```

### CTE Pattern
```sql
WITH ranked AS (
  SELECT *, ROW_NUMBER() OVER (PARTITION BY category ORDER BY price DESC) AS rn
  FROM products
)
SELECT * FROM ranked WHERE rn <= 3
```

## Embedding Stories

Stories in `internal/stories/stories/` are embedded at compile time via:

```go
//go:embed all:stories
var FS embed.FS
```

The `LoadAllStories()` function walks this filesystem and loads all `.json` files.

## Debugging Validation Failures

If a query passes but validation fails:

1. Run with `-validate` to see the diff
2. Check column order matches `expected_columns`
3. Ensure no extra columns in SELECT
4. Verify data types match (integers vs strings)
5. Check `allow_order` and `allow_extra_rows` settings

## Publishing

Stories are embedded in the binary. To distribute a new story, rebuild and release.