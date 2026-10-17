// Package csvjson converts CSV with a header row into newline-delimited JSON
// (one JSON object per line, also known as NDJSON or JSON Lines).
package csvjson

import (
	"errors"
	"io"
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
//
// TODO: implement with encoding/csv (Reader.Read in a loop, not ReadAll)
// and encoding/json (an Encoder writes one value plus '\n' per Encode).
// Validate the header: no empty names (wrap ErrBadHeader and mention the
// 1-based column number, e.g. "column 2"), no duplicates (wrap ErrBadHeader
// and quote the name with %q).
func Convert(dst io.Writer, src io.Reader) error {
	return errors.New("not implemented")
}
