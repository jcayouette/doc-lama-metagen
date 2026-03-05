# Documentation Meta Description Generator

## Project Overview
A modular Go application that generates AI-powered meta descriptions for technical documentation pages. Supports AsciiDoc/Antora and DocBook formats with pluggable content providers.

## Architecture

### Core Pipeline
```
Discovery → Parsing → AI Generation → Output
```

### Module Structure
```
doc-meta-gen/
├── cmd/
│   └── doc-meta-gen/        # Main entry point
├── internal/
│   ├── discovery/            # File discovery logic
│   ├── providers/            # Content provider implementations
│   │   ├── provider.go       # ContentProvider interface
│   │   ├── asciidoc/         # AsciiDoc/Antora provider
│   │   └── docbook/          # DocBook XML provider (future)
│   ├── ai/                   # AI/LLM orchestration
│   │   └── ollama/           # Ollama client
│   ├── models/               # Shared data structures
│   ├── processor/            # Main processing engine
│   └── writer/               # Output writing logic
├── pkg/
│   └── attributes/           # Attribute/entity resolution
└── go.mod
```

## Key Interfaces

### ContentProvider
```go
type ContentProvider interface {
    ID() string
    CanHandle(path string) bool
    Extract(path string, attrs *attributes.Store) (*models.PageContent, error)
}
```

### PageContent
```go
type PageContent struct {
    FilePath    string
    Title       string
    RawContent  string            // Extracted content for AI
    ExistingMeta string           // Current description if exists
    Metadata    map[string]string
}
```

## SUSE Style Guide Rules

Meta descriptions must follow these strict rules:

1. **Length**: 120-160 characters (ONE complete sentence)
2. **Voice**: Active voice - focus on what user can DO or LEARN
3. **Start**: Begin with a verb (e.g., "Learn how to...", "Configure...")
4. **Restrictions**:
   - NO version numbers (unless critical)
   - NO self-referential phrases ("This chapter describes...")
   - NO meta-references ("meta description", "summary")
   - NO conversational filler or preamble
   - NO possessives with apostrophes (use "the YaST tools" not "YaST's tools")
   - NO periods at the end
5. **Tone**: Neutral, professional, direct - avoid jargon and marketing language

## AI Prompt Strategy

The prompt template should:
- Include the style guide rules
- Accept a blacklist of banned terms/brands
- Request only the description (no explanation)
- Include page content with resolved attributes
- Support retry for short descriptions

## Features

### Phase 1: AsciiDoc/Antora (Current)
- [x] Project structure setup
- [ ] Attribute file parser
- [ ] AsciiDoc content extraction
- [ ] Antora-aware navigation skipping
- [ ] Ollama AI client
- [ ] Description validation (120-160 chars)
- [ ] AsciiDoc file writer (`:description:` attribute)
- [ ] HTML report generation

### Phase 2: DocBook (Future)
- [ ] DocBook XML parser
- [ ] Entity resolution
- [ ] `<meta name="description">` injection
- [ ] ITS namespace handling

### Phase 3: Enhancements
- [ ] Dry-run mode
- [ ] Force overwrite flag
- [ ] Brand consistency checking
- [ ] Grammar validation pass
- [ ] Parallel processing
- [ ] Progress bar

## Configuration

### CLI Flags
```bash
--root              # Documentation root directory
--model             # Ollama model (default: llama3.1:8b)
--ollama-url        # Ollama API endpoint
--attributes-file   # Path to attributes/entities file
--type              # File type: asciidoc, docbook, all
--force-overwrite   # Overwrite existing descriptions
--dry-run           # Preview without writing
--html-log          # Generate HTML report
--report-title      # Custom report title
--banned-terms      # Comma-separated blacklist
```

### Example Usage
```bash
doc-meta-gen \
  --root /path/to/docs \
  --attributes-file entities.adoc \
  --model llama3.1:8b \
  --html-log report.html \
  --dry-run
```

## Attribute Resolution

The tool must:
1. Load attribute files (AsciiDoc `:key: value` format)
2. Handle conditional blocks (`ifndef`, `ifeval`)
3. Iteratively resolve nested attributes (`{productname} → "SUSE Linux"`)
4. Provide resolved context to AI for accurate generation

## File Skipping Rules (Antora)

Skip files that match:
- Start with underscore (`_partial.adoc`)
- Navigation files (`nav.adoc`, `nav-*-guide.adoc`)
- Files in `nav/`, `navigation/`, or `partials/` directories

## AI Workflow

1. Extract page content (title + body)
2. Resolve all `{attributes}` in content
3. Send to Ollama with prompt template
4. Sanitize response (remove meta-references, check length)
5. If < 120 chars, retry with "expand" prompt
6. Validate grammar (optional second AI pass)
7. Final cleanup and write back to file

## Dependencies

```go
require (
    github.com/bytesparadise/libasciidoc latest
    github.com/spf13/cobra latest
)
```

## Testing Strategy

1. Unit tests for each provider
2. Mock AI responses for deterministic testing
3. Integration tests with sample docs
4. Benchmark attribute resolution performance

## Success Criteria

- Generates descriptions within 120-160 character range
- Follows all SUSE style guide rules
- Correctly resolves attributes before AI processing
- Modular design allows easy addition of new providers
- Produces actionable HTML reports

---

**Last Updated**: 2026-01-21
**Status**: Phase 1 - Initial Development
