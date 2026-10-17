// Package reminder sends reminders once they fall due.
//
// This package has 3 bugs. Write tests in reminder_test.go that expose
// them (you will need fake Clock and Sender implementations), watch the
// tests fail, then fix the code.
//
// The Scheduler talks to the outside world only through two small
// interfaces, Clock and Sender, so tests can replace both with fakes.
package reminder

import (
	"errors"
	"fmt"
	"slices"
	"time"
)

// Clock reports the current time.
type Clock interface {
	Now() time.Time
}

// SystemClock is a Clock backed by time.Now.
type SystemClock struct{}

// Now returns the current local time.
func (SystemClock) Now() time.Time { return time.Now() }

// Sender delivers one message to one recipient.
type Sender interface {
	Send(to, body string) error
}

// Reminder is a message to deliver to To at or after Due.
type Reminder struct {
	To   string
	Body string
	Due  time.Time
}

// Scheduler holds pending reminders and sends them when they are due.
// A Scheduler is not safe for concurrent use.
type Scheduler struct {
	clock   Clock
	sender  Sender
	pending []Reminder
}

// New returns a Scheduler that reads the time from clock and delivers
// through sender.
func New(clock Clock, sender Sender) *Scheduler {
	return &Scheduler{clock: clock, sender: sender}
}

// Add queues r. It does not send anything, even if r is already due.
func (s *Scheduler) Add(r Reminder) {
	s.pending = append(s.pending, r)
}

// Pending returns a copy of the reminders not yet sent, earliest Due first
// (reminders with equal Due times keep the order they were added in).
func (s *Scheduler) Pending() []Reminder {
	p := slices.Clone(s.pending)
	slices.SortStableFunc(p, byDue)
	return p
}

// Flush sends every pending reminder whose Due time is at or before the
// clock's current time, earliest Due first (ties go in the order they were
// added). Sent reminders are removed. A reminder whose Send fails stays
// pending so that a later Flush retries it; Flush carries on with the rest.
//
// Flush returns how many reminders were sent and all send failures joined
// with errors.Join (nil if there were none).
func (s *Scheduler) Flush() (int, error) {
	now := s.clock.Now()

	var (
		keep []Reminder
		errs []error
		sent int
	)
	for _, r := range s.pending {
		if !r.Due.Before(now) {
			keep = append(keep, r)
			continue
		}
		if err := s.sender.Send(r.To, r.Body); err != nil {
			errs = append(errs, fmt.Errorf("send reminder to %s: %w", r.To, err))
			continue
		}
		sent++
	}
	s.pending = keep
	return sent, errors.Join(errs...)
}

func byDue(a, b Reminder) int { return a.Due.Compare(b.Due) }
