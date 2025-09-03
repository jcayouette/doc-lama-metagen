# AI Meta Description Generator

`gen_descriptions.py` is a command-line utility for automatically generating and inserting meta descriptions into AsciiDoc (`.adoc`) and DocBook (`.xml`) files. It uses a local Ollama instance with a large language model (LLM) to create concise, well-written descriptions based on the document's content.

The script is designed to follow specific technical writing style guides, ensuring the output is professional, active, and consistent. It also generates a detailed HTML report of its operations.

## Features ✨

  * **Dual Format Support**: Processes both **AsciiDoc** and **DocBook XML** files.
  * **Local LLM Integration**: Leverages **Ollama** to keep content processing private and local.
  * **Intelligent Content Extraction**: Smartly extracts relevant text from documents, ignoring boilerplate, headers, and navigation elements.
  * **Style Guide Compliant**: Uses a carefully crafted prompt to generate descriptions that start with a verb, use the active voice, and adhere to character limits (120-160 characters).
  * **Automated Retry**: If the first generated description is too short, the script automatically re-prompts the model for a longer, more detailed version.
  * **Comprehensive HTML Reporting**: Generates an interactive HTML log with statistics, system information, and a filterable list of all actions performed.
  * **Safety First**: Includes a **`--dry-run`** mode to preview all potential changes without modifying any files.
  * **Brand Consistency**: Can use an entities file to check for and correct brand name mismatches in the generated output.

-----

## Installation ⚙️

Follow these steps to set up the script and its dependencies. This guide assumes a Linux-based environment.

### 1\. Install Ollama

Ollama is required to run the local language model.

```bash
curl -fsSL https://ollama.com/install.sh | sh
```

After installation, enable and start the Ollama service:

```bash
sudo systemctl enable --now ollama
```

### 2\. Pull the LLM Model

The script is optimized for `llama3.1:8b`. Pull it from the Ollama library:

```bash
ollama pull llama3.1:8b
```

### 3\. Set Up Python Environment

It's highly recommended to use a virtual environment.

```bash
# Create a virtual environment in a .venv directory
python3 -m venv .venv

# Activate the environment
source .venv/bin/activate

# Install the required Python libraries
pip install requests lxml psutil
```

-----

## Usage 🚀

Run the script from the command line, pointing it to the root directory of your documentation. The `--entities-file` argument is required for providing context to the model.

### Basic Example

This command will scan all `.adoc` and `.xml` files in the `/path/to/your/docs` directory, generate descriptions, and create an HTML report.

```bash
python3 ./gen_descriptions.py /path/to/your/docs --entities-file /path/to/your/entities.ent --html-log report.html
```

### Dry Run Example

To see what the script *would do* without actually changing any files, use the `--dry-run` flag. This is the safest way to test your configuration.

```bash
python3 ./gen_descriptions.py /path/to/your/docs --entities-file /path/to/your/entities.ent --html-log dry_run_report.html --dry-run
```

-----

## Command-Line Arguments

Here is a full list of all available command-line options.

| Argument                | Short | Required | Description                                                                                             | Default                                     |
| ----------------------- | ----- | :------: | ------------------------------------------------------------------------------------------------------- | ------------------------------------------- |
| `root`                  |       |   Yes    | The path to the root directory containing your documentation files.                                     | N/A                                         |
| `--entities-file`       |       |   Yes    | Path to an entities file (`.adoc` or `.ent`) used to extract acronyms and brand names for the AI prompt. | N/A                                         |
| `--model`               |       |    No    | The name of the Ollama model to use for generation.                                                     | `llama3.1:8b`                               |
| `--html-log`            |       |    No    | If provided, the script will generate a detailed HTML report at this path.                              | None                                        |
| `--type`                |       |    No    | The type of files to process. Options: `adoc`, `xml`, `all`.                                            | `all`                                       |
| `--force-overwrite`     |       |    No    | A flag to force the script to overwrite descriptions that already exist in the files.                   | `False`                                     |
| `--dry-run`             |       |    No    | A flag to run the script in a simulation mode. It will log what it would change but won't write to files. | `False`                                     |
| `--report-title`        |       |    No    | A custom title for the generated HTML report. Underscores are converted to spaces.                      | `Description Generation Report`             |
| `--attributes-file`     |       |    No    | Path to an `.adoc` file containing AsciiDoc attributes to resolve `{attribute}` placeholders.           | None                                        |
| `--banned-terms`        |       |    No    | A comma-separated list of case-insensitive terms that should be removed from the final description.     | None                                        |
| `--ollama-url`          |       |    No    | The base URL for the Ollama API endpoint.                                                               | `http://127.0.0.1:11434`                    |
| `--verbose`             | `-v`  |    No    | A flag to enable verbose, debug-level logging in the console output.                                    | `False`                                     |
