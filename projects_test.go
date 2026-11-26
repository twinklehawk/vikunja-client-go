package vikunja

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
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

	client := newTestClient(server.URL)
	_, err := client.ListProjects(context.Background(), ProjectListQuery{})
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

	client := newTestClient(server.URL + "/")
	_, err := client.ListProjects(context.Background(), ProjectListQuery{})
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
		request ProjectListQuery
		want    url.Values
	}{
		{
			name:    "defaults sends nothing",
			request: ProjectListQuery{},
			want:    url.Values{},
		},
		{
			name: "all parameters set",
			request: ProjectListQuery{
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
			request: ProjectListQuery{Query: "inbox"},
			want: url.Values{
				"q": {"inbox"},
			},
		},
		{
			name:    "pagination only",
			request: ProjectListQuery{Page: 3, PerPage: 10},
			want: url.Values{
				"page":     {"3"},
				"per_page": {"10"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery url.Values
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotQuery = r.URL.Query()
					_, _ = w.Write([]byte(`{"items":[]}`))
				}),
			)
			defer server.Close()

			client := newTestClient(server.URL)
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

	client := newTestClient(server.URL)
	resp, err := client.ListProjects(context.Background(), ProjectListQuery{})
	if err != nil {
		t.Fatalf("ListProjects returned error: %v", err)
	}

	want := &PaginatedProjectsResponse{
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

	client := newTestClient(server.URL)
	resp, err := client.ListProjects(context.Background(), ProjectListQuery{})
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
	if apiErr.StatusText != "Unauthorized" {
		t.Errorf("StatusText = %q, want %q", apiErr.StatusText, "Unauthorized")
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

	client := newTestClient(server.URL)
	_, err := client.ListProjects(ctx, ProjectListQuery{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestFindProjectByTitle_SendsRequest(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{name: "simple title", title: "Work"},
		{name: "special characters", title: "a&b=c?d e"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotReq *http.Request
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotReq = r
					// Echo the searched title back so it is an exact match.
					body, _ := json.Marshal(map[string]any{
						"items":       []Project{{ID: 1, Title: r.URL.Query().Get("q")}},
						"total_pages": 1,
					})
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(body)
				}),
			)
			defer server.Close()

			client := newTestClient(server.URL)
			got, err := client.FindProjectByTitle(context.Background(), tt.title)
			if err != nil {
				t.Fatalf("FindProjectByTitle returned error: %v", err)
			}
			if got.Title != tt.title {
				t.Errorf("title = %q, want %q", got.Title, tt.title)
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
			wantQuery := url.Values{"q": {tt.title}, "page": {"1"}}
			if got := gotReq.URL.Query(); !reflect.DeepEqual(got, wantQuery) {
				t.Errorf("query = %v, want %v", got, wantQuery)
			}
		})
	}
}

func TestFindProjectByTitle_ReturnsExactMatch(t *testing.T) {
	tests := []struct {
		name         string
		pages        []string
		want         *Project
		wantRequests int
	}{
		{
			name:         "single exact match",
			pages:        []string{`{"items":[{"id":7,"title":"Work"}],"total_pages":1}`},
			want:         &Project{ID: 7, Title: "Work"},
			wantRequests: 1,
		},
		{
			name: "skips partial matches",
			pages: []string{
				`{"items":[{"id":1,"title":"Workshop"},{"id":2,"title":"Work"}],"total_pages":1}`,
			},
			want:         &Project{ID: 2, Title: "Work"},
			wantRequests: 1,
		},
		{
			name: "skips different case",
			pages: []string{
				`{"items":[{"id":1,"title":"work"},{"id":2,"title":"Work"}],"total_pages":1}`,
			},
			want:         &Project{ID: 2, Title: "Work"},
			wantRequests: 1,
		},
		{
			name: "first of duplicate titles",
			pages: []string{
				`{"items":[{"id":3,"title":"Work"},{"id":4,"title":"Work"}],"total_pages":1}`,
			},
			want:         &Project{ID: 3, Title: "Work"},
			wantRequests: 1,
		},
		{
			name: "match on a later page",
			pages: []string{
				`{"items":[{"id":1,"title":"Workshop"},{"id":2,"title":"Inbox"}],"total_pages":2}`,
				`{"items":[{"id":3,"title":"Work"}],"total_pages":2}`,
			},
			want:         &Project{ID: 3, Title: "Work"},
			wantRequests: 2,
		},
		{
			name: "stops fetching after a match",
			pages: []string{
				`{"items":[{"id":1,"title":"Work"}],"total_pages":3}`,
				`{"items":[{"id":2,"title":"Inbox"}],"total_pages":3}`,
				`{"items":[{"id":3,"title":"Workshop"}],"total_pages":3}`,
			},
			want:         &Project{ID: 1, Title: "Work"},
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQueries []url.Values
			server := pagedServer(t, tt.pages, &gotQueries)

			client := newTestClient(server.URL)
			got, err := client.FindProjectByTitle(context.Background(), "Work")
			if err != nil {
				t.Fatalf("FindProjectByTitle returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("project = %+v, want %+v", got, tt.want)
			}
			if len(gotQueries) != tt.wantRequests {
				t.Errorf("requests = %d, want %d", len(gotQueries), tt.wantRequests)
			}
		})
	}
}

func TestFindProjectByTitle_NoMatch(t *testing.T) {
	tests := []struct {
		name         string
		pages        []string
		wantRequests int
	}{
		{name: "no items", pages: []string{`{"items":[]}`}, wantRequests: 1},
		{name: "null items", pages: []string{`{"items":null}`}, wantRequests: 1},
		{name: "missing items", pages: []string{`{}`}, wantRequests: 1},
		{
			name: "only partial matches across all pages",
			pages: []string{
				`{"items":[{"id":1,"title":"Workshop"}],"total_pages":2}`,
				`{"items":[{"id":2,"title":"work"}],"total_pages":2}`,
			},
			wantRequests: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQueries []url.Values
			server := pagedServer(t, tt.pages, &gotQueries)

			client := newTestClient(server.URL)
			got, err := client.FindProjectByTitle(context.Background(), "Work")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got != nil {
				t.Errorf("project = %+v, want nil", got)
			}
			if want := "no project found matching title 'Work'"; err.Error() != want {
				t.Errorf("error = %q, want %q", err.Error(), want)
			}
			if len(gotQueries) != tt.wantRequests {
				t.Errorf("requests = %d, want %d", len(gotQueries), tt.wantRequests)
			}
		})
	}
}

func TestFindProjectByTitle_APIError(t *testing.T) {
	tests := []struct {
		name      string
		failPage  string
		firstPage string
	}{
		{name: "first page fails", failPage: "1"},
		{
			name:      "later page fails",
			failPage:  "2",
			firstPage: `{"items":[{"id":1,"title":"Workshop"}],"total_pages":2}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Query().Get("page") != tt.failPage {
						_, _ = w.Write([]byte(tt.firstPage))
						return
					}
					w.WriteHeader(http.StatusUnauthorized)
					_, _ = w.Write([]byte(`{"message":"invalid token"}`))
				}),
			)
			defer server.Close()

			client := newTestClient(server.URL)
			got, err := client.FindProjectByTitle(context.Background(), "Work")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got != nil {
				t.Errorf("project = %+v, want nil", got)
			}

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("error = %v (%T), want *APIError", err, err)
			}
			if apiErr.StatusCode != http.StatusUnauthorized {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
			}
		})
	}
}

func TestFindProjectByTitle_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	got, err := client.FindProjectByTitle(context.Background(), "Work")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("project = %+v, want nil", got)
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("error = %v (%T), want wrapped *json.SyntaxError", err, err)
	}
}

func TestFindProjectByTitle_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	got, err := client.FindProjectByTitle(context.Background(), "Work")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("project = %+v, want nil", got)
	}
}

func TestFindProjectByTitle_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":1,"title":"Work"}],"total_pages":1}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	got, err := client.FindProjectByTitle(ctx, "Work")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Errorf("project = %+v, want nil", got)
	}
}

// pagedServer serves pages of projects, recording the query of each request.
func pagedServer(t *testing.T, pages []string, gotQueries *[]url.Values) *httptest.Server {
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
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Inbox"},{"id":2,"title":"Work"}],"page":1,"total_pages":2}`,
		`{"items":[{"id":3,"title":"Home"}],"page":2,"total_pages":2}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	var got []Project
	for p, err := range client.ListProjectsStream(context.Background(), ProjectListQuery{Query: "x", PerPage: 2, Page: 5}) {
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
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Inbox"},{"id":2,"title":"Work"}],"page":1,"total_pages":2}`,
		`{"items":[{"id":3,"title":"Home"}],"page":2,"total_pages":2}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	for p, err := range client.ListProjectsStream(context.Background(), ProjectListQuery{}) {
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
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Inbox"}],"total_pages":5}`,
		`{"items":[],"total_pages":5}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	count := 0
	for _, err := range client.ListProjectsStream(context.Background(), ProjectListQuery{}) {
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

	client := newTestClient(server.URL)
	var got []Project
	var gotErrs []error
	for p, err := range client.ListProjectsStream(context.Background(), ProjectListQuery{}) {
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

func TestListProjectViewBuckets_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ListProjectViewBuckets(
		context.Background(),
		5,
		7,
		ProjectViewBucketListQuery{},
	)
	if err != nil {
		t.Fatalf("ListProjectViewBuckets returned error: %v", err)
	}

	if gotReq.Method != http.MethodGet {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodGet)
	}
	if gotReq.URL.Path != "/api/v2/projects/5/views/7/buckets" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/projects/5/views/7/buckets")
	}
	if got := gotReq.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if got := gotReq.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept header = %q, want %q", got, "application/json")
	}
}

func TestListProjectViewBuckets_QueryParameters(t *testing.T) {
	tests := []struct {
		name  string
		query ProjectViewBucketListQuery
		want  url.Values
	}{
		{
			name:  "defaults sends nothing",
			query: ProjectViewBucketListQuery{},
			want:  url.Values{},
		},
		{
			name:  "all parameters set",
			query: ProjectViewBucketListQuery{Query: "done", Page: 2, PerPage: 50},
			want: url.Values{
				"q":        {"done"},
				"page":     {"2"},
				"per_page": {"50"},
			},
		},
		{
			name:  "query only",
			query: ProjectViewBucketListQuery{Query: "done"},
			want:  url.Values{"q": {"done"}},
		},
		{
			name:  "pagination only",
			query: ProjectViewBucketListQuery{Page: 3, PerPage: 10},
			want: url.Values{
				"page":     {"3"},
				"per_page": {"10"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQuery url.Values
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotQuery = r.URL.Query()
					_, _ = w.Write([]byte(`{"items":[]}`))
				}),
			)
			defer server.Close()

			client := newTestClient(server.URL)
			_, err := client.ListProjectViewBuckets(context.Background(), 5, 7, tt.query)
			if err != nil {
				t.Fatalf("ListProjectViewBuckets returned error: %v", err)
			}

			if !reflect.DeepEqual(gotQuery, tt.want) {
				t.Errorf("query = %v, want %v", gotQuery, tt.want)
			}
		})
	}
}

func TestListProjectViewBuckets_DecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id": 1, "title": "To Do", "position": 100, "project_view_id": 7},
				{"id": 2, "title": "Done", "position": 200.5, "project_view_id": 7}
			],
			"page": 1,
			"per_page": 50,
			"total": 2,
			"total_pages": 1
		}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	got, err := client.ListProjectViewBuckets(
		context.Background(),
		5,
		7,
		ProjectViewBucketListQuery{},
	)
	if err != nil {
		t.Fatalf("ListProjectViewBuckets returned error: %v", err)
	}

	want := &PaginatedBucketsResponse{
		Items: []Bucket{
			{ID: 1, Title: "To Do", Position: 100, ProjectViewID: 7},
			{ID: 2, Title: "Done", Position: 200.5, ProjectViewID: 7},
		},
		Page:       1,
		PerPage:    50,
		Total:      2,
		TotalPages: 1,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("response = %+v, want %+v", got, want)
	}
}

func TestListProjectViewBuckets_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ListProjectViewBuckets(
		context.Background(),
		5,
		7,
		ProjectViewBucketListQuery{},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("response = %+v, want nil", resp)
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v (%T), want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
	if apiErr.Message != `{"message":"not found"}` {
		t.Errorf("Message = %q, want %q", apiErr.Message, `{"message":"not found"}`)
	}
}

func TestListProjectViewBuckets_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ListProjectViewBuckets(
		context.Background(),
		5,
		7,
		ProjectViewBucketListQuery{},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("response = %+v, want nil", resp)
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("error = %v (%T), want wrapped *json.SyntaxError", err, err)
	}
}

func TestListProjectViewBuckets_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	resp, err := client.ListProjectViewBuckets(
		context.Background(),
		5,
		7,
		ProjectViewBucketListQuery{},
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("response = %+v, want nil", resp)
	}
}

func TestListProjectViewBuckets_InvalidIDs(t *testing.T) {
	tests := []struct {
		name          string
		projectID     int64
		projectViewID int64
		wantInErr     string
	}{
		{name: "missing project", projectID: 0, projectViewID: 7, wantInErr: "projectID"},
		{name: "negative project", projectID: -1, projectViewID: 7, wantInErr: "projectID"},
		{name: "missing view", projectID: 5, projectViewID: 0, wantInErr: "projectViewID"},
		{name: "negative view", projectID: 5, projectViewID: -1, wantInErr: "projectViewID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests++
				}),
			)
			defer server.Close()

			client := newTestClient(server.URL)
			resp, err := client.ListProjectViewBuckets(
				context.Background(),
				tt.projectID,
				tt.projectViewID,
				ProjectViewBucketListQuery{},
			)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if resp != nil {
				t.Errorf("response = %+v, want nil", resp)
			}
			if !strings.Contains(err.Error(), tt.wantInErr) {
				t.Errorf("error = %q, want it to mention %s", err.Error(), tt.wantInErr)
			}
			if requests != 0 {
				t.Errorf("requests = %d, want 0", requests)
			}
		})
	}
}

func TestListProjectViewBuckets_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	_, err := client.ListProjectViewBuckets(ctx, 5, 7, ProjectViewBucketListQuery{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}
