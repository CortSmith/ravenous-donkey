package reminder

import "testing"

// Write tests to expose the 3 bugs in reminder.go. Use fake Clock and Sender
// implementations so tests control time and Send behavior.
//
// TODO: Write fakeClock (implements Clock interface):
//   - Holds a time.Time, Now() returns it, Advance(d) moves it forward
//
// TODO: Write fakeSender (implements Sender interface):
//   - Records all Send calls in a slice
//   - Can be configured to fail for certain recipients
//
// TODO: Write test functions (use table-driven subtests where helpful):
//   - TestFlushSendsOnlyDueReminders: add due and future reminders, verify only due are sent
//   - TestFlushLaterAfterClockAdvances: move clock forward, verify previously-future reminders are now sent
//   - TestFlushSendsEarliestFirst: add reminders in reverse order, verify sent in due order
//   - TestFlushKeepsFailedRemindersForRetry: fail a Send, verify reminder stays pending
//   - TestFlushJoinsAllErrors: multiple failures, verify errors.Join combines them

func TestAddThenPending(t *testing.T) {
	s := New(SystemClock{}, nil)
	s.Add(Reminder{To: "alice", Body: "hello", Due: SystemClock{}.Now()})
	if len(s.Pending()) != 1 {
		t.Errorf("Pending() returned %d reminders, want 1", len(s.Pending()))
	}
}
