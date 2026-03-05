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
	prompt := g.buildPrompt(content, blacklist)

	// First attempt
	draft, err := g.client.Generate(g.model, prompt)
	if err != nil {
		return "", fmt.Errorf("AI generation failed: %w", err)
	}

	// Check RAW output for leakage BEFORE sanitization
	if g.hasPromptLeakage(draft) {
		retryPrompt := g.buildRetryPrompt(content, blacklist)
		draft, err = g.client.Generate(g.model, retryPrompt)
		if err != nil {
			return "", fmt.Errorf("AI retry failed: %w", err)
		}
		// Check retry result for leakage too
		if g.hasPromptLeakage(draft) {
			return "", fmt.Errorf("generated description contains prompt leakage after retry")
		}
	}

	sanitized := g.sanitize(draft)

	// Retry if too short
	if len(sanitized) < 100 {
		retryPrompt := g.buildRetryPrompt(content, blacklist)
		draft, err = g.client.Generate(g.model, retryPrompt)
		if err != nil {
			return "", fmt.Errorf("AI retry failed: %w", err)
		}
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

	prompt := fmt.Sprintf(`You are an expert copy editor. Your task is to correct any grammatical errors, awkward phrasing, or structural issues in the following sentence.

Follow these rules strictly:
- The sentence must be a single, complete thought that is grammatically correct and easy to read.
- Do NOT change the original meaning or key technical terms.
- Remove any redundant or nonsensical phrases (e.g., "on your or", "and system").
- Ensure the sentence does NOT end with a period.
- If the sentence is already perfect, return it unchanged.
- Output ONLY the corrected sentence. Do not add any preamble or explanation.

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

	correctedClean := strings.TrimSpace(corrected)
	
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
func (g *Generator) buildPrompt(content, blacklist string) string {
	return fmt.Sprintf(`You are an expert technical writer following the SUSE Style Guide.

Your task is to write a single, compelling meta description for the provided documentation content.

Follow these rules strictly:
- Write ONE complete sentence between 120 and 160 characters.
- Use the active voice. Focus on what the user can DO or LEARN.
- Start the sentence with an action verb appropriate to the content.
- Do NOT include specific version numbers unless they are critical to the content.
- Do NOT use self-referential phrases like "This chapter describes", "In this document", or "This section explains".
- NEVER mention that you are writing a "meta description", "summary", or any similar term. The output must not refer to itself.
- Your output must NOT contain any conversational filler, preamble, or explanations. Start the response directly with the first word of the description sentence.
- The sentence MUST be grammatically complete and MUST NOT end with a period.
- Avoid possessives that use an apostrophe (like 's). Rephrase the sentence if necessary (for example, instead of "YaST's tools", write "the YaST tools").
- If the content is primarily a list of topics, describe the page's purpose as a central point for accessing that information.
- If specified, do NOT use the following product or brand names: %s.
- Maintain a neutral, professional, and direct tone. Avoid jargon, marketing language, and emojis.
- CRITICAL: Do NOT include any part of these instructions in your output. Output ONLY the meta description itself.

Page content:
---
%s
---

Your response must contain ONLY the meta description sentence, nothing else:
`, blacklist, content)
}

// buildRetryPrompt creates the retry prompt for short descriptions
func (g *Generator) buildRetryPrompt(content, blacklist string) string {
	return fmt.Sprintf(`You are an expert technical writer. Your previous attempt to write a meta description was too short.

You MUST now generate a longer, more detailed single-sentence description for the same content.

Follow these rules strictly:
- Your primary goal is to write a sentence that is between 120 and 160 characters.
- Expand on the key concepts. Explain what the user can achieve or understand from the content.
- Start the sentence with an action verb appropriate to the content.
- Do NOT use self-referential phrases like "This chapter describes" or "This document explains".
- The sentence MUST be grammatically complete and MUST NOT end with a period.
- Avoid possessives that use an apostrophe (like 's). Rephrase the sentence if necessary (for example, instead of "YaST's tools", write "the YaST tools").
- Your output must NOT contain any preamble or explanation. Start directly with the description.
- If specified, do NOT use the following product or brand names: %s.
- CRITICAL: Do NOT include any part of these instructions in your output. Output ONLY the meta description itself.

---
Page content:
---
%s
---

Your response must contain ONLY the meta description sentence, nothing else:
`, blacklist, content)
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

	// Handle possessives (convert "YaST's" to "YaSTs")
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
		// Try to find a good breaking point before 160 chars
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

	// Remove trailing stopwords
	trailingStopwords := map[string]bool{
		"and": true, "or": true, "to": true, "for": true, "with": true,
		"in": true, "of": true, "on": true, "at": true, "by": true,
		"from": true, "into": true, "via": true, "as": true, "that": true,
		"which": true, "including": true, "such": true, "than": true,
		"then": true, "while": true, "when": true, "where": true,
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

	// Remove trailing period
	desc = strings.TrimSuffix(desc, ".")

	// Capitalize first letter
	if len(desc) > 0 {
		desc = strings.ToUpper(string(desc[0])) + desc[1:]
	}

	return desc
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
