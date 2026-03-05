package models

// PageContent represents the standardized format for extracted documentation content
type PageContent struct {
	FilePath     string            // Absolute path to the file
	FileType     string            // "AsciiDoc", "DocBook", etc.
	Title        string            // Page title
	RawContent   string            // Extracted content for AI processing
	ExistingMeta string            // Current description if exists
	Metadata     map[string]string // Additional metadata
}

// MetaDescription represents the generated description result
type MetaDescription struct {
	Content      string // The generated description text
	CharCount    int    // Character count
	Status       Status // Processing status
	ErrorMessage string // Error details if failed
}

// Status represents the processing outcome
type Status string

const (
	StatusAdded    Status = "ADDED"
	StatusReplaced Status = "REPLACED"
	StatusUpdated  Status = "UPDATED"
	StatusSkipped  Status = "SKIPPED"
	StatusError    Status = "ERROR"
	StatusDryRun   Status = "DRY_RUN"
	StatusWarning  Status = "WARNING"
)

// ProcessingResult tracks the outcome of processing a single file
type ProcessingResult struct {
	FilePath    string
	FileType    string
	Status      Status
	Description string
	Details     string
	CharCount   int
}

// Config holds the application configuration
type Config struct {
	RootDir            string
	ModelName          string
	OllamaURL          string
	AttributesFile     string
	FileType           string // "asciidoc", "docbook", "all"
	ForceOverwrite     bool
	DryRun             bool
	RemoveDescriptions bool   // Remove description attributes instead of generating
	HTMLLogPath        string
	ReportTitle        string
	BannedTerms        []string
	BuildAttributes    map[string]string // For conditional processing
}
