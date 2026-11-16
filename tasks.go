package vikunja

import (
	"context"
	"fmt"
	"net/http"
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
