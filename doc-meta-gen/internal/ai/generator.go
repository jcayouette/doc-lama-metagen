package ai

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	"github.com/scribe/doc-meta-gen/internal/ai/ollama"
)

// sentenceModel is the completion API. Tests substitute a stub.
type sentenceModel interface {
	Generate(model, prompt string) (string, error)
	Ping() error
}

// Generator handles AI-powered description generation
type Generator struct {
	client      sentenceModel
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

// GenerateDescription creates a meta description using AI.
// A sentence that is cut off, too long, too short, or not about the page title
// is rejected and retried. It is never sliced into a fragment and saved.
func (g *Generator) GenerateDescription(content, title string) (string, error) {
	content = g.removeBannedTerms(content)
	title = g.removeBannedTerms(title)
	blacklist := strings.Join(g.bannedTerms, ", ")

	var last string
	reason := ""
	sawLeak := false
	for attempt := 0; attempt < 3; attempt++ {
		prompt := g.promptFor(attempt, reason, last, content, title, blacklist)
		draft, err := g.client.Generate(g.model, prompt)
		if err != nil {
			return "", fmt.Errorf("AI generation failed: %w", err)
		}
		// Strip <think>…</think> blocks produced by reasoning models (e.g. Qwen3)
		// before any further processing. The thinking text routinely contains
		// phrases that would otherwise trigger leakage detection.
		draft = g.stripThinkingBlocks(draft)
		if g.hasPromptLeakage(draft) {
			sawLeak = true
			reason = "the answer contained instruction text instead of a sentence"
			last = ""
			continue
		}
		sanitized := g.sanitize(draft)
		if g.hasPromptLeakage(sanitized) {
			sawLeak = true
			reason = "the answer contained instruction text instead of a sentence"
			last = ""
			continue
		}
		last = sanitized
		if why := g.reject(title, sanitized); why != "" {
			reason = why
			continue
		}
		return sanitized, nil
	}
	if last == "" && sawLeak {
		return "", fmt.Errorf("generated description contains prompt leakage after retry")
	}
	return "", fmt.Errorf("generated description is incomplete or too long: %s", last)
}

// KeepIfComplete returns candidate when it is a finished sentence about the page.
// Otherwise it returns current, so a grammar pass cannot reintroduce a cut-off ending.
func (g *Generator) KeepIfComplete(title, current, candidate string) string {
	candidate = g.sanitize(candidate)
	if g.reject(title, candidate) != "" || g.hasPromptLeakage(candidate) {
		return current
	}
	return candidate
}

func (g *Generator) acceptable(desc string) bool {
	return g.reject("", desc) == ""
}

// reject reports why desc cannot be saved. An empty string means it can.
// Title grounding is skipped when title is empty.
func (g *Generator) reject(title, desc string) string {
	desc = strings.TrimSpace(desc)
	switch {
	case desc == "":
		return "the sentence was empty"
	case len(desc) < 100:
		return fmt.Sprintf("the sentence was too short (%d characters). Write one finished sentence of 18 to 26 words", len(desc))
	case len(desc) > 160:
		return fmt.Sprintf("the sentence was too long (%d characters). Write one finished sentence of 18 to 26 words", len(desc))
	case incompleteEnding(desc):
		return "the sentence stops before the thought is complete"
	case !grounded(title, desc):
		return "the sentence does not use the subject from the page title: " + title
	default:
		return ""
	}
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
- Do not shorten the sentence by dropping the ending.
- End the sentence with a period. The period is removed later.
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

func (g *Generator) promptFor(attempt int, reason, rejected, content, title, blacklist string) string {
	if attempt == 0 {
		return g.buildPrompt(content, title, blacklist)
	}
	return g.buildFollowUpPrompt(content, title, blacklist, rejected, reason)
}

// buildPrompt creates the main generation prompt.
// Length is checked in code. The model is asked for a word count, not a character budget,
// because a character budget makes it stop mid-phrase.
func (g *Generator) buildPrompt(content, title, blacklist string) string {
	return fmt.Sprintf(`You are a technical writer. Write in ASD-STE100 Simplified Technical English.

Write ONE complete sentence that states what the reader can do or learn from this page.
Use 18 to 26 words. End the sentence with a period. The period is removed before the sentence is saved.

Style:
- Use short, common words and concrete verbs. One idea in the sentence.
- Use the active voice. Start with an action verb that matches the page.
- Use the same term for the same thing. Do not switch synonyms.
- Use at least one specific word from the page title.
- Use the names and terms from the page content. Do not replace them with generic words such as people, project, things, or items.
- The page title is the primary subject. The sentence must match that subject.
- Finish the idea. Do not stop on a conjunction, a preposition, or an adjective.
- If the page is a list of topics, say that the page is the starting point for that information.

Do not:
- Count characters or stop early to meet a character limit
- Use marketing or decorative words (seamless, robust, powerful, comprehensive, easily, simply, leverage, utilize, empower, unlock, dive, explore)
- Name this product, even when the page names it. Do not write a product or brand name from the banned list. Describe the server, proxy, client, or task instead
- Name tools, products, or topics that are not in the title or content
- Include version numbers unless they are essential to the page
- Use self-reference (this chapter describes, this document explains, in this section)
- Call the sentence a summary or similar
- Use apostrophe possessives (write "the YaST tools", not "YaSTs tools")
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

// buildFollowUpPrompt asks for a new sentence after a rejected attempt.
func (g *Generator) buildFollowUpPrompt(content, title, blacklist, rejected, reason string) string {
	if strings.TrimSpace(rejected) == "" {
		rejected = "(none)"
	}
	if strings.TrimSpace(reason) == "" {
		reason = "it was not a finished sentence about this page"
	}
	return fmt.Sprintf(`You are a technical writer. Write in ASD-STE100 Simplified Technical English.

The previous sentence was rejected because %s.
Write ONE new sentence of 18 to 26 words. End it with a period. The period is removed later.

Rejected sentence:
%s

Style:
- Use short, common words and concrete verbs. One idea in the sentence.
- Use the active voice. Start with an action verb that matches the page.
- Use at least one specific word from the page title.
- Use the names and terms from the page content. Do not replace them with generic words such as people, project, things, or items.
- Finish the idea. Do not stop on a conjunction, a preposition, or an adjective.
- If the rejected sentence was too short, add a detail from the page until you have 18 to 26 words. A one-sentence page still needs that length.

Do not:
- Count characters or stop early to meet a character limit
- Repeat the rejected sentence
- Use marketing or decorative words (seamless, robust, powerful, comprehensive, easily, simply, leverage, utilize, empower, unlock, dive, explore)
- Name this product, even when the page names it. Do not write a product or brand name from the banned list
- Use self-reference (this chapter describes, this document explains)
- Use apostrophe possessives
- Add preamble, quotes, labels, or commentary
- Use these product or brand names, if listed: %s

Return the sentence alone. Start with the first word of the sentence.

Page title: %s

Page content:
---
%s
---
`, reason, rejected, blacklist, title, content)
}

// removeBannedTerms deletes banned product and brand names and collapses the gap.
func (g *Generator) removeBannedTerms(text string) string {
	for _, term := range g.bannedTerms {
		term = strings.TrimSpace(term)
		if term == "" {
			continue
		}
		re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(term) + `\b`)
		text = re.ReplaceAllString(text, "")
	}
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
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
	desc = g.removeBannedTerms(desc)

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

	// Keep the first finished sentence when the model adds a second one.
	// Do not slice a long sentence at the last space. That cut is what produced
	// fragments such as "during different". An over-long sentence is left intact
	// so the caller can reject it and ask for a shorter complete sentence.
	if finished := firstFinishedSentence(desc); finished != "" {
		desc = finished
	} else if len(desc) > 160 {
		if clause := cutAtClause(desc); clause != "" {
			desc = clause
		} else if periodIdx := strings.LastIndex(desc[:160], "."); periodIdx >= 100 {
			desc = desc[:periodIdx]
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

// unfinishedTail matches clause endings that still need a complement.
// A denylist of single last words cannot catch "during different" or "where every update".
var unfinishedTail = regexp.MustCompile(`(?i)(?:\bwhere\s+(?:every|each|all|the|a|an|your|any)\s+[a-z0-9-]+|\bto\s+(?:ensure|verify|confirm|control|update|manage|deploy|create|configure|install|remove|delete|add|copy|extract|monitor|analyze|understand|identify|describe)|\b(?:helping|letting|allowing|enabling)\s+you\s+[a-z0-9-]+)$`)

// openModifiers cannot finish a sentence. They modify a noun that was cut off.
var openModifiers = map[string]bool{
	"different": true, "referenced": true, "various": true, "specific": true,
	"respective": true, "corresponding": true, "related": true, "certain": true,
	"particular": true, "given": true, "above": true, "below": true,
}

// incompleteEnding is true when the last word leaves the sentence unfinished.
func incompleteEnding(desc string) bool {
	trimmed := strings.TrimRight(desc, " ,;:-.")
	words := strings.Fields(trimmed)
	if len(words) == 0 {
		return true
	}
	last := strings.ToLower(strings.Trim(words[len(words)-1], ",.;"))
	dangling := map[string]bool{
		"and": true, "or": true, "to": true, "for": true, "with": true,
		"in": true, "of": true, "on": true, "at": true, "by": true,
		"from": true, "into": true, "via": true, "as": true, "that": true,
		"which": true, "including": true, "such": true, "than": true,
		"then": true, "while": true, "when": true, "where": true,
		"the": true, "a": true, "an": true, "your": true, "their": true, "its": true,
		"before": true, "after": true, "if": true, "during": true, "between": true,
		"without": true, "within": true, "about": true, "over": true, "under": true,
		"across": true, "through": true, "whether": true, "because": true, "but": true,
		"following": true, "using": true, "providing": true, "allowing": true,
		"enabling": true, "ensuring": true, "covering": true,
		"removing": true, "adding": true, "creating": true, "sending": true,
		"managing": true, "deploying": true, "deleting": true, "configuring": true,
		"installing": true, "updating": true, "copying": true, "forwarding": true,
		"applying": true, "generating": true, "monitoring": true, "extracting": true,
		"adds": true, "requires": true, "includes": true, "provides": true,
		"supports": true, "allows": true, "lets": true, "helps": true,
	}
	if dangling[last] || openModifiers[last] {
		return true
	}
	return unfinishedTail.MatchString(trimmed)
}

// firstFinishedSentence returns the first sentence when it already fits the limit.
// An empty result means the caller must keep the original text and judge it whole.
func firstFinishedSentence(desc string) string {
	for i := 0; i < len(desc); i++ {
		if desc[i] != '.' {
			continue
		}
		if i+1 < len(desc) && desc[i+1] >= '0' && desc[i+1] <= '9' {
			continue
		}
		if i > 0 && desc[i-1] >= '0' && desc[i-1] <= '9' {
			continue
		}
		candidate := strings.TrimSpace(desc[:i])
		if len(candidate) > 160 {
			return ""
		}
		if len(candidate) >= 100 && !incompleteEnding(candidate) {
			return candidate
		}
	}
	return ""
}

var titleTokenRe = regexp.MustCompile(`[a-z0-9]+`)

var titleStopwords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "for": true, "with": true,
	"your": true, "from": true, "into": true, "this": true, "that": true,
	"page": true, "guide": true, "about": true, "using": true, "how": true,
	"what": true, "when": true, "where": true, "overview": true, "index": true,
	"introduction": true, "reference": true, "other": true, "more": true,
	"than": true, "then": true, "you": true, "are": true, "was": true,
	"its": true, "their": true, "them": true, "they": true, "who": true,
	"why": true, "which": true, "have": true, "has": true, "been": true,
	"will": true, "can": true, "may": true, "not": true, "but": true,
	"also": true, "each": true, "every": true, "all": true, "any": true,
	"some": true, "new": true, "key": true, "main": true, "these": true,
	"those": true, "over": true, "under": true, "only": true, "just": true,
	"here": true, "there": true, "chapter": true, "section": true,
	"documentation": true, "concept": true, "concepts": true,
	"explanation": true, "explanations": true, "summary": true, "notes": true,
}

// grounded reports that desc uses a specific word from the page title.
// Titles made only of generic words, such as Overview, do not constrain the sentence.
func grounded(title, desc string) bool {
	tokens := significantTitleTokens(title)
	if len(tokens) == 0 {
		return true
	}
	descLower := strings.ToLower(desc)
	for _, tok := range tokens {
		if titleTokenIn(descLower, tok) {
			return true
		}
	}
	return false
}

func significantTitleTokens(title string) []string {
	fields := titleTokenRe.FindAllString(strings.ToLower(title), -1)
	var out []string
	seen := map[string]bool{}
	for _, field := range fields {
		if len(field) < 5 || titleStopwords[field] || seen[field] {
			continue
		}
		seen[field] = true
		out = append(out, field)
	}
	return out
}

func titleTokenIn(descLower, tok string) bool {
	if strings.Contains(descLower, tok) {
		return true
	}
	stem := tok
	switch {
	case strings.HasSuffix(stem, "ing") && len(stem) > 5:
		stem = stem[:len(stem)-3]
	case strings.HasSuffix(stem, "s") && len(stem) >= 5:
		stem = stem[:len(stem)-1]
	}
	if stem != tok && len(stem) >= 4 && strings.Contains(descLower, stem) {
		return true
	}
	// configuration/configure and installation/install share a prefix.
	for _, word := range strings.FieldsFunc(descLower, func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	}) {
		if sharedPrefixLen(word, tok) >= 6 {
			return true
		}
	}
	return false
}

func sharedPrefixLen(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

// cutAtClause keeps the longest finished clause that still fits in 100-160 characters.
func cutAtClause(desc string) string {
	window := desc
	if len(window) > 160 {
		window = window[:160]
	}
	best := ""
	consider := func(idx int) {
		if idx < 100 {
			return
		}
		candidate := strings.TrimRight(window[:idx], " ,;:-")
		if len(candidate) >= 100 && len(candidate) <= 160 && !incompleteEnding(candidate) && len(candidate) > len(best) {
			best = candidate
		}
	}
	for i, r := range window {
		if r == ',' || r == ';' {
			consider(i)
		}
	}
	for _, sep := range []string{" and ", " then ", " so "} {
		idx := strings.LastIndex(window, sep)
		consider(idx)
	}
	return best
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
