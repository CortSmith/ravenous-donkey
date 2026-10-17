package jsonio

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

func TestWriteJSON(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		v        any
		wantBody string
	}{
		{"struct", http.StatusOK, person{Name: "Ann", Age: 40}, `{"name":"Ann","age":40}` + "\n"},
		{"created map", http.StatusCreated, map[string]int{"id": 7}, `{"id":7}` + "\n"},
		{"slice", http.StatusOK, []string{"a", "b"}, `["a","b"]` + "\n"},
		{"nil", http.StatusOK, nil, "null\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			if err := WriteJSON(rec, tt.status, tt.v); err != nil {
				t.Fatalf("WriteJSON returned error: %v", err)
			}
			if rec.Code != tt.status {
				t.Errorf("status = %d, want %d", rec.Code, tt.status)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want %q", got, "application/json")
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
		})
	}
}

func TestWriteJSONEncodeFailure(t *testing.T) {
	rec := httptest.NewRecorder()
	// Channels cannot be encoded as JSON.
	err := WriteJSON(rec, http.StatusOK, map[string]any{"ch": make(chan int)})
	if err == nil {
		t.Fatal("WriteJSON(chan) returned nil error, want an encoding error")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d (encode into a buffer before calling WriteHeader)", rec.Code, http.StatusInternalServerError)
	}
	if got := rec.Header().Get("Content-Type"); got == "application/json" {
		t.Errorf("Content-Type = %q; the 500 response is not JSON", got)
	}
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteError(rec, http.StatusNotFound, `no "item" here`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
	if got, want := rec.Body.String(), `{"error":"no \"item\" here"}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func newJSONRequest(body, contentType string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/people", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req
}

func TestDecodeJSONValid(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		want        person
	}{
		{"simple", `{"name":"Ann","age":40}`, "application/json", person{"Ann", 40}},
		{"charset parameter", `{"name":"Bo","age":3}`, "application/json; charset=utf-8", person{"Bo", 3}},
		{"upper-case media type", `{"name":"Cy"}`, "Application/JSON", person{Name: "Cy"}},
		{"surrounding whitespace", "\n  {\"age\": 1}  \n\t", "application/json", person{Age: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got person
			err := DecodeJSON(httptest.NewRecorder(), newJSONRequest(tt.body, tt.contentType), &got)
			if err != nil {
				t.Fatalf("DecodeJSON(%q) error = %v, want nil", tt.body, err)
			}
			if got != tt.want {
				t.Errorf("DecodeJSON(%q) decoded %+v, want %+v", tt.body, got, tt.want)
			}
		})
	}
}

func TestDecodeJSONErrors(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		contentType string
		wantStatus  int
		wantInMsg   string
	}{
		{"missing content type", `{"name":"Ann"}`, "", http.StatusUnsupportedMediaType, "Content-Type"},
		{"wrong content type", `{"name":"Ann"}`, "text/plain", http.StatusUnsupportedMediaType, "Content-Type"},
		{"empty body", "", "application/json", http.StatusBadRequest, "empty"},
		{"syntax error", `{"name":"Ann",}`, "application/json", http.StatusBadRequest, "badly-formed"},
		{"truncated", `{"name":"Ann"`, "application/json", http.StatusBadRequest, "badly-formed"},
		{"wrong type", `{"age":"forty"}`, "application/json", http.StatusBadRequest, `"age"`},
		{"unknown field", `{"name":"Ann","admin":true}`, "application/json", http.StatusBadRequest, `"admin"`},
		{"two values", `{"name":"Ann"}{"name":"Bo"}`, "application/json", http.StatusBadRequest, "single JSON value"},
		{"trailing garbage", `{"name":"Ann"} x`, "application/json", http.StatusBadRequest, "single JSON value"},
		{
			name:        "too large",
			body:        `{"name":"` + strings.Repeat("a", MaxBodyBytes) + `"}`,
			contentType: "application/json",
			wantStatus:  http.StatusRequestEntityTooLarge,
			wantInMsg:   "larger than",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst person
			err := DecodeJSON(httptest.NewRecorder(), newJSONRequest(tt.body, tt.contentType), &dst)
			if err == nil {
				t.Fatalf("DecodeJSON returned nil error, want *RequestError with status %d", tt.wantStatus)
			}
			var reqErr *RequestError
			if !errors.As(err, &reqErr) {
				t.Fatalf("DecodeJSON error = %v (%T), want a *RequestError", err, err)
			}
			if reqErr.Status != tt.wantStatus {
				t.Errorf("status = %d, want %d (msg %q)", reqErr.Status, tt.wantStatus, reqErr.Msg)
			}
			if !strings.Contains(reqErr.Msg, tt.wantInMsg) {
				t.Errorf("msg = %q, want it to contain %q", reqErr.Msg, tt.wantInMsg)
			}
			if reqErr.Error() != reqErr.Msg {
				t.Errorf("Error() = %q, want Msg %q", reqErr.Error(), reqErr.Msg)
			}
		})
	}
}

func TestDecodeJSONSecondValueTooLarge(t *testing.T) {
	// The first value is fine but the request keeps going past the limit:
	// that is still "too large", not merely "more than one value".
	body := `{"name":"Ann"}` + strings.Repeat(" ", MaxBodyBytes)
	var dst person
	err := DecodeJSON(httptest.NewRecorder(), newJSONRequest(body, "application/json"), &dst)
	var reqErr *RequestError
	if !errors.As(err, &reqErr) || reqErr.Status != http.StatusRequestEntityTooLarge {
		t.Errorf("DecodeJSON error = %v, want *RequestError with status 413", err)
	}
}

func TestDecodeJSONNonPointerIsNotClientError(t *testing.T) {
	var dst person
	err := DecodeJSON(httptest.NewRecorder(), newJSONRequest(`{"name":"Ann"}`, "application/json"), dst)
	if err == nil {
		t.Fatal("DecodeJSON(non-pointer) returned nil error")
	}
	var reqErr *RequestError
	if errors.As(err, &reqErr) {
		t.Errorf("DecodeJSON(non-pointer) returned *RequestError %+v; a programming error is not the client's fault", reqErr)
	}
}
