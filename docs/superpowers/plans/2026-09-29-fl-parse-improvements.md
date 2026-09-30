# fl-parse Correctness, Efficiency & Output Improvements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the agreed correctness bugs in fl-parse, overlap Decomp extraction with CSV conversion, speed up row filtering, centralise configuration, and produce clear per-table output with a meaningful exit code.

**Architecture:** Split the monolithic `openSchema()` into small, testable units: `config` (one source of truth for paths/flags), `schema` (pure parser), `csv` (single-file converter returning errors and stats), `querylist` (generic worker pool + Decomp processor). `main` only wires them together, prints results from a single goroutine, and sets the exit code. Each worker runs Decomp then converts *its own* table's file, so extraction and conversion overlap and stale `.txt` files are never picked up.

**Tech Stack:** Go (module `fl-parse`, `go 1.18` in go.mod, so stdlib only, no features newer than 1.18), `testing`, `flag`, `os/exec`.

**Spec:** The code review findings from this conversation (2026-09-29). Items in scope, using the review's numbering:
- Bugs #1 (last table dropped), #2 (schema open failure ignored), #4 (64 KB line limit), #5 (input handle leak), #6 (flush/write errors ignored), #8 (6 workers instead of 5), #9 (WaitGroup Add/Done race).
- All **Efficiency** findings: regex per row, per-row filter selection, extraction/conversion not overlapped, hard-coded worker count, `Split(" ")` -> `Fields`.
- **Maintainability:** `queryCount` correctness and `textOutputPath` triplication.
- **Better output:** per-table results, summary, elapsed time, non-zero exit code, no interleaved worker output.

Explicitly **out of scope** (do not do): #7 table-detection heuristic (`"Disc"` match stays as is), the stale backtick/`-` comment, go.mod bump, `ioutil` cleanup beyond files we rewrite anyway, `.idea/` / `.gitignore`, output encoding.

## Global Constraints

- Default paths and worker count must stay exactly as today: schema `C:\FED_LOG\TOOLS\SCHEMA.txt`, text dir `C:\db_texts`, csv dir `C:\db_csv`, Decomp `C:\FED_LOG\TOOLS\UTILITIES\Decomp.exe`, IMDLST dir `C:\FED_LOG`, workers `5`. Running with no flags behaves as before.
- Decomp is invoked as `Decomp.exe <imdlstDir> "select <cols comma-joined> FROM <TableName>" <textDir>\<lowername-without-2-char-prefix>.txt`. Do not change this contract.
- Output file naming unchanged: lowercase table name with the first 2 characters removed, `.txt` then `.csv`.
- Row-filter semantics must not change: `part_number` drops rows whose first field is not entirely ASCII digits (empty counts as invalid); `colloquial_name` drops rows where field index 2 exists and is empty/whitespace-only. All other tables are unfiltered. Blank lines are skipped and not counted in Total.
- Schema parsing semantics must not change except: the final table is flushed at EOF (bug #1), and whitespace-only lines are skipped instead of producing an empty column name/panicking under `Fields`.
- Tests must run on macOS/Linux with no Windows paths and no real Decomp; use `t.TempDir()`. Code must still `GOOS=windows go build`.
- Commit after each task. Commit messages end with `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>`.

**User decisions (already made):** Implement all Efficiency findings, bugs #1, #2, #4, #5, #6, #8, #9, plus "Better output", `queryCount`, and `textOutputPath` from Maintainability.

---

## File Structure

| File | Status | Responsibility |
|---|---|---|
| `config/config.go` | create | `Config` struct, `Default()`, `Parse(args)` (flags). Single source of all paths/worker count. |
| `config/config_test.go` | create | Defaults + flag override + validation tests. |
| `schema/schema.go` | create | `Table` type, `Parse(io.Reader)`: pure parser of SCHEMA.txt. |
| `schema/schema_test.go` | create | Parser tests, including no trailing blank line. |
| `csv/csvwriter.go` | rewrite | `Convert(name, textDir, csvDir)`, `Stats`, filters, `isDigits`. Returns errors, no panics. |
| `csv/csvwriter_test.go` | create | Converter tests. |
| `querylist/querylist.go` | rewrite | `Run` (generic worker pool), `NewProcessor(cfg)` (Decomp + convert), `FileBase`, `Result`. |
| `querylist/querylist_test.go` | create | Pool tests (worker count, all jobs run, race-free), `FileBase` tests. |
| `main.go` | rewrite | `main` -> `run(args, stdout, stderr) int`; wiring, output, exit code. |
| `scripts/fake-decomp.sh` | create | Fake Decomp for the end-to-end smoke test on macOS/Linux. |

Dependency order: config -> schema -> csv -> querylist -> main.

---

### Task 1: Central configuration (`textOutputPath` triplication)

**Goal:** One `config.Config` holds every path and the worker count, overridable by flags, replacing the three duplicated `textOutputPath` vars.

**Files:**
- Create: `config/config.go`
- Test: `config/config_test.go`

**Acceptance Criteria:**
- [ ] `config.Default()` returns exactly the current hard-coded values and `Workers == 5`.
- [ ] `config.Parse([]string{"-workers","3","-text-dir","/x"})` overrides only those fields.
- [ ] `Parse` returns an error for `-workers 0` and for unknown flags (no `os.Exit`, no output to stdout).

**Verify:** `go test ./config/ -v` -> all PASS

**Steps:**

- [ ] **Step 1: Write the failing tests** in `config/config_test.go`

```go
package config

import "testing"

func TestDefault(t *testing.T) {
	c := Default()
	want := Config{
		SchemaPath:  `C:\FED_LOG\TOOLS\SCHEMA.txt`,
		TextDir:     `C:\db_texts`,
		CSVDir:      `C:\db_csv`,
		DecompPath:  `C:\FED_LOG\TOOLS\UTILITIES\Decomp.exe`,
		IMDListPath: `C:\FED_LOG`,
		Workers:     5,
	}
	if c != want {
		t.Fatalf("Default() = %+v, want %+v", c, want)
	}
}

func TestParseOverrides(t *testing.T) {
	c, err := Parse([]string{"-workers", "3", "-text-dir", "/x"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Workers != 3 || c.TextDir != "/x" {
		t.Fatalf("overrides not applied: %+v", c)
	}
	if c.CSVDir != Default().CSVDir {
		t.Fatalf("untouched field changed: %q", c.CSVDir)
	}
}

func TestParseRejectsBadInput(t *testing.T) {
	if _, err := Parse([]string{"-workers", "0"}); err == nil {
		t.Error("expected error for -workers 0")
	}
	if _, err := Parse([]string{"-nope"}); err == nil {
		t.Error("expected error for unknown flag")
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./config/ -v`
Expected: FAIL, `undefined: Default` (package does not compile).

- [ ] **Step 3: Implement** `config/config.go`

```go
// Package config holds every path and tunable used by fl-parse, so defaults live in one place.
package config

import (
	"flag"
	"fmt"
	"io"
)

type Config struct {
	SchemaPath  string // Fedlog's SCHEMA.txt
	TextDir     string // destination for Decomp-generated .txt files
	CSVDir      string // destination for converted .csv files
	DecompPath  string // Decomp.exe
	IMDListPath string // IMDLST / FED_LOG directory passed to Decomp
	Workers     int    // concurrent Decomp processes
}

// Default returns the values fl-parse has always used.
func Default() Config {
	return Config{
		SchemaPath:  `C:\FED_LOG\TOOLS\SCHEMA.txt`,
		TextDir:     `C:\db_texts`,
		CSVDir:      `C:\db_csv`,
		DecompPath:  `C:\FED_LOG\TOOLS\UTILITIES\Decomp.exe`,
		IMDListPath: `C:\FED_LOG`,
		Workers:     5,
	}
}

// Parse builds a Config from Default() overridden by command-line flags.
func Parse(args []string) (Config, error) {
	cfg := Default()
	fs := flag.NewFlagSet("fl-parse", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.SchemaPath, "schema", cfg.SchemaPath, "path to Fedlog SCHEMA.txt")
	fs.StringVar(&cfg.TextDir, "text-dir", cfg.TextDir, "directory for Decomp .txt output")
	fs.StringVar(&cfg.CSVDir, "csv-dir", cfg.CSVDir, "directory for .csv output")
	fs.StringVar(&cfg.DecompPath, "decomp", cfg.DecompPath, "path to Decomp.exe")
	fs.StringVar(&cfg.IMDListPath, "imdlst", cfg.IMDListPath, "IMDLST (FED_LOG) directory passed to Decomp")
	fs.IntVar(&cfg.Workers, "workers", cfg.Workers, "number of concurrent Decomp processes")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.Workers < 1 {
		return cfg, fmt.Errorf("-workers must be >= 1, got %d", cfg.Workers)
	}
	return cfg, nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./config/ -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add config/
git commit -m "feat: add central config with flag overrides" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Schema parser (bug #1, `queryCount`, `Fields`)

**Goal:** A pure `schema.Parse(io.Reader)` that returns tables, flushing the last table at EOF; the table count is `len(tables)`.

**Files:**
- Create: `schema/schema.go`
- Test: `schema/schema_test.go`

**Acceptance Criteria:**
- [ ] A schema whose last table has no trailing blank line still returns that table (bug #1).
- [ ] Lines starting with `-` are ignored; blank lines terminate a table; a header line is one containing `Disc` (unchanged heuristic).
- [ ] Table name is the first whitespace-separated field of the header; column name is the first field of the trimmed line. Whitespace-only lines are skipped, never panic.
- [ ] A header with no columns produces no table.

**Verify:** `go test ./schema/ -v` -> all PASS

**Steps:**

- [ ] **Step 1: Write the failing tests** in `schema/schema_test.go`

```go
package schema

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	in := "-- comment\n" +
		"XXpart_number Disc\n" +
		"  NIIN CHAR\n" +
		"  PART CHAR\n" +
		"\n" +
		"XXcolloquial_name Disc\n" +
		"  A\n" +
		"  B\n"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []Table{
		{Name: "XXpart_number", Columns: []string{"NIIN", "PART"}},
		{Name: "XXcolloquial_name", Columns: []string{"A", "B"}}, // no trailing blank line
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseSkipsWhitespaceOnlyAndEmptyTables(t *testing.T) {
	in := "XXempty Disc\n\nXXreal Disc\n  COL\n   \n  COL2\n\n"
	got, err := Parse(strings.NewReader(in))
	if err != nil {
		t.Fatal(err)
	}
	want := []Table{{Name: "XXreal", Columns: []string{"COL", "COL2"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseEmpty(t *testing.T) {
	got, err := Parse(strings.NewReader(""))
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./schema/ -v`
Expected: FAIL, `undefined: Parse`.

- [ ] **Step 3: Implement** `schema/schema.go`

```go
// Package schema parses Fedlog's SCHEMA.txt into the tables and columns to query.
package schema

import (
	"bufio"
	"io"
	"strings"
)

type Table struct {
	Name    string
	Columns []string
}

// Parse reads SCHEMA.txt. A table is a header line (contains "Disc") followed by one column per
// line, terminated by a blank line or EOF. Lines starting with "-" are comments.
func Parse(r io.Reader) ([]Table, error) {
	var tables []Table
	var curTable string
	var curCols []string

	flush := func() {
		if curTable != "" && len(curCols) > 0 {
			tables = append(tables, Table{Name: curTable, Columns: curCols})
		}
		curTable, curCols = "", nil
	}

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 {
			flush()
			continue
		}
		if strings.HasPrefix(line, "-") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 { // whitespace-only line
			continue
		}
		if strings.Contains(line, "Disc") {
			curTable = fields[0]
		} else {
			curCols = append(curCols, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	flush() // bug #1: the final table has no terminating blank line
	return tables, nil
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./schema/ -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add schema/
git commit -m "feat: extract schema parser, flush final table at EOF" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 3: CSV converter (bugs #4, #5, #6; regex, per-row filter selection)

**Goal:** `csv.Convert(name, textDir, csvDir)` converts one table file, closes every handle, handles arbitrarily long lines, surfaces flush/write errors, and uses a precomputed filter with a non-regex digit check.

**Files:**
- Rewrite: `csv/csvwriter.go` (removes `GetTxtFilesFromPath`, `retrieveTextFiles`, `WriteContentToCsv`, the `textOutputPath`/`csvOutputPath` vars, and the `regexp` use)
- Test: `csv/csvwriter_test.go`

**Acceptance Criteria:**
- [ ] `part_number` rows with non-digit/empty first field are filtered; colloquial_name rows with empty/whitespace field 2 are filtered; other names unfiltered; `Stats{Total,Valid,Filtered}` are correct; blank lines are not counted.
- [ ] A row with a 200 KB field converts without error (bug #4).
- [ ] CRLF line endings are handled (no `\r` in output fields).
- [ ] Missing input file returns an error (no panic) and creates no CSV.
- [ ] A mid-file failure removes the partial CSV; the input handle is always closed (`defer in.Close()`), and `w.Error()` plus `out.Close()` errors are returned (bugs #5, #6).
- [ ] The CSV dir is created if missing.

**Verify:** `go test ./csv/ -v -race` -> all PASS

**Steps:**

- [ ] **Step 1: Write the failing tests** in `csv/csvwriter_test.go`

```go
package csv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTxt(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name+".txt"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readCSV(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name+".csv"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestConvertPartNumberFiltersBadNIIN(t *testing.T) {
	txt, out := t.TempDir(), filepath.Join(t.TempDir(), "csv")
	writeTxt(t, txt, "part_number", "123|A\n\nabc|B\n|C\n456|D\n")
	stats, err := Convert("part_number", txt, out)
	if err != nil {
		t.Fatal(err)
	}
	if stats != (Stats{Total: 4, Valid: 2, Filtered: 2}) {
		t.Fatalf("stats = %+v", stats)
	}
	if got := readCSV(t, out, "part_number"); got != "123,A\n456,D\n" {
		t.Fatalf("csv = %q", got)
	}
}

func TestConvertColloquialFiltersEmptyName(t *testing.T) {
	txt, out := t.TempDir(), t.TempDir()
	writeTxt(t, txt, "colloquial_name", "1|x|NAME\n2|x|   \n3|x|\n4|x\n")
	stats, err := Convert("colloquial_name", txt, out)
	if err != nil {
		t.Fatal(err)
	}
	// row 4 has no field index 2, so it is kept (unchanged behaviour)
	if stats != (Stats{Total: 4, Valid: 2, Filtered: 2}) {
		t.Fatalf("stats = %+v", stats)
	}
	if got := readCSV(t, out, "colloquial_name"); got != "1,x,NAME\n4,x\n" {
		t.Fatalf("csv = %q", got)
	}
}

func TestConvertUnfilteredTableAndCRLF(t *testing.T) {
	txt, out := t.TempDir(), t.TempDir()
	writeTxt(t, txt, "other", "a|b\r\nc|d\r\n")
	if _, err := Convert("other", txt, out); err != nil {
		t.Fatal(err)
	}
	if got := readCSV(t, out, "other"); got != "a,b\nc,d\n" {
		t.Fatalf("csv = %q", got)
	}
}

func TestConvertLongLine(t *testing.T) {
	txt, out := t.TempDir(), t.TempDir()
	long := strings.Repeat("x", 200*1024)
	writeTxt(t, txt, "other", "1|"+long+"\n")
	stats, err := Convert("other", txt, out)
	if err != nil {
		t.Fatalf("long line failed: %v", err)
	}
	if stats.Valid != 1 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestConvertMissingInput(t *testing.T) {
	out := t.TempDir()
	if _, err := Convert("nope", t.TempDir(), out); err == nil {
		t.Fatal("expected error")
	}
	if _, err := os.Stat(filepath.Join(out, "nope.csv")); !os.IsNotExist(err) {
		t.Fatal("csv should not exist")
	}
}

func TestIsDigits(t *testing.T) {
	for in, want := range map[string]bool{"0123": true, "": false, "12a": false, "1 2": false, "-1": false} {
		if got := isDigits(in); got != want {
			t.Errorf("isDigits(%q) = %v, want %v", in, got, want)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./csv/ -v`
Expected: FAIL, `undefined: Convert` / `Stats`.

- [ ] **Step 3: Implement** by replacing all of `csv/csvwriter.go`

```go
package csv

import (
	"bufio"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Stats reports how many non-empty rows were read, written, and dropped by a table's filter.
type Stats struct {
	Total, Valid, Filtered int
}

// rowFilter reports whether a row should be kept.
type rowFilter func(fields []string) bool

// filters holds the per-table row filters; tables not listed are unfiltered.
var filters = map[string]rowFilter{
	// NIIN (first column) must be digits only.
	"part_number": func(f []string) bool { return isDigits(f[0]) },
	// Skip rows with an empty colloquial_name column (index 2).
	"colloquial_name": func(f []string) bool { return len(f) <= 2 || strings.TrimSpace(f[2]) != "" },
}

// isDigits reports whether s is non-empty and contains only ASCII digits 0-9.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Convert reads <textDir>/<name>.txt (pipe-delimited) and writes <csvDir>/<name>.csv, applying the
// table's row filter. A partially written CSV is removed on failure.
func Convert(name, textDir, csvDir string) (stats Stats, err error) {
	in, err := os.Open(filepath.Join(textDir, name+".txt"))
	if err != nil {
		return stats, err
	}
	defer in.Close()

	if err = os.MkdirAll(csvDir, 0o755); err != nil {
		return stats, err
	}
	csvPath := filepath.Join(csvDir, name+".csv")
	out, err := os.Create(csvPath)
	if err != nil {
		return stats, err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			os.Remove(csvPath)
		}
	}()

	w := csv.NewWriter(out)
	keep := filters[name] // looked up once per file, not per row
	r := bufio.NewReaderSize(in, 1<<20)

	for {
		line, rerr := r.ReadString('\n') // no line-length limit (bug #4)
		if rerr != nil && rerr != io.EOF {
			return stats, rerr
		}
		line = strings.TrimRight(line, "\r\n")
		if line != "" {
			stats.Total++
			fields := strings.Split(line, "|")
			if keep != nil && !keep(fields) {
				stats.Filtered++
			} else {
				if err = w.Write(fields); err != nil {
					return stats, err
				}
				stats.Valid++
			}
		}
		if rerr == io.EOF {
			break
		}
	}

	w.Flush()
	err = w.Error() // bug #6
	return stats, err
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./csv/ -v -race`
Expected: PASS (6 tests). Note: `go build ./...` will fail until Task 4/5 update the callers in `main.go`; that is expected here, so test only `./csv/`.

- [ ] **Step 5: Commit**

```bash
git add csv/
git commit -m "fix: csv converter closes files, has no line limit, checks write errors" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Worker pool and Decomp processor (bugs #8, #9; overlap extraction + conversion)

**Goal:** A generic pool that runs exactly N workers with no WaitGroup race, and a processor that runs Decomp then converts that table's file, returning a `Result` that is reported from a single goroutine.

**Files:**
- Rewrite: `querylist/querylist.go` (removes `QueryParams`, `QueryList`, `QueryDecomp`, `AddQuery`, `InitializeDecompPoolAndRun` and the package vars)
- Test: `querylist/querylist_test.go`

**Acceptance Criteria:**
- [ ] `Run` starts exactly `workers` goroutines (never more), processes every table exactly once, and calls `onResult` from a single goroutine (no locking needed by callers).
- [ ] `go test -race` is clean; no `sync.WaitGroup` counter is touched from a job-sending loop (bug #9).
- [ ] `FileBase("XXPart_Number")` == `"part_number"`; names shorter than 3 chars return an error instead of panicking.
- [ ] `NewProcessor` runs `cfg.DecompPath cfg.IMDListPath "select <cols> FROM <table>" <textDir>/<base>.txt`, captures Decomp's combined output into the error on failure, and does NOT convert on failure (so stale `.txt` files are never used).

**Verify:** `go test ./querylist/ -v -race` -> all PASS

**Steps:**

- [ ] **Step 1: Write the failing tests** in `querylist/querylist_test.go`

```go
package querylist

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fl-parse/schema"
)

func TestFileBase(t *testing.T) {
	got, err := FileBase("XXPart_Number")
	if err != nil || got != "part_number" {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := FileBase("XX"); err == nil {
		t.Fatal("expected error for short name")
	}
}

func TestRunProcessesEveryTableOnceWithBoundedWorkers(t *testing.T) {
	var tables []schema.Table
	for i := 0; i < 50; i++ {
		tables = append(tables, schema.Table{Name: "XXt" + string(rune('a'+i%26)) + string(rune('A'+i/26))})
	}

	const workers = 5
	var running, peak int32
	var mu sync.Mutex
	seen := map[string]int{}

	process := func(tb schema.Table) Result {
		n := atomic.AddInt32(&running, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		atomic.AddInt32(&running, -1)
		mu.Lock()
		seen[tb.Name]++
		mu.Unlock()
		return Result{Table: tb.Name}
	}

	got := 0
	Run(tables, workers, process, func(Result) { got++ }) // onResult is single-goroutine: plain int is safe

	if got != len(tables) {
		t.Fatalf("got %d results, want %d", got, len(tables))
	}
	for name, c := range seen {
		if c != 1 {
			t.Errorf("%s processed %d times", name, c)
		}
	}
	if peak > workers {
		t.Fatalf("peak concurrency %d exceeds %d workers", peak, workers)
	}
	if peak < 2 {
		t.Fatalf("expected concurrency, peak was %d", peak)
	}
}

func TestRunWithNoTables(t *testing.T) {
	Run(nil, 3, func(schema.Table) Result { t.Fatal("unexpected"); return Result{} }, func(Result) {})
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./querylist/ -v -race`
Expected: FAIL, `undefined: FileBase` / `Run` / `Result`.

- [ ] **Step 3: Implement** by replacing all of `querylist/querylist.go`

```go
// Package querylist runs Decomp for each schema table on a bounded worker pool and converts each
// table's output to CSV as soon as its extraction finishes.
package querylist

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fl-parse/config"
	"fl-parse/csv"
	"fl-parse/schema"
)

// Result is the outcome of processing one table.
type Result struct {
	Table   string // schema table name, e.g. XXpart_number
	Name    string // output base name, e.g. part_number
	Stats   csv.Stats
	Err     error
	Elapsed time.Duration
}

// FileBase converts a schema table name to its output base name: prefix removed, lowercased.
func FileBase(table string) (string, error) {
	if len(table) < 3 {
		return "", fmt.Errorf("table name %q too short to strip 2-character prefix", table)
	}
	return strings.ToLower(table[2:]), nil
}

// Run processes every table with exactly `workers` goroutines. onResult is always called from the
// calling goroutine, so callers may print or aggregate without locking.
func Run(tables []schema.Table, workers int, process func(schema.Table) Result, onResult func(Result)) {
	jobs := make(chan schema.Table)
	results := make(chan Result)

	var wg sync.WaitGroup
	wg.Add(workers) // counted up front, before any goroutine can call Done
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for t := range jobs {
				results <- process(t)
			}
		}()
	}

	go func() {
		for _, t := range tables {
			jobs <- t
		}
		close(jobs)
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	for r := range results {
		onResult(r)
	}
}

// NewProcessor returns the production process function: run Decomp for a table, then convert the
// file it produced. Conversion is skipped when Decomp fails, so stale .txt files are never used.
func NewProcessor(cfg config.Config) func(schema.Table) Result {
	return func(t schema.Table) (res Result) {
		start := time.Now()
		res.Table = t.Name
		// named result: the deferred update is visible to the caller
		defer func() { res.Elapsed = time.Since(start) }()

		name, err := FileBase(t.Name)
		if err != nil {
			res.Err = err
			return
		}
		res.Name = name

		txtPath := filepath.Join(cfg.TextDir, name+".txt")
		query := fmt.Sprintf("select %v FROM %v", strings.Join(t.Columns, ","), t.Name)

		cmd := exec.Command(cfg.DecompPath, cfg.IMDListPath, query, txtPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			res.Err = fmt.Errorf("decomp failed: %w: %s", err, strings.TrimSpace(string(out)))
			return
		}

		res.Stats, res.Err = csv.Convert(name, cfg.TextDir, cfg.CSVDir)
		return
	}
}
```

- [ ] **Step 4: Run to verify pass**

Run: `go test ./querylist/ -v -race`
Expected: PASS (3 tests), no race warnings.

- [ ] **Step 5: Commit**

```bash
git add querylist/
git commit -m "fix: bounded worker pool without WaitGroup race; convert per table after Decomp" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 5: `main` wiring and output (bug #2, "Better output", `queryCount`)

**Goal:** `main` parses flags, opens/closes the schema with proper error handling, runs the pool, prints one line per table from a single goroutine, prints a summary with elapsed time, and exits non-zero on any failure.

**Files:**
- Rewrite: `main.go`

**Acceptance Criteria:**
- [ ] A missing/unreadable schema prints an error to stderr and exits 1 without touching the text dir (bug #2).
- [ ] Zero tables found -> error and exit 1.
- [ ] Prints `Tables found: N` where N == `len(tables)`.
- [ ] Each finished table prints one line, `[i/N] ok   part_number  total=… valid=… filtered=…  1.2s` or `[i/N] FAIL part_number  <error>`; lines never interleave.
- [ ] Final summary: `Done: X ok, Y failed in <elapsed>`; exit code 1 if Y > 0 else 0; failed tables re-listed with errors in the summary.
- [ ] The text dir is created (`os.MkdirAll`) before Decomp runs.

**Verify:** `go build ./... && go vet ./... && GOOS=windows go build -o /dev/null ./...` -> no output, exit 0

**Steps:**

- [ ] **Step 1: Replace `main.go`**

```go
package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"fl-parse/config"
	"fl-parse/querylist"
	"fl-parse/schema"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run parses SCHEMA.txt, queries Decomp for every table, converts each result to CSV, and returns
// the process exit code (0 = all tables succeeded).
func run(args []string, stdout, stderr io.Writer) int {
	cfg, err := config.Parse(args)
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		return 2
	}

	tables, err := loadSchema(cfg.SchemaPath)
	if err != nil {
		fmt.Fprintln(stderr, "error reading schema:", err)
		return 1
	}
	if len(tables) == 0 {
		fmt.Fprintln(stderr, "error: no tables found in", cfg.SchemaPath)
		return 1
	}
	fmt.Fprintf(stdout, "Tables found: %d\n", len(tables))

	if err := os.MkdirAll(cfg.TextDir, 0o755); err != nil {
		fmt.Fprintln(stderr, "error creating text dir:", err)
		return 1
	}

	start := time.Now()
	var failed []querylist.Result
	done := 0
	querylist.Run(tables, cfg.Workers, querylist.NewProcessor(cfg), func(r querylist.Result) {
		done++
		if r.Err != nil {
			failed = append(failed, r)
			fmt.Fprintf(stdout, "[%d/%d] FAIL %s  %v\n", done, len(tables), r.Table, r.Err)
			return
		}
		fmt.Fprintf(stdout, "[%d/%d] ok   %s  total=%d valid=%d filtered=%d  %s\n",
			done, len(tables), r.Name, r.Stats.Total, r.Stats.Valid, r.Stats.Filtered,
			r.Elapsed.Round(time.Millisecond))
	})

	fmt.Fprintf(stdout, "Done: %d ok, %d failed in %s\n",
		len(tables)-len(failed), len(failed), time.Since(start).Round(time.Millisecond))
	if len(failed) > 0 {
		fmt.Fprintln(stderr, "Failed tables:")
		for _, r := range failed {
			fmt.Fprintf(stderr, "  %s: %v\n", r.Table, r.Err)
		}
		return 1
	}
	return 0
}

func loadSchema(path string) ([]schema.Table, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return schema.Parse(f)
}
```

- [ ] **Step 2: Build, vet, cross-compile**

Run: `go build ./... && go vet ./... && GOOS=windows go build -o /dev/null ./...`
Expected: no output, exit 0.

- [ ] **Step 3: Run the whole suite**

Run: `go test ./... -race`
Expected: PASS for `config`, `schema`, `csv`, `querylist`; `fl-parse` (main) reports `[no test files]`.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: wire config/schema/pool in main; per-table output, summary, exit codes" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

### Task 6: End-to-end smoke test with a fake Decomp

**Goal:** Prove the full pipeline (including the last-table-without-blank-line bug and the failure/exit-code path) on macOS without the real Decomp.

**Files:**
- Create: `scripts/fake-decomp.sh`
- Scratch (not committed): sample schema in the session scratchpad

**Acceptance Criteria:**
- [ ] With a 3-table schema lacking a trailing blank line, all 3 tables are processed (Tables found: 3), including the last.
- [ ] `part_number.csv` contains only digit-NIIN rows; `colloquial_name.csv` contains no empty-name rows.
- [ ] With a table the fake Decomp fails on, the run prints a `FAIL` line, lists it in the summary, and exits 1; no CSV is produced for it even if an old `.txt` is present.
- [ ] Running with a nonexistent `-schema` exits 1 and creates nothing.

**Verify:** the commands in Steps 2-5 produce the stated results.

**Steps:**

- [ ] **Step 1: Create `scripts/fake-decomp.sh`** (then `chmod +x`)

```sh
#!/bin/sh
# Fake Decomp: $1=imdlst dir, $2=query, $3=output txt path. Fails for tables named *broken*.
case "$2" in
  *broken*) echo "simulated decomp failure" >&2; exit 3 ;;
esac
case "$3" in
  *part_number.txt)     printf '123|A\nabc|B\n456|C\n' > "$3" ;;
  *colloquial_name.txt) printf '1|x|NAME\n2|x|\n' > "$3" ;;
  *)                    printf 'a|b\n' > "$3" ;;
esac
```

- [ ] **Step 2: Happy path**

Write `$S/schema.txt` (where `S` is the session scratchpad dir) with no trailing blank line:

```
-- test schema
XXpart_number Disc
  NIIN
  PART

XXcolloquial_name Disc
  A
  B
  C

XXlast_table Disc
  X
```

Run:

```bash
S=/private/tmp/claude-501/-Users-swisscheese-projects-fl-parse/87d69e1a-879f-4de2-8264-30325a0cd45e/scratchpad
go run . -schema $S/schema.txt -decomp $PWD/scripts/fake-decomp.sh -text-dir $S/txt -csv-dir $S/csv -workers 2
cat $S/csv/part_number.csv $S/csv/colloquial_name.csv; ls $S/csv
echo "exit=$?"
```

Expected: `Tables found: 3`; three `ok` lines (one per table, including `last_table`); `Done: 3 ok, 0 failed`; `part_number.csv` = `123,A` / `456,C`; `colloquial_name.csv` = `1,x,NAME`; csv dir lists all 3 files.

- [ ] **Step 3: Failure path**

Append a `XXbroken Disc` table with a column to the schema, and pre-create `$S/txt/broken.txt` with old content to simulate a stale file. Re-run the Step 2 command.

Expected: a `FAIL broken  decomp failed: exit status 3: simulated decomp failure` line, `Done: 3 ok, 1 failed`, a `Failed tables:` block on stderr, no `broken.csv`, and exit code 1 (`go run` prints `exit status 1`; check with `; echo $?` on the built binary instead: `go build -o $S/fl-parse . && $S/fl-parse ...; echo $?`).

- [ ] **Step 4: Bad schema path**

Run: `go run . -schema /nope/schema.txt -text-dir $S/never`
Expected: `error reading schema: open /nope/schema.txt: no such file or directory`, exit 1, and `$S/never` does not exist.

- [ ] **Step 5: Commit**

```bash
git add scripts/fake-decomp.sh
git commit -m "test: add fake Decomp script for end-to-end smoke test" -m "Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>"
```

---

## Self-Review

**Coverage of requested items**
- Bug #1 -> Task 2 (flush at EOF, test without trailing blank line; also Task 6 step 2).
- Bug #2 -> Task 5 (`loadSchema` error returns exit 1 before anything else runs; Task 6 step 4).
- Bug #4 -> Task 3 (`ReadString`, 200 KB test).
- Bug #5 -> Task 3 (`defer in.Close()`).
- Bug #6 -> Task 3 (`w.Error()`, `out.Close()` error, partial-file removal).
- Bug #8 -> Task 4 (`for i := 0; i < workers`, peak-concurrency test).
- Bug #9 -> Task 4 (`wg.Add(workers)` before goroutines start; `-race` test).
- Efficiency: regex -> `isDigits` (Task 3); per-row filter selection -> `filters` map lookup once per file (Task 3); extraction/conversion overlap -> `NewProcessor` (Task 4); configurable workers -> `-workers` (Task 1); `Split(" ")` -> `Fields` (Task 2).
- `queryCount` -> `len(tables)` (Tasks 2, 5). `textOutputPath` -> `config.Config` (Task 1); all three old vars are deleted in Tasks 3-5.
- Better output -> Task 5.

**Side effects to be aware of (not in the original list, but implied by the design)**
- The stale-file problem (review #3) is fixed as a consequence of converting per-worker and skipping conversion on Decomp failure.
- Short-table-name panic (#10, first half) is guarded in `FileBase`; needed because a panic in a worker would defeat the failure reporting.
- `Convert` calls `MkdirAll` on the CSV dir (#11), needed for testability; the text dir is created in `main`.
- Panics in `csv` are replaced by returned errors; required for per-table failure reporting.

**Placeholder scan:** none.

**Type consistency:** `config.Config` fields (`SchemaPath, TextDir, CSVDir, DecompPath, IMDListPath, Workers`), `schema.Table{Name, Columns}`, `csv.Convert(name, textDir, csvDir) (Stats, error)`, `csv.Stats{Total, Valid, Filtered}`, `querylist.Result{Table, Name, Stats, Err, Elapsed}`, `querylist.Run(tables, workers, process, onResult)`, `querylist.NewProcessor(cfg)`, `querylist.FileBase(table)` are used identically in every task.
