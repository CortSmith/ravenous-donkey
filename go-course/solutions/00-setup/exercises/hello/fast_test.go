package hello

import "testing"

func TestVariantsMatchGreeting(t *testing.T) {
	for _, in := range []string{"Ada", "  Ada ", "", " \t ", "Gödel"} {
		want := Greeting(in)
		if got := GreetingSprintf(in); got != want {
			t.Errorf("GreetingSprintf(%q) = %q, want %q", in, got, want)
		}
		if got := GreetingBuilder(in); got != want {
			t.Errorf("GreetingBuilder(%q) = %q, want %q", in, got, want)
		}
	}
}

func BenchmarkGreeting(b *testing.B) {
	for _, v := range []struct {
		name string
		fn   func(string) string
	}{{"Concat", Greeting}, {"Sprintf", GreetingSprintf}, {"Builder", GreetingBuilder}} {
		b.Run(v.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				v.fn(" Grace Hopper ")
			}
		})
	}
}
