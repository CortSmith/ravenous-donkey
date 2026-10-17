package store

import (
	"path/filepath"
	"testing"
)

// BenchmarkAppend measures the cost of durability: with syncWrites every
// Append waits for the disk (fsync); without it, Append returns once the
// data is in the OS page cache.
//
//	go test ./solutions/10-capstone/milestone2/store/ -run='^$' -bench=Append -benchmem
func BenchmarkAppend(b *testing.B) {
	for _, tc := range []struct {
		name string
		sync bool
	}{{"nosync", false}, {"fsync", true}} {
		b.Run(tc.name, func(b *testing.B) {
			l, err := OpenLog(filepath.Join(b.TempDir(), "bench.log"), tc.sync)
			if err != nil {
				b.Fatal(err)
			}
			defer l.Close()
			r := Record{Op: OpSet, Key: "user:1", Value: "a short value"}
			b.ReportAllocs()
			for b.Loop() {
				if err := l.Append(r); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
