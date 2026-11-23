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

func TestListProjectViews_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ListProjectViews(context.Background(), 5, ProjectViewListQuery{})
	if err != nil {
		t.Fatalf("ListProjectViews returned error: %v", err)
	}

	if gotReq.Method != http.MethodGet {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodGet)
	}
	if gotReq.URL.Path != "/api/v2/projects/5/views" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/projects/5/views")
	}
	if got := gotReq.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if got := gotReq.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept header = %q, want %q", got, "application/json")
	}
}

func TestListProjectViews_QueryParameters(t *testing.T) {
	tests := []struct {
		name  string
		query ProjectViewListQuery
		want  url.Values
	}{
		{
			name:  "defaults sends nothing",
			query: ProjectViewListQuery{},
			want:  url.Values{},
		},
		{
			name: "all parameters set",
			query: ProjectViewListQuery{
				Query:   "kanban board",
				Page:    2,
				PerPage: 50,
			},
			want: url.Values{
				"q":        {"kanban board"},
				"page":     {"2"},
				"per_page": {"50"},
			},
		},
		{
			name:  "query only",
			query: ProjectViewListQuery{Query: "list"},
			want: url.Values{
				"q": {"list"},
			},
		},
		{
			name:  "pagination only",
			query: ProjectViewListQuery{Page: 3, PerPage: 10},
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

			client := newTestClient(server.URL)
			_, err := client.ListProjectViews(context.Background(), 5, tt.query)
			if err != nil {
				t.Fatalf("ListProjectViews returned error: %v", err)
			}

			if !reflect.DeepEqual(gotQuery, tt.want) {
				t.Errorf("query = %v, want %v", gotQuery, tt.want)
			}
		})
	}
}

func TestListProjectViews_DecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id": 1, "project_id": 5, "position": 100, "title": "List", "view_kind": "list"},
				{"id": 2, "project_id": 5, "position": 200.5, "title": "Gantt", "view_kind": "gantt"},
				{"id": 3, "project_id": 5, "position": 300, "title": "Table", "view_kind": "table"},
				{"id": 4, "project_id": 5, "position": 400, "title": "Kanban", "view_kind": "kanban"}
			],
			"page": 1,
			"per_page": 4,
			"total": 5,
			"total_pages": 2
		}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	got, err := client.ListProjectViews(context.Background(), 5, ProjectViewListQuery{})
	if err != nil {
		t.Fatalf("ListProjectViews returned error: %v", err)
	}

	want := &PaginatedProjectViewsResponse{
		Items: []ProjectView{
			{ID: 1, ProjectID: 5, Position: 100, Title: "List", ViewKind: ProjectViewKindList},
			{ID: 2, ProjectID: 5, Position: 200.5, Title: "Gantt", ViewKind: ProjectViewKindGantt},
			{ID: 3, ProjectID: 5, Position: 300, Title: "Table", ViewKind: ProjectViewKindTable},
			{ID: 4, ProjectID: 5, Position: 400, Title: "Kanban", ViewKind: ProjectViewKindKanban},
		},
		Page:       1,
		PerPage:    4,
		Total:      5,
		TotalPages: 2,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("response = %+v, want %+v", got, want)
	}
}

func TestListProjectViews_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"forbidden"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ListProjectViews(context.Background(), 5, ProjectViewListQuery{})
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
	if apiErr.StatusCode != http.StatusForbidden {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusForbidden)
	}
	if apiErr.Message != `{"message":"forbidden"}` {
		t.Errorf("Message = %q, want %q", apiErr.Message, `{"message":"forbidden"}`)
	}
}

func TestListProjectViews_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ListProjectViews(context.Background(), 5, ProjectViewListQuery{})
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

func TestListProjectViews_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	resp, err := client.ListProjectViews(context.Background(), 5, ProjectViewListQuery{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("response = %+v, want nil", resp)
	}
}

func TestListProjectViews_InvalidProjectID(t *testing.T) {
	tests := []struct {
		name      string
		projectID int64
	}{
		{name: "missing", projectID: 0},
		{name: "negative", projectID: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
			}))
			defer server.Close()

			client := newTestClient(server.URL)
			resp, err := client.ListProjectViews(context.Background(), tt.projectID, ProjectViewListQuery{})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if resp != nil {
				t.Errorf("response = %+v, want nil", resp)
			}
			if !strings.Contains(err.Error(), "projectID") {
				t.Errorf("error = %q, want it to mention projectID", err.Error())
			}
			if requests != 0 {
				t.Errorf("requests = %d, want 0", requests)
			}
		})
	}
}

func TestListProjectViews_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	_, err := client.ListProjectViews(ctx, 5, ProjectViewListQuery{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}
