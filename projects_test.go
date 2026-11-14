package vikunja

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
)

func TestListProjects_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	_, err := client.ListProjects(context.Background(), ListProjectsRequest{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}

	if gotReq.Method != http.MethodGet {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodGet)
	}
	if gotReq.URL.Path != "/api/v2/projects" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/projects")
	}
	if got := gotReq.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if got := gotReq.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept header = %q, want %q", got, "application/json")
	}
}

func TestListProjects_QueryParameters(t *testing.T) {
	tests := []struct {
		name    string
		request ListProjectsRequest
		want    url.Values
	}{
		{
			name:    "defaults only send is_archived",
			request: ListProjectsRequest{},
			want:    url.Values{"is_archived": {"false"}},
		},
		{
			name: "all parameters set",
			request: ListProjectsRequest{
				Query:      "my project",
				IsArchived: true,
				Expand:     "permissions",
				Page:       2,
				PerPage:    50,
			},
			want: url.Values{
				"q":           {"my project"},
				"is_archived": {"true"},
				"expand":      {"permissions"},
				"page":        {"2"},
				"per_page":    {"50"},
			},
		},
		{
			name:    "query only",
			request: ListProjectsRequest{Query: "inbox"},
			want: url.Values{
				"q":           {"inbox"},
				"is_archived": {"false"},
			},
		},
		{
			name:    "pagination only",
			request: ListProjectsRequest{Page: 3, PerPage: 10},
			want: url.Values{
				"is_archived": {"false"},
				"page":        {"3"},
				"per_page":    {"10"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery url.Values
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.Query()
				_, _ = w.Write([]byte(`{"items":[]}`))
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-token")
			_, err := client.ListProjects(context.Background(), tt.request)
			if err != nil {
				t.Fatalf("ListProjects returned error: %v", err)
			}

			if !reflect.DeepEqual(gotQuery, tt.want) {
				t.Errorf("query = %v, want %v", gotQuery, tt.want)
			}
		})
	}
}

func TestListProjects_DecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id": 1, "title": "Inbox"},
				{"id": 42, "title": "Work"}
			],
			"page": 1,
			"per_page": 50,
			"total": 2,
			"total_pages": 1
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	resp, err := client.ListProjects(context.Background(), ListProjectsRequest{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}

	want := &ListProjectsResponse{
		Items: []Project{
			{Id: 1, Title: "Inbox"},
			{Id: 42, Title: "Work"},
		},
		Page:       1,
		PerPage:    50,
		Total:      2,
		TotalPages: 1,
	}
	if !reflect.DeepEqual(resp, want) {
		t.Errorf("response = %+v, want %+v", resp, want)
	}
}

func TestListProjects_ApiError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-token")
	resp, err := client.ListProjects(context.Background(), ListProjectsRequest{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("response = %+v, want nil", resp)
	}

	var apiErr *ApiError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v (%T), want *ApiError", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
	if apiErr.StatusText != "401 Unauthorized" {
		t.Errorf("StatusText = %q, want %q", apiErr.StatusText, "401 Unauthorized")
	}
	if apiErr.Message != `{"message":"invalid token"}` {
		t.Errorf("Message = %q, want %q", apiErr.Message, `{"message":"invalid token"}`)
	}
}

func TestListProjects_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := NewClient(server.URL, "test-token")
	_, err := client.ListProjects(ctx, ListProjectsRequest{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}
