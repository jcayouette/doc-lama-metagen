package ai

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/scribe/doc-meta-gen/internal/ai/ollama"
)

// Generator handles AI-powered description generation
type Generator struct {
	client      *ollama.Client
	model       string
	bannedTerms []string
}

// NewGenerator creates a new AI generator
func NewGenerator(ollamaURL, model string, bannedTerms []string) *Generator {
	return &Generator{
		client:      ollama.NewClient(ollamaURL),
		model:       model,
		bannedTerms: bannedTerms,
	}
}

// GenerateDescription creates a meta description using AI
func (g *Generator) GenerateDescription(content, title string) (string, error) {
	blacklist := strings.Join(g.bannedTerms, ", ")
	prompt := g.buildPrompt(content, title, blacklist)

	// First attempt
	draft, err := g.client.Generate(g.model, prompt)
	if err != nil {
		return "", fmt.Errorf("AI generation failed: %w", err)
	}

	// Strip <think>…</think> blocks produced by reasoning models (e.g. Qwen3)
	// before any further processing — the thinking text routinely contains
	// phrases that would otherwise trigger leakage detection.
	draft = g.stripThinkingBlocks(draft)

	// Check RAW output for leakage BEFORE sanitization
	if g.hasPromptLeakage(draft) {
		retryPrompt := g.buildRetryPrompt(content, title, blacklist)
		draft, err = g.client.Generate(g.model, retryPrompt)
		if err != nil {
			return "", fmt.Errorf("AI retry failed: %w", err)
		}
		draft = g.stripThinkingBlocks(draft)
		// Check retry result for leakage too
		if g.hasPromptLeakage(draft) {
			return "", fmt.Errorf("generated description contains prompt leakage after retry")
		}
	}

	sanitized := g.sanitize(draft)

	// Retry if too short
	if len(sanitized) < 100 {
		retryPrompt := g.buildRetryPrompt(content, title, blacklist)
		draft, err = g.client.Generate(g.model, retryPrompt)
		if err != nil {
			return "", fmt.Errorf("AI retry failed: %w", err)
		}
		draft = g.stripThinkingBlocks(draft)
		// Check raw output again
		if g.hasPromptLeakage(draft) {
			return "", fmt.Errorf("generated description contains prompt leakage after retry")
		}
		sanitized = g.sanitize(draft)
	}

	// Final validation: reject if STILL has leakage in raw or sanitized
	if g.hasPromptLeakage(draft) || g.hasPromptLeakage(sanitized) {
		return "", fmt.Errorf("generated description contains prompt leakage: %s", sanitized)
	}

	// Validate length
	if len(sanitized) < 100 {
		return "", fmt.Errorf("generated description too short: %d characters", len(sanitized))
	}

	return sanitized, nil
}

// ValidateGrammar uses AI to check and correct grammar
func (g *Generator) ValidateGrammar(sentence string) (string, error) {
	if sentence == "" {
		return "", nil
	}

	prompt := fmt.Sprintf(`You are a copy editor for ASD-STE100 Simplified Technical English.

Correct grammar and awkward phrasing in the following sentence.

- Keep one complete thought. Keep the original meaning and technical terms.
- Keep short, common words. Do not add marketing or decorative wording.
- Remove redundant or broken phrases (for example, "on your or", "and system").
- Do not end the sentence with a period.
- If the sentence is already correct, return it unchanged.
- Return the corrected sentence alone. No preamble.

Original sentence:
---
%s
---
Corrected sentence:
`, sentence)

	corrected, err := g.client.Generate(g.model, prompt)
	if err != nil {
		// If validation fails, return original
		return sentence, nil
	}

	correctedClean := strings.TrimSpace(g.stripThinkingBlocks(corrected))

	// Check for prompt leakage in grammar validation output
	if g.hasPromptLeakage(correctedClean) {
		// If leakage detected, return original instead
		return sentence, nil
	}

	if correctedClean != "" && correctedClean != sentence {
		return correctedClean, nil
	}

	return sentence, nil
}

// Ping checks if Ollama is available
func (g *Generator) Ping() error {
	return g.client.Ping()
}

// buildPrompt creates the main generation prompt
func (g *Generator) buildPrompt(content, title, blacklist string) string {
	return fmt.Sprintf(`You are a technical writer. Write in ASD-STE100 Simplified Technical English.

Write ONE complete sentence (120-160 characters) that states what the reader can do or learn from this page.

Style:
- Use short, common words and concrete verbs. One idea in the sentence.
- Use the active voice. Start with an action verb that matches the page.
- Use the same term for the same thing. Do not switch synonyms.
- Use only terms that appear in the page title or content.
- The page title is the primary subject. The sentence must match that subject.
- If the page is a list of topics, say that the page is the starting point for that information.

Do not:
- Use marketing or decorative words (seamless, robust, powerful, comprehensive, easily, simply, leverage, utilize, empower, unlock, dive, explore)
- Name tools, products, or topics that are not in the title or content
- Include version numbers unless they are essential to the page
- Use self-reference (this chapter describes, this document explains, in this section)
- Call the sentence a summary or similar
- Use apostrophe possessives (write "the YaST tools", not "YaSTs tools")
- End the sentence with a period
- Add preamble, quotes, labels, or commentary
- Use these product or brand names, if listed: %s

Return the sentence alone. Start with the first word of the sentence.

Page title: %s

Page content:
---
%s
---
`, blacklist, title, content)
}

// buildRetryPrompt creates the retry prompt for short descriptions
func (g *Generator) buildRetryPrompt(content, title, blacklist string) string {
	return fmt.Sprintf(`You are a technical writer. Write in ASD-STE100 Simplified Technical English.

The previous sentence was too short. Write ONE longer sentence (120-160 characters) about the same page.

Style:
- Use short, common words and concrete verbs. One idea in the sentence.
- Use the active voice. Start with an action verb that matches the page.
- Expand on what the reader can do or learn. Stay with terms from the title and content.
- The page title is the primary subject. The sentence must match that subject.

Do not:
- Use marketing or decorative words (seamless, robust, powerful, comprehensive, easily, simply, leverage, utilize, empower, unlock, dive, explore)
- Name tools, products, or topics that are not in the title or content
- Use self-reference (this chapter describes, this document explains)
- Use apostrophe possessives
- End the sentence with a period
- Add preamble, quotes, labels, or commentary
- Use these product or brand names, if listed: %s

Return the sentence alone. Start with the first word of the sentence.

Page title: %s

Page content:
---
%s
---
`, blacklist, title, content)
}

// sanitize cleans and validates the AI response
func (g *Generator) sanitize(draft string) string {
	desc := html.UnescapeString(draft)

	// CRITICAL: Remove any leaked prompt instructions - must be done FIRST before other processing
	// More aggressive leakage detection: if we see these phrases, assume everything before the last
	// occurrence is leakage and take only what comes after
	leakagePrefixes := []string{
		"follow these rules strictly:",
		"here is the corrected sentence:",
		"here's the corrected sentence:",
		"corrected sentence:",
		"your task is to",
		"you must now",
		"output only",
		"your response must",
	}

	descLower := strings.ToLower(desc)
	for _, prefix := range leakagePrefixes {
		if idx := strings.LastIndex(descLower, prefix); idx >= 0 {
			// Found leakage - take everything after the prefix and any trailing punctuation/whitespace
			after := desc[idx+len(prefix):]
			after = strings.TrimSpace(after)
			// Remove leading colons, newlines, etc
			after = regexp.MustCompile(`^[\s:\-—\n\r]+`).ReplaceAllString(after, "")
			desc = strings.TrimSpace(after)
			descLower = strings.ToLower(desc)
		}
	}

	// Also remove standalone leakage fragments at the beginning
	leakagePatterns := []string{
		`(?i)^\s*follow these rules strictly[:\s]*`,
		`(?i)^\s*here'?s? the corrected sentence[:\s]*`,
		`(?i)^\s*corrected sentence[:\s]*`,
		`(?i)^\s*your task is to[^.]*\.?\s*`,
		`(?i)^\s*you must now[^.]*\.?\s*`,
		`(?i)^\s*output only[:\s]*`,
		`(?i)^\s*critical[:\s]+`,
		`(?i)^\s*important[:\s]+`,
		`(?i)^\s*note[:\s]+`,
		`(?i)^\s*remember[:\s]+`,
		`(?i)^\s*meta description[:\s]*`,
	}

	for _, pattern := range leakagePatterns {
		re := regexp.MustCompile(pattern)
		desc = re.ReplaceAllString(desc, "")
	}

	// Remove any leading/trailing whitespace, colons, dashes
	desc = strings.TrimSpace(desc)
	desc = regexp.MustCompile(`^[\s:\-—]+`).ReplaceAllString(desc, "")
	desc = strings.TrimSpace(desc)

	// Clean up spaces after leakage removal
	desc = regexp.MustCompile(`\s+`).ReplaceAllString(desc, " ")
	desc = strings.TrimSpace(desc)

	// Handle possessives (convert possessive forms)
	desc = regexp.MustCompile(`(\w+)'s\b`).ReplaceAllString(desc, "${1}s")
	desc = strings.ReplaceAll(desc, "'", "")

	// Remove banned terms
	for _, term := range g.bannedTerms {
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
		desc = re.ReplaceAllString(desc, "")
	}

	// Remove forbidden characters
	forbiddenRe := regexp.MustCompile(`[>:|"""'']`)
	desc = forbiddenRe.ReplaceAllString(desc, " ")

	// Remove self-referential patterns
	metaPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)^\s*This\s+(guide|page|document|section)\s+(describes|covers|explains|provides)\s+`),
		regexp.MustCompile(`(?i)^\s*In\s+this\s+(guide|page|document|section)\s+`),
		regexp.MustCompile(`(?i)^\s*The\s+(guide|page|document|section)\s+(describes|covers|explains|provides)\s+`),
	}

	for _, pat := range metaPatterns {
		desc = pat.ReplaceAllString(desc, "")
	}

	// Remove leading prepositions
	desc = regexp.MustCompile(`(?i)^(by|with|through|using)\s+`).ReplaceAllString(desc, "")

	// Collapse spaces and trim
	desc = regexp.MustCompile(`\s+`).ReplaceAllString(desc, " ")
	desc = strings.Trim(desc, " ,;:-")

	if desc == "" {
		return ""
	}

	// Truncate if too long (avoid cutting product names)
	if len(desc) > 160 {
		// First preference: cut at a natural sentence boundary (period) within the 100-160 range.
		// The model often generates a complete sentence that merely exceeds 160 chars;
		// preserving the sentence end avoids dangling/incomplete endings.
		if periodIdx := strings.LastIndex(desc[:160], "."); periodIdx >= 100 {
			desc = desc[:periodIdx] // period itself is stripped later
		} else {
			// Fall back to word-boundary truncation
			desc = desc[:160]
			lastSpace := strings.LastIndex(desc, " ")
			if lastSpace != -1 {
				// Check if we're potentially cutting a product name
				afterSpace := desc[lastSpace+1:]
				// Common product name fragments that shouldn't be cut
				fragments := []string{"SUSE", "Multi-Linux", "Linux", "Enterprise", "Manager", "Server"}

				isCuttingProduct := false
				for _, frag := range fragments {
					if strings.HasPrefix(frag, afterSpace) {
						isCuttingProduct = true
						break
					}
				}

				// If cutting a product name, try to find an earlier break point
				if isCuttingProduct {
					earlierSpace := strings.LastIndex(desc[:lastSpace], " ")
					if earlierSpace != -1 && earlierSpace > 100 {
						lastSpace = earlierSpace
					}
				}

				desc = desc[:lastSpace]
			}
		}
	}

	// Remove trailing stopwords
	trailingStopwords := map[string]bool{
		"and": true, "or": true, "to": true, "for": true, "with": true,
		"in": true, "of": true, "on": true, "at": true, "by": true,
		"from": true, "into": true, "via": true, "as": true, "that": true,
		"which": true, "including": true, "such": true, "than": true,
		"then": true, "while": true, "when": true, "where": true,
		// Articles and determiners that signal an incomplete noun phrase
		"the": true, "a": true, "an": true,
		// Gerunds that open a new dependent clause when trailing
		"covering": true, "ensuring": true,
		"using": true, "providing": true, "allowing": true, "enabling": true,
	}

	words := strings.Fields(strings.TrimRight(desc, " ,;:-."))
	for len(words) > 0 {
		lastWord := strings.ToLower(strings.Trim(words[len(words)-1], ",.;"))
		if !trailingStopwords[lastWord] {
			break
		}
		words = words[:len(words)-1]
	}
	desc = strings.Join(words, " ")

	// Remove dangling comparative fragments that read as incomplete endings
	// Examples: "... and more secure", "... or less complex", "... and better"
	danglingComparativeRe := regexp.MustCompile(`(?i)\s+(and|or)\s+(more|less|better|worse|additional)\s+([a-z-]+)?$`)
	if danglingComparativeRe.MatchString(desc) {
		desc = danglingComparativeRe.ReplaceAllString(desc, "")
		desc = strings.Trim(desc, " ,;:-")
	}

	// Remove trailing period
	desc = strings.TrimSuffix(desc, ".")

	// Capitalize first letter
	if len(desc) > 0 {
		desc = strings.ToUpper(string(desc[0])) + desc[1:]
	}

	return desc
}

// stripThinkingBlocks removes <think>…</think> sections produced by reasoning
// models such as Qwen3 before any further processing. The thinking text
// routinely contains phrases ("you must", "your task is", etc.) that would
// otherwise be misidentified as prompt leakage.
func (g *Generator) stripThinkingBlocks(text string) string {
	re := regexp.MustCompile(`(?is)<think>.*?</think>`)
	text = re.ReplaceAllString(text, "")
	return strings.TrimSpace(text)
}

// hasPromptLeakage checks if the description contains instruction fragments
func (g *Generator) hasPromptLeakage(desc string) bool {
	descLower := strings.ToLower(desc)

	// Check for common prompt leakage patterns
	leakageIndicators := []string{
		"follow these rules",
		"follow these steps",
		"here is the corrected",
		"here's the corrected",
		"here is the revised",
		"here's the revised",
		"corrected sentence",
		"revised sentence",
		"your task is",
		"you must",
		"you should",
		"output only",
		"meta description",
		"i have written",
		"i've written",
		"let me",
		"i will",
		"the sentence must",
		"this description",
		"as instructed",
		"according to the",
		"based on the rules",
		"following the guidelines",
		"according to these",
		"based on these",
		"per the instructions",
		"as per the",
	}

	for _, indicator := range leakageIndicators {
		if strings.Contains(descLower, indicator) {
			return true
		}
	}

	return false
}
