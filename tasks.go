package vikunja

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strconv"
)

// Task contains details for a Vikunja task.
type Task struct {
	// ID is the unique, numeric ID of the task.
	ID int64 `json:"id,omitempty"`
	// Title is the title of the task.
	Title string `json:"title"`
	// ProjectID is the ID of the project containing this task.
	ProjectID int64 `json:"project_id"`
	// Description is a description of the task.
	Description string `json:"description"`
	// BucketID is the ID of the bucket the task is in. Only populated when the task is accessed
	// via a view with buckets. To move a task between buckets, the new bucket must be in the same
	// view as the old one.
	BucketID int64 `json:"bucket_id"`
	// Buckets are the task's buckets across all views. Only present when requested via the buckets expand option.
	Buckets []Bucket `json:"buckets"`
	// Labels are the labels on this task. Read-only here; use the label-task endpoints to add or remove labels.
	Labels []Label `json:"labels"`
}

// PaginatedTasksResponse contains the list of [Task]s and pagination information returned from a tasks API.
type PaginatedTasksResponse struct {
	// Items is the list of [Task]s matching the request.
	Items []Task `json:"items"`
	// Page is the current 1-based page number.
	Page int64 `json:"page"`
	// PerPage is the number of items per page.
	PerPage int64 `json:"per_page"`
	// Total is the total number of items matching the request.
	Total int64 `json:"total"`
	// TotalPages is the total number of pages of matching items.
	TotalPages int64 `json:"total_pages"`
}

// CreateTask creates a task in the project specified by task.ProjectID.
// The authenticated user needs write access to that project and becomes the task's creator.
func (c *Client) CreateTask(
	ctx context.Context,
	task Task,
) (*Task, error) {
	if task.ProjectID <= 0 {
		return nil, fmt.Errorf(
			"task.ProjectID must be a positive project ID, got %d",
			task.ProjectID,
		)
	}

	path := fmt.Sprintf("/api/v2/projects/%d/tasks", task.ProjectID)
	req, err := c.newRequestWithBody(ctx, http.MethodPost, path, task)
	if err != nil {
		return nil, err
	}

	respBody := Task{}
	err = c.do(req, &respBody)
	if err != nil {
		return nil, err
	}
	return &respBody, nil
}

type TaskExpand string

const (
	TaskExpandSubtasks         TaskExpand = "subtasks"
	TaskExpandBuckets          TaskExpand = "buckets"
	TaskExpandReactions        TaskExpand = "reactions"
	TaskExpandComments         TaskExpand = "comments"
	TaskExpandCommentCount     TaskExpand = "comment_count"
	TaskExpandTimeEntriesCount TaskExpand = "time_entries_count"
	TaskExpandIsUnread         TaskExpand = "is_unread"
)

// TaskListQuery holds the query parameters shared by the task list APIs.
type TaskListQuery struct {
	// Query is the search query.
	Query string
	// Filter is the query to match tasks by, e.g. "done = false && due_date < now+7d".
	// Combine conditions with && or || (not and/or) and group them with parentheses.
	// Operators: =, !=, >, >=, <, <=, like, in, not in.
	Filter string
	// FilterTimeZone is the timezone used to resolve relative dates in the filter.
	FilterTimeZone string
	// FilterIncludeNulls controls if the filter should match when the task's field is null.
	FilterIncludeNulls bool
	// Expand is the extra embedded elements to expand in the tasks.
	// Some fields, like Buckets, are not expanded unless specified here.
	Expand []TaskExpand
	// Format determines how rich-text fields are exchanged. Supported values are "html" and "markdown".
	Format string
	// Page is a 1-based page number, default is 1.
	Page int64
	// PerPage is the number of items per page, default is controlled by the server configuration.
	PerPage int64
}

// ListTasksInProject returns the tasks in a project, paginated and flat.
// Requires read access to the project.
func (c *Client) ListTasksInProject(
	ctx context.Context,
	projectID int64,
	query TaskListQuery,
) (*PaginatedTasksResponse, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("projectID must be a positive ID, got %d", projectID)
	}

	return c.listTasks(
		ctx,
		fmt.Sprintf("/api/v2/projects/%d/tasks", projectID),
		query,
	)
}

// ListTasksInProjectStream returns an iterator over all tasks in a project matching the request,
// fetching pages on demand.
// query.Page is ignored; iteration always starts at page 1. If a request fails, the error is
// yielded with a zero [Task] and iteration stops.
func (c *Client) ListTasksInProjectStream(
	ctx context.Context,
	projectID int64,
	query TaskListQuery,
) iter.Seq2[Task, error] {
	return func(yield func(Task, error) bool) {
		q := query
		q.Page = 1

		for {
			resp, err := c.ListTasksInProject(ctx, projectID, q)
			if err != nil {
				yield(Task{}, err)
				return
			}
			for _, item := range resp.Items {
				if !yield(item, nil) {
					return
				}
			}
			if len(resp.Items) == 0 || q.Page >= resp.TotalPages {
				return
			}
			q.Page++
		}
	}
}

// ListTasksInProjectView returns the tasks in a project view, paginated and flat.
func (c *Client) ListTasksInProjectView(
	ctx context.Context,
	projectID int64,
	viewID int64,
	query TaskListQuery,
) (*PaginatedTasksResponse, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("projectID must be a positive ID, got %d", projectID)
	}
	if viewID <= 0 {
		return nil, fmt.Errorf("viewID must be a positive ID, got %d", viewID)
	}

	return c.listTasks(
		ctx,
		fmt.Sprintf("/api/v2/projects/%d/views/%d/tasks", projectID, viewID),
		query,
	)
}

// ListTasksInProjectViewStream returns an iterator over all tasks in a project view matching the request,
// fetching pages on demand.
// query.Page is ignored; iteration always starts at page 1. If a request fails, the error is
// yielded with a zero [Task] and iteration stops.
func (c *Client) ListTasksInProjectViewStream(
	ctx context.Context,
	projectID int64,
	viewID int64,
	query TaskListQuery,
) iter.Seq2[Task, error] {
	return func(yield func(Task, error) bool) {
		q := query
		q.Page = 1

		for {
			resp, err := c.ListTasksInProjectView(ctx, projectID, viewID, q)
			if err != nil {
				yield(Task{}, err)
				return
			}
			for _, item := range resp.Items {
				if !yield(item, nil) {
					return
				}
			}
			if len(resp.Items) == 0 || q.Page >= resp.TotalPages {
				return
			}
			q.Page++
		}
	}
}

// addTo adds the non-zero parameters to q.
func (p TaskListQuery) addTo(q url.Values) {
	if p.Query != "" {
		q.Add("q", p.Query)
	}
	if p.Filter != "" {
		q.Add("filter", p.Filter)
	}
	if p.FilterTimeZone != "" {
		q.Add("filter_timezone", p.FilterTimeZone)
	}
	if p.FilterIncludeNulls {
		q.Add("filter_include_nulls", "true")
	}
	if p.Page != 0 {
		q.Add("page", strconv.FormatInt(p.Page, 10))
	}
	if p.PerPage != 0 {
		q.Add("per_page", strconv.FormatInt(p.PerPage, 10))
	}
	if p.Format != "" {
		q.Add("format", p.Format)
	}
	for i := range p.Expand {
		q.Add("expand", string(p.Expand[i]))
	}
}

// listTasks sends a GET request for a paginated list of tasks to path with the given query parameters.
func (c *Client) listTasks(
	ctx context.Context,
	path string,
	params TaskListQuery,
) (*PaginatedTasksResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, path)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	params.addTo(q)
	req.URL.RawQuery = q.Encode()

	respBody := PaginatedTasksResponse{}
	err = c.do(req, &respBody)
	if err != nil {
		return nil, err
	}
	return &respBody, nil
}

// UpdateTaskRequest is the details of the task to update.
// Only non-nil fields are sent and updated, so a field can be set to its zero value,
// e.g. Done: new(false) marks a task as not done and Description: new("") clears the description.
type UpdateTaskRequest struct {
	// BucketID moves the task to this bucket. The bucket must be in the same view as the task's current bucket.
	BucketID *int64 `json:"bucket_id,omitempty"`
	// Description replaces the task's description.
	Description *string `json:"description,omitempty"`
	// Done marks the task as done or not done.
	Done *bool `json:"done,omitempty"`
	// ProjectID moves the task to this project.
	ProjectID *int64 `json:"project_id,omitempty"`
}

// UpdateTask performs a partial update of task data.
// Only non-nil values of the request are updated in the task.
func (c *Client) UpdateTask(
	ctx context.Context,
	taskID int64,
	request UpdateTaskRequest,
) (*Task, error) {
	if taskID <= 0 {
		return nil, fmt.Errorf("taskID must be a positive ID, got %d", taskID)
	}
	if request.BucketID != nil && *request.BucketID <= 0 {
		return nil, fmt.Errorf("request.BucketID must be a positive ID, got %d", *request.BucketID)
	}
	if request.ProjectID != nil && *request.ProjectID <= 0 {
		return nil, fmt.Errorf(
			"request.ProjectID must be a positive ID, got %d",
			*request.ProjectID,
		)
	}

	path := fmt.Sprintf("/api/v2/tasks/%d", taskID)
	req, err := c.newRequestWithBody(ctx, http.MethodPatch, path, request)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/merge-patch+json")

	respBody := Task{}
	err = c.do(req, &respBody)
	if err != nil {
		return nil, err
	}
	return &respBody, nil
}
