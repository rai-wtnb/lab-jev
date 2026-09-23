// 02 Choice: pick one option from a set you define.
//
// What to look for
//   - The answer is always one of your options (no parsing free text).
//   - probabilities shows the distribution across every option.
//   - An "other" option keeps inputs that fit nothing from being forced into a bucket.
//     Options that need no description can be nil (JSON null).
//
// https://docs.typesafe.ai/primitives/choice
package main

import (
	"github.com/rai-wtnb/lab-jev/internal/exutil"
	"github.com/rai-wtnb/lab-jev/typesafe"
)

func main() {
	c := exutil.Client()
	ctx, cancel := exutil.Ctx()
	defer cancel()

	department := typesafe.Choice("Which team should handle this?", map[string]any{
		"billing":   "Payments, invoicing, refunds",
		"technical": "Bugs, outages, integrations",
		"sales":     "Pricing, upgrades, new accounts",
		"other":     nil,
	})

	for _, msg := range []string{
		"Help! My payouts have been failing for 3 days.",
		"We're a 200-person team, what would an enterprise plan cost?",
		"The webhook for payment.succeeded stopped firing after your deploy.",
		"Do you have an office in Tokyo I could visit?",
	} {
		exutil.Title(msg)
		res, err := c.SystemOne(ctx, msg, map[string]typesafe.Question{"department": department})
		if err != nil {
			panic(err)
		}
		exutil.Response(res)
	}
}
