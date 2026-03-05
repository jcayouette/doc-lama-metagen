package asciidoc

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/scribe/doc-meta-gen/internal/models"
	"github.com/scribe/doc-meta-gen/pkg/attributes"
)

// Provider implements the ContentProvider interface for AsciiDoc files
type Provider struct {
	titleRe       *regexp.Regexp
	descRe        *regexp.Regexp
	navGenericRe  *regexp.Regexp
	navGuideRe    *regexp.Regexp
	headingRe     *regexp.Regexp
	listItemRe    *regexp.Regexp
	blockDelimRe  *regexp.Regexp
	inlineCodeRe  *regexp.Regexp
	boldRe        *regexp.Regexp
	italicRe      *regexp.Regexp
	xrefRe        *regexp.Regexp
	linkRe        *regexp.Regexp
	imageRe       *regexp.Regexp
}

// NewProvider creates a new AsciiDoc content provider
func NewProvider() *Provider {
	return &Provider{
		titleRe:      regexp.MustCompile(`^\s*=\s+(.+)$`),
		descRe:       regexp.MustCompile(`^:\s*description\s*:\s*(.*)$`),
		navGenericRe: regexp.MustCompile(`^nav(?:-.+)?\.adoc$`),
		navGuideRe:   regexp.MustCompile(`^nav-.+-guide\.adoc$`),
		headingRe:    regexp.MustCompile(`^==+\s+(.+)$`),
		listItemRe:   regexp.MustCompile(`^[\*\.\-]+\s+(.+)$`),
		blockDelimRe: regexp.MustCompile(`^(----|\.\.\.\.|====|\*\*\*\*|____|\+\+\+\+)$`),
		inlineCodeRe: regexp.MustCompile("`([^`]+)`"),
		boldRe:       regexp.MustCompile(`\*([^*]+)\*`),
		italicRe:     regexp.MustCompile(`_([^_]+)_`),
		xrefRe:       regexp.MustCompile(`xref:\S+\[([^\]]*)\]`),
		linkRe:       regexp.MustCompile(`(https?://\S+)\[([^\]]*)\]`),
		imageRe:      regexp.MustCompile(`image::\S+\[.*?\]`),
	}
}

// ID returns the provider identifier
func (p *Provider) ID() string {
	return "asciidoc"
}

// CanHandle checks if this provider can handle the given file
func (p *Provider) CanHandle(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".adoc") {
		return false
	}

	// Skip files based on Antora conventions
	filename := filepath.Base(path)

	// Skip underscore-prefixed files
	if strings.HasPrefix(filename, "_") {
		return false
	}

	// Skip navigation files
	if p.navGenericRe.MatchString(filename) || p.navGuideRe.MatchString(filename) {
		return false
	}

	// Skip files in nav, navigation, or partials directories
	pathLower := strings.ToLower(path)
	skipDirs := []string{"/nav/", "/navigation/", "/partials/"}
	for _, dir := range skipDirs {
		if strings.Contains(pathLower, dir) {
			return false
		}
	}

	return true
}

// Extract processes an AsciiDoc file and returns page content
func (p *Provider) Extract(path string, attrs *attributes.Store) (*models.PageContent, error) {
	content := &models.PageContent{
		FilePath: path,
		FileType: "AsciiDoc",
		Metadata: make(map[string]string),
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var rawLines []string
	var title string
	var existingDesc string
	headerEnd := 0

	scanner := bufio.NewScanner(file)
	lineNum := 0

	// First pass: extract title, description, and identify header end
	for scanner.Scan() {
		line := scanner.Text()
		lineNum++
		rawLines = append(rawLines, line)

		// Extract title (first line matching pattern)
		if title == "" && p.titleRe.MatchString(line) {
			matches := p.titleRe.FindStringSubmatch(line)
			if len(matches) > 1 {
				title = strings.TrimSpace(matches[1])
				headerEnd = lineNum
			}
			continue
		}

		// Extract existing description
		if p.descRe.MatchString(line) {
			matches := p.descRe.FindStringSubmatch(line)
			if len(matches) > 1 {
				existingDesc = strings.TrimSpace(matches[1])
			}
			continue
		}

		// Detect end of header (first non-attribute line after title)
		if title != "" && line != "" && !strings.HasPrefix(line, ":") {
			headerEnd = lineNum - 1
			break
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Extract body content (after header)
	bodyLines := rawLines[headerEnd:]
	bodyText := strings.Join(bodyLines, "\n")

	// Resolve attributes in the body
	if attrs != nil {
		bodyText = attrs.Resolve(bodyText)
	}

	// Clean and extract plain text
	plainText := p.extractPlainText(bodyText)

	content.Title = title
	content.ExistingMeta = existingDesc
	content.RawContent = plainText

	return content, nil
}

// extractPlainText converts AsciiDoc markup to plain text
func (p *Provider) extractPlainText(text string) string {
	lines := strings.Split(text, "\n")
	var cleanLines []string
	inBlock := false
	blockDepth := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty lines and comments
		if trimmed == "" || strings.HasPrefix(trimmed, "//") {
			continue
		}

		// Handle block delimiters
		if p.blockDelimRe.MatchString(trimmed) {
			if inBlock {
				blockDepth--
				if blockDepth == 0 {
					inBlock = false
				}
			} else {
				inBlock = true
				blockDepth++
			}
			continue
		}

		// Skip lines inside blocks (code, examples, etc.)
		if inBlock {
			continue
		}

		// Remove section headings
		if p.headingRe.MatchString(trimmed) {
			continue
		}

		// Clean list items
		if matches := p.listItemRe.FindStringSubmatch(trimmed); matches != nil {
			trimmed = matches[1]
		}

		// Remove various markup
		trimmed = p.imageRe.ReplaceAllString(trimmed, "")
		trimmed = p.xrefRe.ReplaceAllString(trimmed, "$1")
		trimmed = p.linkRe.ReplaceAllString(trimmed, "$2")
		trimmed = p.inlineCodeRe.ReplaceAllString(trimmed, "$1")
		trimmed = p.boldRe.ReplaceAllString(trimmed, "$1")
		trimmed = p.italicRe.ReplaceAllString(trimmed, "$1")

		// Remove attribute references and other special syntax
		trimmed = regexp.MustCompile(`\[\[.*?\]\]`).ReplaceAllString(trimmed, "")
		trimmed = regexp.MustCompile(`<<.*?>>`).ReplaceAllString(trimmed, "")
		trimmed = regexp.MustCompile(`\{[^\}]+\}`).ReplaceAllString(trimmed, "")

		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	result := strings.Join(cleanLines, " ")
	
	// Collapse multiple spaces
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)

	// Limit length for AI processing (4000 chars)
	if len(result) > 4000 {
		result = result[:4000]
	}

	return result
}

// HasExistingDescription checks if the file already has a description
func (p *Provider) HasExistingDescription(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if p.descRe.MatchString(scanner.Text()) {
			return true, nil
		}
	}

	return false, scanner.Err()
}

// WriteDescription writes the generated description to the file
func (p *Provider) WriteDescription(path string, description string, dryRun bool) error {
	if dryRun {
		return nil
	}

	// Ensure description is single-line (remove any newlines)
	description = strings.ReplaceAll(description, "\n", " ")
	description = strings.ReplaceAll(description, "\r", " ")
	// Collapse multiple spaces
	description = regexp.MustCompile(`\s+`).ReplaceAllString(description, " ")
	description = strings.TrimSpace(description)

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	file.Close()

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Find title line and collect existing descriptions
	titleIdx := -1
	var oldDescription string
	foundDescription := false

	prevDescRe := regexp.MustCompile(`^:\s*prev-description\s*:\s*(.*)$`)

	// First pass: find title and first valid description
	for i, line := range lines {
		if titleIdx == -1 && p.titleRe.MatchString(line) {
			titleIdx = i
		}
		if !foundDescription && p.descRe.MatchString(line) {
			matches := p.descRe.FindStringSubmatch(line)
			if len(matches) > 1 {
				oldDescription = strings.TrimSpace(matches[1])
			}
			foundDescription = true
		}
	}

	// Second pass: remove ALL description and prev-description lines
	var cleanLines []string
	skipNext := false
	for _, line := range lines {
		if skipNext {
			// Skip continuation lines
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(line, ":") {
				// Hit next attribute or empty line, stop skipping
				skipNext = false
				if trimmed != "" {
					cleanLines = append(cleanLines, line)
				}
			}
			continue
		}

		if p.descRe.MatchString(line) || prevDescRe.MatchString(line) {
			// Check if value continues on next line (multi-line attribute)
			matches := p.descRe.FindStringSubmatch(line)
			if len(matches) > 1 {
				value := strings.TrimSpace(matches[1])
				if value == "" || len(value) < 10 {
					// Likely a continuation, skip next lines too
					skipNext = true
				}
			}
			continue // Always skip description/prev-description lines
		}
		cleanLines = append(cleanLines, line)
	}
	lines = cleanLines

	// Rebuild title index after removal
	titleIdx = -1
	for i, line := range lines {
		if p.titleRe.MatchString(line) {
			titleIdx = i
			break
		}
	}

	// Now add new description and prev-description
	descLine := fmt.Sprintf(":description: %s", description)
	
	if titleIdx != -1 {
		// Insert after title
		insertIdx := titleIdx + 1
		
		// Build new slice from scratch to avoid aliasing issues
		var newLines []string
		newLines = append(newLines, lines[:insertIdx]...)
		
		if oldDescription != "" {
			// Add prev-description
			prevDescLine := fmt.Sprintf(":prev-description: %s", oldDescription)
			newLines = append(newLines, prevDescLine)
		}
		
		// Add new description
		newLines = append(newLines, descLine)
		
		// Add remaining lines
		newLines = append(newLines, lines[insertIdx:]...)
		lines = newLines
	} else {
		// No title found, insert at beginning
		if oldDescription != "" {
			prevDescLine := fmt.Sprintf(":prev-description: %s", oldDescription)
			lines = append([]string{prevDescLine, descLine}, lines...)
		} else {
			lines = append([]string{descLine}, lines...)
		}
	}

	// Write back to file
	output := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// RemoveDescriptions removes both :description: and :prev-description: attributes
func (p *Provider) RemoveDescriptions(path string, dryRun bool) error {
	if dryRun {
		return nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	
	// Remove all lines that start with :description: or :prev-description:
	// Also track if we're in a multi-line description value
	var filteredLines []string
	skipNext := false
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Check if this line starts a description attribute
		if strings.HasPrefix(trimmed, ":description:") || strings.HasPrefix(trimmed, ":prev-description:") {
			// Check if the value continues on the next line (no value after colon on this line)
			colonIdx := strings.Index(trimmed, ":")
			if colonIdx >= 0 {
				afterColon := strings.TrimSpace(trimmed[colonIdx+1:])
				secondColonIdx := strings.Index(afterColon, ":")
				if secondColonIdx >= 0 {
					valueAfterSecondColon := strings.TrimSpace(afterColon[secondColonIdx+1:])
					// If there's no value or it looks incomplete, mark to skip next lines
					if valueAfterSecondColon == "" || len(valueAfterSecondColon) < 20 {
						skipNext = true
					}
				}
			}
			continue // Skip this line
		}
		
		// If we're skipping continuation lines, check if this might be one
		if skipNext {
			// If the line doesn't start with a new attribute (doesn't start with :), it might be a continuation
			if !strings.HasPrefix(trimmed, ":") && trimmed != "" {
				continue // Skip continuation line
			} else {
				// New attribute or empty line, stop skipping
				skipNext = false
			}
		}
		
		filteredLines = append(filteredLines, line)
	}

	// Write back to file
	output := strings.Join(filteredLines, "\n")
	if err := os.WriteFile(path, []byte(output), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
