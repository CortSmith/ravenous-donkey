// Package paginate computes page boundaries for showing a list of results
// one page at a time.
//
// This package has 3 bugs. Write tests in paginate_test.go that expose
// them, watch the tests fail, then fix the code.
package paginate

import "errors"

var (
	// ErrInvalidSize is returned when total is negative or perPage is less
	// than 1.
	ErrInvalidSize = errors.New("paginate: invalid size")

	// ErrInvalidPage is returned when the requested page does not exist.
	ErrInvalidPage = errors.New("paginate: page out of range")
)

// Pages reports how many pages are needed to show total items with perPage
// items on each page. The last page may be partly filled. Pages returns 0
// when total is 0 or when the arguments are invalid (total < 0 or
// perPage < 1).
func Pages(total, perPage int) int {
	if total < 0 || perPage < 1 {
		return 0
	}
	return total/perPage + 1
}

// Bounds returns the half-open range [start, end) of item indexes shown on
// the given 1-based page, so that items[start:end] is that page's contents.
//
// Page 1 always exists, even when total is 0 (it is simply empty), so a UI
// can always render a first page. Any other page must satisfy
// 1 <= page <= Pages(total, perPage); otherwise Bounds returns
// ErrInvalidPage. If total < 0 or perPage < 1, Bounds returns ErrInvalidSize.
func Bounds(total, perPage, page int) (start, end int, err error) {
	if total < 0 || perPage < 1 {
		return 0, 0, ErrInvalidSize
	}
	if page == 1 && total == 0 {
		return 0, 0, nil
	}
	if page < 0 || page > Pages(total, perPage) {
		return 0, 0, ErrInvalidPage
	}
	start = (page - 1) * perPage
	end = start + perPage
	return start, end, nil
}
