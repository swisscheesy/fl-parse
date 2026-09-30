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
// line, terminated by EOF or by a blank line once it has both a header and at least one column
// (blank lines before that are ignored). Lines starting with "-" are comments.
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
			if curTable != "" && len(curCols) > 0 {
				flush()
			}
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
