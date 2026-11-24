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
	_, err := client.ListTasksInProject(context.Background(), 5, TaskListQuery{})
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
		name  string
		query TaskListQuery
		want  url.Values
	}{
		{
			name:  "defaults sends nothing",
			query: TaskListQuery{},
			want:  url.Values{},
		},
		{
			name: "all parameters set",
			query: TaskListQuery{
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
			name:  "query only",
			query: TaskListQuery{Query: "milk"},
			want: url.Values{
				"q": {"milk"},
			},
		},
		{
			name: "filter only",
			query: TaskListQuery{
				Filter: "priority >= 3 || labels in 1, 2",
			},
			want: url.Values{
				"filter": {"priority >= 3 || labels in 1, 2"},
			},
		},
		{
			name:  "pagination only",
			query: TaskListQuery{Page: 3, PerPage: 10},
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
			_, err := client.ListTasksInProject(context.Background(), 5, tt.query)
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
	got, err := client.ListTasksInProject(context.Background(), 5, TaskListQuery{})
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
	resp, err := client.ListTasksInProject(context.Background(), 5, TaskListQuery{})
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
	resp, err := client.ListTasksInProject(context.Background(), 5, TaskListQuery{})
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
	resp, err := client.ListTasksInProject(context.Background(), 5, TaskListQuery{})
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
			resp, err := client.ListTasksInProject(context.Background(), tt.projectID, TaskListQuery{})
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
	_, err := client.ListTasksInProject(ctx, 5, TaskListQuery{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestListTasksInProjectStream_IteratesAllPages(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Buy milk","project_id":5},{"id":2,"title":"Buy eggs","project_id":5}],"page":1,"total_pages":2}`,
		`{"items":[{"id":3,"title":"Buy bread","project_id":5}],"page":2,"total_pages":2}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	var got []Task
	query := TaskListQuery{Filter: "done = false", PerPage: 2, Page: 5}
	for task, err := range client.ListTasksInProjectStream(context.Background(), 5, query) {
		if err != nil {
			t.Fatalf("ListTasksInProjectStream returned error: %v", err)
		}
		got = append(got, task)
	}

	want := []Task{
		{ID: 1, Title: "Buy milk", ProjectID: 5},
		{ID: 2, Title: "Buy eggs", ProjectID: 5},
		{ID: 3, Title: "Buy bread", ProjectID: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %+v, want %+v", got, want)
	}

	// query.Page is ignored: iteration starts at page 1, other parameters are passed through.
	wantQueries := []url.Values{
		{"filter": {"done = false"}, "page": {"1"}, "per_page": {"2"}},
		{"filter": {"done = false"}, "page": {"2"}, "per_page": {"2"}},
	}
	if !reflect.DeepEqual(gotQueries, wantQueries) {
		t.Errorf("queries = %v, want %v", gotQueries, wantQueries)
	}
}

func TestListTasksInProjectStream_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		_, _ = w.Write([]byte(`{"items":[],"total_pages":0}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	for _, err := range client.ListTasksInProjectStream(context.Background(), 5, TaskListQuery{}) {
		if err != nil {
			t.Fatalf("ListTasksInProjectStream returned error: %v", err)
		}
	}

	if gotReq == nil {
		t.Fatal("no request was sent")
	}
	if gotReq.Method != http.MethodGet {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodGet)
	}
	if gotReq.URL.Path != "/api/v2/projects/5/tasks" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/projects/5/tasks")
	}
}

func TestListTasksInProjectStream_StopsOnLastPage(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Buy milk"}],"page":1,"total_pages":1}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	count := 0
	for _, err := range client.ListTasksInProjectStream(context.Background(), 5, TaskListQuery{}) {
		if err != nil {
			t.Fatalf("ListTasksInProjectStream returned error: %v", err)
		}
		count++
	}

	if count != 1 {
		t.Errorf("tasks = %d, want 1", count)
	}
	if len(gotQueries) != 1 {
		t.Errorf("requests = %d, want 1", len(gotQueries))
	}
}

func TestListTasksInProjectStream_StopsOnBreak(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Buy milk"},{"id":2,"title":"Buy eggs"}],"page":1,"total_pages":2}`,
		`{"items":[{"id":3,"title":"Buy bread"}],"page":2,"total_pages":2}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	var got []int64
	for task, err := range client.ListTasksInProjectStream(context.Background(), 5, TaskListQuery{}) {
		if err != nil {
			t.Fatalf("ListTasksInProjectStream returned error: %v", err)
		}
		got = append(got, task.ID)
		if task.ID == 1 {
			break
		}
	}

	if want := []int64{1}; !reflect.DeepEqual(got, want) {
		t.Errorf("task IDs = %v, want %v", got, want)
	}
	if len(gotQueries) != 1 {
		t.Errorf("requests = %d, want 1", len(gotQueries))
	}
}

func TestListTasksInProjectStream_StopsOnEmptyPage(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Buy milk"}],"total_pages":5}`,
		`{"items":[],"total_pages":5}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	count := 0
	for _, err := range client.ListTasksInProjectStream(context.Background(), 5, TaskListQuery{}) {
		if err != nil {
			t.Fatalf("ListTasksInProjectStream returned error: %v", err)
		}
		count++
	}

	if count != 1 {
		t.Errorf("tasks = %d, want 1", count)
	}
	if len(gotQueries) != 2 {
		t.Errorf("requests = %d, want 2", len(gotQueries))
	}
}

func TestListTasksInProjectStream_YieldsError(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"items":[{"id":1,"title":"Buy milk"}],"page":1,"total_pages":3}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	var got []Task
	var gotErrs []error
	for task, err := range client.ListTasksInProjectStream(context.Background(), 5, TaskListQuery{}) {
		if err != nil {
			if !reflect.DeepEqual(task, Task{}) {
				t.Errorf("task yielded with error = %+v, want zero Task", task)
			}
			gotErrs = append(gotErrs, err)
			continue
		}
		got = append(got, task)
	}

	if want := []Task{{ID: 1, Title: "Buy milk"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %+v, want %+v", got, want)
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

func TestListTasksInProjectStream_InvalidProjectID(t *testing.T) {
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
			var gotErrs []error
			for task, err := range client.ListTasksInProjectStream(context.Background(), tt.projectID, TaskListQuery{}) {
				if err == nil {
					t.Errorf("unexpected task %+v without error", task)
					continue
				}
				gotErrs = append(gotErrs, err)
			}

			if len(gotErrs) != 1 {
				t.Fatalf("errors = %v, want exactly 1", gotErrs)
			}
			if !strings.Contains(gotErrs[0].Error(), "projectID") {
				t.Errorf("error = %q, want it to mention projectID", gotErrs[0].Error())
			}
			if requests != 0 {
				t.Errorf("requests = %d, want 0", requests)
			}
		})
	}
}

func TestListTasksInProjectStream_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	var gotErrs []error
	for _, err := range client.ListTasksInProjectStream(ctx, 5, TaskListQuery{}) {
		if err != nil {
			gotErrs = append(gotErrs, err)
		}
	}

	if len(gotErrs) != 1 || !errors.Is(gotErrs[0], context.Canceled) {
		t.Errorf("errors = %v, want exactly one context.Canceled", gotErrs)
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
	_, err := client.ListTasksInProjectView(context.Background(), 5, 7, TaskListQuery{})
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
		name  string
		query TaskListQuery
		want  url.Values
	}{
		{
			name:  "defaults sends nothing",
			query: TaskListQuery{},
			want:  url.Values{},
		},
		{
			name: "all parameters set",
			query: TaskListQuery{
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
			name:  "query only",
			query: TaskListQuery{Query: "milk"},
			want: url.Values{
				"q": {"milk"},
			},
		},
		{
			name: "filter only",
			query: TaskListQuery{
				Filter: "priority >= 3 || labels in 1, 2",
			},
			want: url.Values{
				"filter": {"priority >= 3 || labels in 1, 2"},
			},
		},
		{
			name:  "pagination only",
			query: TaskListQuery{Page: 3, PerPage: 10},
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
			_, err := client.ListTasksInProjectView(context.Background(), 5, 7, tt.query)
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
	got, err := client.ListTasksInProjectView(context.Background(), 5, 7, TaskListQuery{})
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
	resp, err := client.ListTasksInProjectView(context.Background(), 5, 7, TaskListQuery{})
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
	resp, err := client.ListTasksInProjectView(context.Background(), 5, 7, TaskListQuery{})
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
	resp, err := client.ListTasksInProjectView(context.Background(), 5, 7, TaskListQuery{})
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
			resp, err := client.ListTasksInProjectView(context.Background(), tt.projectID, tt.viewID, TaskListQuery{})
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
	_, err := client.ListTasksInProjectView(ctx, 5, 7, TaskListQuery{})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestListTasksInProjectViewStream_IteratesAllPages(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Buy milk","project_id":5,"bucket_id":3},{"id":2,"title":"Buy eggs","project_id":5,"bucket_id":3}],"page":1,"total_pages":2}`,
		`{"items":[{"id":3,"title":"Buy bread","project_id":5,"bucket_id":4}],"page":2,"total_pages":2}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	var got []Task
	query := TaskListQuery{Filter: "done = false", PerPage: 2, Page: 5}
	for task, err := range client.ListTasksInProjectViewStream(context.Background(), 5, 7, query) {
		if err != nil {
			t.Fatalf("ListTasksInProjectViewStream returned error: %v", err)
		}
		got = append(got, task)
	}

	want := []Task{
		{ID: 1, Title: "Buy milk", ProjectID: 5, BucketID: 3},
		{ID: 2, Title: "Buy eggs", ProjectID: 5, BucketID: 3},
		{ID: 3, Title: "Buy bread", ProjectID: 5, BucketID: 4},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %+v, want %+v", got, want)
	}

	// query.Page is ignored: iteration starts at page 1, other parameters are passed through.
	wantQueries := []url.Values{
		{"filter": {"done = false"}, "page": {"1"}, "per_page": {"2"}},
		{"filter": {"done = false"}, "page": {"2"}, "per_page": {"2"}},
	}
	if !reflect.DeepEqual(gotQueries, wantQueries) {
		t.Errorf("queries = %v, want %v", gotQueries, wantQueries)
	}
}

func TestListTasksInProjectViewStream_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		_, _ = w.Write([]byte(`{"items":[],"total_pages":0}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	for _, err := range client.ListTasksInProjectViewStream(context.Background(), 5, 7, TaskListQuery{}) {
		if err != nil {
			t.Fatalf("ListTasksInProjectViewStream returned error: %v", err)
		}
	}

	if gotReq == nil {
		t.Fatal("no request was sent")
	}
	if gotReq.Method != http.MethodGet {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodGet)
	}
	if gotReq.URL.Path != "/api/v2/projects/5/views/7/tasks" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/projects/5/views/7/tasks")
	}
}

func TestListTasksInProjectViewStream_StopsOnLastPage(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Buy milk"}],"page":1,"total_pages":1}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	count := 0
	for _, err := range client.ListTasksInProjectViewStream(context.Background(), 5, 7, TaskListQuery{}) {
		if err != nil {
			t.Fatalf("ListTasksInProjectViewStream returned error: %v", err)
		}
		count++
	}

	if count != 1 {
		t.Errorf("tasks = %d, want 1", count)
	}
	if len(gotQueries) != 1 {
		t.Errorf("requests = %d, want 1", len(gotQueries))
	}
}

func TestListTasksInProjectViewStream_StopsOnBreak(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Buy milk"},{"id":2,"title":"Buy eggs"}],"page":1,"total_pages":2}`,
		`{"items":[{"id":3,"title":"Buy bread"}],"page":2,"total_pages":2}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	var got []int64
	for task, err := range client.ListTasksInProjectViewStream(context.Background(), 5, 7, TaskListQuery{}) {
		if err != nil {
			t.Fatalf("ListTasksInProjectViewStream returned error: %v", err)
		}
		got = append(got, task.ID)
		if task.ID == 1 {
			break
		}
	}

	if want := []int64{1}; !reflect.DeepEqual(got, want) {
		t.Errorf("task IDs = %v, want %v", got, want)
	}
	if len(gotQueries) != 1 {
		t.Errorf("requests = %d, want 1", len(gotQueries))
	}
}

func TestListTasksInProjectViewStream_StopsOnEmptyPage(t *testing.T) {
	var gotQueries []url.Values
	server := pagedServer(t, []string{
		`{"items":[{"id":1,"title":"Buy milk"}],"total_pages":5}`,
		`{"items":[],"total_pages":5}`,
	}, &gotQueries)

	client := newTestClient(server.URL)
	count := 0
	for _, err := range client.ListTasksInProjectViewStream(context.Background(), 5, 7, TaskListQuery{}) {
		if err != nil {
			t.Fatalf("ListTasksInProjectViewStream returned error: %v", err)
		}
		count++
	}

	if count != 1 {
		t.Errorf("tasks = %d, want 1", count)
	}
	if len(gotQueries) != 2 {
		t.Errorf("requests = %d, want 2", len(gotQueries))
	}
}

func TestListTasksInProjectViewStream_YieldsError(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Query().Get("page") == "1" {
			_, _ = w.Write([]byte(`{"items":[{"id":1,"title":"Buy milk"}],"page":1,"total_pages":3}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	var got []Task
	var gotErrs []error
	for task, err := range client.ListTasksInProjectViewStream(context.Background(), 5, 7, TaskListQuery{}) {
		if err != nil {
			if !reflect.DeepEqual(task, Task{}) {
				t.Errorf("task yielded with error = %+v, want zero Task", task)
			}
			gotErrs = append(gotErrs, err)
			continue
		}
		got = append(got, task)
	}

	if want := []Task{{ID: 1, Title: "Buy milk"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("tasks = %+v, want %+v", got, want)
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

func TestListTasksInProjectViewStream_InvalidIDs(t *testing.T) {
	tests := []struct {
		name      string
		projectID int64
		viewID    int64
		wantInErr string
	}{
		{name: "missing project", projectID: 0, viewID: 7, wantInErr: "projectID"},
		{name: "negative project", projectID: -1, viewID: 7, wantInErr: "projectID"},
		{name: "missing view", projectID: 5, viewID: 0, wantInErr: "viewID"},
		{name: "negative view", projectID: 5, viewID: -1, wantInErr: "viewID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
			}))
			defer server.Close()

			client := newTestClient(server.URL)
			var gotErrs []error
			for task, err := range client.ListTasksInProjectViewStream(context.Background(), tt.projectID, tt.viewID, TaskListQuery{}) {
				if err == nil {
					t.Errorf("unexpected task %+v without error", task)
					continue
				}
				gotErrs = append(gotErrs, err)
			}

			if len(gotErrs) != 1 {
				t.Fatalf("errors = %v, want exactly 1", gotErrs)
			}
			if !strings.Contains(gotErrs[0].Error(), tt.wantInErr) {
				t.Errorf("error = %q, want it to mention %s", gotErrs[0].Error(), tt.wantInErr)
			}
			if requests != 0 {
				t.Errorf("requests = %d, want 0", requests)
			}
		})
	}
}

func TestListTasksInProjectViewStream_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	var gotErrs []error
	for _, err := range client.ListTasksInProjectViewStream(ctx, 5, 7, TaskListQuery{}) {
		if err != nil {
			gotErrs = append(gotErrs, err)
		}
	}

	if len(gotErrs) != 1 || !errors.Is(gotErrs[0], context.Canceled) {
		t.Errorf("errors = %v, want exactly one context.Canceled", gotErrs)
	}
}

func TestUpdateTask_SendsRequest(t *testing.T) {
	var gotReq *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotReq = r
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":42}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	_, err := client.UpdateTask(context.Background(), 42, UpdateTaskRequest{Done: new(true)})
	if err != nil {
		t.Fatalf("UpdateTask returned error: %v", err)
	}

	if gotReq.Method != http.MethodPatch {
		t.Errorf("method = %q, want %q", gotReq.Method, http.MethodPatch)
	}
	if gotReq.URL.Path != "/api/v2/tasks/42" {
		t.Errorf("path = %q, want %q", gotReq.URL.Path, "/api/v2/tasks/42")
	}
	if got := gotReq.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
	}
	if got := gotReq.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept header = %q, want %q", got, "application/json")
	}
	if got := gotReq.Header.Get("Content-Type"); got != "application/merge-patch+json" {
		t.Errorf("Content-Type header = %q, want %q", got, "application/merge-patch+json")
	}
}

func TestUpdateTask_RequestBody(t *testing.T) {
	// JSON numbers decode to float64 in a map[string]any.
	tests := []struct {
		name    string
		request UpdateTaskRequest
		want    map[string]any
	}{
		{
			name:    "empty request sends empty object",
			request: UpdateTaskRequest{},
			want:    map[string]any{},
		},
		{
			name: "all fields set",
			request: UpdateTaskRequest{
				BucketID:    new(int64(3)),
				Description: new("2 litres"),
				Done:        new(true),
				ProjectID:   new(int64(5)),
			},
			want: map[string]any{
				"bucket_id":   float64(3),
				"description": "2 litres",
				"done":        true,
				"project_id":  float64(5),
			},
		},
		{
			name:    "done only",
			request: UpdateTaskRequest{Done: new(true)},
			want:    map[string]any{"done": true},
		},
		{
			name:    "mark not done sends false",
			request: UpdateTaskRequest{Done: new(false)},
			want:    map[string]any{"done": false},
		},
		{
			name:    "move to bucket only",
			request: UpdateTaskRequest{BucketID: new(int64(3))},
			want:    map[string]any{"bucket_id": float64(3)},
		},
		{
			name:    "move to project only",
			request: UpdateTaskRequest{ProjectID: new(int64(5))},
			want:    map[string]any{"project_id": float64(5)},
		},
		{
			name:    "description only",
			request: UpdateTaskRequest{Description: new("2 litres")},
			want:    map[string]any{"description": "2 litres"},
		},
		{
			name:    "clear description sends empty string",
			request: UpdateTaskRequest{Description: new("")},
			want:    map[string]any{"description": ""},
		},
		{
			name: "zero values sent alongside other fields",
			request: UpdateTaskRequest{
				Description: new(""),
				Done:        new(false),
				ProjectID:   new(int64(5)),
			},
			want: map[string]any{
				"description": "",
				"done":        false,
				"project_id":  float64(5),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
					t.Errorf("failed to decode request body: %v", err)
				}
				_, _ = w.Write([]byte(`{"id":42}`))
			}))
			defer server.Close()

			client := newTestClient(server.URL)
			_, err := client.UpdateTask(context.Background(), 42, tt.request)
			if err != nil {
				t.Fatalf("UpdateTask returned error: %v", err)
			}

			if !reflect.DeepEqual(gotBody, tt.want) {
				t.Errorf("body = %v, want %v", gotBody, tt.want)
			}
		})
	}
}

func TestUpdateTask_DecodesResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": 42,
			"title": "Buy milk",
			"project_id": 5,
			"description": "2 litres",
			"bucket_id": 3,
			"labels": [{"id": 7, "title": "Shopping", "description": "Errands"}]
		}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	got, err := client.UpdateTask(context.Background(), 42, UpdateTaskRequest{BucketID: new(int64(3))})
	if err != nil {
		t.Fatalf("UpdateTask returned error: %v", err)
	}

	want := &Task{
		ID:          42,
		Title:       "Buy milk",
		ProjectID:   5,
		Description: "2 litres",
		BucketID:    3,
		Labels:      []Label{{ID: 7, Title: "Shopping", Description: "Errands"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("task = %+v, want %+v", got, want)
	}
}

func TestUpdateTask_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"forbidden"}`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	task, err := client.UpdateTask(context.Background(), 42, UpdateTaskRequest{Done: new(true)})
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

func TestUpdateTask_InvalidResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := newTestClient(server.URL)
	task, err := client.UpdateTask(context.Background(), 42, UpdateTaskRequest{Done: new(true)})
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

func TestUpdateTask_RequestError(t *testing.T) {
	client := newTestClient("http://[::1")
	task, err := client.UpdateTask(context.Background(), 42, UpdateTaskRequest{Done: new(true)})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if task != nil {
		t.Errorf("task = %+v, want nil", task)
	}
}

func TestUpdateTask_InvalidIDs(t *testing.T) {
	tests := []struct {
		name      string
		taskID    int64
		request   UpdateTaskRequest
		wantInErr string
	}{
		{name: "missing task ID", taskID: 0, request: UpdateTaskRequest{Done: new(true)}, wantInErr: "taskID"},
		{name: "negative task ID", taskID: -1, request: UpdateTaskRequest{Done: new(true)}, wantInErr: "taskID"},
		{name: "zero bucket ID", taskID: 42, request: UpdateTaskRequest{BucketID: new(int64(0))}, wantInErr: "BucketID"},
		{name: "negative bucket ID", taskID: 42, request: UpdateTaskRequest{BucketID: new(int64(-1))}, wantInErr: "BucketID"},
		{name: "zero project ID", taskID: 42, request: UpdateTaskRequest{ProjectID: new(int64(0))}, wantInErr: "ProjectID"},
		{name: "negative project ID", taskID: 42, request: UpdateTaskRequest{ProjectID: new(int64(-1))}, wantInErr: "ProjectID"},
		{
			name:      "invalid project ID alongside valid fields",
			taskID:    42,
			request:   UpdateTaskRequest{BucketID: new(int64(3)), Done: new(true), ProjectID: new(int64(0))},
			wantInErr: "ProjectID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
			}))
			defer server.Close()

			client := newTestClient(server.URL)
			task, err := client.UpdateTask(context.Background(), tt.taskID, tt.request)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if task != nil {
				t.Errorf("task = %+v, want nil", task)
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

func TestUpdateTask_CanceledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":42}`))
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := newTestClient(server.URL)
	_, err := client.UpdateTask(ctx, 42, UpdateTaskRequest{Done: new(true)})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}
