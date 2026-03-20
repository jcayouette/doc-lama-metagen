package processor

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/scribe/doc-meta-gen/internal/ai"
	"github.com/scribe/doc-meta-gen/internal/models"
	"github.com/scribe/doc-meta-gen/internal/providers"
	"github.com/scribe/doc-meta-gen/pkg/attributes"
)

// Processor orchestrates the description generation pipeline
type Processor struct {
	providers  []providers.ContentProvider
	generator  *ai.Generator
	attributes *attributes.Store
	config     *models.Config
}

// NewProcessor creates a new processor
func NewProcessor(
	providers []providers.ContentProvider,
	generator *ai.Generator,
	attrs *attributes.Store,
	config *models.Config,
) *Processor {
	return &Processor{
		providers:  providers,
		generator:  generator,
		attributes: attrs,
		config:     config,
	}
}

// ProcessFile processes a single file
func (p *Processor) ProcessFile(path string) (*models.ProcessingResult, error) {
	result := &models.ProcessingResult{
		FilePath: path,
	}

	// Find appropriate provider
	var provider providers.ContentProvider
	for _, prov := range p.providers {
		if prov.CanHandle(path) {
			provider = prov
			break
		}
	}

	if provider == nil {
		result.Status = models.StatusError
		result.Details = "No provider found for file"
		return result, fmt.Errorf("no provider can handle: %s", path)
	}

	result.FileType = provider.ID()

	// Check existing description
	hasDesc, err := provider.HasExistingDescription(path)
	if err != nil {
		result.Status = models.StatusError
		result.Details = fmt.Sprintf("Failed to check existing description: %v", err)
		return result, err
	}

	if hasDesc && !p.config.ForceOverwrite {
		result.Status = models.StatusSkipped
		result.Details = "File already has a description"
		log.Printf("SKIPPED: %s - %s", path, result.Details)
		return result, nil
	}

	// Extract content
	content, err := provider.Extract(path, p.attributes)
	if err != nil {
		result.Status = models.StatusError
		result.Details = fmt.Sprintf("Failed to extract content: %v", err)
		return result, err
	}

	const minContentLength = 150
	if len(content.RawContent) < minContentLength {
		result.Status = models.StatusWarning
		if content.RawContent == "" {
			result.Details = "Empty content after extraction"
		} else {
			result.Details = fmt.Sprintf("Insufficient content for generation (%d chars, minimum %d)", len(content.RawContent), minContentLength)
		}
		log.Printf("WARNING: %s - %s", path, result.Details)
		return result, nil
	}

	// Generate description
	description, err := p.generator.GenerateDescription(content.RawContent, content.Title)
	if err != nil {
		result.Status = models.StatusError
		result.Details = fmt.Sprintf("AI generation failed: %v", err)
		return result, err
	}

	// Validate grammar
	correctedDesc, err := p.generator.ValidateGrammar(description)
	if err == nil && correctedDesc != "" {
		description = correctedDesc
	}

	// Resolve any attribute references the model echoed from the source content
	// (e.g. {project-name} → "SUSE Security Admission Controller").
	// Resolve() also strips any references that couldn't be resolved.
	if p.attributes != nil {
		description = p.attributes.Resolve(description)
		// Some attribute values contain AsciiDoc markup (xref, link). Strip it,
		// keeping only the display text so the description stays plain text.
		description = regexp.MustCompile(`(?:xref|link):[^\[]+\[([^\]]*)\]`).ReplaceAllString(description, "$1")
		description = regexp.MustCompile(`\s+`).ReplaceAllString(description, " ")
		description = strings.TrimSpace(description)
	}

	result.Description = description
	result.CharCount = len(description)

	// Write description
	if err := provider.WriteDescription(path, description, p.config.DryRun); err != nil {
		result.Status = models.StatusError
		result.Details = fmt.Sprintf("Failed to write description: %v", err)
		return result, err
	}

	// Determine status
	if p.config.DryRun {
		result.Status = models.StatusDryRun
		log.Printf("[DRY RUN] Would update: %s (%d chars)\n  → %s", path, result.CharCount, description)
	} else if hasDesc {
		result.Status = models.StatusReplaced
		log.Printf("REPLACED: %s (%d chars)\n  → %s", path, result.CharCount, description)
	} else {
		result.Status = models.StatusAdded
		log.Printf("ADDED: %s (%d chars)\n  → %s", path, result.CharCount, description)
	}

	return result, nil
}

// ProcessFiles processes multiple files
func (p *Processor) ProcessFiles(paths []string) ([]*models.ProcessingResult, error) {
	results := make([]*models.ProcessingResult, 0, len(paths))

	for _, path := range paths {
		result, err := p.ProcessFile(path)
		if err != nil {
			// Log error but continue processing
			log.Printf("ERROR processing %s: %v", path, err)
		}
		results = append(results, result)
	}

	return results, nil
}
