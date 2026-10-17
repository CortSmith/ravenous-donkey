package fetcher

import (
	"context"
	"errors"
	"testing"
)

func TestFetchAllPool(t *testing.T) {
	in := urls(50)
	got := FetchAllPool(t.Context(), in, 4, echo(len(in)))
	for i, r := range got {
		if r.URL != in[i] || r.Err != nil || r.Body != "body of "+in[i] {
			t.Errorf("results[%d] = %+v", i, r)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for i, r := range FetchAllPool(ctx, in, 4, echo(len(in))) {
		if r.URL != in[i] || !errors.Is(r.Err, context.Canceled) {
			t.Errorf("canceled results[%d] = %+v", i, r)
		}
	}
	if got := FetchAllPool(t.Context(), nil, 4, echo(0)); len(got) != 0 {
		t.Errorf("FetchAllPool(nil) = %v, want empty", got)
	}
}

func BenchmarkFetchAll(b *testing.B) {
	in := urls(10_000)
	fast := func(_ context.Context, url string) (string, error) { return url, nil }
	b.Run("semaphore", func(b *testing.B) {
		for b.Loop() {
			FetchAll(context.Background(), in, 8, fast)
		}
	})
	b.Run("pool", func(b *testing.B) {
		for b.Loop() {
			FetchAllPool(context.Background(), in, 8, fast)
		}
	})
}
