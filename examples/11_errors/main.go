// 11 Error handling.
//
// What to look for
//   - 401: invalid API key.
//   - 422: request validation failed. The body names the offending field (the format is
//     undocumented, so look at the real thing).
//   - 429 / 529: the client retries with exponential backoff (not triggered here;
//     see the tests in typesafe/client_test.go).
//
// https://docs.typesafe.ai/api#errors
package main

import (
	"errors"
	"fmt"

	"github.com/rai-wtnb/lab-jev/internal/exutil"
	"github.com/rai-wtnb/lab-jev/typesafe"
)

func main() {
	ctx, cancel := exutil.Ctx()
	defer cancel()

	exutil.Title("401: invalid API key")
	bad := exutil.Client(typesafe.WithAPIKey("ts-invalid-key"))
	_, err := bad.SystemOne(ctx, "hello", map[string]typesafe.Question{"q": typesafe.Noul("Is this a greeting?")})
	explain(err)

	c := exutil.Client()
	for _, tc := range []struct {
		name string
		q    typesafe.Question
	}{
		{"422: Score with only 1 level (needs at least 2)", typesafe.Score("How good?", "Good")},
		{"422: Score with 11 levels (max 10)", typesafe.Score("Rate 0-10", "0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10")},
		{"422: Choice with empty criteria", typesafe.Choice("Pick one", map[string]any{})},
		{"422: invalid type", typesafe.Question{Type: "maybe", Instructions: "Is it?"}},
	} {
		exutil.Title(tc.name)
		_, err := c.SystemOne(ctx, "hello", map[string]typesafe.Question{"q": tc.q})
		explain(err)
	}

	exutil.Title("422?: model name that does not exist")
	_, err = c.Do(ctx, typesafe.Request{
		Model: "jev-does-not-exist", State: "hello",
		Questions: map[string]typesafe.Question{"q": typesafe.Noul("Is this a greeting?")},
	})
	explain(err)
}

func explain(err error) {
	if err == nil {
		fmt.Println("  no error (the API accepted it)")
		return
	}
	var apiErr *typesafe.APIError
	if !errors.As(err, &apiErr) {
		fmt.Println("  non-API error:", err)
		return
	}
	kind := "other"
	switch {
	case apiErr.IsUnauthorized():
		kind = "auth error → check the key"
	case apiErr.IsValidation():
		kind = "validation error → fix the request (retrying won't help)"
	case apiErr.IsRateLimited(), apiErr.IsOverloaded():
		kind = "transient → back off and retry"
	}
	fmt.Printf("  status=%d (%s)\n", apiErr.StatusCode, kind)
	if apiErr.Type != "" || apiErr.Message != "" {
		fmt.Printf("  error_type=%q message=%q\n", apiErr.Type, apiErr.Message)
	}
	fmt.Printf("  body=%s\n", apiErr.Body)
}
