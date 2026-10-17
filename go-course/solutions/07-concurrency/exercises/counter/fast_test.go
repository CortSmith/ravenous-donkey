package counter

import (
	"strconv"
	"sync"
	"testing"
)

func TestShardedKeyedCounter(t *testing.T) {
	c := NewShardedKeyedCounter()
	var wg sync.WaitGroup
	for g := range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range 1000 {
				c.Inc("k" + strconv.Itoa((g+i)%100))
			}
		}()
	}
	wg.Wait()
	var total int64
	for _, v := range c.Snapshot() {
		total += v
	}
	if total != 16000 {
		t.Errorf("total = %d, want 16000", total)
	}
	if got := c.Get("k0"); got != 160 {
		t.Errorf("Get(k0) = %d, want 160", got)
	}
}

func BenchmarkCounterMutex(b *testing.B) {
	var c Counter
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Inc()
		}
	})
}

func BenchmarkCounterAtomic(b *testing.B) {
	var c AtomicCounter
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Inc()
		}
	})
}

var benchKeys = func() []string {
	ks := make([]string, 1024)
	for i := range ks {
		ks[i] = "key-" + strconv.Itoa(i)
	}
	return ks
}()

func BenchmarkKeyedMutex(b *testing.B) {
	var c KeyedCounter
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Inc(benchKeys[i&1023])
			i++
		}
	})
}

func BenchmarkKeyedSharded(b *testing.B) {
	c := NewShardedKeyedCounter()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Inc(benchKeys[i&1023])
			i++
		}
	})
}
