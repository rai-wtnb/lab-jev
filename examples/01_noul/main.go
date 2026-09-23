// 01 Noul: a yes/no question. The answer is the probability of yes (0 to 1).
//
// What to look for
//   - The answer is a probability, not true/false. Around 0.5 means "can't tell".
//   - How borderline cases shift when criteria define what yes and no mean.
//
// https://docs.typesafe.ai/primitives/noul
package main

import (
	"fmt"

	"github.com/rai-wtnb/lab-jev/internal/exutil"
	"github.com/rai-wtnb/lab-jev/typesafe"
)

func main() {
	c := exutil.Client()
	ctx, cancel := exutil.Ctx()
	defer cancel()

	messages := []string{
		"Help! My payouts have been failing for 3 days.",
		"Whenever you get a chance, could you update my billing address?",
		"Our checkout is down and we're losing sales every minute.",
		"I'd like to eventually move to the annual plan, but no rush.",
		"Can someone look at this today? Not critical, but it's blocking a report I owe my boss on Friday.",
	}

	questions := map[string]typesafe.Question{
		// No criteria: the model judges from instructions alone.
		"urgent_plain": typesafe.Noul("Does this message convey urgency?"),
		// With criteria: spell out what yes and no mean.
		"urgent_defined": typesafe.NoulWith(
			"Does this message convey urgency?",
			"The customer needs action within hours, or business is currently impacted",
			"The request can wait a day or more without harm",
		),
	}

	exutil.Title("Noul: without vs with criteria")
	fmt.Printf("%-8s %-8s  message\n", "plain", "defined")
	for _, m := range messages {
		res, err := c.SystemOne(ctx, m, questions)
		if err != nil {
			panic(err)
		}
		a, b := res.Answers["urgent_plain"].Noul, res.Answers["urgent_defined"].Noul
		fmt.Printf("%.3f    %.3f     %s\n", a, b, m)
	}
}
