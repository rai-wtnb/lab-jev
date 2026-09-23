// 07 Confidence-gated routing: choice says what to do; confidence says whether to act.
//
// What to look for
//   - Ambiguous utterances get lower confidence and are routed to a human.
//   - Riskier actions (approving a transfer) require a higher threshold. Thresholds live in code.
//
// Thresholds match the docs example (0.6 / 0.85).
// https://docs.typesafe.ai/patterns/confidence-routing
// https://docs.typesafe.ai/confidence
package main

import (
	"fmt"

	"github.com/rai-wtnb/lab-jev/internal/exutil"
	"github.com/rai-wtnb/lab-jev/typesafe"
)

const (
	floor        = 0.6  // below this, any action goes to a human
	autoTransfer = 0.85 // threshold to approve a transfer automatically
)

func route(a typesafe.Answer) string {
	switch {
	case a.Confidence < floor:
		return "→ route to a support agent (not confident)"
	case a.Choice == "check_balance":
		return "→ read out the balance (low stakes)"
	case a.Choice == "approve_transfer" && a.Confidence > autoTransfer:
		return "→ approve the transfer (high stakes, high confidence)"
	case a.Choice == "approve_transfer":
		return "→ ask \"Approve this transfer?\" (high stakes, moderate confidence)"
	default:
		return "→ route to a support agent (other)"
	}
}

func main() {
	c := exutil.Client()
	ctx, cancel := exutil.Ctx()
	defer cancel()

	intent := typesafe.Choice("What action is the user requesting?", map[string]any{
		"check_balance":    "Check the balance of an account",
		"approve_transfer": "Approve the pending transfer request",
		"other":            "Something else",
	})

	for _, utterance := range []string{
		"What's my checking account balance?",
		"Yes, approve the pending transfer to my landlord.",
		"Uh, the transfer thing... yeah I guess, whatever it is.",
		"How much do I have, and actually, just approve that thing too.",
		"I want to report a lost card.",
	} {
		res, err := c.SystemOne(ctx, utterance, map[string]typesafe.Question{"intent": intent})
		if err != nil {
			panic(err)
		}
		a := res.Answers["intent"]
		fmt.Printf("\n%q\n  choice=%-16s confidence=%.2f %s\n  %s\n",
			utterance, a.Choice, a.Confidence, exutil.Bar(a.Confidence), route(a))
	}
}
