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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.Query()
				_, _ = w.Write([]byte(`{"items":[]}`))
			}))
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(tt.body))
			}))
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
		t.Errorf("Message = %q, want %q", apiErr.Message, `{"message":"label already exists on task"}`)
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
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
			}))
			defer server.Close()

			client := newTestClient(server.URL)
			err := client.AddTaskLabel(context.Background(), tt.taskID, AddTaskLabelRequest{LabelID: tt.labelID})
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
