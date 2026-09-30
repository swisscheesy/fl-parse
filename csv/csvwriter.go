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
