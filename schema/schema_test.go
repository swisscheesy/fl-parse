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
