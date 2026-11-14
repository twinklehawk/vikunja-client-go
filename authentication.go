package vikunja

import "net/http"

// ApiTokenRequestInterceptor adds an authorization header to [http.Request]s with the configured
// API token.
type apiTokenRequestInterceptor struct {
	apiToken string
}

// Process adds an authorization header with the configured API token to a [http.Request].
func (h *apiTokenRequestInterceptor) process(req *http.Request) error {
	req.Header.Add("Authorization", "Bearer "+h.apiToken)
	return nil
}
