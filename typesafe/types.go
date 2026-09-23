// Package typesafe is a minimal Go client for the TypeSafe System One API (Jev),
// built on the standard library only.
//
// Spec: https://docs.typesafe.ai/api
package typesafe

import (
	"cmp"
	"slices"
	"strconv"
)

// Values of the type field on questions and answers.
const (
	TypeNoul   = "noul"
	TypeChoice = "choice"
	TypeScore  = "score"
)

// DefaultModel matches the official SDKs' default. It is an alias, so what it
// resolves to changes over time. If you tune thresholds against a specific
// version, pin a versioned ID such as "jev-1.13.0".
// https://docs.typesafe.ai/models#aliases
const DefaultModel = "jev-latest"

// Request is the body of POST /v1/systemone.
type Request struct {
	// State is the content to evaluate: a string, map, slice, struct, or
	// anything else that marshals to JSON.
	State any `json:"state"`
	// Model is filled with the Client's default when empty.
	Model string `json:"model"`
	// Questions keys are IDs you choose. They are not sent to the model;
	// answers come back under the same keys.
	Questions map[string]Question `json:"questions"`
}

// Question is a Noul, Choice, or Score. Build one with the constructor functions.
//
// Instructions and each Criteria value can be a string, an object (map), or an
// array (slice). Using an object lets you separate the question from the data
// it refers to.
type Question struct {
	Type         string `json:"type"`
	Instructions any    `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

// NoulCriteria is the optional criteria for a Noul: what yes and no mean.
type NoulCriteria struct {
	True  any `json:"true,omitempty"`
	False any `json:"false,omitempty"`
}

// Noul is a yes/no question. The answer is the probability of yes (0 to 1).
func Noul(instructions any) Question {
	return Question{Type: TypeNoul, Instructions: instructions}
}

// NoulWith is a Noul whose criteria spell out what yes and no mean.
func NoulWith(instructions, whenTrue, whenFalse any) Question {
	return Question{
		Type:         TypeNoul,
		Instructions: instructions,
		Criteria:     NoulCriteria{True: whenTrue, False: whenFalse},
	}
}

// Choice picks one option. criteria maps option -> description (max 255 options).
// Use nil (JSON null) for options that need no description.
func Choice(instructions any, criteria map[string]any) Question {
	return Question{Type: TypeChoice, Instructions: instructions, Criteria: criteria}
}

// Score rates the state along ordered levels (2 to 10). Levels are numbered from 0.
func Score(instructions any, levels ...any) Question {
	return Question{Type: TypeScore, Instructions: instructions, Criteria: levels}
}

// Response is the body returned by POST /v1/systemone.
type Response struct {
	// Model is the versioned model ID that answered (e.g. "jev-1.13.0").
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}

// Usage is the token usage for a request. Only input tokens are billed.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Answer holds any of the three answer kinds. Check Type to know which fields apply.
//
//	noul:   Noul
//	choice: Choice, Probabilities, Confidence
//	score:  Score, Legend, Probabilities, Confidence
type Answer struct {
	Type string `json:"type"`

	Noul float64 `json:"noul,omitempty"`

	Choice string `json:"choice,omitempty"`

	Score float64 `json:"score,omitempty"`
	// Legend maps a Score level number ("0", "1", ...) to its description.
	Legend map[string]string `json:"legend,omitempty"`

	// Probabilities maps option -> probability for a Choice, or
	// level number -> probability for a Score. They sum to 1.
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	// Confidence (0 to 1) summarizes how peaked Probabilities is. Nouls have none.
	Confidence float64 `json:"confidence,omitempty"`
}

// Ranked returns Probabilities sorted from most to least likely.
func (a Answer) Ranked() []Prob {
	out := make([]Prob, 0, len(a.Probabilities))
	for k, p := range a.Probabilities {
		out = append(out, Prob{Key: k, P: p})
	}
	slices.SortFunc(out, func(x, y Prob) int {
		return cmp.Or(cmp.Compare(y.P, x.P), cmp.Compare(x.Key, y.Key))
	})
	return out
}

// Level returns the most likely Score level number and its description.
// Score itself can land between levels; use this when you need a discrete level.
func (a Answer) Level() (int, string) {
	r := a.Ranked()
	if len(r) == 0 {
		return -1, ""
	}
	n, err := strconv.Atoi(r[0].Key)
	if err != nil {
		return -1, ""
	}
	return n, a.Legend[r[0].Key]
}

// NormalizedScore scales Score to 0..1 by the number of levels.
// Useful when combining several Scores with weights (composite scoring).
func (a Answer) NormalizedScore() float64 {
	if len(a.Legend) < 2 {
		return 0
	}
	return a.Score / float64(len(a.Legend)-1)
}

// Prob is an element of Answer.Ranked.
type Prob struct {
	Key string
	P   float64
}

// Model is one entry from GET /v1/models.
type Model struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ReleaseDate string `json:"release_date"`
}

type modelsResponse struct {
	Models []Model `json:"models"`
}
