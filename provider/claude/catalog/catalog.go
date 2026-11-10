// Package catalog is the Claude models the adapter of package provider/claude calls, and their
// prices and efforts, without Anthropic's SDK: the library and the eval runner name and price a
// model by it, so that whatever takes them takes the SDK only with the adapter itself (Д185
// RUN.md spec; R90).
package catalog

import "slices"

// Model is a Claude model the adapter can call: its exact ID (R07 spec), how the board shows it,
// its price per million input and output tokens, in US dollars, and the kind of thinking that turns
// its thinking off, or nothing if its thinking cannot be turned off.
type Model struct {
	ID            string
	Name          string
	Input, Output float64
	ThinkingOff   string
}

// Models are the current Claude models, cheapest first, with the IDs and prices of Anthropic's
// Models overview, read 2026-10-23. Haiku 5.5's price is the one for a request of up to 100 000
// tokens (docs/architecture.md, section 4); the board's are far shorter. Haiku 5.5 turns its
// thinking off with "disabled"; Sonnet 5.5 refuses that and turns it off with "between_tools",
// which without tools leaves an answer of text alone; on Opus 5.5 and Fable 5.1 thinking cannot
// be turned off (Thinking, read 2026-10-25).
var Models = []Model{
	{ID: "claude-haiku-5-5", Name: "Claude Haiku 5.5", Input: 0.10, Output: 0.50, ThinkingOff: "disabled"},
	{ID: "claude-sonnet-5-5", Name: "Claude Sonnet 5.5", Input: 2, Output: 10, ThinkingOff: "between_tools"},
	{ID: "claude-opus-5-5", Name: "Claude Opus 5.5", Input: 4, Output: 20},
	{ID: "claude-fable-5-1", Name: "Claude Fable 5.1", Input: 10, Output: 50},
}

// Efforts are the effort levels, least first. Each model in Models takes all five (Effort, read
// 2026-10-23).
var Efforts = []string{"low", "medium", "high", "xhigh", "max"}

// ThinkingOffEfforts are the levels at which thinking can be turned off: high and below. At xhigh
// and max, a request that turns it off fails (Thinking, read 2026-10-25).
var ThinkingOffEfforts = []string{"low", "medium", "high"}

// NoThinkingEfforts are the levels at which the model can be asked to answer without thinking:
// none if its thinking cannot be turned off.
func (m Model) NoThinkingEfforts() []string {
	if m.ThinkingOff == "" {
		return nil
	}
	return slices.Clone(ThinkingOffEfforts)
}
