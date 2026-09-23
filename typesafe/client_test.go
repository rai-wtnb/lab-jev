package typesafe

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	fast := DefaultRetryPolicy()
	fast.BackoffInitial = time.Millisecond
	fast.BackoffMax = 5 * time.Millisecond
	c, err := New(append([]Option{WithAPIKey("test-key"), WithBaseURL(srv.URL), WithRetryPolicy(fast)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// Requests match the shape of the documented examples (https://docs.typesafe.ai/api).
func TestRequestShape(t *testing.T) {
	var got map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/systemone" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if h := r.Header.Get("Authorization"); h != "Bearer test-key" {
			t.Errorf("Authorization = %q", h)
		}
		b, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		io.WriteString(w, `{"model":"jev-1.13.0","answers":{},"usage":{"input_tokens":1,"output_tokens":1}}`)
	})

	_, err := c.SystemOne(context.Background(), "Help! My payouts have been failing for 3 days.", map[string]Question{
		"is_urgent":   NoulWith("Does this convey urgency?", "Explicitly time-sensitive", "No urgency expressed"),
		"department":  Choice("Which team should handle this?", map[string]any{"billing": "Payments", "other": nil}),
		"frustration": Score("How frustrated is the customer?", "Calm", "Frustrated", "Very angry"),
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `{
	  "model": "jev-latest",
	  "state": "Help! My payouts have been failing for 3 days.",
	  "questions": {
	    "is_urgent":   {"type": "noul", "instructions": "Does this convey urgency?",
	                    "criteria": {"true": "Explicitly time-sensitive", "false": "No urgency expressed"}},
	    "department":  {"type": "choice", "instructions": "Which team should handle this?",
	                    "criteria": {"billing": "Payments", "other": null}},
	    "frustration": {"type": "score", "instructions": "How frustrated is the customer?",
	                    "criteria": ["Calm", "Frustrated", "Very angry"]}
	  }
	}`
	var wantMap map[string]any
	if err := json.Unmarshal([]byte(want), &wantMap); err != nil {
		t.Fatal(err)
	}
	gj, _ := json.Marshal(got)
	wj, _ := json.Marshal(wantMap)
	if string(gj) != string(wj) {
		t.Errorf("request body mismatch\n got: %s\nwant: %s", gj, wj)
	}

	// A Noul without criteria omits the criteria key entirely.
	b, _ := json.Marshal(Noul("q"))
	if string(b) != `{"type":"noul","instructions":"q"}` {
		t.Errorf("Noul without criteria = %s", b)
	}
}

// The documented example responses decode correctly.
func TestDecodeAnswers(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{
		  "model": "jev-1.13.0",
		  "answers": {
		    "is_urgent": {"type": "noul", "noul": 0.95},
		    "department": {"type": "choice", "choice": "billing",
		      "probabilities": {"billing": 0.88, "technical": 0.12, "sales": 0.0}, "confidence": 0.81},
		    "frustration": {"type": "score", "score": 1.05,
		      "legend": {"0": "Calm", "1": "Frustrated", "2": "Very angry"},
		      "probabilities": {"0": 0.0, "1": 0.95, "2": 0.05}, "confidence": 0.92}
		  },
		  "usage": {"input_tokens": 318, "output_tokens": 34}
		}`)
	})
	res, err := c.SystemOne(context.Background(), "x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Model != "jev-1.13.0" || res.Usage.InputTokens != 318 {
		t.Errorf("model/usage = %q %+v", res.Model, res.Usage)
	}
	if a := res.Answers["is_urgent"]; a.Type != TypeNoul || a.Noul != 0.95 {
		t.Errorf("noul = %+v", a)
	}
	d := res.Answers["department"]
	if d.Choice != "billing" || d.Confidence != 0.81 {
		t.Errorf("choice = %+v", d)
	}
	if r := d.Ranked(); r[0].Key != "billing" || r[1].Key != "technical" || r[2].Key != "sales" {
		t.Errorf("ranked = %+v", r)
	}
	f := res.Answers["frustration"]
	if n, s := f.Level(); n != 1 || s != "Frustrated" {
		t.Errorf("level = %d %q", n, s)
	}
	if got := f.NormalizedScore(); got != 0.525 {
		t.Errorf("normalized = %v", got)
	}
}

func TestRetryOn429ThenSuccess(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.Header().Set("Retry-After", "0.001")
			w.WriteHeader(http.StatusTooManyRequests)
			io.WriteString(w, `{"error":"rate limited"}`)
			return
		}
		io.WriteString(w, `{"model":"m","answers":{},"usage":{}}`)
	})
	var retries int
	c.OnRetry = func(int, time.Duration, error) { retries++ }
	if _, err := c.SystemOne(context.Background(), "x", nil); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 3 || retries != 2 {
		t.Errorf("calls=%d retries=%d", calls.Load(), retries)
	}
}

func TestRetryGivesUpAfterMax(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(StatusOverloaded)
	})
	_, err := c.SystemOne(context.Background(), "x", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.IsOverloaded() {
		t.Fatalf("err = %v", err)
	}
	if calls.Load() != 3 { // initial attempt + MaxRetries(2)
		t.Errorf("calls = %d", calls.Load())
	}
}

func TestNoRetryOn422(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, `{"detail":[{"loc":["body","questions"],"msg":"field required"}]}`)
	})
	_, err := c.SystemOne(context.Background(), "x", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.IsValidation() {
		t.Fatalf("err = %v", err)
	}
	if calls.Load() != 1 {
		t.Errorf("calls = %d", calls.Load())
	}
}

func TestModels(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		io.WriteString(w, `{"models":[{"name":"jev-latest","description":"d","release_date":"2026-01-01"}]}`)
	})
	ms, err := c.Models(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) != 1 || ms[0].Name != "jev-latest" {
		t.Errorf("models = %+v", ms)
	}
}

func TestMissingAPIKey(t *testing.T) {
	t.Setenv(APIKeyEnv, "")
	if _, err := New(); err == nil {
		t.Error("expected error without API key")
	}
}

func TestBackoff(t *testing.T) {
	p := DefaultRetryPolicy()
	p.BackoffJitter = 0
	for i, want := range []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second} {
		if got := p.backoff(i); got != want {
			t.Errorf("backoff(%d) = %v, want %v", i, got, want)
		}
	}
}

// A 401 has the shape observed on the real API: {"detail":{"error_type":..., "message":...}}.
func TestAPIErrorDetail(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"detail":{"error_type":"authentication_error","message":"Cannot authenticate with the server."}}`)
	})
	_, err := c.SystemOne(context.Background(), "x", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || !apiErr.IsUnauthorized() {
		t.Fatalf("err = %v", err)
	}
	if apiErr.Type != "authentication_error" || apiErr.Message != "Cannot authenticate with the server." {
		t.Errorf("type=%q message=%q", apiErr.Type, apiErr.Message)
	}
}
