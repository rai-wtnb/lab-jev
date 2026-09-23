package typesafe

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Documented statuses: https://docs.typesafe.ai/api#errors
// net/http has no constant for 529, so define it here.
const StatusOverloaded = 529

// APIError is a non-2xx response.
type APIError struct {
	StatusCode int
	// Body is the raw error body. Its format is not documented.
	// For 422 it describes the offending field.
	Body json.RawMessage
	// Type and Message are set only when Body looks like {"detail":{"error_type":..., "message":...}}
	// (the shape observed on a real 401; other statuses may differ).
	Type       string
	Message    string
	RetryAfter time.Duration
}

func newAPIError(resp *http.Response, body []byte) *APIError {
	e := &APIError{
		StatusCode: resp.StatusCode,
		RetryAfter: parseRetryAfter(resp.Header.Get("Retry-After")),
	}
	if json.Valid(body) {
		e.Body = body
		var v struct {
			Detail struct {
				ErrorType string `json:"error_type"`
				Message   string `json:"message"`
			} `json:"detail"`
		}
		if json.Unmarshal(body, &v) == nil {
			e.Type, e.Message = v.Detail.ErrorType, v.Detail.Message
		}
	} else if len(body) > 0 {
		e.Body, _ = json.Marshal(string(body))
	}
	return e
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("typesafe: %d %s: %s: %s", e.StatusCode, statusText(e.StatusCode), e.Type, e.Message)
	}
	return fmt.Sprintf("typesafe: %d %s: %s", e.StatusCode, statusText(e.StatusCode), e.Body)
}

// Classification helpers.
func (e *APIError) IsUnauthorized() bool { return e.StatusCode == http.StatusUnauthorized }
func (e *APIError) IsValidation() bool   { return e.StatusCode == http.StatusUnprocessableEntity }
func (e *APIError) IsRateLimited() bool  { return e.StatusCode == http.StatusTooManyRequests }
func (e *APIError) IsOverloaded() bool   { return e.StatusCode == StatusOverloaded }

func statusText(code int) string {
	if code == StatusOverloaded {
		return "Overloaded"
	}
	return http.StatusText(code)
}

// ConnectionError means no response was received (connection failure, timeout, etc.).
type ConnectionError struct{ Err error }

func (e *ConnectionError) Error() string { return "typesafe: connection error: " + e.Err.Error() }
func (e *ConnectionError) Unwrap() error { return e.Err }
