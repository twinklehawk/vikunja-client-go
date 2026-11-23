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

func TestCreateTask_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":1,"title":"Buy milk","project_id":5}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.CreateTask(context.Background(), Task{
		Title:       "Buy milk",
		ProjectID:   5,
		Description: "2 litres",
	})
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}

	if gotReq.Method != http.MethodPost {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodPost)
	}
	if gotReq.URL.Path != "/api/v2/projects/5/tasks" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/projects/5/tasks")
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
	wantFields := map[string]any{
		"title":       "Buy milk",
		"project_id":  float64(5),
		"description": "2 litres",
	}
	for key, want := range wantFields {
		if got := gotBody[key]; got != want {
			t.Errorf("body[%q] = %v, want %v", key, got, want)
		}
	}
}

func TestCreateTask_DecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"id": 42,
			"title": "Buy milk",
			"project_id": 5,
			"description": "2 litres"
		}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	got, err := client.CreateTask(context.Background(), Task{Title: "Buy milk", ProjectID: 5})
	if err != nil {
		t.Fatalf("CreateTask returned error: %v", err)
	}

	want := &Task{
		ID:          42,
		Title:       "Buy milk",
		ProjectID:   5,
		Description: "2 litres",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("task = %+v, want %+v", got, want)
	}
}

func TestCreateTask_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"forbidden"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	task, err := client.CreateTask(context.Background(), Task{Title: "Buy milk", ProjectID: 5})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if task != nil {
		t.Errorf("task = %+v, want nil", task)
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

func TestCreateTask_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	task, err := client.CreateTask(context.Background(), Task{Title: "Buy milk", ProjectID: 5})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if task != nil {
		t.Errorf("task = %+v, want nil", task)
	}
	var syntaxErr *json.SyntaxError
	if !errors.As(err, &syntaxErr) {
		t.Errorf("error = %v (%T), want wrapped *json.SyntaxError", err, err)
	}
}

func TestCreateTask_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	task, err := client.CreateTask(context.Background(), Task{Title: "Buy milk", ProjectID: 5})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if task != nil {
		t.Errorf("task = %+v, want nil", task)
	}
}

func TestCreateTask_InvalidProjectID(t *testing.T) {
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
			task, err := client.CreateTask(context.Background(), Task{Title: "Buy milk", ProjectID: tt.projectID})
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if task != nil {
				t.Errorf("task = %+v, want nil", task)
			}
			if !strings.Contains(err.Error(), "task.ProjectID") {
				t.Errorf("error = %q, want it to mention task.ProjectID", err.Error())
			}
			if requests != 0 {
				t.Errorf("requests = %d, want 0", requests)
			}
		})
	}
}

func TestCreateTask_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	_, err := client.CreateTask(ctx, Task{Title: "Buy milk", ProjectID: 5})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestListTasksInProject_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ListTasksInProject(context.Background(), ListTasksInProjectRequest{ProjectID: 5})
	if err != nil {
		t.Fatalf("ListTasksInProject returned error: %v", err)
	}

	if gotReq.Method != http.MethodGet {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodGet)
	}
	if gotReq.URL.Path != "/api/v2/projects/5/tasks" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/projects/5/tasks")
	}
	if got := gotReq.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if got := gotReq.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept header = %q, want %q", got, "application/json")
	}
}

func TestListTasksInProject_QueryParameters(t *testing.T) {
	tests := []struct {
		name    string
		request ListTasksInProjectRequest
		want    url.Values
	}{
		{
			name:    "defaults sends nothing",
			request: ListTasksInProjectRequest{ProjectID: 5},
			want:    url.Values{},
		},
		{
			name: "all parameters set",
			request: ListTasksInProjectRequest{
				ProjectID:          5,
				Query:              "milk",
				Filter:             "done = false && due_date < now+7d",
				FilterTimeZone:     "Europe/Berlin",
				FilterIncludeNulls: true,
				Format:             "markdown",
				Page:               2,
				PerPage:            50,
			},
			want: url.Values{
				"q":                    {"milk"},
				"filter":               {"done = false && due_date < now+7d"},
				"filter_timezone":      {"Europe/Berlin"},
				"filter_include_nulls": {"true"},
				"format":               {"markdown"},
				"page":                 {"2"},
				"per_page":             {"50"},
			},
		},
		{
			name:    "query only",
			request: ListTasksInProjectRequest{ProjectID: 5, Query: "milk"},
			want: url.Values{
				"q": {"milk"},
			},
		},
		{
			name: "filter only",
			request: ListTasksInProjectRequest{
				ProjectID: 5,
				Filter:    "priority >= 3 || labels in 1, 2",
			},
			want: url.Values{
				"filter": {"priority >= 3 || labels in 1, 2"},
			},
		},
		{
			name:    "pagination only",
			request: ListTasksInProjectRequest{ProjectID: 5, Page: 3, PerPage: 10},
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
			_, err := client.ListTasksInProject(context.Background(), tt.request)
			if err != nil {
				t.Fatalf("ListTasksInProject returned error: %v", err)
			}

			if !reflect.DeepEqual(gotQuery, tt.want) {
				t.Errorf("query = %v, want %v", gotQuery, tt.want)
			}
		})
	}
}

func TestListTasksInProject_DecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id": 1, "title": "Buy milk", "project_id": 5, "description": "2 litres"},
				{"id": 2, "title": "Buy eggs", "project_id": 5, "description": ""}
			],
			"page": 1,
			"per_page": 2,
			"total": 3,
			"total_pages": 2
		}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	got, err := client.ListTasksInProject(context.Background(), ListTasksInProjectRequest{ProjectID: 5})
	if err != nil {
		t.Fatalf("ListTasksInProject returned error: %v", err)
	}

	want := &PaginatedTasksResponse{
		Items: []Task{
			{ID: 1, Title: "Buy milk", ProjectID: 5, Description: "2 litres"},
			{ID: 2, Title: "Buy eggs", ProjectID: 5},
		},
		Page:       1,
		PerPage:    2,
		Total:      3,
		TotalPages: 2,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("response = %+v, want %+v", got, want)
	}
}

func TestListTasksInProject_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ListTasksInProject(context.Background(), ListTasksInProjectRequest{ProjectID: 5})
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

func TestListTasksInProject_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ListTasksInProject(context.Background(), ListTasksInProjectRequest{ProjectID: 5})
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

func TestListTasksInProject_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	resp, err := client.ListTasksInProject(context.Background(), ListTasksInProjectRequest{ProjectID: 5})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("response = %+v, want nil", resp)
	}
}

func TestListTasksInProject_InvalidProjectID(t *testing.T) {
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
			resp, err := client.ListTasksInProject(context.Background(), ListTasksInProjectRequest{ProjectID: tt.projectID})
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

func TestListTasksInProject_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	_, err := client.ListTasksInProject(ctx, ListTasksInProjectRequest{ProjectID: 5})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestListTasksInProjectView_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.ListTasksInProjectView(context.Background(), ListTasksInProjectViewRequest{
		ProjectID: 5,
		ViewID:    7,
	})
	if err != nil {
		t.Fatalf("ListTasksInProjectView returned error: %v", err)
	}

	if gotReq.Method != http.MethodGet {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodGet)
	}
	if gotReq.URL.Path != "/api/v2/projects/5/views/7/tasks" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/projects/5/views/7/tasks")
	}
	if got := gotReq.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if got := gotReq.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept header = %q, want %q", got, "application/json")
	}
}

func TestListTasksInProjectView_QueryParameters(t *testing.T) {
	tests := []struct {
		name    string
		request ListTasksInProjectViewRequest
		want    url.Values
	}{
		{
			name:    "defaults sends nothing",
			request: ListTasksInProjectViewRequest{ProjectID: 5, ViewID: 7},
			want:    url.Values{},
		},
		{
			name: "all parameters set",
			request: ListTasksInProjectViewRequest{
				ProjectID:          5,
				ViewID:             7,
				Query:              "milk",
				Filter:             "done = false && due_date < now+7d",
				FilterTimeZone:     "Europe/Berlin",
				FilterIncludeNulls: true,
				Format:             "markdown",
				Page:               2,
				PerPage:            50,
			},
			want: url.Values{
				"q":                    {"milk"},
				"filter":               {"done = false && due_date < now+7d"},
				"filter_timezone":      {"Europe/Berlin"},
				"filter_include_nulls": {"true"},
				"format":               {"markdown"},
				"page":                 {"2"},
				"per_page":             {"50"},
			},
		},
		{
			name:    "query only",
			request: ListTasksInProjectViewRequest{ProjectID: 5, ViewID: 7, Query: "milk"},
			want: url.Values{
				"q": {"milk"},
			},
		},
		{
			name: "filter only",
			request: ListTasksInProjectViewRequest{
				ProjectID: 5,
				ViewID:    7,
				Filter:    "priority >= 3 || labels in 1, 2",
			},
			want: url.Values{
				"filter": {"priority >= 3 || labels in 1, 2"},
			},
		},
		{
			name:    "pagination only",
			request: ListTasksInProjectViewRequest{ProjectID: 5, ViewID: 7, Page: 3, PerPage: 10},
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
			_, err := client.ListTasksInProjectView(context.Background(), tt.request)
			if err != nil {
				t.Fatalf("ListTasksInProjectView returned error: %v", err)
			}

			if !reflect.DeepEqual(gotQuery, tt.want) {
				t.Errorf("query = %v, want %v", gotQuery, tt.want)
			}
		})
	}
}

func TestListTasksInProjectView_DecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id": 1, "title": "Buy milk", "project_id": 5, "description": "2 litres", "bucket_id": 3},
				{"id": 2, "title": "Buy eggs", "project_id": 5, "description": "", "bucket_id": 4}
			],
			"page": 1,
			"per_page": 2,
			"total": 3,
			"total_pages": 2
		}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	got, err := client.ListTasksInProjectView(context.Background(), ListTasksInProjectViewRequest{
		ProjectID: 5,
		ViewID:    7,
	})
	if err != nil {
		t.Fatalf("ListTasksInProjectView returned error: %v", err)
	}

	want := &PaginatedTasksResponse{
		Items: []Task{
			{ID: 1, Title: "Buy milk", ProjectID: 5, Description: "2 litres", BucketID: 3},
			{ID: 2, Title: "Buy eggs", ProjectID: 5, BucketID: 4},
		},
		Page:       1,
		PerPage:    2,
		Total:      3,
		TotalPages: 2,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("response = %+v, want %+v", got, want)
	}
}

func TestListTasksInProjectView_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ListTasksInProjectView(context.Background(), ListTasksInProjectViewRequest{
		ProjectID: 5,
		ViewID:    7,
	})
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

func TestListTasksInProjectView_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	resp, err := client.ListTasksInProjectView(context.Background(), ListTasksInProjectViewRequest{
		ProjectID: 5,
		ViewID:    7,
	})
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

func TestListTasksInProjectView_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	resp, err := client.ListTasksInProjectView(context.Background(), ListTasksInProjectViewRequest{
		ProjectID: 5,
		ViewID:    7,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if resp != nil {
		t.Errorf("response = %+v, want nil", resp)
	}
}

func TestListTasksInProjectView_InvalidIDs(t *testing.T) {
	tests := []struct {
		name      string
		projectID int64
		viewID    int64
		wantInErr string
	}{
		{name: "missing project ID", projectID: 0, viewID: 7, wantInErr: "projectID"},
		{name: "negative project ID", projectID: -1, viewID: 7, wantInErr: "projectID"},
		{name: "missing view ID", projectID: 5, viewID: 0, wantInErr: "viewID"},
		{name: "negative view ID", projectID: 5, viewID: -1, wantInErr: "viewID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
			}))
			defer server.Close()

			client := newTestClient(server.URL)
			resp, err := client.ListTasksInProjectView(context.Background(), ListTasksInProjectViewRequest{
				ProjectID: tt.projectID,
				ViewID:    tt.viewID,
			})
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

func TestListTasksInProjectView_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	_, err := client.ListTasksInProjectView(ctx, ListTasksInProjectViewRequest{ProjectID: 5, ViewID: 7})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}
