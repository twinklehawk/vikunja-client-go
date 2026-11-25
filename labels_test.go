package vikunja

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestListLabels_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ListLabels(context.Background(), LabelListQuery{})
	if err != nil {
		t.Fatalf("ListLabels returned error: %v", err)
	}

	if gotReq.Method != http.MethodGet {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodGet)
	}
	if gotReq.URL.Path != "/api/v2/labels" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/labels")
	}
	if got := gotReq.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if got := gotReq.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept header = %q, want %q", got, "application/json")
	}
	if gotReq.ContentLength != 0 {
		t.Errorf("ContentLength = %d, want 0", gotReq.ContentLength)
	}
}

func TestListLabels_QueryParameters(t *testing.T) {
	tests := []struct {
		name    string
		request LabelListQuery
		want    url.Values
	}{
		{
			name:    "defaults sends nothing",
			request: LabelListQuery{},
			want:    url.Values{},
		},
		{
			name:    "all parameters set",
			request: LabelListQuery{Query: "urgent work", Page: 2, PerPage: 50},
			want: url.Values{
				"q":        {"urgent work"},
				"page":     {"2"},
				"per_page": {"50"},
			},
		},
		{
			name:    "query only",
			request: LabelListQuery{Query: "bug"},
			want:    url.Values{"q": {"bug"}},
		},
		{
			name:    "pagination only",
			request: LabelListQuery{Page: 3, PerPage: 10},
			want: url.Values{
				"page":     {"3"},
				"per_page": {"10"},
			},
		},
		{
			name:    "special characters are encoded",
			request: LabelListQuery{Query: "a&b=c?d"},
			want:    url.Values{"q": {"a&b=c?d"}},
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
			_, err := client.ListLabels(context.Background(), tt.request)
			if err != nil {
				t.Fatalf("ListLabels returned error: %v", err)
			}

			if !reflect.DeepEqual(gotQuery, tt.want) {
				t.Errorf("query = %v, want %v", gotQuery, tt.want)
			}
		})
	}
}

func TestListLabels_DecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id": 1, "title": "Bug", "description": "Something is broken"},
				{"id": 7, "title": "Feature", "description": ""}
			],
			"page": 1,
			"per_page": 50,
			"total": 2,
			"total_pages": 1
		}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ListLabels(context.Background(), LabelListQuery{})
	if err != nil {
		t.Fatalf("ListLabels returned error: %v", err)
	}

	want := &PaginatedLabelResponse{
		Items: []Label{
			{ID: 1, Title: "Bug", Description: "Something is broken"},
			{ID: 7, Title: "Feature", Description: ""},
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

func TestListLabels_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"invalid token"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ListLabels(context.Background(), LabelListQuery{})
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
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
	if apiErr.Message != `{"message":"invalid token"}` {
		t.Errorf("Message = %q, want %q", apiErr.Message, `{"message":"invalid token"}`)
	}
}

func TestListLabels_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ListLabels(context.Background(), LabelListQuery{})
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

func TestListLabels_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	resp, err := client.ListLabels(context.Background(), LabelListQuery{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("response = %+v, want nil", resp)
	}
}

func TestListLabels_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	_, err := client.ListLabels(ctx, LabelListQuery{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestListLabelsStream_IteratesAllPages(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Bug"},{"id":2,"title":"Feature"}],"page":1,"total_pages":2}`,
		`{"items":[{"id":3,"title":"Chore","description":"Housekeeping"}],"page":2,"total_pages":2}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	var got []Label
	query := LabelListQuery{Query: "b", PerPage: 2, Page: 5}
	for label, err := range client.ListLabelsStream(context.Background(), query) {
		if err != nil {
			t.Fatalf("ListLabelsStream returned error: %v", err)
		}
		got = append(got, label)
	}

	want := []Label{
		{ID: 1, Title: "Bug"},
		{ID: 2, Title: "Feature"},
		{ID: 3, Title: "Chore", Description: "Housekeeping"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("labels = %+v, want %+v", got, want)
	}

	// query.Page is ignored: iteration starts at page 1, other parameters are passed through.
	wantQueries := []url.Values{
		{"q": {"b"}, "page": {"1"}, "per_page": {"2"}},
		{"q": {"b"}, "page": {"2"}, "per_page": {"2"}},
	}
	if !reflect.DeepEqual(gotQueries, wantQueries) {
		t.Errorf("queries = %v, want %v", gotQueries, wantQueries)
	}
}

func TestListLabelsStream_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		_, _ = w.Write([]byte(`{"items":[],"total_pages":0}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	for _, err := range client.ListLabelsStream(context.Background(), LabelListQuery{}) {
		if err != nil {
			t.Fatalf("ListLabelsStream returned error: %v", err)
		}
	}

	if gotReq == nil {
		t.Fatal("no request was sent")
	}
	if gotReq.Method != http.MethodGet {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodGet)
	}
	if gotReq.URL.Path != "/api/v2/labels" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/labels")
	}
	if got := gotReq.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if want := (url.Values{"page": {"1"}}); !reflect.DeepEqual(gotReq.URL.Query(), want) {
		t.Errorf("query = %v, want %v", gotReq.URL.Query(), want)
	}
}

func TestListLabelsStream_IsLazy(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		_, _ = w.Write([]byte(`{"items":[],"total_pages":0}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_ = client.ListLabelsStream(context.Background(), LabelListQuery{})

	if requests != 0 {
		t.Errorf("requests = %d, want 0 before iterating", requests)
	}
}

func TestListLabelsStream_StopsOnLastPage(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Bug"}],"page":1,"total_pages":1}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	count := 0
	for _, err := range client.ListLabelsStream(context.Background(), LabelListQuery{}) {
		if err != nil {
			t.Fatalf("ListLabelsStream returned error: %v", err)
		}
		count++
	}

	if count != 1 {
		t.Errorf("labels = %d, want 1", count)
	}
	if len(gotQueries) != 1 {
		t.Errorf("requests = %d, want 1", len(gotQueries))
	}
}

func TestListLabelsStream_StopsOnBreak(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Bug"},{"id":2,"title":"Feature"}],"page":1,"total_pages":2}`,
		`{"items":[{"id":3,"title":"Chore"}],"page":2,"total_pages":2}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	var got []int64
	for label, err := range client.ListLabelsStream(context.Background(), LabelListQuery{}) {
		if err != nil {
			t.Fatalf("ListLabelsStream returned error: %v", err)
		}
		got = append(got, label.ID)
		if label.ID == 1 {
			break
		}
	}

	if want := []int64{1}; !reflect.DeepEqual(got, want) {
		t.Errorf("label IDs = %v, want %v", got, want)
	}
	if len(gotQueries) != 1 {
		t.Errorf("requests = %d, want 1", len(gotQueries))
	}
}

func TestListLabelsStream_StopsOnEmptyPage(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Bug"}],"total_pages":5}`,
		`{"items":[],"total_pages":5}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	count := 0
	for _, err := range client.ListLabelsStream(context.Background(), LabelListQuery{}) {
		if err != nil {
			t.Fatalf("ListLabelsStream returned error: %v", err)
		}
		count++
	}

	if count != 1 {
		t.Errorf("labels = %d, want 1", count)
	}
	if len(gotQueries) != 2 {
		t.Errorf("requests = %d, want 2", len(gotQueries))
	}
}

func TestListLabelsStream_YieldsError(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"items":[{"id":1,"title":"Bug"}],"page":1,"total_pages":3}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	var got []Label
	var gotErrs []error
	for label, err := range client.ListLabelsStream(context.Background(), LabelListQuery{}) {
		if err != nil {
			if !reflect.DeepEqual(label, Label{}) {
				t.Errorf("label yielded with error = %+v, want zero Label", label)
			}
			gotErrs = append(gotErrs, err)
			continue
		}
		got = append(got, label)
	}

	if want := []Label{{ID: 1, Title: "Bug"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("labels = %+v, want %+v", got, want)
	}
	if len(gotErrs) != 1 {
		t.Fatalf("errors = %v, want exactly 1", gotErrs)
	}
	var apiErr *APIError
	if !errors.As(gotErrs[0], &apiErr) || apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("error = %v, want *APIError with status 500", gotErrs[0])
	}
	// Iteration stops after the error; page 3 is never requested.
	if requests != 2 {
		t.Errorf("requests = %d, want 2", requests)
	}
}

func TestListLabelsStream_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	var gotErrs []error
	for label, err := range client.ListLabelsStream(context.Background(), LabelListQuery{}) {
		if err == nil {
			t.Errorf("unexpected label %+v without error", label)
			continue
		}
		gotErrs = append(gotErrs, err)
	}

	if len(gotErrs) != 1 {
		t.Errorf("errors = %v, want exactly 1", gotErrs)
	}
}

func TestListLabelsStream_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":1,"title":"Bug"}],"total_pages":1}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	var gotErrs []error
	for label, err := range client.ListLabelsStream(ctx, LabelListQuery{}) {
		if err == nil {
			t.Errorf("unexpected label %+v without error", label)
			continue
		}
		gotErrs = append(gotErrs, err)
	}

	if len(gotErrs) != 1 {
		t.Fatalf("errors = %v, want exactly 1", gotErrs)
	}
	if !errors.Is(gotErrs[0], context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", gotErrs[0])
	}
}

func TestFindLabelByTitle_SendsRequest(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{name: "simple title", title: "Bug"},
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
						"items":       []Label{{ID: 1, Title: r.URL.Query().Get("q")}},
						"total_pages": 1,
					})
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write(body)
				}),
			)
			defer server.Close()

			client := newTestClient(server.URL)
			got, err := client.FindLabelByTitle(context.Background(), tt.title)
			if err != nil {
				t.Fatalf("FindLabelByTitle returned error: %v", err)
			}
			if got.Title != tt.title {
				t.Errorf("title = %q, want %q", got.Title, tt.title)
			}

			if gotReq.Method != http.MethodGet {
				t.Errorf("method = %q, want %q", gotReq.Method, http.MethodGet)
			}
			if gotReq.URL.Path != "/api/v2/labels" {
				t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/labels")
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

func TestFindLabelByTitle_ReturnsExactMatch(t *testing.T) {
	tests := []struct {
		name         string
		pages        []string
		want         *Label
		wantRequests int
	}{
		{
			name:         "single exact match",
			pages:        []string{`{"items":[{"id":7,"title":"Bug"}],"total_pages":1}`},
			want:         &Label{ID: 7, Title: "Bug"},
			wantRequests: 1,
		},
		{
			name: "skips partial matches",
			pages: []string{
				`{"items":[{"id":1,"title":"Bugfix"},{"id":2,"title":"Bug"}],"total_pages":1}`,
			},
			want:         &Label{ID: 2, Title: "Bug"},
			wantRequests: 1,
		},
		{
			name: "skips different case",
			pages: []string{
				`{"items":[{"id":1,"title":"bug"},{"id":2,"title":"Bug"}],"total_pages":1}`,
			},
			want:         &Label{ID: 2, Title: "Bug"},
			wantRequests: 1,
		},
		{
			name: "first of duplicate titles",
			pages: []string{
				`{"items":[{"id":3,"title":"Bug"},{"id":4,"title":"Bug"}],"total_pages":1}`,
			},
			want:         &Label{ID: 3, Title: "Bug"},
			wantRequests: 1,
		},
		{
			name: "match on a later page",
			pages: []string{
				`{"items":[{"id":1,"title":"Bugfix"},{"id":2,"title":"Feature"}],"total_pages":2}`,
				`{"items":[{"id":3,"title":"Bug"}],"total_pages":2}`,
			},
			want:         &Label{ID: 3, Title: "Bug"},
			wantRequests: 2,
		},
		{
			name: "stops fetching after a match",
			pages: []string{
				`{"items":[{"id":1,"title":"Bug"}],"total_pages":3}`,
				`{"items":[{"id":2,"title":"Feature"}],"total_pages":3}`,
				`{"items":[{"id":3,"title":"Bugfix"}],"total_pages":3}`,
			},
			want:         &Label{ID: 1, Title: "Bug"},
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQueries []url.Values
			server := pagedServer(t, tt.pages, &gotQueries)

			client := newTestClient(server.URL)
			got, err := client.FindLabelByTitle(context.Background(), "Bug")
			if err != nil {
				t.Fatalf("FindLabelByTitle returned error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("label = %+v, want %+v", got, tt.want)
			}
			if len(gotQueries) != tt.wantRequests {
				t.Errorf("requests = %d, want %d", len(gotQueries), tt.wantRequests)
			}
		})
	}
}

func TestFindLabelByTitle_NoMatch(t *testing.T) {
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
				`{"items":[{"id":1,"title":"Bugfix"}],"total_pages":2}`,
				`{"items":[{"id":2,"title":"bug"}],"total_pages":2}`,
			},
			wantRequests: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotQueries []url.Values
			server := pagedServer(t, tt.pages, &gotQueries)

			client := newTestClient(server.URL)
			got, err := client.FindLabelByTitle(context.Background(), "Bug")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got != nil {
				t.Errorf("label = %+v, want nil", got)
			}
			if want := "no label found matching title 'Bug'"; err.Error() != want {
				t.Errorf("error = %q, want %q", err.Error(), want)
			}
			if len(gotQueries) != tt.wantRequests {
				t.Errorf("requests = %d, want %d", len(gotQueries), tt.wantRequests)
			}
		})
	}
}

func TestFindLabelByTitle_APIError(t *testing.T) {
	tests := []struct {
		name      string
		failPage  string
		firstPage string
	}{
		{name: "first page fails", failPage: "1"},
		{
			name:      "later page fails",
			failPage:  "2",
			firstPage: `{"items":[{"id":1,"title":"Bugfix"}],"total_pages":2}`,
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
			got, err := client.FindLabelByTitle(context.Background(), "Bug")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if got != nil {
				t.Errorf("label = %+v, want nil", got)
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

func TestFindLabelByTitle_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	got, err := client.FindLabelByTitle(context.Background(), "Bug")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("label = %+v, want nil", got)
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("error = %v (%T), want wrapped *json.SyntaxError", err, err)
	}
}

func TestFindLabelByTitle_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	got, err := client.FindLabelByTitle(context.Background(), "Bug")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if got != nil {
		t.Errorf("label = %+v, want nil", got)
	}
}

func TestFindLabelByTitle_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"id":1,"title":"Bug"}],"total_pages":1}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	got, err := client.FindLabelByTitle(ctx, "Bug")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if got != nil {
		t.Errorf("label = %+v, want nil", got)
	}
}

func TestAddTaskLabel_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"label_id":3}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	err := client.AddTaskLabel(context.Background(), 42, AddTaskLabelRequest{LabelID: 3})
	if err != nil {
		t.Fatalf("AddTaskLabel returned error: %v", err)
	}

	if gotReq.Method != http.MethodPost {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodPost)
	}
	if gotReq.URL.Path != "/api/v2/tasks/42/labels" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/tasks/42/labels")
	}
	if got := gotReq.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if got := gotReq.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept header = %q, want %q", got, "application/json")
	}
	if got := gotReq.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type header = %q, want %q", got, "application/json")
	}

	// JSON numbers decode to float64 in a map[string]any.
	wantBody := map[string]any{"label_id": float64(3)}
	if len(gotBody) != len(wantBody) {
		t.Errorf("body = %v, want %v", gotBody, wantBody)
	}
	for key, want := range wantBody {
		if got := gotBody[key]; got != want {
			t.Errorf("body[%q] = %v, want %v", key, got, want)
		}
	}
}

func TestAddTaskLabel_IgnoresResponseBody(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "empty", body: ""},
		{name: "json", body: `{"label_id":3,"created":"2026-01-01T00:00:00Z"}`},
		{name: "not json", body: "not json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusCreated)
					_, _ = w.Write([]byte(tt.body))
				}),
			)
			defer server.Close()

			client := newTestClient(server.URL)
			err := client.AddTaskLabel(context.Background(), 42, AddTaskLabelRequest{LabelID: 3})
			if err != nil {
				t.Errorf("AddTaskLabel returned error: %v", err)
			}
		})
	}
}

func TestAddTaskLabel_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"label already exists on task"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	err := client.AddTaskLabel(context.Background(), 42, AddTaskLabelRequest{LabelID: 3})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v (%T), want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusConflict {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusConflict)
	}
	if apiErr.Message != `{"message":"label already exists on task"}` {
		t.Errorf(
			"Message = %q, want %q",
			apiErr.Message,
			`{"message":"label already exists on task"}`,
		)
	}
}

func TestAddTaskLabel_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	err := client.AddTaskLabel(context.Background(), 42, AddTaskLabelRequest{LabelID: 3})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAddTaskLabel_InvalidIDs(t *testing.T) {
	tests := []struct {
		name      string
		taskID    int64
		labelID   int64
		wantInErr string
	}{
		{name: "missing task ID", taskID: 0, labelID: 3, wantInErr: "taskID"},
		{name: "negative task ID", taskID: -1, labelID: 3, wantInErr: "taskID"},
		{name: "missing label ID", taskID: 42, labelID: 0, wantInErr: "LabelID"},
		{name: "negative label ID", taskID: 42, labelID: -1, wantInErr: "LabelID"},
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
			err := client.AddTaskLabel(
				context.Background(),
				tt.taskID,
				AddTaskLabelRequest{LabelID: tt.labelID},
			)
			if err == nil {
				t.Fatal("expected error, got nil")
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

func TestAddTaskLabel_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	err := client.AddTaskLabel(ctx, 42, AddTaskLabelRequest{LabelID: 3})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestRemoveTaskLabel_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	err := client.RemoveTaskLabel(context.Background(), 42, 3)
	if err != nil {
		t.Fatalf("RemoveTaskLabel returned error: %v", err)
	}

	if gotReq.Method != http.MethodDelete {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodDelete)
	}
	if gotReq.URL.Path != "/api/v2/tasks/42/labels/3" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/tasks/42/labels/3")
	}
	if gotReq.URL.RawQuery != "" {
		t.Errorf("query = %q, want empty", gotReq.URL.RawQuery)
	}
	if got := gotReq.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if got := gotReq.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept header = %q, want %q", got, "application/json")
	}
	if gotReq.ContentLength != 0 {
		t.Errorf("ContentLength = %d, want 0", gotReq.ContentLength)
	}
}

func TestRemoveTaskLabel_IgnoresResponseBody(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
	}{
		{name: "204 empty", status: http.StatusNoContent, body: ""},
		{name: "200 json", status: http.StatusOK, body: `{"message":"Successfully deleted."}`},
		{name: "200 not json", status: http.StatusOK, body: "not json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.status)
					_, _ = w.Write([]byte(tt.body))
				}),
			)
			defer server.Close()

			client := newTestClient(server.URL)
			err := client.RemoveTaskLabel(context.Background(), 42, 3)
			if err != nil {
				t.Errorf("RemoveTaskLabel returned error: %v", err)
			}
		})
	}
}

func TestRemoveTaskLabel_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"label does not exist on task"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	err := client.RemoveTaskLabel(context.Background(), 42, 3)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v (%T), want *APIError", err, err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusNotFound)
	}
	if apiErr.Message != `{"message":"label does not exist on task"}` {
		t.Errorf(
			"Message = %q, want %q",
			apiErr.Message,
			`{"message":"label does not exist on task"}`,
		)
	}
}

func TestRemoveTaskLabel_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	err := client.RemoveTaskLabel(context.Background(), 42, 3)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestRemoveTaskLabel_InvalidIDs(t *testing.T) {
	tests := []struct {
		name      string
		taskID    int64
		labelID   int64
		wantInErr string
	}{
		{name: "missing task ID", taskID: 0, labelID: 3, wantInErr: "taskID"},
		{name: "negative task ID", taskID: -1, labelID: 3, wantInErr: "taskID"},
		{name: "missing label ID", taskID: 42, labelID: 0, wantInErr: "labelID"},
		{name: "negative label ID", taskID: 42, labelID: -1, wantInErr: "labelID"},
		{name: "both invalid reports task ID first", taskID: 0, labelID: 0, wantInErr: "taskID"},
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
			err := client.RemoveTaskLabel(context.Background(), tt.taskID, tt.labelID)
			if err == nil {
				t.Fatal("expected error, got nil")
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

func TestRemoveTaskLabel_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	err := client.RemoveTaskLabel(ctx, 42, 3)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}
