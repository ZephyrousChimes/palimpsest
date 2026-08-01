package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/ZephyrousChimes/palimpsest/internal/config"
	"github.com/ZephyrousChimes/palimpsest/internal/llm"
	"github.com/ZephyrousChimes/palimpsest/internal/okf"
)

func main() {
	cfg, err := config.Load()
	must(err, "load config")

	okf.SetWikiRoot(cfg.WikiRoot)

	if len(os.Args) < 2 {
		log.Fatal("usage: palimpsest <source-file.txt>")
	}
	sourcePath := os.Args[1]

	data, err := os.ReadFile(sourcePath)
	must(err, "read source file")

	client, err := newCompleter(cfg)
	must(err, "init llm client")

	fmt.Printf("Distilling %s (vendor=%s, model=%s) ...\n", sourcePath, cfg.LLMVendor, cfg.Model)
	verbose := os.Getenv("VERBOSE") != ""
	concepts, err := okf.Distill(client, cfg.Model, string(data), sourcePath, verbose)
	must(err, "distill")

	fmt.Printf("\nExtracted %d concept(s):\n\n", len(concepts))
	for i, c := range concepts {
		c.Id = filepath.Join("concepts", slugify(c.Title)+".md")

		must(c.Write(), fmt.Sprintf("write concept %d", i))

		fmt.Printf("[%d] %s\n    -> %s\n    %s\n\n", i+1, c.Title, c.Id, c.Description)
	}

	fmt.Println("Done. Written under", okf.WikiRoot())
}

func newCompleter(cfg *config.Config) (llm.Completer, error) {
	switch cfg.LLMVendor {
	case "openrouter":
		return llm.NewOpenRouterClient(cfg.OpenRouterAPIKey)
	default:
		return llm.NewAnthropicClient(cfg.AnthropicAPIKey)
	}
}

func slugify(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+('a'-'A'))
		case r == ' ' || r == '-' || r == '_':
			out = append(out, '-')
		}
	}
	return string(out)
}

func must(err error, ctx string) {
	if err != nil {
		log.Fatalf("FATAL [%s]: %v", ctx, err)
	}
}
