# doc-meta-gen

A modular Go application that generates AI-powered meta descriptions for technical documentation. Supports AsciiDoc/Antora and DocBook formats using local LLMs via Ollama.

## Features

- **Modular Architecture**: Clean separation between content providers, AI generation, and output writing
- **AsciiDoc/Antora Support**: Extracts content from `.adoc` files with full attribute resolution
- **DocBook Support**: (Coming soon) XML parsing with entity resolution
- **Local AI**: Uses Ollama for privacy-focused, local LLM inference
- **SUSE Style Guide**: Enforces technical writing best practices
- **Attribute Resolution**: Handles complex AsciiDoc conditionals and nested attributes
- **Smart Skipping**: Automatically skips navigation files and partials

## Installation

### Prerequisites

1. **Ollama** - Install and start Ollama:
   ```bash
   curl -fsSL https://ollama.com/install.sh | sh
   sudo systemctl enable --now ollama
   ```

2. **Pull an LLM model**:
   ```bash
   ollama pull llama3.1:8b
   ```

3. **Go 1.21+** - Install from https://go.dev/dl/

### Build

```bash
cd doc-meta-gen
go mod tidy
go build -o doc-meta-gen ./cmd/doc-meta-gen
```

## Usage

### Basic Command

```bash
./doc-meta-gen --root /path/to/docs
```

### Common Options

```bash
./doc-meta-gen \
  --root /path/to/docs \
  --attributes-file entities.adoc \
  --model llama3.1:8b \
  --type asciidoc \
  --dry-run
```

### All Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--root` | Root directory of documentation files (required) | - |
| `--model` | Ollama model name | `llama3.1:8b` |
| `--ollama-url` | Ollama API endpoint | `http://127.0.0.1:11434` |
| `--attributes-file` | Path to AsciiDoc attributes file | - |
| `--type` | File type: `asciidoc`, `docbook`, `all` | `all` |
| `--force-overwrite` | Overwrite existing descriptions | `false` |
| `--dry-run` | Preview without writing files | `false` |
| `--html-log` | Path to HTML report | - |
| `--report-title` | Custom HTML report title | `Description Generation Report` |
| `--banned-terms` | Comma-separated blacklist | - |
| `-a` | Build attribute (repeatable) | - |

### Examples

**Dry run with attributes:**
```bash
./doc-meta-gen \
  --root ./docs \
  --attributes-file attributes.adoc \
  --dry-run
```

**Process only AsciiDoc with custom model:**
```bash
./doc-meta-gen \
  --root ./docs \
  --type asciidoc \
  --model mistral:latest
```

**Conditional build attributes:**
```bash
./doc-meta-gen \
  --root ./docs \
  --attributes-file attributes.adoc \
  -a build-type=product \
  -a platform=linux
```

## Architecture

### Module Structure

```
doc-meta-gen/
├── cmd/doc-meta-gen/          # CLI entry point
├── internal/
│   ├── discovery/             # File scanning
│   ├── providers/             # Content extractors
│   │   ├── provider.go        # Interface definition
│   │   ├── asciidoc/          # AsciiDoc implementation
│   │   └── docbook/           # DocBook (future)
│   ├── ai/                    # AI orchestration
│   │   ├── generator.go       # Description generation
│   │   └── ollama/            # Ollama client
│   ├── models/                # Shared data structures
│   ├── processor/             # Processing pipeline
│   └── writer/                # Output writing
└── pkg/
    └── attributes/            # Attribute resolution
```

### Pipeline Flow

```
Discovery → Parsing → AI Generation → Validation → Output
```

1. **Discovery**: Scan directories for processable files
2. **Parsing**: Extract content with resolved attributes
3. **AI Generation**: Create description using LLM
4. **Validation**: Check grammar and length
5. **Output**: Write back to file

## Style Guide Rules

Generated descriptions must:

- Be 120-160 characters (ONE sentence)
- Use active voice (focus on what user can DO)
- Start with a verb ("Learn", "Configure", "Deploy")
- Avoid version numbers
- Never use self-referential phrases ("This chapter describes...")
- Not end with a period
- Avoid possessives with apostrophes
- Maintain neutral, professional tone

## Development

### Adding a New Provider

1. Create a new package in `internal/providers/`
2. Implement the `ContentProvider` interface:
   ```go
   type ContentProvider interface {
       ID() string
       CanHandle(path string) bool
       Extract(path string, attrs *attributes.Store) (*models.PageContent, error)
       HasExistingDescription(path string) (bool, error)
       WriteDescription(path string, description string, dryRun bool) error
   }
   ```
3. Register in `main.go`:
   ```go
   providerList := []providers.ContentProvider{
       asciidoc.NewProvider(),
       yournew.NewProvider(),
   }
   ```

### Testing

```bash
go test ./...
```

### Running Tests

```bash
# Unit tests
go test -v ./internal/...

# Integration test with sample docs
./doc-meta-gen --root ./testdata --dry-run
```

## Troubleshooting

### Ollama Connection Failed

Ensure Ollama is running:
```bash
systemctl status ollama
curl http://127.0.0.1:11434/api/tags
```

### Model Not Found

Pull the required model:
```bash
ollama pull llama3.1:8b
```

### Descriptions Too Short

Try a more capable model:
```bash
./doc-meta-gen --root ./docs --model llama3.1:70b
```

## Roadmap

- [x] AsciiDoc/Antora provider
- [x] Attribute resolution with conditionals
- [x] Ollama integration
- [x] Grammar validation pass
- [ ] DocBook XML provider
- [ ] HTML report generation
- [ ] Parallel processing
- [ ] Progress bar
- [ ] Brand consistency checking
- [ ] Unit tests
- [ ] Integration tests

## License

See [LICENSE](../LICENSE) file.

## Contributing

Contributions welcome! Please ensure:
- Code follows Go conventions (`go fmt`, `go vet`)
- New providers implement the full interface
- Tests cover new functionality

---

**See also**: [PROJECT.md](../PROJECT.md) for detailed architecture documentation
