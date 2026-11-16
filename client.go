// Vikunja provides a client for the Vikunja V2 API.
//
// Example usage:
//
// client := NewClient("https://vikunja-url", "apiToken")
//
//	req := ListProjectsRequest {
//	    Query: "project-name"
//	}
//
// resp, err := client.ListProjects(context.Background(), ListProjectsRequest{})
package vikunja

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is a Vikunja API client.
type Client struct {
	httpClient          *http.Client
	baseURL             string
	requestInterceptors []requestInterceptor
}

// APIError represents an error when calling a Vikunja API.
type APIError struct {
	StatusCode int
	StatusText string
	Message    string
}

// Error returns information about the [APIError].
func (e *APIError) Error() string {
	return fmt.Sprintf("%d - %s: %s", e.StatusCode, e.StatusText, e.Message)
}

// requestInterceptor modifies a [http.Request] before the request is sent.
type requestInterceptor interface {
	process(req *http.Request) error
}

type acceptRequestInterceptor struct{}

func (i *acceptRequestInterceptor) process(req *http.Request) error {
	req.Header.Set("Accept", "application/json")
	return nil
}

// NewClient creates a new [Client] for making requests to a Vikunja instance.
func NewClient(baseURL string, apiToken string) *Client {
	interceptors := []requestInterceptor{
		&apiTokenRequestInterceptor{apiToken: apiToken},
		&acceptRequestInterceptor{},
	}

	return &Client{
		httpClient:          &http.Client{Timeout: 30 * time.Second},
		baseURL:             baseURL,
		requestInterceptors: interceptors,
	}
}

func (c *Client) do(req *http.Request, respBody any) error {
	for _, interceptor := range c.requestInterceptors {
		err := interceptor.process(req)
		if err != nil {
			return fmt.Errorf("running interceptor: %w", err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &APIError{
			StatusCode: resp.StatusCode,
			StatusText: resp.Status,
			Message:    string(respData),
		}
	}

	if respBody != nil {
		if err := json.Unmarshal(respData, respBody); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	}

	return nil
}
