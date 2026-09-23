package providers

import (
	"github.com/scribe/doc-meta-gen/internal/models"
	"github.com/scribe/doc-meta-gen/pkg/attributes"
)

// ContentProvider defines the interface for extracting content from different file formats
type ContentProvider interface {
	// ID returns a unique identifier for this provider (e.g., "asciidoc", "docbook")
	ID() string

	// CanHandle determines if this provider can process the given file
	CanHandle(path string) bool

	// Extract processes a file and returns standardized page content
	Extract(path string, attrs *attributes.Store) (*models.PageContent, error)

	// HasExistingDescription checks if the file already has a meta description
	HasExistingDescription(path string) (bool, error)

	// WriteDescription writes the generated description back to the file.
	// When updateRevdate is true, :revdate: is set to today. When false, :revdate:
	// and :page-revdate: are left unchanged.
	WriteDescription(path string, description string, dryRun bool, updateRevdate bool) error

	// RemoveDescriptions removes both :description: and :prev-description: attributes
	RemoveDescriptions(path string, dryRun bool) error
}
