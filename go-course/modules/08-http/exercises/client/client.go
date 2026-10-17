// Package client is a small, well-behaved HTTP client for an "items" JSON API.
// It shows the habits every Go HTTP client should have: a timeout, a context
// on every request, status codes checked explicitly, response bodies always
// drained and closed, and errors wrapped with context.
package client

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Item is the resource served by the API. Price is in cents.
type Item struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}

// ErrNotFound is returned (wrapped) when the server answers 404.
var ErrNotFound = errors.New("item not found")

// maxErrorBody is how much of an unexpected response body is kept in a
// StatusError.
const maxErrorBody = 512

// StatusError reports an unexpected HTTP status. Body holds at most the first
// 512 bytes of the response body, with surrounding whitespace trimmed.
type StatusError struct {
	Code int
	Body string
}

func (e *StatusError) Error() string {
	// TODO: implement Error
	return ""
}

// Client talks to the items API rooted at a base URL.
// It is safe for concurrent use.
type Client struct {
	// TODO: add fields
}

// New returns a Client for baseURL (for example "http://localhost:8080")
// whose every request, including reading the response body, is limited to
// timeout.
func New(baseURL string, timeout time.Duration) *Client {
	// TODO: implement New
	_ = baseURL
	_ = timeout
	return nil
}

// GetItem fetches GET {base}/items/{id}. The id is path-escaped. A 404 is
// reported as an error wrapping ErrNotFound and any other non-200 status as
// a *StatusError.
func (c *Client) GetItem(ctx context.Context, id string) (Item, error) {
	// TODO: implement GetItem
	_ = ctx
	_ = id
	return Item{}, nil
}

// CreateItem sends POST {base}/items with it as a JSON body and returns the
// item the server created. It expects 201 Created; anything else is a
// *StatusError.
func (c *Client) CreateItem(ctx context.Context, it Item) (Item, error) {
	// TODO: implement CreateItem
	_ = ctx
	_ = it
	return Item{}, nil
}

// do sends req, checks for the wanted status and decodes the JSON body into
// dst. The body is always drained and closed so the connection can be reused.
func (c *Client) do(req *http.Request, want int, dst any) error {
	// TODO: implement do
	_ = req
	_ = want
	_ = dst
	return nil
}
