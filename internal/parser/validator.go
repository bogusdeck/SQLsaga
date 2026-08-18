// Package parser validates a user's SQL query against a challenge's expected
// output and rewrites raw errors as friendly, beginner-facing messages.
package parser

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Validation describes the expected output of a query. It mirrors the
// Challenge.Validation JSON shape in the story files.
type Validation struct {
	ExpectedColumns []string         `json:"expected_columns"`
	ExpectedRows    []map[string]any `json:"expected_rows"`
	AllowOrder      bool             `json:"allow_order"`
	AllowExtraRows  bool             `json:"allow_extra_rows"`
}

// Result captures the columns and rows returned by a query.
type Result struct {
	Columns []string
	Rows    []map[string]any
}

// RunResult bundles a Result with timing and an error if any.
type RunResult struct {
	Result     Result
	ExecMillis float64
	Err        error
	ReadOnly   bool // true if the query was rejected by the read-only guard
}

// Diff summarises the differences between a user result and the expected one.
type Diff struct {
	Matched         bool
	Reason          string
	MissingRows     []map[string]any
	ExtraRows       []map[string]any
	ColumnMismatch  bool
	ExpectedCols    []string
	ActualCols      []string
	RowCountMatch   bool
	ExpectedRowN    int
	ActualRowN      int
	FirstDifference string
}

// FriendlyErrors maps SQLite-style error fragments to human hints.
var FriendlyErrors = map[string]string{
	"syntax error":        "Almost! Check your SQL syntax. Did you miss a comma, quote, or keyword?",
	"no such table":       "Hmm, that table doesn't exist. Check the schema for the correct table name.",
	"no such column":      "That column doesn't exist. Check the schema for available columns.",
	"ambiguous column":    "That column name is ambiguous. Try specifying the table name too.",
	"misuse of aggregate": "You used an aggregate (COUNT, SUM, ...) outside a GROUP BY. Try adding a GROUP BY.",
	"near":                "Syntax issue near the highlighted token. Check spelling and punctuation.",
}

// forbidden leading verbs for the read-only guard. These are checked against
// the first non-whitespace, non-comment token (lowercased).
var forbiddenFirst = []string{
	"drop", "alter", "attach", "detach", "pragma", "vacuum", "reindex",
	"create", "insert", "update", "delete", "replace", "truncate", "grant", "revoke",
	"copy", "load", "savepoint", "release",
}

// IsReadOnly returns nil if the query is allowed; otherwise an error explaining why.
func IsReadOnly(query string) error {
	stripped := stripSQLComments(query)
	if strings.TrimSpace(stripped) == "" {
		return errors.New("query is empty")
	}
	first := firstToken(stripped)
	if first == "" {
		return errors.New("could not detect the first SQL keyword")
	}
	for _, bad := range forbiddenFirst {
		if first == bad {
			return fmt.Errorf("read-only mode: %s statements are not allowed", strings.ToUpper(bad))
		}
	}
	return nil
}

func stripSQLComments(q string) string {
	var b strings.Builder
	for i := 0; i < len(q); i++ {
		if i+1 < len(q) && q[i] == '-' && q[i+1] == '-' {
			for i < len(q) && q[i] != '\n' {
				i++
			}
			continue
		}
		if i+1 < len(q) && q[i] == '/' && q[i+1] == '*' {
			i += 2
			for i+1 < len(q) && !(q[i] == '*' && q[i+1] == '/') {
				i++
			}
			i++
			continue
		}
		b.WriteByte(q[i])
	}
	return b.String()
}

func firstToken(q string) string {
	fields := strings.FieldsFunc(q, func(r rune) bool {
		return !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'))
	})
	if len(fields) == 0 {
		return ""
	}
	return strings.ToLower(fields[0])
}

// DefaultQueryTimeout is how long a single user query is allowed to run.
const DefaultQueryTimeout = 3 * time.Second

// Run executes the user query inside a fresh in-memory database that already
// has the challenge's schema and sample data loaded.
func Run(dsn, schema, sampleData, userQuery string, timeout time.Duration) RunResult {
	start := time.Now()
	if err := IsReadOnly(userQuery); err != nil {
		return RunResult{Err: err, ExecMillis: elapsedMillis(start), ReadOnly: true}
	}
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/"
	}
	baseConn, err := sql.Open("mysql", dsn)
	if err != nil {
		return RunResult{Err: fmt.Errorf("open mysql: %w", err), ExecMillis: elapsedMillis(start)}
	}
	dbName := fmt.Sprintf("sqlquest_%d", time.Now().UnixNano())
	if _, err := baseConn.Exec("CREATE DATABASE " + dbName); err != nil {
		baseConn.Close()
		return RunResult{Err: fmt.Errorf("create temp db: %w", err), ExecMillis: elapsedMillis(start)}
	}
	defer func() {
		baseConn.Exec("DROP DATABASE " + dbName)
		baseConn.Close()
	}()
	
	dbDSN := dsn + dbName
	if !strings.HasSuffix(dsn, "/") {
		dbDSN = dsn + "/" + dbName
	}
	conn, err := sql.Open("mysql", dbDSN)
	if err != nil {
		return RunResult{Err: fmt.Errorf("open temp db: %w", err), ExecMillis: elapsedMillis(start)}
	}
	defer conn.Close()
	conn.SetMaxOpenConns(1)

	if err := execMulti(conn, schema); err != nil {
		return RunResult{Err: fmt.Errorf("schema: %w", err), ExecMillis: elapsedMillis(start)}
	}
	if err := execMulti(conn, sampleData); err != nil {
		return RunResult{Err: fmt.Errorf("sample data: %w", err), ExecMillis: elapsedMillis(start)}
	}

	if timeout <= 0 {
		timeout = DefaultQueryTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	rows, err := conn.QueryContext(ctx, userQuery)
	if err != nil {
		return RunResult{Err: friendly(err), ExecMillis: elapsedMillis(start)}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return RunResult{Err: err, ExecMillis: elapsedMillis(start)}
	}
	var out []map[string]any
	for rows.Next() {
		holders := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range holders {
			ptrs[i] = &holders[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return RunResult{Err: err, ExecMillis: elapsedMillis(start)}
		}
		row := make(map[string]any, len(cols))
		for i, h := range holders {
			row[cols[i]] = normalizeValue(h)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return RunResult{Err: err, ExecMillis: elapsedMillis(start)}
	}
	return RunResult{
		Result:     Result{Columns: cols, Rows: out},
		ExecMillis: elapsedMillis(start),
	}
}

func execMulti(conn *sql.DB, sqlText string) error {
	for _, stmt := range splitStatements(sqlText) {
		s := strings.TrimSpace(stmt)
		if s == "" {
			continue
		}
		if _, err := conn.Exec(s); err != nil {
			return fmt.Errorf("%w\n  in: %s", err, truncate(s, 80))
		}
	}
	return nil
}

func splitStatements(s string) []string {
	var (
		out      []string
		buf      strings.Builder
		inSingle bool
		inDouble bool
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case ';':
			if !inSingle && !inDouble {
				out = append(out, buf.String())
				buf.Reset()
				continue
			}
		}
		buf.WriteByte(c)
	}
	if strings.TrimSpace(buf.String()) != "" {
		out = append(out, buf.String())
	}
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func elapsedMillis(start time.Time) float64 {
	return float64(time.Since(start).Microseconds()) / 1000.0
}

func normalizeValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case int64:
		return int(x)
	case float64:
		return x
	default:
		return v
	}
}

func friendly(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	for k, v := range FriendlyErrors {
		if strings.Contains(msg, k) {
			return fmt.Errorf("%s\n\n(original: %s)", v, err.Error())
		}
	}
	return err
}

// CompareResults diffs the user's Result against a Validation spec.
func CompareResults(got Result, want Validation) Diff {
	diff := Diff{
		ExpectedCols: append([]string{}, want.ExpectedColumns...),
		ActualCols:   append([]string{}, got.Columns...),
	}

	if !sameColumnsCI(want.ExpectedColumns, got.Columns) {
		diff.ColumnMismatch = true
		diff.Reason = fmt.Sprintf("expected columns %v, got %v", want.ExpectedColumns, got.Columns)
		return diff
	}

	expected := want.ExpectedRows
	diff.ExpectedRowN = len(expected)
	diff.ActualRowN = len(got.Rows)
	diff.RowCountMatch = len(expected) == len(got.Rows)

	colOrder := want.ExpectedColumns
	sign := func(row map[string]any) string {
		parts := make([]string, len(colOrder))
		for i, c := range colOrder {
			parts[i] = fmt.Sprintf("%v", normalize(row[c]))
		}
		return strings.Join(parts, "\x1f")
	}
	expectedSet := map[string]map[string]any{}
	for _, r := range expected {
		expectedSet[sign(r)] = r
	}
	gotSet := map[string]map[string]any{}
	for _, r := range got.Rows {
		gotSet[sign(r)] = r
	}

	for k, r := range expectedSet {
		if _, ok := gotSet[k]; !ok {
			diff.MissingRows = append(diff.MissingRows, r)
		}
	}
	for k, r := range gotSet {
		if _, ok := expectedSet[k]; !ok {
			diff.ExtraRows = append(diff.ExtraRows, r)
		}
	}
	sortRows(diff.MissingRows, colOrder)
	sortRows(diff.ExtraRows, colOrder)

	if !want.AllowExtraRows && len(diff.ExtraRows) > 0 {
		diff.Reason = fmt.Sprintf("unexpected extra rows: %d", len(diff.ExtraRows))
	} else if len(diff.MissingRows) > 0 {
		diff.Reason = fmt.Sprintf("missing expected rows: %d", len(diff.MissingRows))
	} else {
		diff.Matched = true
	}

	if !diff.Matched && diff.FirstDifference == "" {
		if len(diff.ExtraRows) > 0 {
			diff.FirstDifference = fmt.Sprintf("extra row: %v", diff.ExtraRows[0])
		} else if len(diff.MissingRows) > 0 {
			diff.FirstDifference = fmt.Sprintf("missing row: %v", diff.MissingRows[0])
		}
	}
	return diff
}

func sameColumnsCI(want, got []string) bool {
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if !strings.EqualFold(want[i], got[i]) {
			return false
		}
	}
	return true
}

func normalize(v any) any {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(x)
	case int64:
		return float64(x)
	case int:
		return float64(x)
	case float64:
		return math.Round(x*1e6) / 1e6
	default:
		return x
	}
}

func sortRows(rows []map[string]any, cols []string) {
	sort.SliceStable(rows, func(i, j int) bool {
		for _, c := range cols {
			a := fmt.Sprintf("%v", normalize(rows[i][c]))
			b := fmt.Sprintf("%v", normalize(rows[j][c]))
			if a == b {
				continue
			}
			return a < b
		}
		return false
	})
}

// String renders a Diff for the results pane.
func (d Diff) String() string {
	if d.Matched {
		return "Result matches expected output."
	}
	var b strings.Builder
	if d.ColumnMismatch {
		b.WriteString("Columns differ.\n")
		b.WriteString(fmt.Sprintf("  expected: %s\n", strings.Join(d.ExpectedCols, ", ")))
		b.WriteString(fmt.Sprintf("  got:      %s\n", strings.Join(d.ActualCols, ", ")))
		return b.String()
	}
	if d.Reason != "" {
		b.WriteString(d.Reason + "\n")
	}
	if d.FirstDifference != "" {
		b.WriteString("  first diff: " + d.FirstDifference + "\n")
	}
	return b.String()
}

// Explain returns a textual EXPLAIN QUERY PLAN for the user's query.
func Explain(dsn, schema, sampleData, userQuery string) (string, error) {
	if err := IsReadOnly(userQuery); err != nil {
		return "", err
	}
	if dsn == "" {
		dsn = os.Getenv("MYSQL_DSN")
	}
	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/"
	}
	baseConn, err := sql.Open("mysql", dsn)
	if err != nil {
		return "", err
	}
	dbName := fmt.Sprintf("sqlquest_%d", time.Now().UnixNano())
	if _, err := baseConn.Exec("CREATE DATABASE " + dbName); err != nil {
		baseConn.Close()
		return "", err
	}
	defer func() {
		baseConn.Exec("DROP DATABASE " + dbName)
		baseConn.Close()
	}()
	
	dbDSN := dsn + dbName
	if !strings.HasSuffix(dsn, "/") {
		dbDSN = dsn + "/" + dbName
	}
	conn, err := sql.Open("mysql", dbDSN)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if err := execMulti(conn, schema); err != nil {
		return "", err
	}
	if err := execMulti(conn, sampleData); err != nil {
		return "", err
	}
	rows, err := conn.Query("EXPLAIN " + userQuery)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	var b strings.Builder
	holders := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range holders {
		ptrs[i] = &holders[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return "", err
		}
		parts := make([]string, len(cols))
		for i, h := range holders {
			parts[i] = fmt.Sprintf("%v", normalizeValue(h))
		}
		b.WriteString(strings.Join(parts, " | ") + "\n")
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return b.String(), nil
}

// TestConnection attempts to ping the provided MySQL DSN to verify connectivity.
func TestConnection(dsn string) error {
	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/"
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Ping()
}
