// 12 Language differences: send the same content in English and Japanese and compare.
//
// The docs say English is the primary training language; other languages, including CJK,
// are accepted but with lower accuracy. Check how confidence shifts on your own data
// before relying on Japanese input. (The Japanese strings below are test data.)
//
// https://docs.typesafe.ai/models#language-support
package main

import (
	"fmt"

	"github.com/rai-wtnb/lab-jev/internal/exutil"
	"github.com/rai-wtnb/lab-jev/typesafe"
)

type pair struct{ en, ja string }

func main() {
	c := exutil.Client()
	ctx, cancel := exutil.Ctx()
	defer cancel()

	// Compare switching only the state's language vs. also writing the questions in Japanese.
	qEN := map[string]typesafe.Question{
		"department": typesafe.Choice("Which team should handle this?", map[string]any{
			"billing": "Payments, invoicing, refunds", "technical": "Bugs, outages, integrations",
			"sales": "Pricing, upgrades, new accounts",
		}),
		"is_urgent": typesafe.Noul("Does this message convey urgency?"),
	}
	qJA := map[string]typesafe.Question{
		"department": typesafe.Choice("どのチームが対応すべきか？", map[string]any{
			"billing": "支払い、請求書、返金", "technical": "バグ、障害、連携",
			"sales": "料金、アップグレード、新規契約",
		}),
		"is_urgent": typesafe.Noul("このメッセージは緊急性を伝えているか？"),
	}

	for _, p := range []pair{
		{"Help! My payouts have been failing for 3 days.", "助けてください！3日前から入金が失敗し続けています。"},
		{"What would an enterprise plan cost for 200 seats?", "200 席分のエンタープライズプランはいくらになりますか？"},
		{"The webhook stopped firing after your deploy.", "御社のデプロイ以降、Webhook が発火しなくなりました。"},
		{"I'd like to change the name on my invoice, no rush.", "請求書の宛名を変更したいのですが、急ぎではありません。"},
	} {
		exutil.Title(p.en)
		fmt.Printf("  %-24s %-12s %-6s %s\n", "", "department", "conf", "is_urgent")
		for _, v := range []struct {
			label string
			state string
			q     map[string]typesafe.Question
		}{
			{"EN state / EN questions", p.en, qEN},
			{"JA state / EN questions", p.ja, qEN},
			{"JA state / JA questions", p.ja, qJA},
		} {
			res, err := c.SystemOne(ctx, v.state, v.q)
			if err != nil {
				panic(err)
			}
			d, u := res.Answers["department"], res.Answers["is_urgent"]
			fmt.Printf("  %-24s %-12s %.2f   %.3f  (in=%d tok)\n", v.label, d.Choice, d.Confidence, u.Noul, res.Usage.InputTokens)
		}
	}
}
