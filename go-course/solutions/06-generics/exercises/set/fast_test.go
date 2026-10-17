package set

import "testing"

func TestIntersectFast(t *testing.T) {
	var nilSet *Set[int]
	assertSet(t, "big ∩ small", Of(1, 2, 3, 4, 5).IntersectFast(Of(4, 5, 6)), 4, 5)
	assertSet(t, "small ∩ big", Of(4, 5, 6).IntersectFast(Of(1, 2, 3, 4, 5)), 4, 5)
	assertSet(t, "a ∩ nil", Of(1).IntersectFast(nilSet))
	assertSet(t, "nil ∩ a", nilSet.IntersectFast(Of(1)))
}

func benchSets() (big, small *Set[int]) {
	big, small = &Set[int]{}, &Set[int]{}
	for i := range 100_000 {
		big.Add(i)
	}
	for i := range 100 {
		small.Add(i * 7)
	}
	return big, small
}

// BenchmarkIntersect calls big.Intersect(small): the readable version walks
// all 100k elements of big; the fast version walks the 100 of small.
func BenchmarkIntersect(b *testing.B) {
	big, small := benchSets()
	b.Run("readable", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			big.Intersect(small)
		}
	})
	b.Run("fast", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			big.IntersectFast(small)
		}
	})
}
