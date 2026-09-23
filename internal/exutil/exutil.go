// Package exutil holds helpers shared by the examples (client setup and output).
package exutil

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/rai-wtnb/lab-jev/typesafe"
)

// Client builds a client from environment variables and exits on failure.
// extra options are applied after the environment-based ones.
//
//	TYPESAFE_API_KEY  required
//	TYPESAFE_MODEL    optional (default jev-latest)
//	TYPESAFE_BASE_URL optional
func Client(extra ...typesafe.Option) *typesafe.Client {
	opts := []typesafe.Option{
		typesafe.WithOnRetry(func(n int, wait time.Duration, err error) {
			log.Printf("retry #%d in %v: %v", n, wait.Round(time.Millisecond), err)
		}),
	}
	if m := os.Getenv("TYPESAFE_MODEL"); m != "" {
		opts = append(opts, typesafe.WithModel(m))
	}
	if u := os.Getenv("TYPESAFE_BASE_URL"); u != "" {
		opts = append(opts, typesafe.WithBaseURL(u))
	}
	c, err := typesafe.New(append(opts, extra...)...)
	if err != nil {
		log.Fatal(err)
	}
	return c
}

// Ctx returns a context with a timeout for the examples.
func Ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)
}

// Title prints a heading.
func Title(s string) {
	fmt.Printf("\n\033[1m== %s ==\033[0m\n", s)
}

// Timed runs f and prints how long it took.
func Timed[T any](label string, f func() (T, error)) T {
	start := time.Now()
	v, err := f()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("(%s: %v)\n", label, time.Since(start).Round(time.Millisecond))
	return v
}

// JSON pretty-prints a value (handy for inspecting request content).
func JSON(label string, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Printf("%s:\n%s\n", label, b)
}

// Response prints every answer, sorted by ID.
func Response(res *typesafe.Response) {
	fmt.Printf("model=%s  usage: in=%d out=%d\n", res.Model, res.Usage.InputTokens, res.Usage.OutputTokens)
	for _, id := range slices.Sorted(maps.Keys(res.Answers)) {
		Answer(id, res.Answers[id])
	}
}

// Answer prints a single answer according to its type.
func Answer(id string, a typesafe.Answer) {
	switch a.Type {
	case typesafe.TypeNoul:
		fmt.Printf("  %-22s noul   %.3f %s\n", id, a.Noul, Bar(a.Noul))
	case typesafe.TypeChoice:
		fmt.Printf("  %-22s choice %q  confidence=%.2f\n", id, a.Choice, a.Confidence)
		for _, p := range a.Ranked() {
			fmt.Printf("  %-22s        %-20s %.3f %s\n", "", p.Key, p.P, Bar(p.P))
		}
	case typesafe.TypeScore:
		n, desc := a.Level()
		fmt.Printf("  %-22s score  %.2f (top level: %d %q)  confidence=%.2f\n", id, a.Score, n, desc, a.Confidence)
		for i := range len(a.Legend) {
			k := fmt.Sprint(i)
			fmt.Printf("  %-22s        %s %-40s %.3f %s\n", "", k, trunc(a.LevelText(k), 40), a.Probabilities[k], Bar(a.Probabilities[k]))
		}
	default:
		fmt.Printf("  %-22s (unknown type %q) %+v\n", id, a.Type, a)
	}
}

// Bar renders a 0..1 value as a simple bar.
func Bar(p float64) string {
	n := min(max(int(p*20+0.5), 0), 20)
	return strings.Repeat("█", n) + strings.Repeat("·", 20-n)
}

func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
