package vikunja

import (
	"context"
	"fmt"
	"net/http"
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
