package vikunja

import (
	"context"
	"fmt"
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
	// The bucket the task is in. Only populated when the task is accessed via a view with buckets.
	// To move a task between buckets, the new bucket must be in the same view as the old one.
	BucketID int64 `json:"bucket_id"`
	// The labels on this task. Read-only here; use the label-task endpoints to add or remove labels.
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
		return nil, fmt.Errorf("task.ProjectID must be a positive project ID, got %d", task.ProjectID)
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

// ListTasksInProjectRequest specifies the parameters for the ListTasksInProject API.
type ListTasksInProjectRequest struct {
	// ProjectID is the ID of the project.
	ProjectID int64
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
	request ListTasksInProjectRequest,
) (*PaginatedTasksResponse, error) {
	if request.ProjectID <= 0 {
		return nil, fmt.Errorf("projectID must be a positive ID, got %d", request.ProjectID)
	}

	return c.listTasks(
		ctx,
		fmt.Sprintf("/api/v2/projects/%d/tasks", request.ProjectID),
		taskListQuery{
			Query:              request.Query,
			Filter:             request.Filter,
			FilterTimeZone:     request.FilterTimeZone,
			FilterIncludeNulls: request.FilterIncludeNulls,
			Format:             request.Format,
			Page:               request.Page,
			PerPage:            request.PerPage,
		},
	)
}

// ListTasksInProjectViewRequest specifies the parameters for the ListTasksInProjectView API.
type ListTasksInProjectViewRequest struct {
	// ProjectID is the ID of the project.
	ProjectID int64
	// ViewID is the ID of the project view.
	ViewID int64
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
	// Format determines how rich-text fields are exchanged. Supported values are "html" and "markdown".
	Format string
	// Page is a 1-based page number, default is 1.
	Page int64
	// PerPage is the number of items per page, default is controlled by the server configuration.
	PerPage int64
}

// ListTasksInProjectView returns the tasks in a project view, paginated and flat.
func (c *Client) ListTasksInProjectView(
	ctx context.Context,
	request ListTasksInProjectViewRequest,
) (*PaginatedTasksResponse, error) {
	if request.ProjectID <= 0 {
		return nil, fmt.Errorf("projectID must be a positive ID, got %d", request.ProjectID)
	}
	if request.ViewID <= 0 {
		return nil, fmt.Errorf("viewID must be a positive ID, got %d", request.ViewID)
	}

	return c.listTasks(
		ctx,
		fmt.Sprintf("/api/v2/projects/%d/views/%d/tasks", request.ProjectID, request.ViewID),
		taskListQuery{
			Query:              request.Query,
			Filter:             request.Filter,
			FilterTimeZone:     request.FilterTimeZone,
			FilterIncludeNulls: request.FilterIncludeNulls,
			Format:             request.Format,
			Page:               request.Page,
			PerPage:            request.PerPage,
		},
	)
}

// taskListQuery holds the query parameters shared by the task list APIs.
type taskListQuery struct {
	Query              string
	Filter             string
	FilterTimeZone     string
	FilterIncludeNulls bool
	Format             string
	Page               int64
	PerPage            int64
}

// addTo adds the non-zero parameters to q.
func (p taskListQuery) addTo(q url.Values) {
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
}

// listTasks sends a GET request for a paginated list of tasks to path with the given query parameters.
func (c *Client) listTasks(
	ctx context.Context,
	path string,
	params taskListQuery,
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
