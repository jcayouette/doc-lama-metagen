# Documentation Meta Description Generator

AI-powered meta description generator for technical documentation using local LLMs via Ollama.

## Overview

Uses local AI models via Ollama to automatically generate SEO-friendly meta descriptions for technical documentation (AsciiDoc and DocBook formats), following the SUSE Technical Writing Style Guide.

📖 **Documentation**:
- [doc-meta-gen/README.md](doc-meta-gen/README.md) - Complete guide
- [doc-meta-gen/USAGE.md](doc-meta-gen/USAGE.md) - Getting started
- [docs/planning/](docs/planning/) - Architecture and planning documents

> A legacy Python prototype is preserved in [`python-prototype/`](python-prototype/README.md) for reference.

---

## ⚙️ Setup and Installation

These instructions target Linux (openSUSE/SLES).

### 1\. Install Go

Requires **Go 1.25.6 or later**.

**openSUSE / SLES:**
```bash
sudo zypper install go
```

**Verify:**
```bash
go version
# go version go1.25.6 linux/amd64
```

> Alternatively, download directly from [go.dev/dl](https://go.dev/dl/).

### 2\. Clone the repository and build the binary

```bash
git clone git@github.com:jcayouette/doc-lama-metagen.git
cd doc-lama-metagen/doc-meta-gen

# Download dependencies
go mod tidy

# Build the binary
go build -o doc-meta-gen ./cmd/doc-meta-gen

# Verify
./doc-meta-gen --help
```

The resulting `doc-meta-gen` binary is self-contained — no runtime dependencies required.

### 3\. Install Ollama and pull a model

Ollama runs the local AI model. A compatible GPU gives the best performance, but CPU-only mode works too.

```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh | sh

# Enable and start the Ollama service
sudo systemctl enable --now ollama

# Recommended model (default — best quality)
ollama pull qwen3:14b

# Lighter alternative for lower-memory systems
ollama pull llama3.1:8b
```

> Use `--model llama3.1:8b` on the command line to switch models at runtime.

---

## 🚀 Usage

### All Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--root` | Root directory of documentation files (required) | - |
| `--model` | Ollama model name | `qwen3:14b` |
| `--ollama-url` | Ollama API endpoint | `http://127.0.0.1:11434` |
| `--type` | File type filter: `asciidoc`, `docbook`, `all` | `all` |
| `--attributes-file` | Path to `.adoc` or `.ent` attributes/entity file (repeatable) | - |
| `-a` | Build attribute for conditional parsing, e.g. `build-type=product` (repeatable) | - |
| `--force-overwrite` | Overwrite existing descriptions | `false` |
| `--dry-run` | Preview without writing files | `false` |
| `--html-log` | Path to save an HTML report | - |
| `--report-title` | Custom HTML report title | `Description Generation Report` |
| `--banned-terms` | Comma-separated list of terms to forbid in output | - |

### Examples

**Dry run (preview only):**
```bash
./doc-meta-gen --root /path/to/docs --dry-run
```

**AsciiDoc with attributes file and HTML report:**
```bash
./doc-meta-gen \
  --root /path/to/docs \
  --type asciidoc \
  --attributes-file attributes.adoc \
  --html-log report.html
```

**DocBook XML with multiple entity files:**
```bash
cd /path/to/doc-sleha && /path/to/doc-meta-gen/doc-meta-gen \
  --root xml \
  --type docbook \
  --attributes-file xml/phrases-decl.ent \
  --attributes-file xml/product-entities.ent \
  --attributes-file xml/network-entities.ent \
  --attributes-file xml/generic-entities.ent \
  --html-log report-meta-descriptions.html \
  --report-title "SLES HA Meta Descriptions"
```

> `--attributes-file` is repeatable — pass it once per file to load `.ent` entity definitions or AsciiDoc attribute files.

**Force overwrite existing descriptions:**
```bash
./doc-meta-gen --root /path/to/docs --type docbook --force-overwrite
```
