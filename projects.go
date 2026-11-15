package vikunja

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strconv"
)

// Project contains details for a Vikunja project.
type Project struct {
	// ID is the unique, numeric ID of the project.
	ID int64 `json:"id"`
	// Title is the title of the project.
	Title string `json:"title"`
}

// ListProjectsRequest specifies the parameters for searching for projects.
type ListProjectsRequest struct {
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

// ListProjectsResponse contains the list of [Project]s and pagination information returned
// from a ListProjects request.
type ListProjectsResponse struct {
	Items      []Project `json:"items"`
	Page       int64     `json:"page"`
	PerPage    int64     `json:"per_page"`
	Total      int64     `json:"total"`
	TotalPages int64     `json:"total_pages"`
}

// ListProjects returns the projects the authenticated user has access to (owned plus shared,
// with child projects of accessible parents), paginated. Archived projects are excluded unless
// IsArchived=true. Pass Expand=permissions to include each project's max_permission for the caller.
func (c *Client) ListProjects(
	ctx context.Context,
	request ListProjectsRequest,
) (*ListProjectsResponse, error) {
	url, err := url.JoinPath(c.baseURL, "/api/v2/projects")
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create listProjects request: %w", err)
	}

	q := req.URL.Query()
	if request.Query != "" {
		q.Add("q", request.Query)
	}
	q.Add("is_archived", strconv.FormatBool(request.IsArchived))
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

	respBody := ListProjectsResponse{}
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
	request ListProjectsRequest,
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
