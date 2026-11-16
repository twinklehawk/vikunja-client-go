package vikunja

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
