// Package csvjson converts CSV with a header row into newline-delimited JSON
// (one JSON object per line, also known as NDJSON or JSON Lines).
package csvjson

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrNoHeader is returned when the input does not even contain a header row.
var ErrNoHeader = errors.New("csvjson: missing header row")

// ErrBadHeader is wrapped by errors describing an invalid header row: an
// empty column name or a column name that appears twice.
var ErrBadHeader = errors.New("csvjson: invalid header")

// Convert reads CSV from src and writes one JSON object per data row to dst,
// each followed by '\n'. The first row is the header: its fields become the
// object keys, and every value is a JSON string. A UTF-8 byte order mark
// before the header is ignored, as are completely blank lines.
//
// Convert streams: it holds one record in memory at a time, so output for
// earlier rows may already have been written when an error is returned.
// Errors from the CSV parser (for example a row with the wrong number of
// fields) are returned wrapped, so errors.As(err, new(*csv.ParseError))
// still works and reports the line number.
func Convert(dst io.Writer, src io.Reader) error {
	cr := csv.NewReader(src)
	// FieldsPerRecord = 0 (the default) means "every record must have as
	// many fields as the first one", which is exactly the header rule.

	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return ErrNoHeader
	}
	if err != nil {
		return fmt.Errorf("csvjson: reading header: %w", err)
	}
	if err := checkHeader(header); err != nil {
		return err
	}

	enc := json.NewEncoder(dst) // Encode writes each value followed by '\n'
	row := make(map[string]string, len(header))
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("csvjson: %w", err)
		}
		for i, key := range header {
			row[key] = rec[i] // same keys every time, so reusing the map is safe
		}
		if err := enc.Encode(row); err != nil {
			return fmt.Errorf("csvjson: writing row: %w", err)
		}
	}
}

// checkHeader strips a leading byte order mark (in place) and rejects empty
// or duplicate column names.
func checkHeader(header []string) error {
	header[0] = strings.TrimPrefix(header[0], "\uFEFF")
	seen := make(map[string]bool, len(header))
	for i, name := range header {
		if name == "" {
			return fmt.Errorf("%w: column %d has an empty name", ErrBadHeader, i+1)
		}
		if seen[name] {
			return fmt.Errorf("%w: duplicate column %q", ErrBadHeader, name)
		}
		seen[name] = true
	}
	return nil
}
