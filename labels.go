package vikunja

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
)

// Label contains details for a Vikunja label.
type Label struct {
	// ID is the unique, numeric ID of the label.
	ID int64 `json:"id,omitempty"`
	// Title is the title of the label.
	Title string `json:"title"`
	// Description is a description of the label.
	Description string `json:"description"`
}

// PaginatedLabelResponse contains the list of [Label]s and pagination information returned
// from a labels request.
type PaginatedLabelResponse struct {
	// Items is the list of [Label]s matching the request.
	Items []Label `json:"items"`
	// Page is the current 1-based page number.
	Page int64 `json:"page"`
	// PerPage is the number of items per page.
	PerPage int64 `json:"per_page"`
	// Total is the total number of items matching the request.
	Total int64 `json:"total"`
	// TotalPages is the total number of pages of matching items.
	TotalPages int64 `json:"total_pages"`
}

// LabelListQuery specifies the parameters for searching for labels.
type LabelListQuery struct {
	// Query is the search query.
	Query string
	// Page is a 1-based page number, default is 1.
	Page int64
	// PerPage is the number of items per page, default is controlled by the server configuration.
	PerPage int64
}

// ListLabels returns the labels visible to the authenticated user — their own plus any used on tasks they can access.
func (c *Client) ListLabels(
	ctx context.Context,
	query LabelListQuery,
) (*PaginatedLabelResponse, error) {
	req, err := c.newRequest(
		ctx,
		http.MethodGet,
		"/api/v2/labels",
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

	respBody := PaginatedLabelResponse{}
	err = c.do(req, &respBody)
	if err != nil {
		return nil, err
	}
	return &respBody, nil
}

// AddTaskLabelRequest specifies the parameters for the AddTaskLabel API.
type AddTaskLabelRequest struct {
	// LabelID is the ID of the label to associate to the task.
	LabelID int64 `json:"label_id"`
}

// AddTaskLabel adds a label to a task. This method fails if the label is already on the task.
// Requires write access to the task and access to the label.
func (c *Client) AddTaskLabel(
	ctx context.Context,
	taskID int64,
	request AddTaskLabelRequest,
) error {
	if taskID <= 0 {
		return fmt.Errorf("taskID must be a positive ID, got %d", taskID)
	}
	if request.LabelID <= 0 {
		return fmt.Errorf("request.LabelID must be a positive ID, got %d", request.LabelID)
	}

	path := fmt.Sprintf("/api/v2/tasks/%d/labels", taskID)
	req, err := c.newRequestWithBody(ctx, http.MethodPost, path, request)
	if err != nil {
		return err
	}

	err = c.do(req, nil)
	if err != nil {
		return err
	}
	return nil
}

// RemoveTaskLabel removes a label from a task.
// Requires write access to the task.
func (c *Client) RemoveTaskLabel(
	ctx context.Context,
	taskID int64,
	labelID int64,
) error {
	if taskID <= 0 {
		return fmt.Errorf("taskID must be a positive ID, got %d", taskID)
	}
	if labelID <= 0 {
		return fmt.Errorf("labelID must be a positive ID, got %d", labelID)
	}

	req, err := c.newRequest(
		ctx,
		http.MethodDelete,
		fmt.Sprintf("/api/v2/tasks/%d/labels/%d", taskID, labelID),
	)
	if err != nil {
		return err
	}

	err = c.do(req, nil)
	if err != nil {
		return err
	}
	return nil
}
