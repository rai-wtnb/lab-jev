// 09 Composite scoring: split a complex judgment into independent Scores and weight them in code.
//
// What to look for
//   - Instead of asking "is this candidate good?", score each dimension separately.
//   - Weights live in code, so the same answers rank differently per role (IC / manager).
//   - Each state (resume) is its own request, so they are sent concurrently with goroutines.
//
// https://docs.typesafe.ai/patterns/composite-scoring
package main

import (
	"cmp"
	"fmt"
	"slices"
	"sync"

	"github.com/rai-wtnb/lab-jev/internal/exutil"
	"github.com/rai-wtnb/lab-jev/typesafe"
)

var questions = map[string]typesafe.Question{
	"python_depth": typesafe.Score(
		"How much depth of python experience does this candidate have, based on the supplied resume?",
		"No Python experience mentioned",
		"Mentioned but no detail",
		"Used in projects, some specifics",
		"Primary language, multiple projects",
		"Deep expertise: architecture, performance, libraries",
	),
	"team_leadership": typesafe.Score(
		"How much experience does this candidate have managing or leading engineering teams?",
		"No management experience mentioned",
		"Informal mentorship or tech lead role",
		"Led a small team or project",
		"Managed a team with direct reports",
		"Managed multiple teams or an engineering org",
	),
	"system_design": typesafe.Score(
		"How much experience does this candidate have designing large-scale or distributed systems?",
		"No architecture work mentioned",
		"Contributed to design discussions",
		"Designed components of a larger system",
		"Owned architecture of a significant system",
		"Designed systems at very large scale across many teams",
	),
	"generalist": typesafe.Score(
		"How broad is this candidate's experience across different areas of software engineering?",
		"Single narrow specialty",
		"Two related areas",
		"Several areas of the stack",
		"Worked across most of the stack",
		"Broad experience across stack, product, and operations",
	),
}

// Weights sum to 1; same values as the docs example.
var profiles = map[string]map[string]float64{
	"senior IC":   {"python_depth": 0.40, "team_leadership": 0.10, "system_design": 0.40, "generalist": 0.10},
	"eng manager": {"python_depth": 0.15, "team_leadership": 0.40, "system_design": 0.20, "generalist": 0.25},
}

var resumes = map[string]string{
	"Aiko": "Staff engineer, 9 years. Python is my primary language: I maintain an open-source async ORM, " +
		"profiled and cut p99 latency of our ML serving stack by 60%. Designed the event-sourcing backbone " +
		"used by 30 services. Mentor two juniors informally.",
	"Ben": "Engineering manager for 3 teams (22 engineers). Previously tech lead on payments. " +
		"Hands-on mostly in Java; occasional Python scripting. Ran hiring, performance reviews, and roadmap planning.",
	"Chen": "Full-stack developer, 4 years. React, Node, some Python/Django, Terraform, on-call for production. " +
		"Built our company's first CI pipeline and a small analytics dashboard.",
}

func main() {
	c := exutil.Client()
	ctx, cancel := exutil.Ctx()
	defer cancel()

	// One request per resume, sent concurrently (rate limit: 1,200 req/min).
	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		answers = map[string]map[string]typesafe.Answer{}
	)
	for name, resume := range resumes {
		wg.Go(func() {
			res, err := c.SystemOne(ctx, resume, questions)
			if err != nil {
				panic(err)
			}
			mu.Lock()
			answers[name] = res.Answers
			mu.Unlock()
		})
	}
	wg.Wait()

	exutil.Title("Normalized score per dimension (0..1)")
	dims := []string{"python_depth", "team_leadership", "system_design", "generalist"}
	fmt.Printf("  %-6s", "")
	for _, d := range dims {
		fmt.Printf(" %16s", d)
	}
	fmt.Println()
	names := []string{"Aiko", "Ben", "Chen"}
	for _, n := range names {
		fmt.Printf("  %-6s", n)
		for _, d := range dims {
			a := answers[n][d]
			fmt.Printf("  %.2f (conf %.2f)", a.NormalizedScore(), a.Confidence)
		}
		fmt.Println()
	}

	for _, p := range []string{"senior IC", "eng manager"} {
		exutil.Title("Ranking: " + p)
		type row struct {
			name  string
			total float64
		}
		var rows []row
		for _, n := range names {
			var total float64
			for d, w := range profiles[p] {
				total += w * answers[n][d].NormalizedScore()
			}
			rows = append(rows, row{n, total})
		}
		slices.SortFunc(rows, func(a, b row) int { return cmp.Compare(b.total, a.total) })
		for i, r := range rows {
			fmt.Printf("  %d. %-6s %.3f %s\n", i+1, r.name, r.total, exutil.Bar(r.total))
		}
	}
}
