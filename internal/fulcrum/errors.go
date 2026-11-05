// Decodes Core's structured API errors without discarding field-level details.
// HTTP status remains available even when a proxy returns a non-JSON body.
package fulcrum

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type APIError struct {
	Status   int            `json:"status"`
	Message  string         `json:"message"`
	Template string         `json:"template,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
	Errors   []ErrorDetail  `json:"errors,omitempty"`
}

type ErrorDetail struct {
	Path     string         `json:"path"`
	Message  string         `json:"message"`
	Template string         `json:"template,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

func (e *APIError) Error() string {
	message := fmt.Sprintf("HTTP %d: %s", e.Status, e.Message)
	for _, detail := range e.Errors {
		message += fmt.Sprintf("; %s: %s", detail.Path, detail.Message)
	}
	return message
}

func readAPIError(resp *http.Response) *APIError {
	var apiErr APIError
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&apiErr); err != nil {
		apiErr = APIError{}
	}
	apiErr.Status = resp.StatusCode
	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(resp.StatusCode)
	}
	return &apiErr
}
