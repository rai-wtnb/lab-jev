// 06 GET /v1/models: list the model names your account can use.
//
// What to look for
//   - The list contains aliases (jev-latest / jev-preview).
//   - The version that actually answered appears in the systemone response's model field.
//     Once you tune thresholds, pin a versioned ID (TYPESAFE_MODEL=jev-1.13.0).
//
// https://docs.typesafe.ai/models
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

	exutil.Title("GET /v1/models")
	models, err := c.Models(ctx)
	if err != nil {
		panic(err)
	}
	for _, m := range models {
		fmt.Printf("  %-14s %-12s %s\n", m.Name, m.ReleaseDate, m.Description)
	}

	exutil.Title("What each alias resolves to (response model)")
	for _, m := range models {
		res, err := c.Do(ctx, typesafe.Request{
			Model:     m.Name,
			State:     "ping",
			Questions: map[string]typesafe.Question{"q": typesafe.Noul("Is this a greeting?")},
		})
		if err != nil {
			fmt.Printf("  %-14s error: %v\n", m.Name, err)
			continue
		}
		fmt.Printf("  %-14s → %s\n", m.Name, res.Model)
	}
}
