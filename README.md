# lab-jev

Sample projects for getting a feel for TypeSafe's **Jev** (a System One model).
There is no official Go SDK, so these call the [HTTP API](https://docs.typesafe.ai/api) directly using only the standard library (no external dependencies).

- Docs: https://docs.typesafe.ai/introduction
- Tooling: [mise](https://mise.jdx.dev/) (Go pinned to 1.27.1; see `mise.toml`)

## Setup

```bash
mise install                 # install Go
cp .env.example .env         # add TYPESAFE_API_KEY (mise loads .env automatically)
mise run test                # client unit tests (no API key needed)
```

## Examples

| Task | Directory | What you can try |
| --- | --- | --- |
| `mise run ex:noul` | `01_noul` | Probability of yes, and how criteria that define yes/no change it |
| `mise run ex:choice` | `02_choice` | Classification, the probability distribution, `other` (null criteria) |
| `mise run ex:score` | `03_score` | Rubric scoring, and scores landing between levels |
| `mise run ex:multi` | `04_multi_questions` | Three question types in one request; latency with 1 vs 12 questions |
| `mise run ex:structured` | `05_structured_state` | Object/array state, `` `path` `` references, structured instructions |
| `mise run ex:models` | `06_models` | `GET /v1/models` and which version each alias resolves to |
| `mise run ex:confidence` | `07_confidence_routing` | Different confidence thresholds per action |
| `mise run ex:fanout` | `08_fan_out` | Ask every question you might need up front, then pick what matters in code |
| `mise run ex:composite` | `09_composite_scoring` | Weighted combination of per-dimension Scores; concurrent requests with goroutines |
| `mise run ex:intent` | `10_intent_routing` | Jev as a front classifier that picks the handler |
| `mise run ex:errors` | `11_errors` | Real 401 / 422 response bodies |
| `mise run ex:language` | `12_language` | Confidence differences between English and Japanese input |

`mise run ex:all` runs every example in order (it makes a few dozen API calls).

Environment variables:

- `TYPESAFE_MODEL=jev-1.13.0`: pin a version (default `jev-latest`)
- `TYPESAFE_BASE_URL`: point at a different endpoint

## Layout

```
typesafe/        Minimal client (standard library only)
  types.go       Request / Question (Noul, Choice, Score) / Response / Answer
  client.go      Client.SystemOne, Client.Do, Client.Models, retries
  errors.go      APIError (401/422/429/529 helpers), ConnectionError
internal/exutil  Output helpers shared by the examples
examples/NN_*/   One example each (also runnable with go run ./examples/NN_*)
```

### Using the client

```go
c, _ := typesafe.New() // reads TYPESAFE_API_KEY
res, err := c.SystemOne(ctx, "Help! My payouts have been failing for 3 days.", map[string]typesafe.Question{
    "is_urgent":  typesafe.Noul("Does this convey urgency?"),
    "department": typesafe.Choice("Which team should handle this?", map[string]any{
        "billing": "Payments, invoicing, refunds", "technical": "Bugs, outages, integrations",
    }),
    "frustration": typesafe.Score("How frustrated is the customer?", "Calm", "Frustrated", "Very angry"),
})
a := res.Answers["department"]
if a.Confidence < 0.6 { /* hand off to a human */ }
```

Retry defaults mirror the Python SDK's `RetryPolicy` (source: https://docs.typesafe.ai/sdk/python/api/retries):

- Up to 2 retries
- Backoff from 0.5s up to 5s, with ±25% jitter
- Retried statuses: 408 / 429 / 5xx (including 529)
- `Retry-After` header takes precedence when present

## Not covered by the docs

- **Error body format**: undocumented. A real 401 returns `{"detail":{"error_type":"authentication_error","message":...}}`. The 422 shape is unconfirmed; run `11_errors` to see it.
- **Unknown model names**: behavior is unconfirmed (the last case in `11_errors` tries it).
- **Japanese accuracy**: the docs only say English is primary and CJK accuracy is lower. `12_language` lets you compare directly.
