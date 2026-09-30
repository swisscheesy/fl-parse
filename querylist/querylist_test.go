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
