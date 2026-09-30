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
