package anagram

import "testing"

func TestIsAnagramFastMatches(t *testing.T) {
	pairs := [][2]string{
		{"", ""}, {"listen", "silent"}, {"Dormitory!", "Dirty, room."}, {"aab", "abb"},
		{"Ärger", "regär"}, {"ΑΒΓ", "γβα"}, {"é", "é"}, {"abc", "cbä"},
		{"Tom Marvolo Riddle", "I am Lord Voldemort"}, {"a1b2", "2b1a"}, {"123", "124"},
		{"ab\xff", "ba"}, {"ascii", "ascïi"},
	}
	for _, p := range pairs {
		if got, want := IsAnagramFast(p[0], p[1]), IsAnagram(p[0], p[1]); got != want {
			t.Errorf("IsAnagramFast(%q, %q) = %v, want %v", p[0], p[1], got, want)
		}
	}
}

func TestIsAnagramFastASCIIDoesNotAllocate(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		IsAnagramFast("Tom Marvolo Riddle", "I am Lord Voldemort")
	})
	if allocs != 0 {
		t.Errorf("IsAnagramFast on ASCII allocated %.0f times, want 0", allocs)
	}
}

func BenchmarkIsAnagram(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		IsAnagram("Tom Marvolo Riddle", "I am Lord Voldemort")
	}
}

func BenchmarkIsAnagramFast(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		IsAnagramFast("Tom Marvolo Riddle", "I am Lord Voldemort")
	}
}
