// Package querylist runs Decomp for each schema table on a bounded worker pool and converts each
// table's output to CSV as soon as its extraction finishes.
package querylist

import (
	"fmt"
	"os"
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

		// a stale file from a previous run must not be mistaken for this run's output
		if err := os.Remove(txtPath); err != nil && !os.IsNotExist(err) {
			res.Err = fmt.Errorf("removing stale %s: %w", txtPath, err)
			return
		}

		cmd := exec.Command(cfg.DecompPath, cfg.IMDListPath, query, txtPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			res.Err = fmt.Errorf("decomp failed: %w: %s", err, strings.TrimSpace(string(out)))
			return
		}

		res.Stats, res.Err = csv.Convert(name, cfg.TextDir, cfg.CSVDir)
		return
	}
}
