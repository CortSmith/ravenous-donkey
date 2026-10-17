package stackqueue

import (
	"math/rand/v2"
	"testing"
)

var _ container[int] = (*RingQueue[int])(nil)

func TestRingQueueAgainstQueue(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	var ring RingQueue[int]
	var ref Queue[int]
	for i := range 50_000 {
		if r.IntN(2) == 0 {
			ring.Push(i)
			ref.Push(i)
			continue
		}
		gv, gok := ring.Pop()
		wv, wok := ref.Pop()
		if gv != wv || gok != wok || ring.Len() != ref.Len() {
			t.Fatalf("op %d: RingQueue.Pop() = (%d, %v) len %d, want (%d, %v) len %d", i, gv, gok, ring.Len(), wv, wok, ref.Len())
		}
	}
}

// BenchmarkQueueChurn keeps ~1000 elements queued and does one push and one
// pop per iteration: the access pattern of a work queue in steady state.
func BenchmarkQueueChurn(b *testing.B) {
	const depth = 1000
	b.Run("slice", func(b *testing.B) {
		var q Queue[int]
		for i := range depth {
			q.Push(i)
		}
		b.ReportAllocs()
		for b.Loop() {
			q.Push(1)
			q.Pop()
		}
	})
	b.Run("ring", func(b *testing.B) {
		var q RingQueue[int]
		for i := range depth {
			q.Push(i)
		}
		b.ReportAllocs()
		for b.Loop() {
			q.Push(1)
			q.Pop()
		}
	})
}
