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
