package llm

// Completer is the one thing any LLM vendor client must do:
// take a system prompt + user message, return text.
//
// Everything else in this codebase (Distill, findBoundary,
// extractConcept, ...) depends ONLY on this interface, never on
// a concrete vendor type. That's what makes swapping vendors —
// Anthropic, OpenRouter, a local ollama model — a one-line change
// at the call site that constructs the client, and nothing else.
type Completer interface {
	Complete(model, system, userMsg string, maxTokens int) (string, error)
}

// Model name constants. Different vendors use different naming
// schemes for "the same tier of model" — these are grouped by
// vendor so call sites stay honest about which client they need.
const (
	AnthropicHaiku  = "claude-haiku-4-5-20251001"
	AnthropicSonnet = "claude-sonnet-4-6"

	// OpenRouter model IDs follow "provider/model:variant" — the
	// ":free" suffix selects the zero-cost rate-limited tier.
	// WARNING: the free-model roster changes often (models get
	// delisted with no notice). If a call fails with "unavailable
	// for free", check openrouter.ai/models filtered to Price: Free
	// and override via LLM_MODEL rather than trusting these constants.
	OpenRouterFreeQwen = "qwen/qwen3-coder:free"
	OpenRouterAutoFree = "openrouter/free" // auto-router across free models
)
