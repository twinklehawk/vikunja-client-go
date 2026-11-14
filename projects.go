package vikunja

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

// Project contains details for a Vikunja project.
type Project struct {
	// Id is the unique, numeric ID of the project.
	Id int64 `json:"id"`
	// Title is the title of the project.
	Title string `json:"title"`
}

// ListProjectsRequest specifies the parameters for searching for projects.
type ListProjectsRequest struct {
	// Query is the search query.
	Query      string
	IsArchived bool
	Expand     string
	Page       int64
	PerPage    int64
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
// is_archived=true. Pass expand=permissions to include each project's max_permission for the caller.
func (c *Client) ListProjects(
	ctx context.Context,
	request ListProjectsRequest,
) (*ListProjectsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseUrl+"/api/v2/projects", http.NoBody)
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
	req.URL.RawQuery = q.Encode()

	respBody := ListProjectsResponse{}
	err = c.do(req, respBody)
	if err != nil {
		return nil, err
	}
	return &respBody, nil
}
