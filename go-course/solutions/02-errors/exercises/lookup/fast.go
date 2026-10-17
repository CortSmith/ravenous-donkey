package lookup

import "strconv"

// NotFoundError is a cheap alternative to fmt.Errorf("user %d: %w", id,
// ErrNotFound). It stores only the ID; the message is built lazily in Error,
// which is never called when the caller just checks errors.Is and moves on
// (as Greet does).
//
// It is a value type (not a pointer) holding one int, so returning it in an
// error interface usually costs a single 8-byte allocation instead of
// fmt.Errorf's formatting work plus a *fmt.wrapError and a string.
type NotFoundError struct{ ID int }

func (e NotFoundError) Error() string {
	return "user " + strconv.Itoa(e.ID) + ": not found"
}

// Is makes errors.Is(err, ErrNotFound) report true without storing the
// sentinel. Use a custom Is only when you need this kind of equivalence.
func (e NotFoundError) Is(target error) bool { return target == ErrNotFound }

// FastMapStore is MapStore with a lazily formatted not-found error.
type FastMapStore map[int]User

// Get behaves like MapStore.Get, with identical error text.
func (m FastMapStore) Get(id int) (User, error) {
	u, ok := m[id]
	if !ok {
		return User{}, NotFoundError{ID: id}
	}
	return u, nil
}
