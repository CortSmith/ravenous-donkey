// Package client is a small, well-behaved HTTP client for an "items" JSON API.
// It shows the habits every Go HTTP client should have: a timeout, a context
// on every request, status codes checked explicitly, response bodies always
// drained and closed, and errors wrapped with context.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
	return fmt.Sprintf("unexpected status %d: %s", e.Code, e.Body)
}

// Client talks to the items API rooted at a base URL.
// It is safe for concurrent use.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a Client for baseURL (for example "http://localhost:8080")
// whose every request, including reading the response body, is limited to
// timeout.
func New(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: timeout},
	}
}

// GetItem fetches GET {base}/items/{id}. The id is path-escaped. A 404 is
// reported as an error wrapping ErrNotFound and any other non-200 status as
// a *StatusError.
func (c *Client) GetItem(ctx context.Context, id string) (Item, error) {
	if id == "" {
		return Item{}, errors.New("get item: empty id")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/items/"+url.PathEscape(id), nil)
	if err != nil {
		return Item{}, fmt.Errorf("get item %q: %w", id, err)
	}
	req.Header.Set("Accept", "application/json")

	var it Item
	if err := c.do(req, http.StatusOK, &it); err != nil {
		var se *StatusError
		if errors.As(err, &se) && se.Code == http.StatusNotFound {
			err = ErrNotFound
		}
		return Item{}, fmt.Errorf("get item %q: %w", id, err)
	}
	return it, nil
}

// CreateItem sends POST {base}/items with it as a JSON body and returns the
// item the server created. It expects 201 Created; anything else is a
// *StatusError.
func (c *Client) CreateItem(ctx context.Context, it Item) (Item, error) {
	body, err := json.Marshal(it)
	if err != nil {
		return Item{}, fmt.Errorf("create item: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/items", bytes.NewReader(body))
	if err != nil {
		return Item{}, fmt.Errorf("create item: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var created Item
	if err := c.do(req, http.StatusCreated, &created); err != nil {
		return Item{}, fmt.Errorf("create item: %w", err)
	}
	return created, nil
}

// do sends req, checks for the wanted status and decodes the JSON body into
// dst. The body is always drained and closed so the connection can be reused.
func (c *Client) do(req *http.Request, want int, dst any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return err // already a *url.Error with method and URL
	}
	defer func() {
		// Reading to EOF lets the Transport put the connection back in its
		// pool; closing alone would force a new TCP (and TLS) handshake.
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != want {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		return &StatusError{Code: resp.StatusCode, Body: strings.TrimSpace(string(b))}
	}
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
