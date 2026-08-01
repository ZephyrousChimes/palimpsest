package config

import (
	"fmt"
	"os"
)

// Config holds every setting the app needs, read once at startup.
// Nothing else in the codebase should call os.Getenv directly —
// everything reaches config through this struct, passed explicitly.
type Config struct {
	LLMVendor        string // "anthropic" (default) | "openrouter"
	AnthropicAPIKey  string
	OpenRouterAPIKey string
	Model            string // model name/ID, vendor-specific format
	WikiRoot         string
}

// Load reads and validates configuration from the process environment.
// Fails loudly, at startup, if anything required is missing —
// deliberately, so the program never limps along with bad config.
func Load() (*Config, error) {
	c := &Config{
		LLMVendor:        os.Getenv("LLM_VENDOR"),
		AnthropicAPIKey:  os.Getenv("ANTHROPIC_API_KEY"),
		OpenRouterAPIKey: os.Getenv("OPENROUTER_API_KEY"),
		Model:            os.Getenv("LLM_MODEL"),
		WikiRoot:         os.Getenv("WIKI_ROOT"),
	}

	if c.LLMVendor == "" {
		c.LLMVendor = "anthropic" // default
	}

	switch c.LLMVendor {
	case "anthropic":
		if c.AnthropicAPIKey == "" {
			return nil, fmt.Errorf("[config] ANTHROPIC_API_KEY is required when LLM_VENDOR=anthropic")
		}
		if c.Model == "" {
			c.Model = "claude-haiku-4-5-20251001"
		}
	case "openrouter":
		if c.OpenRouterAPIKey == "" {
			return nil, fmt.Errorf("[config] OPENROUTER_API_KEY is required when LLM_VENDOR=openrouter")
		}
		if c.Model == "" {
			c.Model = "meta-llama/llama-3.3-70b-instruct"
		}
	default:
		return nil, fmt.Errorf("[config] unknown LLM_VENDOR: %q (want anthropic or openrouter)", c.LLMVendor)
	}

	if c.WikiRoot == "" {
		c.WikiRoot = "./data/wiki" // sensible default, dev-friendly
	}

	return c, nil
}
