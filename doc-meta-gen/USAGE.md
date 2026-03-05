# Getting Started with doc-meta-gen

This guide walks you through using doc-meta-gen to generate AI-powered meta descriptions for your documentation.

## Quick Start

### 1. Prerequisites Check

Ensure Ollama is running:
```bash
systemctl status ollama
```

Test Ollama connection:
```bash
curl http://127.0.0.1:11434/api/tags
```

### 2. Your First Run

Test with the included sample files:
```bash
cd /home/scribe/doc-lama-metagen/doc-meta-gen

./doc-meta-gen \
  --root ./testdata \
  --attributes-file ./testdata/attributes.adoc \
  --dry-run
```

This will:
- Scan the `testdata/` directory
- Find `.adoc` files (skipping `nav.adoc`)
- Load attributes from `attributes.adoc`
- Generate descriptions using AI
- Show what would be written (without modifying files)

### 3. Check the Output

You should see output like:
```
Starting doc-meta-gen
Root directory: ./testdata
Model: llama3.1:8b
Ollama URL: http://127.0.0.1:11434
Loading attributes from: ./testdata/attributes.adoc
Loaded 3 attributes
Checking Ollama connection...
✓ Ollama is available
Scanning for files...
Found 3 potential files
Processing 2 files (type filter: all)
*** DRY RUN MODE - No files will be modified ***
[DRY RUN] Would update: ./testdata/network-guide.adoc (145 chars)
[DRY RUN] Would update: ./testdata/security-hardening.adoc (152 chars)

=== Processing Complete ===
Total time: 15.32 seconds
Files processed: 2
Added: 0
Replaced: 0
Skipped: 0
Errors: 0
Warnings: 0
Dry run: 2 files would be modified
```

### 4. Actually Write the Descriptions

Remove `--dry-run` to write changes:
```bash
./doc-meta-gen \
  --root ./testdata \
  --attributes-file ./testdata/attributes.adoc
```

### 5. Verify the Changes

Check that descriptions were added:
```bash
head -n 3 ./testdata/network-guide.adoc
```

You should see:
```asciidoc
= Configuring Network Services
:description: Learn how to configure DHCP, DNS, and firewall rules on your system using YaST and command-line tools for comprehensive network management
```

## Common Use Cases

### Processing a Real Documentation Project

```bash
./doc-meta-gen \
  --root /path/to/your/docs \
  --attributes-file /path/to/your/entities.adoc \
  --type asciidoc \
  --model llama3.1:8b
```

### Using a Different Model

For better quality (if you have more RAM/VRAM):
```bash
./doc-meta-gen \
  --root ./docs \
  --model llama3.1:70b
```

Or use a faster model:
```bash
./doc-meta-gen \
  --root ./docs \
  --model mistral:7b
```

### Overwriting Existing Descriptions

Force regeneration of all descriptions:
```bash
./doc-meta-gen \
  --root ./docs \
  --force-overwrite
```

### Banning Specific Terms

Exclude competitor or incorrect product names:
```bash
./doc-meta-gen \
  --root ./docs \
  --banned-terms "Windows,macOS,RedHat"
```

### Conditional Builds

For documentation that uses conditionals:
```bash
./doc-meta-gen \
  --root ./docs \
  --attributes-file attributes.adoc \
  -a build-type=product \
  -a platform=x86_64 \
  -a variant=enterprise
```

This resolves conditionals like:
```asciidoc
ifeval::["{build-type}" == "product"]
This content only appears in product builds.
endif::[]
```

### Using Environment Variables

Set Ollama URL via environment:
```bash
export OLLAMA_URL=http://192.168.1.100:11434
./doc-meta-gen --root ./docs
```

## Understanding File Selection

### What Gets Processed

The tool will process:
- ✅ `.adoc` files in any directory
- ✅ Files with content (skips empty files)
- ✅ Files without `_` prefix

### What Gets Skipped

The tool automatically skips:
- ❌ Files starting with `_` (e.g., `_partial.adoc`)
- ❌ Navigation files (`nav.adoc`, `nav-admin-guide.adoc`)
- ❌ Files in `nav/`, `navigation/`, or `partials/` directories
- ❌ Files with existing `:description:` (unless `--force-overwrite`)

### Example Directory Structure

```
docs/
├── index.adoc              ✅ Processed
├── installation.adoc       ✅ Processed
├── _attributes.adoc        ❌ Skipped (underscore prefix)
├── nav.adoc                ❌ Skipped (navigation file)
├── partials/
│   └── version.adoc        ❌ Skipped (in partials dir)
└── modules/
    ├── admin/
    │   ├── nav.adoc        ❌ Skipped (navigation)
    │   └── config.adoc     ✅ Processed
    └── user/
        └── tutorial.adoc   ✅ Processed
```

## Attribute Resolution

### How Attributes Work

1. Attributes are loaded from the specified file
2. Conditionals (`ifndef`, `ifeval`) are evaluated
3. Nested attributes are resolved iteratively
4. All `{attribute}` references in content are replaced
5. Resolved content is sent to the AI

### Example Attributes File

```asciidoc
:productname: SUSE Linux Enterprise Server
:productnumber: 15
:version: {productnumber} SP6

ifndef::platform[]
:platform: x86_64
endif::[]

ifeval::["{platform}" == "x86_64"]
:arch-notes: Intel and AMD processors
endif::[]
```

### In Your Documentation

```asciidoc
= Installing {productname}

This guide covers {productname} {version} installation on {platform} systems.

{arch-notes}
```

Becomes (after resolution):
```
Installing SUSE Linux Enterprise Server

This guide covers SUSE Linux Enterprise Server 15 SP6 installation on x86_64 systems.

Intel and AMD processors
```

## Output Formats

### Console Output

Standard logging shows:
- Files being processed
- Status (ADDED, REPLACED, SKIPPED)
- Character counts
- Final statistics

### File Output

For AsciiDoc files, descriptions are written as document attributes:
```asciidoc
= Page Title
:description: Your generated description here

Content starts here...
```

### HTML Reports (Coming Soon)

Will generate detailed reports with:
- Processing timeline
- Success/failure status
- Generated descriptions
- Statistics and metrics

## Troubleshooting

### "ERROR: Cannot connect to Ollama"

**Solution**: Start Ollama service
```bash
sudo systemctl start ollama
```

### "ERROR: Model not found"

**Solution**: Pull the model first
```bash
ollama pull llama3.1:8b
```

### Descriptions Too Short

The tool automatically retries if descriptions are < 100 chars. If still failing:

1. Try a larger model:
   ```bash
   ollama pull llama3.1:70b
   ./doc-meta-gen --root ./docs --model llama3.1:70b
   ```

2. Check if source content is too short
3. Verify attributes are resolving correctly

### Descriptions Don't Follow Style Guide

Ensure you're using a capable model. Smaller models may struggle:
- ✅ Good: `llama3.1:8b`, `llama3.1:70b`, `mistral:7b`
- ⚠️ May struggle: `tinyllama`, `phi`

### Attributes Not Resolving

Check that:
1. Attributes file path is correct
2. Attribute syntax is valid (`:key: value`)
3. Conditionals are properly closed (`endif::[]`)

Debug by checking loaded attributes:
```bash
# The tool logs "Loaded N attributes"
./doc-meta-gen --root ./docs --attributes-file attrs.adoc
```

## Performance Tips

### Processing Large Projects

For projects with 100+ files:

1. **Use dry-run first** to validate:
   ```bash
   ./doc-meta-gen --root ./docs --dry-run
   ```

2. **Process in batches** by directory:
   ```bash
   ./doc-meta-gen --root ./docs/admin
   ./doc-meta-gen --root ./docs/user
   ```

3. **Use a faster model** for initial runs:
   ```bash
   ./doc-meta-gen --root ./docs --model mistral:7b
   ```

### Optimizing Ollama

For best performance with RTX 5070 Ti:

1. Ensure GPU is being used:
   ```bash
   nvidia-smi
   ```

2. Use quantized models for speed:
   ```bash
   ollama pull llama3.1:8b-q4_0
   ```

3. Adjust Ollama settings in `/etc/systemd/system/ollama.service`:
   ```ini
   [Service]
   Environment="OLLAMA_NUM_PARALLEL=4"
   Environment="OLLAMA_MAX_LOADED_MODELS=2"
   ```

## Best Practices

1. **Always test with `--dry-run` first**
2. **Use version control** to track changes
3. **Review generated descriptions** for accuracy
4. **Maintain an attributes file** for consistency
5. **Ban competitor terms** using `--banned-terms`
6. **Use meaningful build attributes** for conditionals

## Next Steps

- Read [PROJECT.md](../PROJECT.md) for architecture details
- Review [README.md](README.md) for complete flag documentation
- Check the Python implementation for comparison
- Contribute improvements via pull requests

---

**Questions?** Check the troubleshooting section or review the log output for specific error messages.
