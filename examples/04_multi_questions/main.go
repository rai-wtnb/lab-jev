// 04 Send several questions in one request.
//
// What to look for
//   - Choice / Noul / Score can be mixed in one request; answers come back under your IDs.
//   - Questions are evaluated in parallel, so adding questions supposedly barely changes
//     latency. Compare latency and input tokens for 1 vs 12 questions.
//     (Billing is per input token; output tokens are free: https://docs.typesafe.ai/models)
//
// https://docs.typesafe.ai/primitives#ask-multiple-questions-together
package main

import (
	"fmt"
	"maps"
	"time"

	"github.com/rai-wtnb/lab-jev/internal/exutil"
	"github.com/rai-wtnb/lab-jev/typesafe"
)

const state = "Our API integration started returning 500 errors on every request about 20 minutes ago, " +
	"and we can't process any customer orders until this is fixed."

func main() {
	c := exutil.Client()
	ctx, cancel := exutil.Ctx()
	defer cancel()

	// The same three questions as the docs example.
	questions := map[string]typesafe.Question{
		"department": typesafe.Choice("Which team should handle this", map[string]any{
			"billing":   "Payment or subscription issues",
			"technical": "Bugs or integration problems",
			"sales":     "Pricing or account questions",
		}),
		"is_urgent": typesafe.Noul("The message conveys urgency or time-sensitivity"),
		"frustration": typesafe.Score("How frustrated the customer appears",
			"Calm, just stating facts",
			"Frustrated but civil",
			"Very angry, strong language",
		),
	}

	exutil.Title("Three question types in one request")
	res := exutil.Timed("latency", func() (*typesafe.Response, error) { return c.SystemOne(ctx, state, questions) })
	exutil.Response(res)

	// Nine extra Nouls (12 questions total).
	more := map[string]string{
		"mentions_api":        "Does the message mention an API?",
		"mentions_error_code": "Does the message mention a specific HTTP error code?",
		"revenue_impact":      "Is the customer's revenue currently impacted?",
		"asks_refund":         "Does the customer ask for a refund?",
		"mentions_duration":   "Does the message say how long the problem has lasted?",
		"is_feature_request":  "Is this a feature request?",
		"is_security_issue":   "Does this describe a security incident?",
		"is_polite":           "Is the message polite?",
		"asks_callback":       "Does the customer ask for a phone call?",
	}
	many := maps.Clone(questions)
	for k, v := range more {
		many[k] = typesafe.Noul(v)
	}

	exutil.Title("Latency: 1 vs 12 questions (3 runs each)")
	one := map[string]typesafe.Question{"is_urgent": questions["is_urgent"]}
	for _, tc := range []struct {
		name string
		q    map[string]typesafe.Question
	}{{"1 q", one}, {"12 q", many}} {
		var total time.Duration
		var in int
		for range 3 {
			start := time.Now()
			r, err := c.SystemOne(ctx, state, tc.q)
			if err != nil {
				panic(err)
			}
			total += time.Since(start)
			in = r.Usage.InputTokens
		}
		fmt.Printf("  %-5s avg %v  input_tokens=%d\n", tc.name, (total / 3).Round(time.Millisecond), in)
	}

	exutil.Title("Answers to all 12 questions")
	res = exutil.Timed("latency", func() (*typesafe.Response, error) { return c.SystemOne(ctx, state, many) })
	exutil.Response(res)
}
