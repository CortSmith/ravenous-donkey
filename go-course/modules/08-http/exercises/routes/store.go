package routes

import (
	"slices"
	"sync"
)

// Note is a short piece of text identified by a positive integer ID.
type Note struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}

// Store is an in-memory, concurrency-safe collection of notes.
// It is provided complete; the exercise is the HTTP layer in routes.go.
type Store struct {
	mu     sync.Mutex
	notes  map[int]Note
	nextID int
}

// NewStore returns an empty Store whose first note gets ID 1.
func NewStore() *Store {
	return &Store{notes: make(map[int]Note), nextID: 1}
}

// Add stores a new note and returns it with its assigned ID.
func (s *Store) Add(text string) Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := Note{ID: s.nextID, Text: text}
	s.notes[n.ID] = n
	s.nextID++
	return n
}

// Get returns the note with the given ID.
func (s *Store) Get(id int) (Note, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n, ok := s.notes[id]
	return n, ok
}

// Delete removes the note with the given ID and reports whether it existed.
func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.notes[id]
	delete(s.notes, id)
	return ok
}

// List returns all notes ordered by ID. It never returns nil.
func (s *Store) List() []Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Note, 0, len(s.notes))
	for _, n := range s.notes {
		out = append(out, n)
	}
	slices.SortFunc(out, func(a, b Note) int { return a.ID - b.ID })
	return out
}

// Latest returns the note with the highest ID.
func (s *Store) Latest() (Note, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var latest Note
	for id, n := range s.notes {
		if id > latest.ID {
			latest = n
		}
	}
	return latest, latest.ID != 0
}
