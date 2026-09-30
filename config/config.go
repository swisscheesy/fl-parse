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
