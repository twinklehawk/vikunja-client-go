package vikunja

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
)

// ProjectViewKind is the kind of a [ProjectView], which controls how its tasks are displayed.
type ProjectViewKind string

// Supported [ProjectViewKind] values.
const (
	ProjectViewKindList   ProjectViewKind = "list"
	ProjectViewKindGantt  ProjectViewKind = "gantt"
	ProjectViewKindTable  ProjectViewKind = "table"
	ProjectViewKindKanban ProjectViewKind = "kanban"
)

// ProjectView contains details for a view of a Vikunja project.
type ProjectView struct {
	// ID is the unique, numeric ID of the view.
	ID int64 `json:"id"`
	// ProjectID is the ID of the project containing the view.
	ProjectID int64 `json:"project_id"`
	// Position is the position of the view in the project, which controls sorting.
	Position float64 `json:"position"`
	// Title is the title of the view.
	Title string `json:"title"`
	// ViewKind is the kind of the view.
	ViewKind ProjectViewKind `json:"view_kind"`
}

// ProjectViewListQuery specifies the parameters for searching for project views.
type ProjectViewListQuery struct {
	// Query is the search query.
	Query string
	// Page is a 1-based page number, default is 1.
	Page int64
	// PerPage is the number of items per page, default is controlled by the server configuration.
	PerPage int64
}

// PaginatedProjectViewsResponse contains the list of [ProjectView]s and pagination information returned
// from a project views request.
type PaginatedProjectViewsResponse struct {
	// Items is the list of [ProjectView]s matching the request.
	Items []ProjectView `json:"items"`
	// Page is the current 1-based page number.
	Page int64 `json:"page"`
	// PerPage is the number of items per page.
	PerPage int64 `json:"per_page"`
	// Total is the total number of items matching the request.
	Total int64 `json:"total"`
	// TotalPages is the total number of pages of matching items.
	TotalPages int64 `json:"total_pages"`
}

// ListProjectViews returns all views of a project. The list is not paginated by the server but is
// returned in the standard list envelope.
// Requires read access to the project.
func (c *Client) ListProjectViews(
	ctx context.Context,
	projectID int64,
	query ProjectViewListQuery,
) (*PaginatedProjectViewsResponse, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("projectID must be a positive ID, got %d", projectID)
	}

	req, err := c.newRequest(
		ctx,
		http.MethodGet,
		fmt.Sprintf("/api/v2/projects/%d/views", projectID),
	)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	if query.Query != "" {
		q.Add("q", query.Query)
	}
	if query.Page != 0 {
		q.Add("page", strconv.FormatInt(query.Page, 10))
	}
	if query.PerPage != 0 {
		q.Add("per_page", strconv.FormatInt(query.PerPage, 10))
	}
	req.URL.RawQuery = q.Encode()

	respBody := PaginatedProjectViewsResponse{}
	err = c.do(req, &respBody)
	if err != nil {
		return nil, err
	}
	return &respBody, nil
}

// FindProjectViewByTitle finds a single project view in a project by its title.
// Returns an error if no views match or the first match if there are multiple matches.
func (c *Client) FindProjectViewByTitle(
	ctx context.Context,
	projectID int64,
	title string,
) (*ProjectView, error) {
	views, err := c.ListProjectViews(ctx, projectID, ProjectViewListQuery{Query: title})
	if err != nil {
		return nil, err
	}
	index := slices.IndexFunc(views.Items, func(v ProjectView) bool { return v.Title == title })
	if index == -1 {
		return nil, fmt.Errorf("no project view found matching title '%s'", title)
	}
	return &views.Items[index], nil
}
