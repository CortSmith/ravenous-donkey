package shortener

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestServer returns a Server with a test store, logger, and time function.
func newTestServer(t *testing.T) (*Server, *http.Client) {
	t.Helper()
	s := NewServer(Config{
		Store:   NewMemoryStore(),
		Logger:  nil, // discard logs
		BaseURL: "http://localhost:8080",
		NewCode: func() (string, error) { return RandomCode() },
		Now: func() time.Time {
			return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		},
	})
	return s, &http.Client{}
}

func TestPostLinksCreateSuccess(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	body := bytes.NewReader([]byte(`{"url":"https://example.com/very/long/path"}`))
	resp, err := client.Post(srv.URL+"/links", "application/json", body)
	if err != nil {
		t.Fatalf("POST /links error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var gotResp struct {
		Code      string `json:"code"`
		URL       string `json:"url"`
		ShortURL  string `json:"short_url"`
		Visits    int64  `json:"visits"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&gotResp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if gotResp.URL != "https://example.com/very/long/path" {
		t.Errorf("url = %q, want %q", gotResp.URL, "https://example.com/very/long/path")
	}
	if len(gotResp.Code) != 7 {
		t.Errorf("generated code length = %d, want 7", len(gotResp.Code))
	}
	if !strings.Contains(gotResp.ShortURL, gotResp.Code) {
		t.Errorf("short_url = %q, want to contain code %q", gotResp.ShortURL, gotResp.Code)
	}
	if gotResp.Visits != 0 {
		t.Errorf("visits = %d, want 0", gotResp.Visits)
	}

	// Check Location header
	loc := resp.Header.Get("Location")
	if !strings.Contains(loc, gotResp.Code) {
		t.Errorf("Location header = %q, want to contain code %q", loc, gotResp.Code)
	}
}

func TestPostLinksCustomCode(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	body := bytes.NewReader([]byte(`{"url":"https://example.com","code":"mycode"}`))
	resp, err := client.Post(srv.URL+"/links", "application/json", body)
	if err != nil {
		t.Fatalf("POST /links error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var gotResp struct {
		Code string `json:"code"`
	}
	json.NewDecoder(resp.Body).Decode(&gotResp)
	if gotResp.Code != "mycode" {
		t.Errorf("code = %q, want %q", gotResp.Code, "mycode")
	}
}

func TestPostLinksValidation(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	tests := []struct {
		name   string
		body   string
		status int
		errMsg string
	}{
		{
			"missing url",
			`{"url":""}`,
			http.StatusBadRequest,
			"url is required",
		},
		{
			"invalid scheme",
			`{"url":"ftp://example.com"}`,
			http.StatusBadRequest,
			"must be an absolute http or https URL",
		},
		{
			"no host",
			`{"url":"https://"}`,
			http.StatusBadRequest,
			"must be an absolute http or https URL",
		},
		{
			"url too long",
			`{"url":"https://example.com/` + strings.Repeat("a", 2100) + `"}`,
			http.StatusBadRequest,
			"url is too long",
		},
		{
			"code too short",
			`{"url":"https://example.com","code":"hi"}`,
			http.StatusBadRequest,
			"code must be 4-32",
		},
		{
			"code too long",
			`{"url":"https://example.com","code":"` + strings.Repeat("a", 40) + `"}`,
			http.StatusBadRequest,
			"code must be 4-32",
		},
		{
			"code invalid chars",
			`{"url":"https://example.com","code":"code!!!"}`,
			http.StatusBadRequest,
			"code must be 4-32",
		},
		{
			"malformed json",
			`{invalid}`,
			http.StatusBadRequest,
			"request body must be a JSON object",
		},
		{
			"unknown field",
			`{"url":"https://example.com","extra":"field"}`,
			http.StatusBadRequest,
			"request body must be a JSON object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := bytes.NewReader([]byte(tt.body))
			resp, err := client.Post(srv.URL+"/links", "application/json", body)
			if err != nil {
				t.Fatalf("POST /links error: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.status {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.status)
			}

			var errResp struct {
				Error string `json:"error"`
			}
			json.NewDecoder(resp.Body).Decode(&errResp)
			if !strings.Contains(errResp.Error, tt.errMsg) {
				t.Errorf("error = %q, want to contain %q", errResp.Error, tt.errMsg)
			}
		})
	}
}

func TestPostLinksDuplicateCode(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	// First request
	body := bytes.NewReader([]byte(`{"url":"https://example.com","code":"duplicate"}`))
	resp1, _ := client.Post(srv.URL+"/links", "application/json", body)
	io.ReadAll(resp1.Body)
	resp1.Body.Close()

	// Second request with same code
	body = bytes.NewReader([]byte(`{"url":"https://different.com","code":"duplicate"}`))
	resp2, err := client.Post(srv.URL+"/links", "application/json", body)
	if err != nil {
		t.Fatalf("POST /links error: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusConflict {
		t.Errorf("status = %d, want %d", resp2.StatusCode, http.StatusConflict)
	}

	var errResp struct {
		Error string `json:"error"`
	}
	json.NewDecoder(resp2.Body).Decode(&errResp)
	if !strings.Contains(errResp.Error, "code already in use") {
		t.Errorf("error = %q, want to contain 'code already in use'", errResp.Error)
	}
}

func TestGetRedirect(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	// Create a link
	createBody := bytes.NewReader([]byte(`{"url":"https://example.com/target","code":"test1"}`))
	createResp, err := client.Post(srv.URL+"/links", "application/json", createBody)
	if err != nil {
		t.Fatalf("POST /links error: %v", err)
	}
	io.ReadAll(createResp.Body)
	createResp.Body.Close()

	// Redirect (with Follow=false to not follow the redirect)
	req, _ := http.NewRequest("GET", srv.URL+"/test1", nil)
	noFollowClient := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	redirectResp, err := noFollowClient.Do(req)
	if err != nil {
		t.Fatalf("GET /test1 error: %v", err)
	}
	defer redirectResp.Body.Close()

	if redirectResp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want %d", redirectResp.StatusCode, http.StatusFound)
	}

	loc := redirectResp.Header.Get("Location")
	if loc != "https://example.com/target" {
		t.Errorf("Location = %q, want %q", loc, "https://example.com/target")
	}
}

func TestGetRedirectNotFound(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/nonexistent", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /nonexistent error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetStats(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	// Create a link
	createBody := bytes.NewReader([]byte(`{"url":"https://example.com","code":"stats1"}`))
	createResp, _ := client.Post(srv.URL+"/links", "application/json", createBody)
	io.ReadAll(createResp.Body)
	createResp.Body.Close()

	// Redirect to increment visits
	redirectReq, _ := http.NewRequest("GET", srv.URL+"/stats1", nil)
	redirectResp, _ := client.Do(redirectReq)
	io.ReadAll(redirectResp.Body)
	redirectResp.Body.Close()

	// Get stats
	statsReq, _ := http.NewRequest("GET", srv.URL+"/links/stats1/stats", nil)
	statsResp, err := client.Do(statsReq)
	if err != nil {
		t.Fatalf("GET /links/stats1/stats error: %v", err)
	}
	defer statsResp.Body.Close()

	if statsResp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", statsResp.StatusCode, http.StatusOK)
	}

	var statsBody struct {
		Code   string `json:"code"`
		Visits int64  `json:"visits"`
	}
	json.NewDecoder(statsResp.Body).Decode(&statsBody)

	if statsBody.Code != "stats1" {
		t.Errorf("code = %q, want %q", statsBody.Code, "stats1")
	}
	if statsBody.Visits != 1 {
		t.Errorf("visits = %d, want 1 (did visit counter increment?)", statsBody.Visits)
	}
}

func TestGetStatsNotFound(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	req, _ := http.NewRequest("GET", srv.URL+"/links/nonexistent/stats", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /links/nonexistent/stats error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestContentType(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	body := bytes.NewReader([]byte(`{"url":"https://example.com"}`))
	resp, err := client.Post(srv.URL+"/links", "application/json", body)
	if err != nil {
		t.Fatalf("POST /links error: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestBodyTooLarge(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	// Create a body with valid JSON that's larger than MaxBodyBytes (4 KiB)
	// MaxBytesReader will cause Read to return an error once the limit is exceeded
	largeJSON := `{"url":"https://` + strings.Repeat("a", MaxBodyBytes) + `"}`
	largeBody := bytes.NewReader([]byte(largeJSON))
	resp, err := client.Post(srv.URL+"/links", "application/json", largeBody)
	if err != nil {
		t.Fatalf("POST /links error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusRequestEntityTooLarge)
	}
}

func TestVisitCounterIncrement(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	// Create a link
	createBody := bytes.NewReader([]byte(`{"url":"https://example.com","code":"counter"}`))
	createResp, _ := client.Post(srv.URL+"/links", "application/json", createBody)
	io.ReadAll(createResp.Body)
	createResp.Body.Close()

	// Redirect multiple times
	for i := 0; i < 5; i++ {
		redirectReq, _ := http.NewRequest("GET", srv.URL+"/counter", nil)
		redirectResp, _ := client.Do(redirectReq)
		io.ReadAll(redirectResp.Body)
		redirectResp.Body.Close()
	}

	// Check stats
	statsReq, _ := http.NewRequest("GET", srv.URL+"/links/counter/stats", nil)
	statsResp, err := client.Do(statsReq)
	if err != nil {
		t.Fatalf("GET /links/counter/stats error: %v", err)
	}
	defer statsResp.Body.Close()

	var statsBody struct {
		Visits int64 `json:"visits"`
	}
	json.NewDecoder(statsResp.Body).Decode(&statsBody)

	if statsBody.Visits != 5 {
		t.Errorf("visits = %d, want 5 (check if redirects increment the counter)", statsBody.Visits)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	// PUT to /links should not be allowed
	req, _ := http.NewRequest("PUT", srv.URL+"/links", bytes.NewReader([]byte(`{}`)))
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("PUT /links error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("PUT /links status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}

	// DELETE to /code should not be allowed
	req, _ = http.NewRequest("DELETE", srv.URL+"/testcode", nil)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("DELETE /code error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /code status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestConcurrentCreatesAndRedirects(t *testing.T) {
	s, client := newTestServer(t)
	srv := httptest.NewServer(s)
	defer srv.Close()

	// Concurrently create and redirect
	const n = 10
	done := make(chan bool, n)
	digits := "0123456789abcdefghij"

	for i := 0; i < n; i++ {
		go func(id int) {
			// Create
			digit := string(digits[id])
			code := "conc" + digit + digit // e.g., "conc00", "conc11"
			url := "https://example.com/" + digit
			createBody := bytes.NewReader([]byte(`{"url":"` + url + `","code":"` + code + `"}`))
			createResp, _ := client.Post(srv.URL+"/links", "application/json", createBody)
			io.ReadAll(createResp.Body)
			createResp.Body.Close()

			// Redirect
			redirectReq, _ := http.NewRequest("GET", srv.URL+"/"+code, nil)
			noFollowClient := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}}
			redirectResp, _ := noFollowClient.Do(redirectReq)
			io.ReadAll(redirectResp.Body)
			redirectResp.Body.Close()

			done <- true
		}(i)
	}

	for i := 0; i < n; i++ {
		<-done
	}

	// Verify all were created
	for i := 0; i < n; i++ {
		digit := string(digits[i])
		code := "conc" + digit + digit
		statsReq, _ := http.NewRequest("GET", srv.URL+"/links/"+code+"/stats", nil)
		statsResp, _ := client.Do(statsReq)
		if statsResp.StatusCode != http.StatusOK {
			t.Errorf("link %s not found (concurrent create failed?)", code)
		}
		io.ReadAll(statsResp.Body)
		statsResp.Body.Close()
	}
}
