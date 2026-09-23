// 08 Speculative fan-out: ask every question you might need in one request; let code decide what matters.
//
// What to look for
//   - Instead of two round trips (category, then severity if it's a bug), ask both up front.
//   - If it isn't a bug report, bug_severity is simply ignored. Extra questions are nearly free.
//
// https://docs.typesafe.ai/patterns/fan-out
package main

import (
	"fmt"

	"github.com/rai-wtnb/lab-jev/internal/exutil"
	"github.com/rai-wtnb/lab-jev/typesafe"
)

var questions = map[string]typesafe.Question{
	"category": typesafe.Choice("Determine the broad category of this support ticket", map[string]any{
		"bug_report":      "The user is reporting something that is broken or producing errors",
		"billing":         "Charges, invoices, refunds, subscriptions",
		"feature_request": "The user is requesting new functionality",
		"account":         "Login, permissions, profile, security",
	}),
	"bug_severity": typesafe.Score("How severe is the reported issue",
		"Cosmetic; no impact to functionality",
		"Broken or degraded feature; workaround exists",
		"Blocking issue; no workaround exists",
	),
	"has_repro_steps":  typesafe.Noul("Does the ticket include steps to reproduce the problem?"),
	"refund_requested": typesafe.Noul("Does the customer ask for a refund or reimbursement?"),
	"frustration": typesafe.Score("How frustrated the customer appears",
		"Calm", "Frustrated but civil", "Very angry",
	),
}

// triage combines the answers into a next action. Irrelevant answers are never read.
func triage(ans map[string]typesafe.Answer) string {
	cat := ans["category"]
	switch cat.Choice {
	case "bug_report":
		sev := ans["bug_severity"]
		repro := ans["has_repro_steps"].Noul > 0.5
		if sev.Score >= 1.5 {
			return fmt.Sprintf("escalate to on-call as P1 (severity=%.2f, repro=%v)", sev.Score, repro)
		}
		return fmt.Sprintf("add to backlog (severity=%.2f, repro=%v)", sev.Score, repro)
	case "billing":
		if ans["refund_requested"].Noul > 0.5 {
			return "send to refund queue"
		}
		return "send to billing team"
	case "feature_request":
		return "log in product feature requests"
	default:
		return "send to account support"
	}
}

func main() {
	c := exutil.Client()
	ctx, cancel := exutil.Ctx()
	defer cancel()

	for _, ticket := range []string{
		"Hi, I placed an order (#98423) last Thursday and was charged twice. I also can't log in after the site update, " +
			"and adding Apple Pay would be really helpful. This is getting frustrating.",
		"Since v2.3 the dashboard crashes on load. Steps: 1) log in 2) click Reports 3) white screen. " +
			"Nobody on our team can work.",
		"The logo on the invoice PDF is slightly blurry.",
		"Would love a dark mode!",
	} {
		exutil.Title(ticket)
		res := exutil.Timed("latency", func() (*typesafe.Response, error) { return c.SystemOne(ctx, ticket, questions) })
		exutil.PrintResponse(res)
		fmt.Printf("  ⇒ %s\n", triage(res.Answers))
	}
}
