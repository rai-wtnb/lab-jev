package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	DefaultBaseURL = "https://api.typesafe.ai"
	// APIKeyEnv is the same variable the official SDKs read.
	APIKeyEnv = "TYPESAFE_API_KEY"
)

// RetryPolicy configures retries. Defaults mirror the Python SDK's RetryPolicy.
// https://docs.typesafe.ai/sdk/python/api/retries
type RetryPolicy struct {
	// MaxRetries is the number of retries after the initial attempt; 0 disables retries.
	MaxRetries int
	// Backoff doubles from BackoffInitial, capped at BackoffMax.
	BackoffInitial time.Duration
	BackoffMax     time.Duration
	// BackoffJitter is the random spread applied to each wait (0.25 means ±25%).
	BackoffJitter float64
	// RetryStatus reports whether a status code should be retried.
	RetryStatus func(status int) bool
	// RespectRetryAfter makes the Retry-After header take precedence over backoff.
	RespectRetryAfter bool
	// RetryNetworkErrors also retries connection errors and timeouts.
	RetryNetworkErrors bool
}

// DefaultRetryPolicy: max_retries=2, backoff 0.5s..5s, jitter 0.25,
// retrying statuses {408, 429, 500-599} (including 529 Overloaded).
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:     2,
		BackoffInitial: 500 * time.Millisecond,
		BackoffMax:     5 * time.Second,
		BackoffJitter:  0.25,
		RetryStatus: func(s int) bool {
			return s == http.StatusRequestTimeout || s == http.StatusTooManyRequests || (s >= 500 && s < 600)
		},
		RespectRetryAfter:  true,
		RetryNetworkErrors: true,
	}
}

// Client is a TypeSafe API client. It is safe for concurrent use.
type Client struct {
	apiKey  string
	baseURL string
	model   string
	http    *http.Client
	retry   RetryPolicy
	// OnRetry, if set, is called before each retry (useful for logging).
	OnRetry func(attempt int, wait time.Duration, err error)
}

// Option configures a Client in New.
type Option func(*Client)

func WithAPIKey(key string) Option         { return func(c *Client) { c.apiKey = key } }
func WithBaseURL(u string) Option          { return func(c *Client) { c.baseURL = u } }
func WithModel(m string) Option            { return func(c *Client) { c.model = m } }
func WithRetryPolicy(p RetryPolicy) Option { return func(c *Client) { c.retry = p } }

// WithHTTPClient replaces the HTTP client (to tune timeouts or transport).
// The default times out after 30 seconds per attempt.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.http = h } }

func WithOnRetry(f func(int, time.Duration, error)) Option {
	return func(c *Client) { c.OnRetry = f }
}

// New creates a client. The API key defaults to the TYPESAFE_API_KEY environment variable.
func New(opts ...Option) (*Client, error) {
	c := &Client{
		apiKey:  os.Getenv(APIKeyEnv),
		baseURL: DefaultBaseURL,
		model:   DefaultModel,
		http:    &http.Client{Timeout: 30 * time.Second},
		retry:   DefaultRetryPolicy(),
	}
	for _, o := range opts {
		o(c)
	}
	if c.apiKey == "" {
		return nil, fmt.Errorf("typesafe: missing API key (set the %s environment variable)", APIKeyEnv)
	}
	return c, nil
}

// SystemOne evaluates questions against state (POST /v1/systemone).
// Every question sees the same state and is evaluated in parallel, independently.
func (c *Client) SystemOne(ctx context.Context, state any, questions map[string]Question) (*Response, error) {
	return c.Do(ctx, Request{State: state, Questions: questions})
}

// Do sends req as-is, using the Client's default model when Model is empty.
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	if req.Model == "" {
		req.Model = c.model
	}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("typesafe: encoding request: %w", err)
	}
	var res Response
	if err := c.send(ctx, http.MethodPost, "/v1/systemone", body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// Models lists the model names your account can use, currently aliases (GET /v1/models).
func (c *Client) Models(ctx context.Context) ([]Model, error) {
	var res modelsResponse
	if err := c.send(ctx, http.MethodGet, "/v1/models", nil, &res); err != nil {
		return nil, err
	}
	return res.Models, nil
}

// send performs the request with retries and decodes a successful body into out.
func (c *Client) send(ctx context.Context, method, path string, body []byte, out any) error {
	for attempt := 0; ; attempt++ {
		err := c.sendOnce(ctx, method, path, body, out)
		if err == nil {
			return nil
		}
		wait, retryable := c.shouldRetry(err, attempt)
		if !retryable {
			return err
		}
		if c.OnRetry != nil {
			c.OnRetry(attempt+1, wait, err)
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return errors.Join(ctx.Err(), err)
		case <-t.C:
		}
	}
}

func (c *Client) sendOnce(ctx context.Context, method, path string, body []byte, out any) error {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return &ConnectionError{Err: err}
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ConnectionError{Err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return newAPIError(resp, data)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("typesafe: decoding response: %w", err)
	}
	return nil
}

func (c *Client) shouldRetry(err error, attempt int) (time.Duration, bool) {
	p := c.retry
	if attempt >= p.MaxRetries {
		return 0, false
	}
	var apiErr *APIError
	var connErr *ConnectionError
	switch {
	case errors.As(err, &apiErr):
		if p.RetryStatus == nil || !p.RetryStatus(apiErr.StatusCode) {
			return 0, false
		}
		if p.RespectRetryAfter && apiErr.RetryAfter > 0 {
			return apiErr.RetryAfter, true
		}
	case errors.As(err, &connErr):
		if !p.RetryNetworkErrors {
			return 0, false
		}
	default:
		return 0, false
	}
	return p.backoff(attempt), true
}

func (p RetryPolicy) backoff(attempt int) time.Duration {
	d := min(p.BackoffInitial<<attempt, p.BackoffMax)
	if p.BackoffJitter > 0 {
		d = time.Duration(float64(d) * (1 + p.BackoffJitter*(2*rand.Float64()-1)))
	}
	return d
}

// parseRetryAfter parses a Retry-After header (seconds or an HTTP date).
func parseRetryAfter(h string) time.Duration {
	if h == "" {
		return 0
	}
	if s, err := strconv.ParseFloat(h, 64); err == nil && s > 0 {
		return time.Duration(s * float64(time.Second))
	}
	if t, err := http.ParseTime(h); err == nil {
		return max(time.Until(t), 0)
	}
	return 0
}
