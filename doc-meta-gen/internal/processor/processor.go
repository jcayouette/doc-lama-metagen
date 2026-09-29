package processor

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/scribe/doc-meta-gen/internal/ai"
	"github.com/scribe/doc-meta-gen/internal/discovery"
	"github.com/scribe/doc-meta-gen/internal/models"
	"github.com/scribe/doc-meta-gen/internal/providers"
	"github.com/scribe/doc-meta-gen/pkg/attributes"
)

// Processor orchestrates the description generation pipeline
type Processor struct {
	providers   []providers.ContentProvider
	generator   *ai.Generator
	attributes  *attributes.Store
	config      *models.Config
	antoraCache map[string]map[string]string
}

// NewProcessor creates a new processor
func NewProcessor(
	providers []providers.ContentProvider,
	generator *ai.Generator,
	attrs *attributes.Store,
	config *models.Config,
) *Processor {
	return &Processor{
		providers:   providers,
		generator:   generator,
		attributes:  attrs,
		config:      config,
		antoraCache: make(map[string]map[string]string),
	}
}

// ProcessFile processes a single file (generate or skip).
func (p *Processor) ProcessFile(path string) (*models.ProcessingResult, error) {
	return p.processFile(path, "", "")
}

// processFile generates a description, or writes copiedDescription when it is non-empty.
// copiedFrom is the source version label used only for logging/details.
func (p *Processor) processFile(path, copiedDescription, copiedFrom string) (*models.ProcessingResult, error) {
	result := &models.ProcessingResult{
		FilePath: path,
	}

	provider := p.providerFor(path)
	if provider == nil {
		result.Status = models.StatusError
		result.Details = "No provider found for file"
		return result, fmt.Errorf("no provider can handle: %s", path)
	}

	result.FileType = provider.ID()

	hasDesc, err := provider.HasExistingDescription(path)
	if err != nil {
		result.Status = models.StatusError
		result.Details = fmt.Sprintf("Failed to check existing description: %v", err)
		return result, err
	}

	if hasDesc && !p.config.ForceOverwrite {
		existing := p.readExistingDescription(provider, path)
		result.Status = models.StatusSkipped
		result.Description = existing
		result.CharCount = len(existing)
		result.Details = "File already has a description"
		log.Printf("SKIPPED: %s - %s", path, result.Details)
		return result, nil
	}

	var description string
	if copiedDescription != "" {
		description = copiedDescription
	} else {
		generated, genErr := p.generateDescription(provider, path, result)
		if genErr != nil {
			return result, genErr
		}
		if generated == "" {
			return result, nil
		}
		description = generated
	}

	result.Description = description
	result.CharCount = len(description)

	if err := provider.WriteDescription(path, description, p.config.DryRun, p.config.UpdateRevdate); err != nil {
		result.Status = models.StatusError
		result.Details = fmt.Sprintf("Failed to write description: %v", err)
		return result, err
	}

	switch {
	case p.config.DryRun:
		result.Status = models.StatusDryRun
		if copiedFrom != "" {
			result.Details = fmt.Sprintf("would copy from version %s", copiedFrom)
			log.Printf("[DRY RUN] Would copy from %s: %s (%d chars)\n  → %s", copiedFrom, path, result.CharCount, description)
		} else {
			log.Printf("[DRY RUN] Would update: %s (%d chars)\n  → %s", path, result.CharCount, description)
		}
	case copiedFrom != "":
		result.Status = models.StatusCopied
		result.Details = fmt.Sprintf("copied from version %s", copiedFrom)
		log.Printf("COPIED from %s: %s (%d chars)\n  → %s", copiedFrom, path, result.CharCount, description)
	case hasDesc:
		result.Status = models.StatusReplaced
		log.Printf("REPLACED: %s (%d chars)\n  → %s", path, result.CharCount, description)
	default:
		result.Status = models.StatusAdded
		log.Printf("ADDED: %s (%d chars)\n  → %s", path, result.CharCount, description)
	}

	return result, nil
}

func (p *Processor) generateDescription(provider providers.ContentProvider, path string, result *models.ProcessingResult) (string, error) {
	content, err := provider.Extract(path, p.attrsFor(path))
	if err != nil {
		result.Status = models.StatusError
		result.Details = fmt.Sprintf("Failed to extract content: %v", err)
		return "", err
	}

	const minContentLength = 150
	if len(content.RawContent) < minContentLength && strings.TrimSpace(content.Title) == "" && strings.TrimSpace(content.RawContent) == "" {
		result.Status = models.StatusWarning
		result.Details = "Empty content after extraction"
		log.Printf("WARNING: %s - %s", path, result.Details)
		return "", nil
	}

	description, err := p.generator.GenerateDescription(content.RawContent, content.Title)
	if err != nil {
		result.Status = models.StatusError
		result.Details = fmt.Sprintf("AI generation failed: %v", err)
		return "", err
	}

	correctedDesc, err := p.generator.ValidateGrammar(description)
	if err == nil && correctedDesc != "" {
		description = correctedDesc
	}

	if attrs := p.attrsFor(path); attrs != nil {
		description = attrs.Resolve(description)
		description = regexp.MustCompile(`(?:xref|link):[^\[]+\[([^\]]*)\]`).ReplaceAllString(description, "$1")
		description = regexp.MustCompile(`\s+`).ReplaceAllString(description, " ")
		description = strings.TrimSpace(description)
	}

	return description, nil
}

func (p *Processor) readExistingDescription(provider providers.ContentProvider, path string) string {
	content, err := provider.Extract(path, p.attrsFor(path))
	if err != nil || content == nil {
		return ""
	}
	return content.ExistingMeta
}

func (p *Processor) providerFor(path string) providers.ContentProvider {
	for _, prov := range p.providers {
		if prov.CanHandle(path) {
			return prov
		}
	}
	return nil
}

func (p *Processor) attrsFor(path string) *attributes.Store {
	antoraPath := discovery.FindAntoraYml(path, p.config.RootDir)
	if antoraPath == "" {
		return p.attributes
	}

	attrs, ok := p.antoraCache[antoraPath]
	if !ok {
		attrs = discovery.AntoraAttributes(path, p.config.RootDir)
		p.antoraCache[antoraPath] = attrs
		if len(attrs) > 0 {
			log.Printf("Loaded %d attributes from %s", len(attrs), antoraPath)
		}
	}

	if len(attrs) == 0 {
		return p.attributes
	}

	store := p.attributes.Clone()
	store.LoadFromMap(attrs)
	return store
}

// ProcessFiles processes multiple files, generating from the chosen version of
// each Antora component and copying those descriptions onto matching paths in
// older versions.
func (p *Processor) ProcessFiles(paths []string) ([]*models.ProcessingResult, error) {
	groups, ungrouped := discovery.GroupByComponent(paths, p.config.RootDir)
	results := make([]*models.ProcessingResult, 0, len(paths))

	for _, group := range groups {
		descByRel := make(map[string]string, len(group.SourceFiles))

		for _, f := range group.SourceFiles {
			result, err := p.processFile(f.Path, "", "")
			if err != nil {
				log.Printf("ERROR processing %s: %v", f.Path, err)
			}
			if result != nil {
				results = append(results, result)
				if result.Description != "" {
					descByRel[f.RelPath] = result.Description
				}
			}
		}

		for _, f := range group.OtherFiles {
			if desc, ok := descByRel[f.RelPath]; ok && desc != "" {
				result, err := p.processFile(f.Path, desc, group.SourceVersion)
				if err != nil {
					log.Printf("ERROR copying description to %s: %v", f.Path, err)
				}
				if result != nil {
					results = append(results, result)
				}
				continue
			}

			result, err := p.processFile(f.Path, "", "")
			if err != nil {
				log.Printf("ERROR processing %s: %v", f.Path, err)
			}
			if result != nil {
				results = append(results, result)
			}
		}
	}

	for _, path := range ungrouped {
		result, err := p.processFile(path, "", "")
		if err != nil {
			log.Printf("ERROR processing %s: %v", path, err)
		}
		if result != nil {
			results = append(results, result)
		}
	}

	return results, nil
}
