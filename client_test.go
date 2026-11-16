package vikunja

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func newTestClient(baseURL string) *Client {
	return NewClient(WithBaseURL(baseURL), WithAPIToken("test-token"))
}

func newTestRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	return req
}

type testContextKey struct{}

func TestNewRequest_BuildsRequest(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		method  string
		path    string
		wantURL string
	}{
		{
			name:    "base without trailing slash",
			baseURL: "https://vikunja.example",
			method:  http.MethodGet,
			path:    "/api/v2/projects",
			wantURL: "https://vikunja.example/api/v2/projects",
		},
		{
			name:    "base with trailing slash",
			baseURL: "https://vikunja.example/",
			method:  http.MethodGet,
			path:    "/api/v2/projects",
			wantURL: "https://vikunja.example/api/v2/projects",
		},
		{
			name:    "base with path prefix",
			baseURL: "https://example.com/vikunja",
			method:  http.MethodDelete,
			path:    "api/v2/projects/1",
			wantURL: "https://example.com/vikunja/api/v2/projects/1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), testContextKey{}, "value")
			client := newTestClient(tt.baseURL)

			req, err := client.newRequest(ctx, tt.method, tt.path)
			if err != nil {
				t.Fatalf("newRequest returned error: %v", err)
			}

			if req.Method != tt.method {
				t.Errorf("method = %q, want %q", req.Method, tt.method)
			}
			if got := req.URL.String(); got != tt.wantURL {
				t.Errorf("URL = %q, want %q", got, tt.wantURL)
			}
			if req.Body != http.NoBody {
				t.Errorf("body = %v, want http.NoBody", req.Body)
			}
			if req.ContentLength != 0 {
				t.Errorf("ContentLength = %d, want 0", req.ContentLength)
			}
			if got := req.Header.Get("Content-Type"); got != "" {
				t.Errorf("Content-Type = %q, want empty", got)
			}
			if req.Context().Value(testContextKey{}) != "value" {
				t.Error("request does not use the provided context")
			}
		})
	}
}

func TestNewRequest_Errors(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		method     string
		wantPrefix string
	}{
		{name: "invalid base URL", baseURL: "http://[::1", method: http.MethodGet, wantPrefix: "building URL: "},
		{name: "invalid method", baseURL: "https://vikunja.example", method: "BAD METHOD", wantPrefix: "creating request: "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(tt.baseURL)

			req, err := client.newRequest(context.Background(), tt.method, "/api/v2/projects")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if req != nil {
				t.Errorf("request = %v, want nil", req)
			}
			if !strings.HasPrefix(err.Error(), tt.wantPrefix) {
				t.Errorf("error = %q, want prefix %q", err.Error(), tt.wantPrefix)
			}
		})
	}
}

func TestNewRequestWithBody_BuildsRequest(t *testing.T) {
	ctx := context.WithValue(context.Background(), testContextKey{}, "value")
	client := newTestClient("https://vikunja.example/")
	reqBody := struct {
		Title string `json:"title"`
		Done  bool   `json:"done"`
	}{Title: "Buy milk", Done: true}

	req, err := client.newRequestWithBody(ctx, http.MethodPost, "/api/v2/tasks", reqBody)
	if err != nil {
		t.Fatalf("newRequestWithBody returned error: %v", err)
	}

	if req.Method != http.MethodPost {
		t.Errorf("method = %q, want %q", req.Method, http.MethodPost)
	}
	if want := "https://vikunja.example/api/v2/tasks"; req.URL.String() != want {
		t.Errorf("URL = %q, want %q", req.URL.String(), want)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
	if req.Context().Value(testContextKey{}) != "value" {
		t.Error("request does not use the provided context")
	}

	const wantBody = `{"title":"Buy milk","done":true}`
	if req.ContentLength != int64(len(wantBody)) {
		t.Errorf("ContentLength = %d, want %d", req.ContentLength, len(wantBody))
	}
	gotBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}
	if string(gotBody) != wantBody {
		t.Errorf("body = %s, want %s", gotBody, wantBody)
	}

	// GetBody lets the transport replay the body on redirects and retries.
	if req.GetBody == nil {
		t.Fatal("GetBody = nil, want non-nil")
	}
	replay, err := req.GetBody()
	if err != nil {
		t.Fatalf("GetBody returned error: %v", err)
	}
	replayBody, err := io.ReadAll(replay)
	if err != nil {
		t.Fatalf("failed to read replayed body: %v", err)
	}
	if string(replayBody) != wantBody {
		t.Errorf("replayed body = %s, want %s", replayBody, wantBody)
	}
}

func TestNewRequestWithBody_SentToServer(t *testing.T) {
	var gotContentType string
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	req, err := client.newRequestWithBody(context.Background(), http.MethodPut, "/api/v2/projects", map[string]string{"title": "Inbox"})
	if err != nil {
		t.Fatalf("newRequestWithBody returned error: %v", err)
	}
	if err := client.do(req, nil); err != nil {
		t.Fatalf("do returned error: %v", err)
	}

	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotContentType, "application/json")
	}
	if gotBody["title"] != "Inbox" {
		t.Errorf("body = %v, want title Inbox", gotBody)
	}
}

func TestNewRequestWithBody_Errors(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		method     string
		reqBody    any
		wantPrefix string
		wantAs     any
	}{
		{
			name:       "invalid base URL",
			baseURL:    "http://[::1",
			method:     http.MethodPost,
			reqBody:    map[string]string{},
			wantPrefix: "building URL: ",
			wantAs:     new(*url.Error),
		},
		{
			name:       "unmarshalable body",
			baseURL:    "https://vikunja.example",
			method:     http.MethodPost,
			reqBody:    make(chan int),
			wantPrefix: "marshaling request: ",
			wantAs:     new(*json.UnsupportedTypeError),
		},
		{
			name:       "invalid method",
			baseURL:    "https://vikunja.example",
			method:     "BAD METHOD",
			reqBody:    map[string]string{},
			wantPrefix: "creating request: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newTestClient(tt.baseURL)

			req, err := client.newRequestWithBody(context.Background(), tt.method, "/api/v2/tasks", tt.reqBody)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if req != nil {
				t.Errorf("request = %v, want nil", req)
			}
			if !strings.HasPrefix(err.Error(), tt.wantPrefix) {
				t.Errorf("error = %q, want prefix %q", err.Error(), tt.wantPrefix)
			}
			if tt.wantAs != nil && !errors.As(err, tt.wantAs) {
				t.Errorf("error = %v (%T), want wrapped %T", err, err, tt.wantAs)
			}
		})
	}
}

func TestDo_AcceptsAny2xx(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   map[string]string
	}{
		{name: "200 with body", status: http.StatusOK, body: `{"a":"b"}`, want: map[string]string{"a": "b"}},
		{name: "201 with body", status: http.StatusCreated, body: `{"a":"b"}`, want: map[string]string{"a": "b"}},
		{name: "204 without body", status: http.StatusNoContent, body: "", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			client := newTestClient(server.URL)
			var got map[string]string
			if err := client.do(newTestRequest(t, server.URL), &got); err != nil {
				t.Fatalf("do returned error: %v", err)
			}
			if len(got) != len(tt.want) || got["a"] != tt.want["a"] {
				t.Errorf("body = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDo_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	err := client.do(newTestRequest(t, server.URL), nil)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v (%T), want *APIError", err, err)
	}
	if want := "404 - Not Found: not found"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestDo_LimitsErrorBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(strings.Repeat("x", maxErrorBodySize+100)))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	err := client.do(newTestRequest(t, server.URL), nil)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v (%T), want *APIError", err, err)
	}
	if len(apiErr.Message) != maxErrorBodySize {
		t.Errorf("len(Message) = %d, want %d", len(apiErr.Message), maxErrorBodySize)
	}
}

func TestNewClient_WithUserAgent(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want string
	}{
		{name: "default", opts: nil, want: DefaultUserAgent},
		{name: "custom", opts: []Option{WithUserAgent("my-app/1.0")}, want: "my-app/1.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("User-Agent")
			}))
			defer server.Close()

			opts := append(tt.opts, WithBaseURL(server.URL))
			client := NewClient(opts...)
			if err := client.do(newTestRequest(t, server.URL), nil); err != nil {
				t.Fatalf("do returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("User-Agent = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewClient_WithHTTPClient(t *testing.T) {
	custom := &http.Client{}
	if got := NewClient(WithBaseURL("http://vikunja.invalid"), WithHTTPClient(custom)).httpClient; got != custom {
		t.Errorf("httpClient = %p, want %p", got, custom)
	}
	if got := NewClient(WithBaseURL("http://vikunja.invalid"), WithHTTPClient(nil)).httpClient; got == nil {
		t.Error("httpClient = nil, want default client")
	}
}

func TestNewClient_WithApiToken(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want string
	}{
		{name: "default", opts: nil, want: ""},
		{name: "custom", opts: []Option{WithAPIToken("test-token")}, want: "Bearer test-token"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("Authorization")
			}))
			defer server.Close()

			opts := append(tt.opts, WithBaseURL(server.URL))
			client := NewClient(opts...)
			if err := client.do(newTestRequest(t, server.URL), nil); err != nil {
				t.Fatalf("do returned error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Authorization = %q, want %q", got, tt.want)
			}
		})
	}
}
