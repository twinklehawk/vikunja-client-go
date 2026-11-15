package vikunja

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
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

func TestListProjects_RemovesDuplicateSlashInURL(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL+"/", "test-token")
	_, err := client.ListProjects(context.Background(), ListProjectsRequest{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}

	if gotReq.URL.Path != "/api/v2/projects" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/projects")
	}
}

func TestListProjects_QueryParameters(t *testing.T) {
	tests := []struct {
		name    string
		request ListProjectsRequest
		want    url.Values
	}{
		{
			name:    "defaults sends nothing",
			request: ListProjectsRequest{},
			want:    url.Values{},
		},
		{
			name: "all parameters set",
			request: ListProjectsRequest{
				Query:      "my project",
				IsArchived: true,
				Expand:     "permissions",
				Format:     "markdown",
				Page:       2,
				PerPage:    50,
			},
			want: url.Values{
				"q":           {"my project"},
				"is_archived": {"true"},
				"expand":      {"permissions"},
				"format":      {"markdown"},
				"page":        {"2"},
				"per_page":    {"50"},
			},
		},
		{
			name:    "query only",
			request: ListProjectsRequest{Query: "inbox"},
			want: url.Values{
				"q": {"inbox"},
			},
		},
		{
			name:    "pagination only",
			request: ListProjectsRequest{Page: 3, PerPage: 10},
			want: url.Values{
				"page":     {"3"},
				"per_page": {"10"},
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
			{ID: 1, Title: "Inbox"},
			{ID: 42, Title: "Work"},
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

	var apiErr *APIError
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

// pagedProjectsServer serves pages of projects, recording the query of each request.
func pagedProjectsServer(t *testing.T, pages []string, gotQueries *[]url.Values) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotQueries = append(*gotQueries, r.URL.Query())
		page, err := strconv.Atoi(r.URL.Query().Get("page"))
		if err != nil || page < 1 || page > len(pages) {
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(pages[page-1]))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestListProjectsStream_IteratesAllPages(t *testing.T) {
	var gotQueries []url.Values
	server := pagedProjectsServer(t, []string{
		`{"items":[{"id":1,"title":"Inbox"},{"id":2,"title":"Work"}],"page":1,"total_pages":2}`,
		`{"items":[{"id":3,"title":"Home"}],"page":2,"total_pages":2}`,
	}, &gotQueries)

	client := NewClient(server.URL, "test-token")
	var got []Project
	for p, err := range client.ListProjectsStream(context.Background(), ListProjectsRequest{Query: "x", PerPage: 2, Page: 5}) {
		if err != nil {
			t.Fatalf("ListProjectsStream returned error: %v", err)
		}
		got = append(got, p)
	}

	want := []Project{{ID: 1, Title: "Inbox"}, {ID: 2, Title: "Work"}, {ID: 3, Title: "Home"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("projects = %+v, want %+v", got, want)
	}

	wantQueries := []url.Values{
		{"q": {"x"}, "page": {"1"}, "per_page": {"2"}},
		{"q": {"x"}, "page": {"2"}, "per_page": {"2"}},
	}
	if !reflect.DeepEqual(gotQueries, wantQueries) {
		t.Errorf("queries = %v, want %v", gotQueries, wantQueries)
	}
}

func TestListProjectsStream_StopsOnBreak(t *testing.T) {
	var gotQueries []url.Values
	server := pagedProjectsServer(t, []string{
		`{"items":[{"id":1,"title":"Inbox"},{"id":2,"title":"Work"}],"page":1,"total_pages":2}`,
		`{"items":[{"id":3,"title":"Home"}],"page":2,"total_pages":2}`,
	}, &gotQueries)

	client := NewClient(server.URL, "test-token")
	for p, err := range client.ListProjectsStream(context.Background(), ListProjectsRequest{}) {
		if err != nil {
			t.Fatalf("ListProjectsStream returned error: %v", err)
		}
		if p.ID == 1 {
			break
		}
	}

	if len(gotQueries) != 1 {
		t.Errorf("requests = %d, want 1", len(gotQueries))
	}
}

func TestListProjectsStream_StopsOnEmptyPage(t *testing.T) {
	var gotQueries []url.Values
	server := pagedProjectsServer(t, []string{
		`{"items":[{"id":1,"title":"Inbox"}],"total_pages":5}`,
		`{"items":[],"total_pages":5}`,
	}, &gotQueries)

	client := NewClient(server.URL, "test-token")
	count := 0
	for _, err := range client.ListProjectsStream(context.Background(), ListProjectsRequest{}) {
		if err != nil {
			t.Fatalf("ListProjectsStream returned error: %v", err)
		}
		count++
	}

	if count != 1 {
		t.Errorf("projects = %d, want 1", count)
	}
	if len(gotQueries) != 2 {
		t.Errorf("requests = %d, want 2", len(gotQueries))
	}
}

func TestListProjectsStream_YieldsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"items":[{"id":1,"title":"Inbox"}],"page":1,"total_pages":2}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token")
	var got []Project
	var gotErrs []error
	for p, err := range client.ListProjectsStream(context.Background(), ListProjectsRequest{}) {
		if err != nil {
			gotErrs = append(gotErrs, err)
			continue
		}
		got = append(got, p)
	}

	if want := []Project{{ID: 1, Title: "Inbox"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("projects = %+v, want %+v", got, want)
	}
	if len(gotErrs) != 1 {
		t.Fatalf("errors = %v, want exactly 1", gotErrs)
	}
	var apiErr *APIError
	if !errors.As(gotErrs[0], &apiErr) || apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("error = %v, want *ApiError with status 500", gotErrs[0])
	}
}
