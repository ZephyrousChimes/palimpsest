package okf

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Concept struct {
	Id          string
	Type        string // "concept" | "relation"
	Title       string
	Description string
	Resource    string // path to the corporeal source
	Tags        []string
	Timestamp   string
	Body        string
}

// wikiRoot is set once, at startup, via SetWikiRoot — called from main
// after config.Load(). Defaults to "./data/wiki" if never set, so
// existing callers (and tests) keep working without change.
var wikiRoot = "./data/wiki"

// SetWikiRoot configures where the store reads/writes concept files.
// Call this once, early in main, right after loading config.
func SetWikiRoot(path string) {
	wikiRoot = path
}

func WikiRoot() string {
	return wikiRoot
}

func (concept Concept) Write() error {
	path := filepath.Join(WikiRoot(), concept.Id)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("[OKF] failed to create directory: %w", err)
	}

	content := concept.render()

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("[OKF] failed to write file: %w", err)
	}

	return nil
}

func Read(id string) (*Concept, error) {
	path := filepath.Join(WikiRoot(), id)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("[OKF] failed to read file: %w", err)
	}

	content := string(data)

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("[OKF] malformed concept file. id=%v", id)
	}

	frontmatter := parts[1]
	body := strings.TrimSpace(parts[2])

	c := Concept{Id: id}
	scanner := bufio.NewScanner(strings.NewReader(frontmatter))
	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		switch key {
		case "type":
			c.Type = val
		case "title":
			c.Title = val
		case "description":
			c.Description = val
		case "resource":
			c.Resource = val
		}
	}
	c.Body = body

	return &c, nil
}

func (concept Concept) render() string {
	return fmt.Sprintf("---\ntype: %s\ntitle: %s\ndescription: %s\nresource: %s\n---\n\n%s",
		concept.Type, concept.Title, concept.Description, concept.Resource, concept.Body)
}
