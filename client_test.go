package vikunja

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
