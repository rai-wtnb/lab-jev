// 10 Intent routing: use cheap, fast Jev as a front classifier to pick a handler
// (deterministic code / specialist LLM / human).
//
// What to look for
//   - Ask intent (Choice) and complexity (Score) together, and route on both confidences.
//   - Expensive LLMs are only called when actually needed.
//
// https://docs.typesafe.ai/patterns/intent-routing
package main

import (
	"fmt"

	"github.com/rai-wtnb/lab-jev/internal/exutil"
	"github.com/rai-wtnb/lab-jev/typesafe"
)

var questions = map[string]typesafe.Question{
	"intent": typesafe.Choice("The primary intent of this customer message", map[string]any{
		"order_status":     "Asking about an existing order",
		"product_question": "Asking about a product before buying",
		"return_exchange":  "Wants to return or exchange something",
		"complaint":        "Unhappy with experience, wants resolution",
	}),
	"complexity": typesafe.Score("How complex is this request to resolve",
		"Simple lookup or standard procedure",
		"Requires some judgment or multi-step process",
		"Unusual situation, edge case, or escalation needed",
	),
}

// route follows the same branches as the flowchart in the docs.
func route(intent, complexity typesafe.Answer) string {
	if intent.Confidence < 0.5 {
		return "human agent (ambiguous intent)"
	}
	switch intent.Choice {
	case "order_status":
		return "order lookup (deterministic code, no LLM)"
	case "product_question":
		return "product specialist LLM"
	case "return_exchange":
		return "returns specialist LLM"
	case "complaint":
		if complexity.Score > 1 || complexity.Confidence < 0.5 {
			return "human agent (complex complaint)"
		}
		return "complaint resolution LLM"
	}
	return "human agent"
}

func main() {
	c := exutil.Client()
	ctx, cancel := exutil.Ctx()
	defer cancel()

	for _, msg := range []string{
		"Where is my order #5521? It said 2-day shipping.",
		"Does the X200 blender work with 220V outlets?",
		"The jacket is too small, can I swap it for a large?",
		"Your courier left my package in the rain and the electronics inside are ruined. " +
			"I was also charged customs fees that your site said were included. I want this sorted.",
		"The item arrived a day late. Not great.",
	} {
		res, err := c.SystemOne(ctx, msg, questions)
		if err != nil {
			panic(err)
		}
		i, cx := res.Answers["intent"], res.Answers["complexity"]
		fmt.Printf("\n%q\n  intent=%s (conf %.2f)  complexity=%.2f (conf %.2f)\n  ⇒ %s\n",
			msg, i.Choice, i.Confidence, cx.Score, cx.Confidence, route(i, cx))
	}
}
