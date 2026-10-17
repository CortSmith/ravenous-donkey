package inventory

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
)

var _ Store = (*MemoryStore)(nil)

func TestMemoryStoreCRUD(t *testing.T) {
	s := NewMemoryStore()
	w := Product{SKU: "W-1", Name: "Widget", PriceCents: 100, Quantity: 5}

	if _, err := s.Get("W-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get on empty store error = %v, want ErrNotFound", err)
	}
	if err := s.Add(w); err != nil {
		t.Fatalf("Add(%v) = %v, want nil", w, err)
	}
	if err := s.Add(w); !errors.Is(err, ErrDuplicate) {
		t.Errorf("second Add(%v) = %v, want ErrDuplicate", w, err)
	}
	got, err := s.Get("W-1")
	if err != nil || got != w {
		t.Errorf("Get(W-1) = %v, %v; want %v, nil", got, err, w)
	}

	w.Quantity = 9
	if err := s.Update(w); err != nil {
		t.Fatalf("Update = %v, want nil", err)
	}
	if got, _ := s.Get("W-1"); got.Quantity != 9 {
		t.Errorf("after Update, Quantity = %d, want 9", got.Quantity)
	}
	if err := s.Update(Product{SKU: "nope", Name: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update(missing) = %v, want ErrNotFound", err)
	}

	if err := s.Delete("W-1"); err != nil {
		t.Fatalf("Delete = %v, want nil", err)
	}
	if err := s.Delete("W-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second Delete = %v, want ErrNotFound", err)
	}
	if _, err := s.Get("W-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after Delete error = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreErrorsHaveContext(t *testing.T) {
	s := NewMemoryStore()
	_, err := s.Get("ABC-9")
	if err == nil || err == ErrNotFound {
		t.Fatalf("Get(ABC-9) error = %v; want ErrNotFound wrapped with context (use %%w)", err)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(ABC-9) error = %v, want it to wrap ErrNotFound", err)
	}
}

func TestMemoryStoreReturnsCopies(t *testing.T) {
	s := NewMemoryStore()
	if err := s.Add(Product{SKU: "A", Name: "a", Quantity: 1}); err != nil {
		t.Fatal(err)
	}
	p, _ := s.Get("A")
	p.Quantity = 100 // modifying the copy must not change the store
	if got, _ := s.Get("A"); got.Quantity != 1 {
		t.Errorf("store changed through a Get result: Quantity = %d, want 1", got.Quantity)
	}
	list := s.List()
	list[0].Quantity = 100
	if got, _ := s.Get("A"); got.Quantity != 1 {
		t.Errorf("store changed through a List result: Quantity = %d, want 1", got.Quantity)
	}
}

func TestMemoryStoreListSorted(t *testing.T) {
	s := NewMemoryStore()
	if got := s.List(); len(got) != 0 {
		t.Errorf("List on empty store = %v, want empty", got)
	}
	for _, sku := range []string{"C-3", "A-1", "B-2", "A-10"} {
		if err := s.Add(Product{SKU: sku, Name: sku}); err != nil {
			t.Fatal(err)
		}
	}
	var skus []string
	for _, p := range s.List() {
		skus = append(skus, p.SKU)
	}
	if want := []string{"A-1", "A-10", "B-2", "C-3"}; !slices.Equal(skus, want) {
		t.Errorf("List SKUs = %v, want %v", skus, want)
	}
}

// Run with -race to make this test meaningful.
func TestMemoryStoreConcurrent(t *testing.T) {
	s := NewMemoryStore()
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sku := fmt.Sprintf("P-%02d", i)
			if err := s.Add(Product{SKU: sku, Name: sku, Quantity: i}); err != nil {
				t.Errorf("Add(%s) = %v", sku, err)
			}
			_, _ = s.Get(sku)
			_ = s.List()
		}()
	}
	wg.Wait()
	if got := len(s.List()); got != 20 {
		t.Errorf("len(List()) = %d after 20 concurrent Adds, want 20", got)
	}
}
