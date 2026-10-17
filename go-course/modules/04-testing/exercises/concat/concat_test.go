package concat

import "testing"

// Write tests to expose the 2 bugs in concat.go (one wrong answer, one
// broken performance promise).
//
// TODO: Write TestJoinersMatchStringsJoin:
//   - Use strings.Join as an oracle (the gold standard)
//   - Loop over joiners table and test cases (nil, empty, plain, with empty
//     strings at different positions, etc.)
//   - For each, verify the joiner matches strings.Join
//
// TODO: Write TestJoinBuilderAllocatesOnce:
//   - Use testing.AllocsPerRun to verify JoinBuilder's "allocates exactly once" promise
//   - Test with non-trivial input
//   - Fail if allocations > 1
//
// TODO: Write BenchmarkJoin:
//   - For each size (10, 100, 1000), build input outside timed loop
//   - Sub-benchmark for each joiner with b.Run
//   - Use b.Loop() and b.ReportAllocs()
//   - Run: go test ./modules/04-testing/exercises/concat/ -run='^$' -bench=. -benchmem

var joiners = []struct {
	name string
	join func([]string, string) string
}{
	{"Plus", JoinPlus},
	{"Buffer", JoinBuffer},
	{"Builder", JoinBuilder},
}

func TestJoinersSimple(t *testing.T) {
	elems := []string{"a", "b"}
	for _, j := range joiners {
		got := j.join(elems, ",")
		if len(got) == 0 {
			t.Errorf("%s(%q, \",\") returned empty", j.name, elems)
		}
	}
}
