// Package assembly implements the per-turn context-assembly pipeline (design Section 4.4): it
// assembles a budgeted context in priority order, most relevant first, and stops before the window
// is full, trimming the last includable part rather than blowing the budget or the bill. The logic
// is pure and deterministic; pulling the pieces from Postgres lives in builder.go.
package assembly

import "strings"

// Part is one piece of context. Parts are supplied in priority order (highest first). Trimmable
// parts (large tool output) may be truncated to fit the remaining budget instead of dropped.
type Part struct {
	Name      string
	Content   string
	Trimmable bool
}

// Assembled is the result of budgeting: the joined text, its token count, and what happened to each
// part.
type Assembled struct {
	Text      string
	Tokens    int
	Included  []string // parts included in full
	Truncated []string // parts included but trimmed to fit
	Dropped   []string // parts that did not fit at all
}

// TokenCounter estimates the token count of a string. The runtime can inject a real tokenizer;
// EstimateTokens is a reasonable default.
type TokenCounter func(string) int

// EstimateTokens approximates tokens at ~4 characters per token. Deterministic and dependency-free.
func EstimateTokens(s string) int {
	if s == "" {
		return 0
	}
	return (len(s) + 3) / 4
}

// Assemble includes parts in order until the token budget is reached. A part that fits is included
// whole; a trimmable part that does not fit is truncated to the remaining budget; a non-trimmable
// part that does not fit is dropped (and assembly stops, since lower-priority parts cannot jump
// ahead of a higher-priority one that was skipped). budget <= 0 means unbounded.
func Assemble(parts []Part, budget int, count TokenCounter) Assembled {
	if count == nil {
		count = EstimateTokens
	}
	var (
		out  Assembled
		segs []string
	)
	for _, p := range parts {
		if p.Content == "" {
			continue
		}
		cost := count(p.Content)
		remaining := budget - out.Tokens

		if budget <= 0 || cost <= remaining {
			segs = append(segs, p.Content)
			out.Tokens += cost
			out.Included = append(out.Included, p.Name)
			continue
		}
		// Does not fit.
		if p.Trimmable && remaining > 0 {
			trimmed := truncateToTokens(p.Content, remaining, count)
			if trimmed != "" {
				segs = append(segs, trimmed)
				out.Tokens += count(trimmed)
				out.Truncated = append(out.Truncated, p.Name)
			} else {
				out.Dropped = append(out.Dropped, p.Name)
			}
		} else {
			out.Dropped = append(out.Dropped, p.Name)
		}
		break // budget is full; do not include lower-priority parts after a skip
	}
	out.Text = strings.Join(segs, "\n\n")
	return out
}

// truncateToTokens cuts s down to at most maxTokens tokens (by the given counter), appending an
// ellipsis marker. Uses a character-ratio first cut, then trims to fit.
func truncateToTokens(s string, maxTokens int, count TokenCounter) string {
	if maxTokens <= 0 {
		return ""
	}
	if count(s) <= maxTokens {
		return s
	}
	const marker = "\n…[trimmed]"
	// Approximate, then shrink until it fits including the marker.
	cut := maxTokens * 4
	if cut > len(s) {
		cut = len(s)
	}
	for cut > 0 {
		candidate := s[:cut] + marker
		if count(candidate) <= maxTokens {
			return candidate
		}
		cut -= 16
	}
	return ""
}
