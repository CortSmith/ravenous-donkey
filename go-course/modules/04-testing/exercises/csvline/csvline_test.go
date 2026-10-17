package csvline

import "testing"

// Write comprehensive tests to expose the bugs in csvline.go. This package
// has 2 bugs that are shy: simple examples pass fine.
//
// TODO: Write TestJoin (table-driven) with edge cases:
//   - nil, empty fields, plain strings, fields with commas/newlines/quotes
//   - Check your output against the spec in csvline.go doc comment
//
// TODO: Write TestSplit (table-driven) with:
//   - plain lines, quoted fields, escaped quotes, empty fields
//   - Error cases: unterminated quotes, junk after closing quote
//
// TODO: Write FuzzRoundTrip(f *testing.F):
//   - Seeds: simple strings, ones with commas/newlines/quotes
//   - Target: Join three fields, Split the line, verify Split(Join(...)) == input
//   - Run: go test ./modules/04-testing/exercises/csvline/ -run='^$' -fuzz=FuzzRoundTrip -fuzztime=20s
//
// TODO: Write FuzzSplit(f *testing.F):
//   - Seeds: various CSV lines, malformed ones
//   - Target: Split arbitrary strings; verify no panic, errors are expected types,
//     results survive Join/Split round-trip
//   - Run: go test ./modules/04-testing/exercises/csvline/ -run='^$' -fuzz=FuzzSplit -fuzztime=20s

func TestJoinPlain(t *testing.T) {
	got := Join([]string{"a", "b", "c"})
	want := "a,b,c"
	if got != want {
		t.Errorf("Join(%q) = %q, want %q", []string{"a", "b", "c"}, got, want)
	}
}

func TestSplitPlain(t *testing.T) {
	got, err := Split("a,b,c")
	if err != nil {
		t.Fatalf("Split(%q) error = %v", "a,b,c", err)
	}
	if len(got) != 3 {
		t.Errorf("Split(%q) returned %d fields, want 3", "a,b,c", len(got))
	}
}
