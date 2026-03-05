# Documentation Meta Description Generator

AI-powered meta description generator for technical documentation using local LLMs via Ollama.

## 🎯 Overview

This repository contains **two implementations** of an AI-powered meta description generator:

1. **🐹 Go Implementation** ([`doc-meta-gen/`](doc-meta-gen/)) - Modern, modular, production-ready
2. **🐍 Python Implementation** ([`doc-lama-metagen.py`](doc-lama-metagen.py)) - Original reference implementation

Both use local AI models via Ollama to automatically generate SEO-friendly meta descriptions for technical documentation (AsciiDoc and DocBook formats), following the SUSE Technical Writing Style Guide.

## 🐹 Go Implementation (Recommended)

**Location**: [`doc-meta-gen/`](doc-meta-gen/)

Modern, modular implementation with clean architecture:

### Advantages
- ✅ **Modular design**: Provider-based architecture for easy extensibility
- ✅ **Type safety**: Compile-time error checking
- ✅ **Single binary**: No dependencies, easy deployment
- ✅ **Better performance**: Lower memory usage (~50-80 MB)
- ✅ **Production-ready**: Interface-driven, testable code

### Quick Start
```bash
cd doc-meta-gen
go build -o doc-meta-gen ./cmd/doc-meta-gen
./doc-meta-gen --root ./testdata --dry-run
```

📖 **Documentation**: 
- [README.md](doc-meta-gen/README.md) - Complete guide
- [USAGE.md](doc-meta-gen/USAGE.md) - Getting started
- [PROJECT.md](PROJECT.md) - Architecture details

---

## 🐍 Python Implementation (Reference)

**Location**: [`doc-lama-metagen.py`](doc-lama-metagen.py)

Original proof-of-concept with full feature set:

### Features

  * **AI-Powered Content**: Leverages local large language models (like Llama 3.1) to generate high-quality, context-aware descriptions.
  * **AsciiDoc & DocBook Support**: Processes `.adoc` and `.xml` files, correctly inserting descriptions as AsciiDoc attributes or XML tags.
  * **Style Guide Compliant**: Follows strict rules to produce descriptions that are active, complete sentences within a specified character limit (120-160).
  * **Grammar Validation**: Includes an AI-powered validation step to correct grammatical errors and awkward phrasing in the generated text.
  * **Brand & Acronym Aware**: Uses an optional entities file to ensure brand consistency and correct usage of acronyms.
  * **Conditional Attributes**: Supports complex AsciiDoc attribute files with `ifeval` and `ifndef` directives via a command-line flag.
  * **Detailed Reporting**: Generates an interactive HTML report to review all changes, skips, and errors.

-----

## ⚙️ Setup and Installation

Follow these steps to set up the required tools and Python environment. These instructions are based on a Linux environment like openSUSE Leap.

### 1\. Install Ollama

Ollama runs the local AI model. You'll need a compatible GPU for the best performance, but it can also run in CPU-only mode.

```bash
# Download and run the Ollama installation script
curl -fsSL https://ollama.com/install.sh | sh

# Enable and start the Ollama service
sudo systemctl enable --now ollama
```

### 2\. Pull the AI Model

This script is optimized for `llama3.1:8b`. Pull it using the following command:

```bash
ollama pull llama3.1:8b
```

### 3\. Set Up the Python Environment

It is highly recommended to use a Python virtual environment (`venv`) to manage dependencies.

```bash
# Navigate to the repository directory
cd /path/to/doc-lama-metagen

# Create a virtual environment named .venv
python3 -m venv .venv

# Activate the virtual environment
source .venv/bin/activate

# Install the required Python libraries
pip install requests lxml psutil
```

You are now ready to run the script. Remember to activate the virtual environment (`source .venv/bin/activate`) in your terminal session each time you want to use it.

-----

## 🚀 Usage

The script is run from the command line. The only required argument is the path to the directory containing your documentation files.

### Command-Line Arguments

Here is a complete list of all available options and flags:

| Argument | Shorthand | Description | Required |
| --- | --- | --- | --- |
| `root` | | Path to the root directory of your documentation files. | **Yes** |
| `--model` | | Ollama model to use. Defaults to `llama3.1:8b`. | No |
| `--ollama-url` | | Base URL for the Ollama API. Defaults to `http://127.0.0.1:11434`. | No |
| `--type` | | Choose which file types to process: `adoc`, `xml`, or `all`. Defaults to `all`. | No |
| `--force-overwrite` | | Overwrite existing meta descriptions if found. | No |
| `--dry-run` | | Preview changes without writing to any files. Highly recommended for the first run. | No |
| `--html-log` | | Path to save a detailed HTML report (e.g., `report.html`). | No |
| `--report-title`| | Custom title for the HTML report. Defaults to "Description Generation Report". | No |
| `--verbose` | `-v` | Enable verbose DEBUG level logging to the console. | No |
| `--attributes-file` | | Path to an `.adoc` file containing AsciiDoc attributes to be resolved. | No |
| `--build-attributes` | `-a` | Set a build attribute for conditional parsing (e.g., `build-type=product`). Can be used multiple times. | No |
| `--entities-file`| | Optional path to an entities file (`.adoc` or `.ent`) for brand/acronym awareness. | No |
| `--banned-terms`| | Comma-separated list of terms to forbid in the final description. | No |

### Examples

#### Basic Run

Process all `.adoc` and `.xml` files in a directory, showing what would change without modifying files.

```bash
python3 doc-lama-metagen.py /path/to/my-docs --dry-run
```

#### Generating Descriptions with an HTML Report

Process only AsciiDoc files and generate an interactive report of all actions.

```bash
python3 doc-lama-metagen.py /path/to/my-docs --type adoc --html-log generation_report.html
```

#### Advanced Run for a Conditional AsciiDoc Project

Process a project that uses a complex attributes file (like Kubewarden), setting the build context to `product`. This example also uses an entities file for brand consistency.

```bash
python3 doc-lama-metagen.py /path/to/kubewarden/docs \
  --attributes-file /path/to/kubewarden/attributes.adoc \
  --entities-file /path/to/kubewarden/entities.adoc \
  -a build-type=product \
  --html-log kubewarden_report.html
```

#### Forcing an Update

Overwrite all existing meta descriptions in a DocBook XML project. **Use with caution\!**

```bash
python3 doc-lama-metagen.py /path/to/xml-docs --type xml --force-overwrite
```
