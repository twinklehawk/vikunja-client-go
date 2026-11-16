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

// DefaultUserAgent is the User-Agent header sent with requests unless overridden with [WithUserAgent].
const DefaultUserAgent = "vikunja-client-go"

// maxErrorBodySize is the maximum number of bytes of an error response body read into an [APIError].
const maxErrorBodySize = 64 << 10

// DefaultBaseURL is the base URL used unless overridden with [WithBaseURL].
const DefaultBaseURL = "https://vikunja.cloud"

// Client is a Vikunja API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	apiToken   string
}

// Option configures a [Client].
type Option func(*Client)

// WithHTTPClient sets the [http.Client] used to send requests. By default, a client with a
// 30 second timeout is used. A nil client is ignored.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		if httpClient != nil {
			c.httpClient = httpClient
		}
	}
}

// WithUserAgent sets the User-Agent header sent with requests. By default, [DefaultUserAgent] is used.
func WithUserAgent(userAgent string) Option {
	return func(c *Client) {
		c.userAgent = userAgent
	}
}

// WithAPIToken sets the API token to use for authenticating requests.
func WithAPIToken(apiToken string) Option {
	return func(c *Client) {
		c.apiToken = apiToken
	}
}

// WithBaseURL sets the base URL used for making requests. By default, [DefaultBaseURL] is used.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
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

// NewClient creates a new [Client] for making requests to a Vikunja instance.
func NewClient(opts ...Option) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    DefaultBaseURL,
		userAgent:  DefaultUserAgent,
	}
	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *Client) do(req *http.Request, respBody any) (err error) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		respData, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
		if err != nil {
			return fmt.Errorf("reading error response: %w", err)
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			StatusText: http.StatusText(resp.StatusCode),
			Message:    string(respData),
		}
	}

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if respBody != nil && len(respData) > 0 {
		if err := json.Unmarshal(respData, respBody); err != nil {
			return fmt.Errorf("unmarshaling response: %w", err)
		}
	}

	return nil
}
