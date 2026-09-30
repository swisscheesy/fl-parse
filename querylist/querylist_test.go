package querylist

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fl-parse/config"
	"fl-parse/csv"
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

func skipUnlessUnix(t *testing.T, bin string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("unix-only")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("%s not available", bin)
	}
}

func TestProcessorDoesNotUseStaleTxtWhenDecompWritesNothing(t *testing.T) {
	skipUnlessUnix(t, "/usr/bin/true")
	textDir, csvDir := t.TempDir(), t.TempDir()
	stale := filepath.Join(textDir, "foo.txt")
	if err := os.WriteFile(stale, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{DecompPath: "/usr/bin/true", TextDir: textDir, CSVDir: csvDir}
	res := NewProcessor(cfg)(schema.Table{Name: "XXfoo", Columns: []string{"A"}})
	if res.Err == nil {
		t.Fatal("expected error when Decomp writes no file")
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("stale txt should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(csvDir, "foo.csv")); !os.IsNotExist(err) {
		t.Fatalf("foo.csv should not exist, stat err = %v", err)
	}
}

func TestProcessorDecompFailure(t *testing.T) {
	skipUnlessUnix(t, "/usr/bin/false")
	cfg := config.Config{DecompPath: "/usr/bin/false", TextDir: t.TempDir(), CSVDir: t.TempDir()}
	res := NewProcessor(cfg)(schema.Table{Name: "XXfoo", Columns: []string{"A"}})
	if res.Err == nil {
		t.Fatal("expected error")
	}
	if res.Stats != (csv.Stats{}) {
		t.Fatalf("stats should be zero, got %+v", res.Stats)
	}
	if res.Elapsed <= 0 {
		t.Fatalf("Elapsed = %v, want > 0", res.Elapsed)
	}
}
