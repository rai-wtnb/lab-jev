// 03 Score: rate where the state sits on an ordered set of levels (a rubric).
//
// What to look for
//   - score is the probability-weighted mean of level numbers (from 0), so it can land
//     between levels (e.g. 1.05).
//   - The most likely level (Answer.Level) may differ from the rounded score.
//   - NormalizedScore maps to 0..1 so Scores with different level counts are comparable.
//
// https://docs.typesafe.ai/primitives/score
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

	frustration := typesafe.Score("How frustrated is the customer?",
		"Calm, just stating facts",
		"Mildly annoyed",
		"Frustrated but civil",
		"Very angry, strong language",
	)

	for _, msg := range []string{
		"Hi, just letting you know the export button is greyed out for me.",
		"This is the second time this week the export has failed. Please fix it.",
		"Three tickets, zero answers. I am done waiting. Cancel my account NOW.",
	} {
		exutil.Title(msg)
		res, err := c.SystemOne(ctx, msg, map[string]typesafe.Question{"frustration": frustration})
		if err != nil {
			panic(err)
		}
		exutil.Response(res)
		a := res.Answers["frustration"]
		fmt.Printf("  → normalized %.2f\n", a.NormalizedScore())
	}
}
