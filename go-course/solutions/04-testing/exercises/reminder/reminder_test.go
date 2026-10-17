package reminder

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// fakeClock is a Clock whose time only moves when the test says so.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

// message is one call recorded by fakeSender.
type message struct{ To, Body string }

// fakeSender records every successful Send and fails for recipients listed
// in fail. It is a fake (a small working implementation), not a mock: tests
// inspect its state afterwards instead of scripting expected calls.
type fakeSender struct {
	sent []message
	fail map[string]error // recipient -> error to return
}

func (s *fakeSender) Send(to, body string) error {
	if err := s.fail[to]; err != nil {
		return err
	}
	s.sent = append(s.sent, message{to, body})
	return nil
}

// Compile-time checks that the fakes satisfy the interfaces.
var (
	_ Clock  = (*fakeClock)(nil)
	_ Sender = (*fakeSender)(nil)
)

var t0 = time.Date(2025, time.March, 1, 9, 0, 0, 0, time.UTC)

// newTestScheduler wires a Scheduler to fresh fakes set to t0.
func newTestScheduler(t *testing.T) (*Scheduler, *fakeClock, *fakeSender) {
	t.Helper()
	clock := &fakeClock{now: t0}
	sender := &fakeSender{fail: map[string]error{}}
	return New(clock, sender), clock, sender
}

func TestFlushSendsOnlyDueReminders(t *testing.T) {
	tests := []struct {
		name     string
		due      time.Time
		wantSent int
	}{
		{"in the past", t0.Add(-time.Hour), 1},
		{"exactly now", t0, 1},
		{"one nanosecond in the future", t0.Add(time.Nanosecond), 0},
		{"tomorrow", t0.Add(24 * time.Hour), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _, sender := newTestScheduler(t)
			s.Add(Reminder{To: "ann", Body: "hi", Due: tt.due})

			sent, err := s.Flush()
			if err != nil {
				t.Fatalf("Flush() error = %v, want nil", err)
			}
			if sent != tt.wantSent || len(sender.sent) != tt.wantSent {
				t.Errorf("Flush() sent = %d (sender saw %d), want %d", sent, len(sender.sent), tt.wantSent)
			}
			if got, want := len(s.Pending()), 1-tt.wantSent; got != want {
				t.Errorf("len(Pending()) after Flush = %d, want %d", got, want)
			}
		})
	}
}

func TestFlushLaterAfterClockAdvances(t *testing.T) {
	s, clock, sender := newTestScheduler(t)
	s.Add(Reminder{To: "ann", Body: "standup", Due: t0.Add(10 * time.Minute)})

	if sent, _ := s.Flush(); sent != 0 {
		t.Fatalf("Flush() before due sent %d, want 0", sent)
	}
	clock.Advance(10 * time.Minute)
	if sent, _ := s.Flush(); sent != 1 {
		t.Fatalf("Flush() at due time sent %d, want 1", sent)
	}
	want := []message{{"ann", "standup"}}
	if !slices.Equal(sender.sent, want) {
		t.Errorf("sender got %v, want %v", sender.sent, want)
	}
	if sent, _ := s.Flush(); sent != 0 {
		t.Errorf("second Flush() sent %d, want 0 (no double sends)", sent)
	}
}

func TestFlushSendsEarliestFirst(t *testing.T) {
	s, _, sender := newTestScheduler(t)
	// Added out of order on purpose; "b1" and "b2" tie and must keep
	// insertion order.
	s.Add(Reminder{To: "c", Body: "third", Due: t0.Add(-1 * time.Minute)})
	s.Add(Reminder{To: "a", Body: "first", Due: t0.Add(-3 * time.Minute)})
	s.Add(Reminder{To: "b1", Body: "second", Due: t0.Add(-2 * time.Minute)})
	s.Add(Reminder{To: "b2", Body: "second too", Due: t0.Add(-2 * time.Minute)})

	if _, err := s.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
	want := []message{{"a", "first"}, {"b1", "second"}, {"b2", "second too"}, {"c", "third"}}
	if !slices.Equal(sender.sent, want) {
		t.Errorf("send order = %v, want %v", sender.sent, want)
	}
}

func TestFlushKeepsFailedRemindersForRetry(t *testing.T) {
	s, _, sender := newTestScheduler(t)
	errDown := errors.New("mail server down")
	sender.fail["bob"] = errDown

	s.Add(Reminder{To: "ann", Body: "a", Due: t0})
	s.Add(Reminder{To: "bob", Body: "b", Due: t0})

	sent, err := s.Flush()
	if sent != 1 {
		t.Errorf("first Flush() sent = %d, want 1", sent)
	}
	if !errors.Is(err, errDown) {
		t.Errorf("first Flush() error = %v, want it to wrap %v", err, errDown)
	}
	pending := s.Pending()
	if len(pending) != 1 || pending[0].To != "bob" {
		t.Fatalf("Pending() after failed send = %v, want only bob's reminder", pending)
	}

	// The server recovers: the retry must go through.
	delete(sender.fail, "bob")
	sent, err = s.Flush()
	if sent != 1 || err != nil {
		t.Errorf("retry Flush() = (%d, %v), want (1, nil)", sent, err)
	}
	if n := len(s.Pending()); n != 0 {
		t.Errorf("len(Pending()) after retry = %d, want 0", n)
	}
}

func TestFlushJoinsAllErrors(t *testing.T) {
	s, _, sender := newTestScheduler(t)
	errA, errB := errors.New("a failed"), errors.New("b failed")
	sender.fail["a"], sender.fail["b"] = errA, errB
	s.Add(Reminder{To: "a", Due: t0})
	s.Add(Reminder{To: "ok", Due: t0})
	s.Add(Reminder{To: "b", Due: t0})

	sent, err := s.Flush()
	if sent != 1 {
		t.Errorf("Flush() sent = %d, want 1 (one failure must not stop the rest)", sent)
	}
	for _, want := range []error{errA, errB} {
		if !errors.Is(err, want) {
			t.Errorf("Flush() error = %v, want it to wrap %v", err, want)
		}
	}
}

func TestFlushEmpty(t *testing.T) {
	s, _, _ := newTestScheduler(t)
	if sent, err := s.Flush(); sent != 0 || err != nil {
		t.Errorf("Flush() on empty scheduler = (%d, %v), want (0, nil)", sent, err)
	}
}

func TestPendingReturnsSortedCopy(t *testing.T) {
	s, _, _ := newTestScheduler(t)
	s.Add(Reminder{To: "late", Due: t0.Add(2 * time.Hour)})
	s.Add(Reminder{To: "early", Due: t0.Add(time.Hour)})

	p := s.Pending()
	if len(p) != 2 || p[0].To != "early" || p[1].To != "late" {
		t.Fatalf("Pending() = %v, want [early late]", p)
	}
	p[0].To = "mutated"
	if s.Pending()[0].To != "early" {
		t.Error("modifying the slice returned by Pending changed the scheduler's state")
	}
}

func TestSystemClock(t *testing.T) {
	before := time.Now()
	got := SystemClock{}.Now()
	if got.Before(before) || got.After(time.Now()) {
		t.Errorf("SystemClock.Now() = %v, not between %v and now", got, before)
	}
}
