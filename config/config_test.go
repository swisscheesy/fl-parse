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
