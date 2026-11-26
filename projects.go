package vikunja

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"strconv"
)

// Project contains details for a Vikunja project.
type Project struct {
	// ID is the unique, numeric ID of the project.
	ID int64 `json:"id"`
	// Title is the title of the project.
	Title string `json:"title"`
}

// ProjectListQuery specifies the parameters for searching for projects.
type ProjectListQuery struct {
	// Query is the search query.
	Query string
	// IsArchived controls if archived projects are returned, by default archived projects are excluded.
	IsArchived bool
	// Expand indicates additional fields to expand.
	// If set to "permissions", each returned project includes the max permission the requesting user has on it.
	// Currently only "permissions" is supported.
	Expand string
	// Format determines how rich-text fields are exchanged. Supported values are "html" and "markdown".
	Format string
	// Page is a 1-based page number, default is 1.
	Page int64
	// PerPage is the number of items per page, default is controlled by the server configuration.
	PerPage int64
}

// PaginatedProjectsResponse contains the list of [Project]s and pagination information returned
// from a projects request.
type PaginatedProjectsResponse struct {
	// Items is the list of [Project]s matching the request.
	Items []Project `json:"items"`
	// Page is the current 1-based page number.
	Page int64 `json:"page"`
	// PerPage is the number of items per page.
	PerPage int64 `json:"per_page"`
	// Total is the total number of items matching the request.
	Total int64 `json:"total"`
	// TotalPages is the total number of pages of matching items.
	TotalPages int64 `json:"total_pages"`
}

// ListProjects returns the projects the authenticated user has access to (owned plus shared,
// with child projects of accessible parents), paginated. Archived projects are excluded unless
// IsArchived=true. Pass Expand=permissions to include each project's max_permission for the caller.
func (c *Client) ListProjects(
	ctx context.Context,
	request ProjectListQuery,
) (*PaginatedProjectsResponse, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v2/projects")
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	if request.Query != "" {
		q.Add("q", request.Query)
	}
	if request.IsArchived {
		q.Add("is_archived", "true")
	}
	if request.Expand != "" {
		q.Add("expand", request.Expand)
	}
	if request.Page != 0 {
		q.Add("page", strconv.FormatInt(request.Page, 10))
	}
	if request.PerPage != 0 {
		q.Add("per_page", strconv.FormatInt(request.PerPage, 10))
	}
	if request.Format != "" {
		q.Add("format", request.Format)
	}
	req.URL.RawQuery = q.Encode()

	respBody := PaginatedProjectsResponse{}
	err = c.do(req, &respBody)
	if err != nil {
		return nil, err
	}
	return &respBody, nil
}

// ListProjectsStream returns an iterator over all projects matching the request, fetching pages on demand.
// request.Page is ignored; iteration always starts at page 1. If a request fails, the error is
// yielded with a zero [Project] and iteration stops.
func (c *Client) ListProjectsStream(
	ctx context.Context,
	request ProjectListQuery,
) iter.Seq2[Project, error] {
	return func(yield func(Project, error) bool) {
		req := request
		req.Page = 1

		for {
			resp, err := c.ListProjects(ctx, req)
			if err != nil {
				yield(Project{}, err)
				return
			}
			for _, item := range resp.Items {
				if !yield(item, nil) {
					return
				}
			}
			if len(resp.Items) == 0 || req.Page >= resp.TotalPages {
				return
			}
			req.Page++
		}
	}
}

// FindProjectByTitle finds a single project by its title.
// Returns an error if no projects match or the first match if there are multiple matches.
func (c *Client) FindProjectByTitle(ctx context.Context, title string) (*Project, error) {
	for p, err := range c.ListProjectsStream(ctx, ProjectListQuery{Query: title}) {
		if err != nil {
			return nil, err
		}
		if p.Title == title {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("no project found matching title '%s'", title)
}

// Bucket contains data for a Vikunja bucket.
type Bucket struct {
	ID            int64   `json:"id"`
	Title         string  `json:"title"`
	Position      float64 `json:"position"`
	ProjectViewID int64   `json:"project_view_id"`
}

// PaginatedBucketsResponse contains the list of [Bucket]s and pagination information returned
// from a buckets request.
type PaginatedBucketsResponse struct {
	// Items is the list of [Bucket]s matching the request.
	Items []Bucket `json:"items"`
	// Page is the current 1-based page number.
	Page int64 `json:"page"`
	// PerPage is the number of items per page.
	PerPage int64 `json:"per_page"`
	// Total is the total number of items matching the request.
	Total int64 `json:"total"`
	// TotalPages is the total number of pages of matching items.
	TotalPages int64 `json:"total_pages"`
}

// ProjectViewBucketListQuery specifies the parameters for searching for project view buckets.
type ProjectViewBucketListQuery struct {
	// Query is the search query.
	Query string
	// Page is a 1-based page number, default is 1.
	Page int64
	// PerPage is the number of items per page, default is controlled by the server configuration.
	PerPage int64
}

// ListProjectViewBuckets returns all kanban buckets of a project view, ordered by position.
// The list is not paginated by the server but is returned in the standard list envelope.
// Requires read access to the project.
func (c *Client) ListProjectViewBuckets(
	ctx context.Context,
	projectID int64,
	projectViewID int64,
	query ProjectViewBucketListQuery,
) (*PaginatedBucketsResponse, error) {
	if projectID <= 0 {
		return nil, fmt.Errorf("projectID must be a positive ID, got %d", projectID)
	}
	if projectViewID <= 0 {
		return nil, fmt.Errorf("projectViewID must be a positive ID, got %d", projectViewID)
	}

	req, err := c.newRequest(
		ctx,
		http.MethodGet,
		fmt.Sprintf("/api/v2/projects/%d/views/%d/buckets", projectID, projectViewID),
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

	respBody := PaginatedBucketsResponse{}
	err = c.do(req, &respBody)
	if err != nil {
		return nil, err
	}
	return &respBody, nil
}
