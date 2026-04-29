package docbook

import (
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"

	"github.com/scribe/doc-meta-gen/internal/models"
	"github.com/scribe/doc-meta-gen/pkg/attributes"
)

const itsNamespace = "http://www.w3.org/2005/11/its"

// allowedDoctypes lists the DocBook root element types that need meta descriptions.
// sect1–sect5, book, article, and set are intentionally excluded.
var allowedDoctypes = map[string]bool{
	"appendix": true,
	"chapter":  true,
	"preface":  true,
}

// Provider implements the ContentProvider interface for DocBook XML files.
type Provider struct {
	doctypeRe      *regexp.Regexp
	titleRe        *regexp.Regexp
	abstractRe     *regexp.Regexp
	existingMetaRe *regexp.Regexp
	innerMetaRe    *regexp.Regexp
	xmlTagRe       *regexp.Regexp
	entityRe       *regexp.Regexp
	xmlnsAttrRe    *regexp.Regexp
	infoTagRe      *regexp.Regexp
	commentRe      *regexp.Regexp
	cdataRe        *regexp.Regexp
	multiSpaceRe   *regexp.Regexp
}

// NewProvider creates a new DocBook content provider.
func NewProvider() *Provider {
	return &Provider{
		// Matches <!DOCTYPE chapter  or  <!DOCTYPE appendix  etc.
		doctypeRe: regexp.MustCompile(`(?i)<!DOCTYPE\s+(\w+)`),
		// First <title> element in the document (captures inner content)
		titleRe: regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`),
		// <abstract> element inside <info> (captures inner content)
		abstractRe: regexp.MustCompile(`(?is)<abstract[^>]*>(.*?)</abstract>`),
		// Matches self-closing or full <meta name="description" ...> element
		existingMetaRe: regexp.MustCompile(`(?is)<meta\s+name="description"[^>]*/?>(?:.*?</meta>)?`),
		// Extracts text from <meta ...>text</meta>
		innerMetaRe: regexp.MustCompile(`(?is)<meta[^>]*>(.*?)</meta>`),
		// Matches any XML/SGML tag
		xmlTagRe: regexp.MustCompile(`<[^>]+>`),
		// Matches XML named entity references like &productname; (not numeric &#…;)
		entityRe: regexp.MustCompile(`&[a-zA-Z][a-zA-Z0-9.-]*;`),
		// Matches any xmlns or xmlns:prefix attribute (to strip from <info>)
		xmlnsAttrRe: regexp.MustCompile(`\s+xmlns(?::\w+)?="[^"]+"`),
		// Matches the <info> opening tag (word-boundary to avoid <informaltable> etc.)
		infoTagRe: regexp.MustCompile(`(?i)<\binfo\b[^>]*>`),
		// XML comments
		commentRe: regexp.MustCompile(`(?s)<!--.*?-->`),
		// CDATA sections
		cdataRe: regexp.MustCompile(`(?s)<!\[CDATA\[.*?\]\]>`),
		// Multiple whitespace characters
		multiSpaceRe: regexp.MustCompile(`\s+`),
	}
}

// ID returns the provider identifier.
func (p *Provider) ID() string {
	return "docbook"
}

// CanHandle checks if this provider can process the given file.
// Only DocBook XML files whose DOCTYPE is appendix, chapter, or preface are handled.
func (p *Provider) CanHandle(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".xml") {
		return false
	}
	doctype, err := p.detectDoctype(path)
	if err != nil || doctype == "" {
		return false
	}
	return allowedDoctypes[strings.ToLower(doctype)]
}

// detectDoctype reads the first 2 KB of the file to find the <!DOCTYPE …> declaration.
func (p *Provider) detectDoctype(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 2048)
	n, _ := f.Read(buf)
	if n == 0 {
		return "", nil
	}

	if m := p.doctypeRe.FindStringSubmatch(string(buf[:n])); len(m) > 1 {
		return m[1], nil
	}
	return "", nil
}

// Extract processes a DocBook XML file and returns standardised page content.
func (p *Provider) Extract(path string, attrs *attributes.Store) (*models.PageContent, error) {
	content := &models.PageContent{
		FilePath: path,
		FileType: "DocBook XML",
		Metadata: make(map[string]string),
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	fileContent := string(raw)

	// Resolve XML entity references (&productname; etc.) so the AI sees
	// the actual product names rather than unresolved entity placeholders.
	if attrs != nil {
		fileContent = attrs.ResolveXML(fileContent)
	}

	// Extract title from the first <title> element in the document
	title := ""
	if m := p.titleRe.FindStringSubmatch(fileContent); len(m) > 1 {
		title = p.cleanText(m[1])
	}

	// Extract existing meta description if already present
	existingMeta := ""
	if m := p.existingMetaRe.FindString(fileContent); m != "" {
		if im := p.innerMetaRe.FindStringSubmatch(m); len(im) > 1 {
			existingMeta = strings.TrimSpace(p.cleanText(im[1]))
		}
	}

	// Build content payload: prefer <abstract>, fall back to full document text
	payload := ""
	if m := p.abstractRe.FindStringSubmatch(fileContent); len(m) > 1 {
		payload = p.extractPlainText(m[1])
	}
	if payload == "" {
		payload = p.extractPlainText(fileContent)
	}

	content.Title = title
	content.ExistingMeta = existingMeta
	content.RawContent = payload

	return content, nil
}

// HasExistingDescription checks if the file already contains a meta description element.
func (p *Provider) HasExistingDescription(path string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("failed to read file: %w", err)
	}
	return p.existingMetaRe.Match(raw), nil
}

// WriteDescription inserts or replaces the meta description in the DocBook XML file.
//
// Placement rules:
//   - The element is placed inside the <info> block (created if absent).
//   - xmlns:its="http://www.w3.org/2005/11/its" is added to the root element
//     if not already present.
//   - xmlns attributes are stripped from the <info> opening tag to avoid
//     duplicate namespace declarations.
func (p *Provider) WriteDescription(path string, description string, dryRun bool) error {
	if dryRun {
		return nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	fileContent := string(raw)

	// Normalise whitespace in the description (mirrors AsciiDoc provider behaviour).
	description = p.multiSpaceRe.ReplaceAllString(description, " ")
	description = strings.TrimSpace(description)

	// XML-escape only the characters that are unsafe inside element content.
	// Avoid html.EscapeString which also escapes apostrophes (&#39;) and quotes
	// unnecessarily, and would double-encode any &entity; that slipped through.
	escapedDesc := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	).Replace(description)
	newMeta := fmt.Sprintf(`<meta name="description" its:translate="yes">%s</meta>`, escapedDesc)

	// Determine the root element name from the DOCTYPE declaration
	doctype, _ := p.detectDoctype(path)
	if doctype == "" {
		return fmt.Errorf("no DOCTYPE declaration found in %s", path)
	}

	rootTagRe := regexp.MustCompile(fmt.Sprintf(`(?i)<%s(?:\s[^>]*)?>`, regexp.QuoteMeta(doctype)))

	// 1. Ensure xmlns:its is present on the root opening element.
	if rootTagMatch := rootTagRe.FindString(fileContent); rootTagMatch != "" {
		if !strings.Contains(rootTagMatch, "xmlns:its") {
			// Insert the namespace declaration right after the element name.
			nameEndIdx := strings.IndexAny(rootTagMatch[1:], " \t\n\r>") + 1
			newRootTag := rootTagMatch[:nameEndIdx] +
				fmt.Sprintf(` xmlns:its="%s"`, itsNamespace) +
				rootTagMatch[nameEndIdx:]
			fileContent = strings.Replace(fileContent, rootTagMatch, newRootTag, 1)
		}
	}

	// 2. Find the <info> opening tag; create a minimal <info> block if absent.
	infoMatch := p.infoTagRe.FindString(fileContent)
	if infoMatch == "" {
		// Insert <info></info> immediately after the (possibly updated) root tag.
		if rootTagMatch := rootTagRe.FindString(fileContent); rootTagMatch != "" {
			replacement := rootTagMatch + "\n  <info></info>"
			fileContent = strings.Replace(fileContent, rootTagMatch, replacement, 1)
			infoMatch = p.infoTagRe.FindString(fileContent)
		}
	}

	if infoMatch == "" {
		return fmt.Errorf("no <info> block found and could not create one in %s", path)
	}

	// 3. Strip any xmlns declarations from the <info> opening tag to prevent
	//    duplicate namespace declarations after inserting the meta element.
	cleanInfoTag := p.xmlnsAttrRe.ReplaceAllString(infoMatch, "")
	if cleanInfoTag != infoMatch {
		fileContent = strings.Replace(fileContent, infoMatch, cleanInfoTag, 1)
		infoMatch = cleanInfoTag
	}

	// 4. Replace the first existing meta description, or insert a new one
	//    right after the <info> opening tag.
	if loc := p.existingMetaRe.FindStringIndex(fileContent); loc != nil {
		fileContent = fileContent[:loc[0]] + newMeta + fileContent[loc[1]:]
	} else {
		fileContent = strings.Replace(fileContent, infoMatch, infoMatch+"\n    "+newMeta, 1)
	}

	return os.WriteFile(path, []byte(fileContent), 0644)
}

// RemoveDescriptions removes the <meta name="description"> element from the file.
func (p *Provider) RemoveDescriptions(path string, dryRun bool) error {
	if dryRun {
		return nil
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}
	fileContent := string(raw)

	// Match the element along with its preceding newline+indent so we don't
	// leave a blank line behind after removal.
	removeMetaRe := regexp.MustCompile(`(?is)\n[ \t]*<meta\s+name="description"[^>]*/?>(?:.*?</meta>)?`)
	if !removeMetaRe.MatchString(fileContent) {
		return nil
	}

	fileContent = removeMetaRe.ReplaceAllString(fileContent, "")
	return os.WriteFile(path, []byte(fileContent), 0644)
}

// extractPlainText strips all XML markup from the given content and returns
// plain text suitable for AI processing (max 4000 chars).
func (p *Provider) extractPlainText(xmlContent string) string {
	// Remove XML comments and CDATA sections first
	text := p.commentRe.ReplaceAllString(xmlContent, " ")
	text = p.cdataRe.ReplaceAllString(text, " ")
	// Strip named entity references (e.g. &productname; → "") so the AI
	// receives the surrounding prose without unresolved entity names.
	text = p.entityRe.ReplaceAllString(text, "")
	// Strip all remaining XML tags
	text = p.xmlTagRe.ReplaceAllString(text, " ")
	// Decode standard character entities (&amp; &lt; &gt; etc.)
	text = html.UnescapeString(text)
	// Collapse whitespace
	text = p.multiSpaceRe.ReplaceAllString(text, " ")
	text = strings.TrimSpace(text)

	// Limit to 4000 chars for AI processing
	if len(text) > 4000 {
		text = text[:4000]
	}
	return text
}

// cleanText strips XML tags and entities from a short string (e.g. a title).
func (p *Provider) cleanText(s string) string {
	s = p.entityRe.ReplaceAllString(s, "")
	s = p.xmlTagRe.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	s = p.multiSpaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
