# Python Prototype (Legacy)

This directory contains the original Python proof-of-concept for the Documentation Meta Description Generator. It has been superseded by the Go implementation in [`doc-meta-gen/`](../doc-meta-gen/).

It is retained here for reference only. No further development is planned.

---

## Features

- **AI-Powered Content**: Leverages local large language models (like Llama 3.1) to generate high-quality, context-aware descriptions.
- **AsciiDoc & DocBook Support**: Processes `.adoc` and `.xml` files, correctly inserting descriptions as AsciiDoc attributes or XML tags.
- **Style Guide Compliant**: Follows strict rules to produce descriptions that are active, complete sentences within a specified character limit (120-160).
- **Grammar Validation**: Includes an AI-powered validation step to correct grammatical errors and awkward phrasing in the generated text.
- **Brand & Acronym Aware**: Uses an optional entities file to ensure brand consistency and correct usage of acronyms.
- **Conditional Attributes**: Supports complex AsciiDoc attribute files with `ifeval` and `ifndef` directives via a command-line flag.
- **Detailed Reporting**: Generates an interactive HTML report to review all changes, skips, and errors.

---

## Setup

### 1\. Install Ollama and pull a model

```bash
curl -fsSL https://ollama.com/install.sh | sh
sudo systemctl enable --now ollama
ollama pull llama3.1:8b
```

### 2\. Set up a Python virtual environment

```bash
cd /path/to/doc-lama-metagen

python3 -m venv .venv
source .venv/bin/activate
pip install requests lxml psutil
```

Activate the virtual environment (`source .venv/bin/activate`) each time before running the script.

---

## Usage

### Command-Line Arguments

| Argument | Shorthand | Description | Required |
| --- | --- | --- | --- |
| `root` | | Path to the root directory of your documentation files. | **Yes** |
| `--model` | | Ollama model to use. Defaults to `llama3.1:8b`. | No |
| `--ollama-url` | | Base URL for the Ollama API. Defaults to `http://127.0.0.1:11434`. | No |
| `--type` | | Choose which file types to process: `adoc`, `xml`, or `all`. Defaults to `all`. | No |
| `--force-overwrite` | | Overwrite existing meta descriptions if found. | No |
| `--dry-run` | | Preview changes without writing to any files. | No |
| `--html-log` | | Path to save a detailed HTML report (e.g., `report.html`). | No |
| `--report-title` | | Custom title for the HTML report. | No |
| `--verbose` | `-v` | Enable verbose DEBUG level logging. | No |
| `--attributes-file` | | Path to an `.adoc` file containing AsciiDoc attributes to be resolved. | No |
| `--build-attributes` | `-a` | Set a build attribute for conditional parsing (e.g., `build-type=product`). Repeatable. | No |
| `--entities-file` | | Optional path to an entities file (`.adoc` or `.ent`) for brand/acronym awareness. | No |
| `--banned-terms` | | Comma-separated list of terms to forbid in the final description. | No |

### Examples

**Basic dry run:**
```bash
python3 doc-lama-metagen.py /path/to/my-docs --dry-run
```

**AsciiDoc with HTML report:**
```bash
python3 doc-lama-metagen.py /path/to/my-docs --type adoc --html-log generation_report.html
```

**Conditional AsciiDoc project with entities file:**
```bash
python3 doc-lama-metagen.py /path/to/kubewarden/docs \
  --attributes-file /path/to/kubewarden/attributes.adoc \
  --entities-file /path/to/kubewarden/entities.adoc \
  -a build-type=product \
  --html-log kubewarden_report.html
```

**Force overwrite existing descriptions:**
```bash
python3 doc-lama-metagen.py /path/to/xml-docs --type xml --force-overwrite
```
