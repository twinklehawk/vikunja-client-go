package vikunja

import "net/http"

// apiTokenRequestInterceptor adds an authorization header to [http.Request]s with the configured
// API token.
type apiTokenRequestInterceptor struct {
	apiToken string
}

// process adds an authorization header with the configured API token to a [http.Request].
func (h *apiTokenRequestInterceptor) process(req *http.Request) error {
	req.Header.Set("Authorization", "Bearer "+h.apiToken)
	return nil
}
