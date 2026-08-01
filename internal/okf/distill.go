package okf

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ZephyrousChimes/palimpsest/internal/llm"
)

// ThetaTokens is the target window size for boundary detection,
// per LumberChunker's empirically-tuned value (Duarte et al., 2024).
const ThetaTokens = 550

// paragraph is one unit of the source, tagged with an incremental ID —
// exactly LumberChunker's "ID XXXX: <text>" scheme.
type paragraph struct {
	ID   int
	Text string
}

// splitParagraphs breaks raw source text into paragraphs on blank lines.
func splitParagraphs(source string) []paragraph {
	raw := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n\n")
	var paras []paragraph
	id := 1
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		paras = append(paras, paragraph{ID: id, Text: p})
		id++
	}
	return paras
}

// roughTokens is a cheap token-count estimate (chars/4), good enough
// for windowing decisions — no need for a real tokenizer here.
func roughTokens(s string) int {
	return len(s) / 4
}

// findBoundary asks the LLM whether this window contains a second,
// independently-new claim beyond the first paragraph — and if so,
// where it starts. Returns (boundaryID, noSplit, err): when noSplit
// is true, boundaryID is meaningless and should be ignored.
//
// The explicit NO_SPLIT path exists because earlier testing showed
// the model would always find SOME boundary when only ever asked
// "where does the next claim start" — including splitting a
// paragraph that was purely restating the previous one in different
// words ("to restate the same point more concretely..."). Without a
// legitimate way to answer "there is no second claim here," the
// model has no way to express that judgment, so it never did.
func findBoundary(client llm.Completer, model string, window []paragraph) (boundaryID int, noSplit bool, err error) {
	var sb strings.Builder
	for _, p := range window {
		fmt.Fprintf(&sb, "ID %04d: %s\n\n", p.ID, p.Text)
	}

	prompt := fmt.Sprintf(`You will receive an English document with paragraphs identified by 'ID XXXX: <text>'.

Task: determine whether this text contains a SECOND independently-new
claim beyond the first paragraph's claim, or whether everything here
is really just one claim — possibly restated, elaborated, rephrased,
or made "more concrete" in different words.

If a later paragraph restates, elaborates, rephrases, or re-derives
the SAME claim already made earlier — even with different wording,
more detail, or phrases like "to restate this more concretely" —
that is NOT a new claim. Answer NO_SPLIT in that case.

Only identify a boundary if some paragraph asserts something
genuinely new that was not already claimed earlier in this window —
for example, a different mechanism, a different limitation, or a
different proposed solution. Do not split just because the topic
continues; only split when the ASSERTION changes.

If everything in this window is one claim, respond with exactly:
NO_SPLIT

Otherwise, respond with exactly: Answer: ID XXXX
(the ID of the paragraph where the first genuinely new claim begins)

Document:
%s`, sb.String())

	resp, err := client.Complete(model, "", prompt, 64)
	if err != nil {
		return 0, false, fmt.Errorf("findBoundary: %w", err)
	}

	return parseAnswerID(resp)
}

// parseAnswerID extracts either a NO_SPLIT signal or an integer ID
// from a response like "Answer: ID 0019" — robust to markdown/
// punctuation noise (bold markers, periods, etc.) that different
// models add around the number, since not every model follows the
// prompt's exact output format cleanly.
func parseAnswerID(resp string) (id int, noSplit bool, err error) {
	if strings.Contains(strings.ToUpper(resp), "NO_SPLIT") {
		return 0, true, nil
	}

	idx := strings.Index(resp, "ID")
	if idx == -1 {
		return 0, false, fmt.Errorf("no 'ID' or 'NO_SPLIT' found in response: %q", resp)
	}
	rest := resp[idx+2:]

	var digits strings.Builder
	for _, r := range rest {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
			continue
		}
		if digits.Len() > 0 {
			break // stop at the first non-digit once digits have started
		}
		// haven't started collecting yet — skip whitespace/punctuation
		// (":", "*", etc.) and keep scanning for the first digit
	}

	if digits.Len() == 0 {
		return 0, false, fmt.Errorf("no ID digits found in response: %q", resp)
	}

	trimmed := strings.TrimLeft(digits.String(), "0")
	if trimmed == "" {
		return 0, false, nil // e.g. "0000"
	}
	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, false, fmt.Errorf("could not parse ID from %q: %w", digits.String(), err)
	}
	return n, false, nil
}

// FindBoundaries runs a sliding-window loop over the source,
// returning the paragraph IDs where each span ends. On NO_SPLIT,
// the window grows to include one more paragraph and asks again,
// rather than terminating — this is what lets two paragraphs that
// are genuinely the same claim stay merged into one span, instead
// of always being forced apart.
func FindBoundaries(client llm.Completer, model string, paras []paragraph) ([]int, error) {
	var boundaries []int
	startIdx := 0

	for startIdx < len(paras) {
		// build a window until it exceeds ThetaTokens
		tokens := 0
		endIdx := startIdx
		for endIdx < len(paras) && (tokens < ThetaTokens || endIdx == startIdx) {
			tokens += roughTokens(paras[endIdx].Text)
			endIdx++
		}

		window := paras[startIdx:endIdx]
		if len(window) == 1 {
			// nothing left to compare against — this is the final span
			boundaries = append(boundaries, window[0].ID)
			break
		}

		boundaryID, noSplit, err := findBoundary(client, model, window)
		if err != nil {
			return nil, err
		}

		if noSplit {
			if endIdx >= len(paras) {
				// nothing left to grow into — whole remaining window is one span
				boundaries = append(boundaries, paras[len(paras)-1].ID)
				break
			}
			// grow the window by one more paragraph and ask again,
			// instead of forcing a boundary that doesn't exist
			extended := paras[startIdx : endIdx+1]
			boundaryID, noSplit, err = findBoundary(client, model, extended)
			if err != nil {
				return nil, err
			}
			if noSplit {
				// still no second claim even with one more paragraph —
				// fall back to treating the whole extended window as
				// one span, rather than looping indefinitely
				boundaries = append(boundaries, extended[len(extended)-1].ID)
				if endIdx+1 >= len(paras) {
					break
				}
				startIdx = endIdx + 1
				continue
			}
		}

		if boundaryID <= paras[startIdx].ID {
			// LLM failed to find a real shift — fall back to window end
			boundaryID = window[len(window)-1].ID
		}
		boundaries = append(boundaries, boundaryID)

		// next window starts at the boundary paragraph
		nextStart := -1
		for i, p := range paras {
			if p.ID == boundaryID {
				nextStart = i
				break
			}
		}
		if nextStart == -1 || nextStart <= startIdx {
			break
		}
		startIdx = nextStart
	}

	return boundaries, nil
}

// extractedConcept is the shape we ask the LLM to fill in per span.
type extractedConcept struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Body        string `json:"body"`
	Quote       string `json:"quote"`
}

// extractConcept turns one bounded span of source text into a Concept,
// via a separate, single-purpose LLM call — deliberately not the same
// call as boundary-finding.
//
// verbose=true prints the raw model response before parsing — use
// this to tell apart two different failure modes: the model itself
// only synthesizing part of the span it was given (a prompting
// problem), versus our JSON parsing silently truncating a longer,
// complete response (a code bug).
func extractConcept(client llm.Completer, model string, span string, sourcePath string, verbose bool) (*Concept, error) {
	prompt := fmt.Sprintf(`Extract ONE distinct concept from this text span.
Give:
- title: short name for the concept
- description: one-line summary
- body: a clear synthesized explanation (may add framing, but must not
  introduce claims beyond what the span supports)
- quote: the exact source text (or the most load-bearing sentence(s))
  that justifies this concept

IMPORTANT: the span may contain more than one idea. Your body must
synthesize ALL of the content in the span, not just the first idea
you see. Do not drop or ignore any part of the span.

Text span:
%s

Respond as JSON only, no other text: {"title": "...", "description": "...", "body": "...", "quote": "..."}`, span)

	resp, err := client.Complete(model, "", prompt, 1024)
	if err != nil {
		return nil, fmt.Errorf("extractConcept: %w", err)
	}

	if verbose {
		fmt.Printf("[extractConcept] raw response (%d chars):\n---\n%s\n---\n", len(resp), resp)
	}

	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")

	var ec extractedConcept
	if err := json.Unmarshal([]byte(resp), &ec); err != nil {
		return nil, fmt.Errorf("extractConcept: parse response: %w (raw: %s)", err, resp)
	}

	if verbose {
		fmt.Printf("[extractConcept] parsed body (%d chars):\n---\n%s\n---\n", len(ec.Body), ec.Body)
	}

	return &Concept{
		Type:        "concept",
		Title:       ec.Title,
		Description: ec.Description,
		Resource:    sourcePath,
		Body:        ec.Body,
	}, nil
}

// Distill runs the full ingestion pipeline on one corporeal source:
// find boundaries (LumberChunker-style), then extract one concept
// per resulting span. No placement, no dedup — those are separate
// steps, run later.
//
// Set verbose=true to print boundaries found and each span's raw
// content before extraction — useful for debugging cases where
// content silently vanishes between boundary-detection and
// extraction (e.g. a boundary merging two distinct ideas into one
// span, or an extraction call failing without surfacing why).
func Distill(client llm.Completer, model string, sourceText string, sourcePath string, verbose bool) ([]*Concept, error) {
	paras := splitParagraphs(sourceText)
	if len(paras) == 0 {
		return nil, fmt.Errorf("Distill: source has no paragraphs")
	}

	if verbose {
		fmt.Printf("[distill] %d paragraphs found, IDs 1..%d\n", len(paras), paras[len(paras)-1].ID)
	}

	boundaries, err := FindBoundaries(client, model, paras)
	if err != nil {
		return nil, fmt.Errorf("Distill: %w", err)
	}

	if verbose {
		fmt.Printf("[distill] boundaries found: %v (expect one boundary per concept, "+
			"including the final paragraph's own ID as the last one)\n", boundaries)
	}

	var concepts []*Concept
	spanStart := 0
	for spanIdx, boundaryID := range boundaries {
		var spanParas []paragraph
		for _, p := range paras {
			if p.ID >= paras[spanStart].ID && p.ID < boundaryID {
				spanParas = append(spanParas, p)
			}
		}
		if len(spanParas) == 0 {
			// boundary equals span start — single paragraph span
			spanParas = []paragraph{paras[spanStart]}
		}

		var sb strings.Builder
		for _, p := range spanParas {
			sb.WriteString(p.Text)
			sb.WriteString("\n\n")
		}
		spanText := sb.String()

		if verbose {
			paraIDs := make([]int, len(spanParas))
			for i, p := range spanParas {
				paraIDs[i] = p.ID
			}
			fmt.Printf("[distill] span %d: paragraph IDs %v (%d chars)\n---\n%s\n---\n",
				spanIdx, paraIDs, len(spanText), strings.TrimSpace(spanText))
		}

		c, err := extractConcept(client, model, spanText, sourcePath, verbose)
		if err != nil {
			if verbose {
				fmt.Printf("[distill] span %d: extraction FAILED: %v\n", spanIdx, err)
			}
			return nil, fmt.Errorf("Distill: %w", err)
		}

		if verbose {
			fmt.Printf("[distill] span %d -> concept %q\n", spanIdx, c.Title)
		}

		concepts = append(concepts, c)

		// advance spanStart to the boundary paragraph
		for i, p := range paras {
			if p.ID == boundaryID {
				spanStart = i
				break
			}
		}
	}

	return concepts, nil
}
