// 05 Structured state and instructions.
//
// What to look for
//   - state can be a JSON object or array, not just a string (pass Go structs or maps directly).
//   - Writing a backticked path like `ticket.messages[0].text` in instructions tells the model
//     which part of the state the question is about.
//   - instructions can itself be an object, separating the question from the data it refers to.
//
// https://docs.typesafe.ai/concepts/state
// https://docs.typesafe.ai/primitives#reference-specific-fields
// https://docs.typesafe.ai/api#question-types
package main

import (
	"github.com/rai-wtnb/lab-jev/internal/exutil"
	"github.com/rai-wtnb/lab-jev/typesafe"
)

// Build the state as Go structs and send it as JSON as-is.
type Message struct {
	From string `json:"from"`
	Text string `json:"text"`
}

type Charge struct {
	AmountUSD int    `json:"amount_usd"`
	Status    string `json:"status"`
}

type SupportCase struct {
	Ticket struct {
		Subject  string    `json:"subject"`
		Messages []Message `json:"messages"`
	} `json:"ticket"`
	Order struct {
		ID      string   `json:"id"`
		Charges []Charge `json:"charges"`
	} `json:"order"`
	RefundPolicy string `json:"refund_policy"`
}

func main() {
	c := exutil.Client()
	ctx, cancel := exutil.Ctx()
	defer cancel()

	// --- 1. Object state + path references ------------------------------
	var sc SupportCase
	sc.Ticket.Subject = "Duplicate charge"
	sc.Ticket.Messages = []Message{
		{"customer", "I was charged twice for order A-104. Please refund the duplicate."},
		{"support", "We are checking the charges."},
	}
	sc.Order.ID = "A-104"
	sc.Order.Charges = []Charge{{49, "captured"}, {49, "captured"}}
	sc.RefundPolicy = "Duplicate charges are eligible for a refund."

	exutil.Title("Object state + `path` references")
	exutil.PrintJSON("state", sc)
	res, err := c.SystemOne(ctx, sc, map[string]typesafe.Question{
		"refund_requested": typesafe.Noul("Does `ticket.messages[0].text` request a refund?"),
		"policy_supports_refund": typesafe.Noul(
			"Does `refund_policy` support the refund requested in `ticket.messages[0].text`, given `order.charges`?"),
		// The same question pointed at the support agent's message should flip.
		"support_requested_refund": typesafe.Noul("Does `ticket.messages[1].text` request a refund?"),
	})
	if err != nil {
		panic(err)
	}
	exutil.PrintResponse(res)

	// --- 2. instructions as an object -----------------------------------
	exutil.Title("Structured instructions (question + reference data)")
	resume := "Jon Smith — Software Engineer at Google (2019–2024). Based in Oakland, CA. " +
		"Built distributed storage systems in Go and C++."
	res, err = c.SystemOne(ctx, resume, map[string]typesafe.Question{
		"same_person": typesafe.Noul(map[string]any{
			"potential_duplicate": map[string]string{
				"name":          "John Smith",
				"location":      "Oakland, California",
				"last_employer": "Google",
			},
			"question": "Is the resume for the same person as `potential_duplicate`?",
		}),
		"different_person": typesafe.Noul(map[string]any{
			"potential_duplicate": map[string]string{
				"name":          "John Smith",
				"location":      "Austin, Texas",
				"last_employer": "Dell",
			},
			"question": "Is the resume for the same person as `potential_duplicate`?",
		}),
	})
	if err != nil {
		panic(err)
	}
	exutil.PrintResponse(res)

	// --- 3. Array state -------------------------------------------------
	exutil.Title("Array state (chat log)")
	chat := []string{"Hi", "My customer number is TS1337.", "My card was charged twice."}
	res, err = c.SystemOne(ctx, chat, map[string]typesafe.Question{
		"has_customer_id": typesafe.Noul("Does the conversation include a customer number?"),
		"topic": typesafe.Choice("What is the conversation mainly about?", map[string]any{
			"billing": nil, "login": nil, "shipping": nil, "other": nil,
		}),
	})
	if err != nil {
		panic(err)
	}
	exutil.PrintResponse(res)
}
