// Vikunja provides a client for the Vikunja V2 API.
//
// Exmample usage:
//
// client := NewClient("https://vikunja-url", "apiToken")
//
//	req := ListProjectsRequest {
//	    Query: "project-name"
//	}
//
// resp, err := client.ListProjects(context.Background())
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
	baseUrl             string
	requestInterceptors []requestInterceptor
}

// ApiError represents an error when calling a Vikunja API.
type ApiError struct {
	StatusCode int
	StatusText string
	Message    string
}

// Error returns information about the [ApiError].
func (e *ApiError) Error() string {
	return fmt.Sprintf("%d - %s: %s", e.StatusCode, e.StatusText, e.Message)
}

// RequestInterceptor modified a [http.Request] before the request is sent.
type requestInterceptor interface {
	process(req *http.Request) error
}

type acceptRequestInterceptor struct{}

func (i *acceptRequestInterceptor) process(req *http.Request) error {
	req.Header.Set("Accept", "application/json")
	return nil
}

// NewClient creates a new [Client] for making requests to a Vikunja instance.
func NewClient(baseUrl string, apiToken string) *Client {
	interceptors := make([]requestInterceptor, 2)
	interceptors[0] = &apiTokenRequestInterceptor{apiToken: apiToken}
	interceptors[1] = &acceptRequestInterceptor{}

	return &Client{
		httpClient:          &http.Client{Timeout: 30 * time.Second},
		baseUrl:             baseUrl,
		requestInterceptors: interceptors,
	}
}

func (c *Client) do(req *http.Request, respBody any) error {
	for _, interceptor := range c.requestInterceptors {
		err := interceptor.process(req)
		if err != nil {
			return fmt.Errorf("failed to run interceptor: %w", err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return &ApiError{
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
