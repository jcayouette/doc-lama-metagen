#!/usr/bin/env python3
#
# gen_descriptions.py — A generic script to generate meta descriptions for AsciiDoc and DocBook files.
#
# --- Setup Guide (e.g., openSUSE Leap 15.6) ---
# 1. Install Ollama:
#    curl -fsSL https://ollama.com/install.sh | sh
#    sudo systemctl enable --now ollama
# 2. Pull Model:
#    ollama pull llama3.1:8b
# 3. Setup Python Environment:
#    python3 -m venv .venv
#    source .venv/bin/activate
#    pip install requests lxml psutil
# 4. Run Script:
#    python3 ./gen_descriptions.py /path/to/docs --entities-file /path/to/entities.adoc --html-log report.html
# --- End Guide ---

import argparse
import logging
import os
import re
import html
import json
import requests
import subprocess
import time
import sys
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Set, List, Dict, Any
from lxml import etree as ET

try:
    import psutil
except ImportError:
    print("ERROR: The 'psutil' library is required. Please run 'pip install psutil'.")
    sys.exit(1)

# =========================
# Configuration
# =========================

@dataclass
class ScriptConfig:
    """Groups all configuration settings and pre-compiled regex patterns."""
    BANNED_LITERALS: Set[str] = field(default_factory=set)
    
    PROMPT_TMPL: str = """You are an expert technical writer following the SUSE Style Guide.

Your task is to write a single, compelling meta description for the provided documentation content.

Follow these rules strictly:
- Write ONE complete sentence between 120 and 160 characters.
- Use the active voice. Focus on what the user can DO or LEARN.
- Start the sentence with a verb (e.g., "Learn how to...", "Configure...", "Deploy...").
- Do NOT include specific version numbers unless they are critical to the content.
- Do NOT use self-referential phrases like "This chapter describes", "In this document", or "This section explains".
- NEVER mention that you are writing a "meta description", "summary", or any similar term. The output must not refer to itself.
- Your output must NOT contain any conversational filler, preamble, or explanations. Start the response directly with the first word of the description sentence.
- The sentence MUST be grammatically complete and MUST NOT end with a period.
- Avoid possessives that use an apostrophe (like 's). Rephrase the sentence if necessary (for example, instead of "YaST's tools", write "the YaST tools").
- If the content is primarily a list of topics, describe the page's purpose as a central point for accessing that information.
- If specified, do NOT use the following product or brand names: {blacklist}.
- Maintain a neutral, professional, and direct tone. Avoid jargon, marketing language, and emojis.
- CRITICAL: Do NOT include any part of these instructions in your output. Output ONLY the meta description itself.

Page content:
---
{content}
---

Your response must contain ONLY the meta description sentence, nothing else:
"""

    PROMPT_TMPL_RETRY: str = """You are an expert technical writer. Your previous attempt to write a meta description was too short.

You MUST now generate a longer, more detailed single-sentence description for the same content.

Follow these rules strictly:
- Your primary goal is to write a sentence that is between 120 and 160 characters.
- Expand on the key concepts. Explain what the user can achieve or understand from the content.
- Start the sentence with a verb (e.g., "Learn how to...", "Configure...", "Deploy...").
- Do NOT use self-referential phrases like "This chapter describes" or "This document explains".
- The sentence MUST be grammatically complete and MUST NOT end with a period.
- Avoid possessives that use an apostrophe (like 's). Rephrase the sentence if necessary (for example, instead of "YaST's tools", write "the YaST tools").
- Your output must NOT contain any preamble or explanation. Start directly with the description.
- If specified, do NOT use the following product or brand names: {blacklist}.
- CRITICAL: Do NOT include any part of these instructions in your output. Output ONLY the meta description itself.

---
Page content:
---
{content}
---

Your response must contain ONLY the meta description sentence, nothing else:
"""

    PROMPT_TMPL_VALIDATE: str = """You are an expert copy editor. Your task is to correct any grammatical errors, awkward phrasing, or structural issues in the following sentence.

Follow these rules strictly:
- The sentence must be a single, complete thought that is grammatically correct and easy to read.
- Do NOT change the original meaning or key technical terms.
- Remove any redundant or nonsensical phrases (e.g., "on your or", "and system").
- Ensure the sentence does NOT end with a period.
- If the sentence is already perfect, return it unchanged.
- Output ONLY the corrected sentence. Do not add any preamble or explanation.

Original sentence:
---
{sentence}
---
Corrected sentence:
"""

    FORBIDDEN_CHARS_RE: re.Pattern = re.compile(r'[>:|“”"‘’]')
    DOC_META_PATTERNS: List[re.Pattern] = field(init=False)
    TRAILING_STOPWORDS: Set[str] = field(default_factory=lambda: set("and or to for with in of on at by from into via as that which including such as than then while when where".split()))
    TITLE_RE: re.Pattern = re.compile(r"^\s*=\s+.+")
    DESC_RE: re.Pattern = re.compile(r"^:\s*description\s*:\s*", re.IGNORECASE)
    NAV_GENERIC_RE: re.Pattern = re.compile(r"^nav(?:-.+)?\.adoc$", re.IGNORECASE)
    NAV_GUIDE_RE: re.Pattern = re.compile(r"^nav-.+-guide\.adoc$", re.IGNORECASE)

    def __post_init__(self):
        self.DOC_META_PATTERNS = [re.compile(p, re.IGNORECASE) for p in [r"^\s*This\s+(guide|page|document|section)\s+(describes|covers|explains|provides)\s+", r"^\s*In\s+this\s+(guide|page|document|section)\s+", r"^\s*The\s+(guide|page|document|section)\s+(describes|covers|explains|provides)\s+"]]

# =========================
# System Info & HTML Report
# =========================

def get_system_info():
    info = {"ollama_version": "Not Found", "gpu_info": "NVIDIA GPU not detected", "ram_total": "N/A"}
    try:
        result = subprocess.run(['ollama', '--version'], capture_output=True, text=True, check=True)
        info['ollama_version'] = result.stdout.strip()
    except (FileNotFoundError, subprocess.CalledProcessError):
        logging.warning("Could not determine Ollama version. Is 'ollama' in your system's PATH?")
    try:
        result = subprocess.run(['nvidia-smi', '--query-gpu=gpu_name,memory.total', '--format=csv,noheader'], capture_output=True, text=True, check=True)
        info['gpu_info'] = result.stdout.strip()
    except (FileNotFoundError, subprocess.CalledProcessError): pass
    try:
        mem = psutil.virtual_memory()
        info['ram_total'] = f"{mem.total / (1024**3):.2f} GB"
    except Exception as e:
        logging.warning(f"Could not determine system RAM: {e}")
    return info

def add_html_log_entry(log_list, file_path, file_type, status, details=""):
    if log_list is None: return
    log_list.append({"timestamp": datetime.now().strftime('%H:%M:%S'), "filepath": str(file_path), "type": file_type, "status": status, "details": details})

def generate_html_report(output_path, directory, model, duration, files_processed, files_changed, log_entries, system_info, report_title):
    logging.info(f"Generating HTML report at {output_path}")

    def truncate_path(path_str):
        """Shortens the path to start from the 'doc-<product>' directory."""
        try:
            parts = path_str.split(os.path.sep)
            for i, part in enumerate(parts):
                if part.startswith('doc-'):
                    return os.path.join(*parts[i:])
            if len(parts) >= 2:
                return os.path.join(parts[-2], parts[-1])
            return path_str
        except Exception:
            return path_str

    status_styles = {
        "ADDED":    {"bg": "bg-green-500", "border": "border-green-500"},
        "REPLACED": {"bg": "bg-green-500", "border": "border-green-500"},
        "UPDATED":  {"bg": "bg-indigo-500","border": "border-indigo-500"},
        "DRY_RUN":  {"bg": "bg-blue-500",  "border": "border-blue-500"},
        "SKIPPED":  {"bg": "bg-yellow-500","border": "border-yellow-500"},
        "WARNING":  {"bg": "bg-yellow-500","border": "border-yellow-500"},
        "ERROR":    {"bg": "bg-red-500",   "border": "border-red-500"}
    }
    default_style = {"bg": "bg-gray-500", "border": "border-gray-500"}

    icon_files = '<svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" /></svg>'
    icon_changes = '<svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>'
    icon_time = '<svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" /></svg>'
    icon_model = '<svg xmlns="http://www.w3.org/2000/svg" class="h-8 w-8 text-slate-400" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" /></svg>'

    try:
        with open(output_path, 'w', encoding='utf-8') as f:
            f.write(f"""<!DOCTYPE html>
<html lang="en" class="bg-slate-100">
<head>
    <meta charset="UTF-8">
    <title>{html.escape(report_title)}: AI Generated Meta Descriptions</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="font-sans text-slate-800">
    <div class="container mx-auto p-4 sm:p-6 lg:p-8">
        
        <div class="mb-8">
            <h1 class="text-4xl font-bold text-slate-900">{html.escape(report_title)}</h1>
            <p class="text-xl text-slate-600 mt-1">AI Generated Meta Descriptions</p>
            <p class="text-sm text-slate-400 mt-2">Report generated on {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}</p>
        </div>

        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
            <div class="bg-white p-5 rounded-xl shadow-lg flex items-center justify-between"><div class="info"><div class="text-sm font-medium text-slate-500">Files Processed</div><div class="mt-1 text-3xl font-semibold text-slate-900">{files_processed}</div></div><div class="icon">{icon_files}</div></div>
            <div class="bg-white p-5 rounded-xl shadow-lg flex items-center justify-between"><div class="info"><div class="text-sm font-medium text-slate-500">Files Changed</div><div class="mt-1 text-3xl font-semibold text-green-600">{files_changed}</div></div><div class="icon">{icon_changes}</div></div>
            <div class="bg-white p-5 rounded-xl shadow-lg flex items-center justify-between"><div class="info"><div class="text-sm font-medium text-slate-500">Processing Time</div><div class="mt-1 text-3xl font-semibold text-slate-900">{duration:.2f}s</div></div><div class="icon">{icon_time}</div></div>
            <div class="bg-white p-5 rounded-xl shadow-lg flex items-center justify-between"><div class="info"><div class="text-sm font-medium text-slate-500">Model Used</div><div class="mt-1 text-2xl font-semibold text-slate-900 truncate">{html.escape(model)}</div></div><div class="icon">{icon_model}</div></div>
        </div>

        <div class="bg-white rounded-xl shadow-lg overflow-hidden">
            <div class="p-5 border-b border-slate-200 flex flex-col sm:flex-row justify-between items-center">
                <div>
                    <h2 class="text-xl font-semibold text-slate-900">Processing Log</h2>
                    <p class="text-sm text-slate-500 mt-1">Detailed log of all file operations.</p>
                </div>
                <input type="text" id="searchInput" onkeyup="filterTable()" placeholder="Filter by path, status, or details..." class="mt-4 sm:mt-0 w-full sm:w-64 px-3 py-2 border border-slate-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500">
            </div>
            <div class="overflow-x-auto">
                <table class="min-w-full" id="logTable">
                    <thead class="bg-slate-100 text-slate-600 text-sm font-semibold uppercase">
                        <tr>
                            <th class="py-3 px-6 text-left">Timestamp</th>
                            <th class="py-3 px-6 text-left">Filepath</th>
                            <th class="py-3 px-6 text-left">Type</th>
                            <th class="py-3 px-6 text-left">Status</th>
                            <th class="py-3 px-6 text-left">Details</th>
                        </tr>
                    </thead>
                    <tbody class="text-sm">""")
            for entry in log_entries:
                style = status_styles.get(entry["status"].upper(), default_style)
                f.write(f"""<tr class="border-b border-slate-200 hover:bg-slate-50">
                    <td class="py-3 px-6 whitespace-nowrap border-l-4 {style['border']}">{entry["timestamp"]}</td>
                    <td class="py-3 px-6 font-medium text-slate-700" title="{html.escape(entry["filepath"])}">{truncate_path(entry["filepath"])}</td>
                    <td class="py-3 px-6 whitespace-nowrap">{entry["type"]}</td>
                    <td class="py-3 px-6"><span class="text-white text-xs font-bold mr-2 px-2.5 py-1 rounded-full {style['bg']}">{entry["status"]}</span></td>
                    <td class="py-3 px-6 font-mono text-xs text-slate-600 break-words">{html.escape(str(entry["details"]))}</td>
                </tr>""")
            f.write("""</tbody>
                </table>
            </div>
        </div>
        
        <div class="mt-8 text-center text-sm text-slate-500">
            <p>Ollama: {html.escape(system_info['ollama_version'])} &bull; GPU: {html.escape(system_info['gpu_info'])} &bull; System RAM: {html.escape(system_info['ram_total'])}</p>
            <p class="mt-1">Source Directory: {html.escape(directory)}</p>
        </div>
    </div>
    <script>
    function filterTable() {{
        var input, filter, table, tr, i;
        input = document.getElementById("searchInput");
        filter = input.value.toUpperCase();
        table = document.getElementById("logTable");
        tr = table.getElementsByTagName("tr");
        for (i = 1; i < tr.length; i++) {{ // Start from 1 to skip header row
            var pathTd = tr[i].getElementsByTagName("td")[1];
            var statusTd = tr[i].getElementsByTagName("td")[3];
            var detailsTd = tr[i].getElementsByTagName("td")[4];
            if (pathTd && statusTd && detailsTd) {{
                var pathText = pathTd.textContent || pathTd.innerText;
                var statusText = statusTd.textContent || statusTd.innerText;
                var detailsText = detailsTd.textContent || detailsTd.innerText;
                if (pathText.toUpperCase().indexOf(filter) > -1 || 
                    statusText.toUpperCase().indexOf(filter) > -1 || 
                    detailsText.toUpperCase().indexOf(filter) > -1) {{
                    tr[i].style.display = "";
                }} else {{
                    tr[i].style.display = "none";
                }}
            }}
        }}
    }}
    </script>
</body>
</html>""")
    except IOError as e:
        logging.critical(f"Failed to write HTML report to {output_path}: {e}")

# =========================
# Core Logic
# =========================

def load_brand_config_from_entities(filepath: str) -> List[Dict[str, Any]]:
    """Dynamically builds brand configuration from an entities file."""
    if not filepath or not os.path.exists(filepath):
        if not filepath:
            logging.info("Entities file not provided. Skipping brand configuration.")
        else:
            logging.warning(f"Entities file not found at {filepath}. Cannot build brand configuration.")
        return []
    
    brands = []
    content = Path(filepath).read_text(encoding='utf-8')
    
    regex = re.compile(r'<!ENTITY\s+([\w-]+)\s+"([^"]+)">')
    matches = regex.findall(content)
    
    for key, name in matches:
        family = 'opensuse' if 'opensuse' in name.lower() or 'leap' in name.lower() else 'suse'
        brands.append({'key': key, 'name': name.strip(), 'family': family})
        
    logging.info(f"Loaded {len(brands)} brands from entities file for consistency checking.")
    return brands

def post_process_description(description: str, file_path: str, brands: List[Dict[str, Any]]):
    """Checks for brand consistency and removes incorrect brand keywords."""
    if not brands:
        return description, None

    file_context = None
    sorted_brands = sorted(brands, key=lambda b: len(b['key']), reverse=True)
    for brand in sorted_brands:
        if re.search(r'\b' + re.escape(brand['key']) + r'\b', file_path, re.IGNORECASE):
            file_context = brand
            break
    
    if not file_context:
        return description, None

    banned_keywords = set()
    for brand in brands:
        if brand['family'] != file_context['family']:
            banned_keywords.add(brand['name'])
            for word in brand['name'].split():
                if len(word) > 3:
                    banned_keywords.add(word)

    if not banned_keywords:
        return description, None

    original_description = description
    for keyword in sorted(list(banned_keywords), key=len, reverse=True):
        pattern = r'\b' + re.escape(keyword) + r'\b'
        if re.search(pattern, description, re.IGNORECASE):
            description = re.sub(pattern, '', description, flags=re.IGNORECASE)

    if description != original_description:
        logging.warning(f"Correcting brand mismatch in {file_path}.")
        # More robust cleanup for dangling grammar
        description = re.sub(r'\s{2,}', ' ', description).strip() # Collapse spaces
        description = re.sub(r'(\w+)\s+,', r'\1,', description) # Fix space before comma: "word ," -> "word,"
        description = re.sub(r'(,\s*){2,}', ',', description) # Fix multiple commas: ", ," -> ","
        description = re.sub(r',\s*(and|or)\s*,', ',', description) # Fix dangling conjunctions between commas
        description = re.sub(r'\b(and|or|a|the|on|of)\s*$', '', description, flags=re.IGNORECASE) # Dangling conjunctions/articles at the end
        description = re.sub(r'\b(on|of)\s+(and|or)\b', '', description, flags=re.IGNORECASE) # "on and", "of or"
        description = re.sub(r',\s*\.', '.', description) # " ,." -> "."
        description = description.replace(' ,', ',').replace(' .', '.')
        description = re.sub(r'\s{2,}', ' ', description).strip() # Final space collapse

        return description.strip(), "UPDATED"

    return description, None

def resolve_entities_in_string(text: str, brands: List[Dict[str, Any]]):
    """Replaces any &entity; references in a string with their full names."""
    if not brands:
        return text
    sorted_brands = sorted(brands, key=lambda b: len(b['key']), reverse=True)
    for brand in sorted_brands:
        entity_ref = f"&{brand['key']};"
        if entity_ref in text:
            text = text.replace(entity_ref, brand['name'])
    return text

def call_ollama(model: str, prompt: str, base_url: str, timeout=120) -> str:
    """Calls the Ollama API."""
    try:
        r = requests.post(f"{base_url.rstrip('/')}/api/generate", json={"model": model, "prompt": prompt, "stream": False}, timeout=timeout)
        r.raise_for_status()
        return (r.json().get("response") or "").strip()
    except requests.exceptions.RequestException as e:
        logging.error(f"Ollama API call failed: {e}")
        return ""

def validate_and_correct_grammar(sentence: str, model: str, base_url: str, config: ScriptConfig) -> str:
    """Uses a second LLM call to act as a grammar and structure validator."""
    if not sentence:
        return ""
    
    logging.info(f"Validating grammar for draft: '{sentence}'")
    prompt = config.PROMPT_TMPL_VALIDATE.format(sentence=sentence)
    
    # Use the existing call_ollama function for the correction
    corrected_sentence = call_ollama(model, prompt, base_url)
    
    if corrected_sentence and corrected_sentence != sentence:
        logging.info(f"Grammar corrected to: '{corrected_sentence}'")
        return corrected_sentence
    
    logging.info("Grammar check passed without changes.")
    return sentence

def sanitize_and_finalize(draft: str, config: ScriptConfig) -> str:
    """Runs the full cleaning and finalization pipeline."""
    desc = html.unescape(draft)
    
    # CRITICAL: Remove any leaked prompt instructions - must be done FIRST before other processing
    # Using simple case-insensitive string replacement for more reliable matching
    leakage_phrases = [
        "Follow these rules strictly",
        "Your task is to",
        "You must now",
        "Output only",
        "Critical:",
        "Important:",
        "Note:",
        "Remember:",
        "Your response must contain only",
    ]
    
    # Remove leaked phrases (case-insensitive)
    for phrase in leakage_phrases:
        # Create a regex that matches the phrase with flexible whitespace
        pattern = re.compile(re.escape(phrase), re.IGNORECASE)
        desc = pattern.sub('', desc)
    
    # Clean up any resulting multiple spaces after leakage removal
    desc = re.sub(r'\s+', ' ', desc).strip()
    
    # Intelligently handle possessives before removing other characters
    desc = re.sub(r"(\w+)'s\b", r"\1s", desc)
    desc = desc.replace("'", "")

    desc = re.sub(r"\s+", " ", desc).strip()
    for name in sorted(config.BANNED_LITERALS, key=len, reverse=True):
        desc = re.sub(rf"\b{re.escape(name)}\b", "", desc, flags=re.IGNORECASE)
    desc = config.FORBIDDEN_CHARS_RE.sub(" ", desc)
    for pat in config.DOC_META_PATTERNS: desc = pat.sub("", desc).strip()
    desc = re.sub(r"^(by|with|through|using)\s+", "", desc, flags=re.IGNORECASE)
    desc = re.sub(r"\s{2,}", " ", desc).strip(" ,;:-")
    if not desc: return ""
    if len(desc) > 160:
        desc = desc[:161]
        last_space = desc.rfind(" ")
        if last_space != -1: desc = desc[:last_space]
    words = desc.rstrip(" ,;:-.").split()
    while words and words[-1].lower().strip(",.;") in config.TRAILING_STOPWORDS: words.pop()
    desc = " ".join(words)
    # MODIFIED: Remove trailing period instead of adding one.
    if desc.endswith('.'):
        desc = desc[:-1]
    if desc: desc = desc[0].upper() + desc[1:]
    return desc

# =========================
# File Processors
# =========================

def should_skip(path: Path, config: ScriptConfig) -> bool:
    """Determines if a file should be skipped based on Antora conventions."""
    name = path.name
    if name.startswith('_'):
        logging.debug(f"Skipping {path}: starts with underscore")
        return True
    if config.NAV_GENERIC_RE.match(name) or config.NAV_GUIDE_RE.match(name):
        logging.debug(f"Skipping {path}: navigation file")
        return True
    # Skip files in nav, navigation, or partials directories
    path_parts_lower = [part.lower() for part in path.parts]
    if any(p in ("nav", "navigation", "partials") for p in path_parts_lower):
        logging.debug(f"Skipping {path}: in nav/navigation/partials directory")
        return True
    return False

def load_adoc_attributes(file_path: Path) -> dict:
    """A simple parser for standard, non-conditional AsciiDoc attribute files."""
    attributes = {}
    try:
        with file_path.open('r', encoding='utf-8') as f:
            for line in f:
                # This regex handles :attr: value and :attr:
                match = re.match(r'^:([a-zA-Z0-9_-]+):(?:\s+(.*))?$', line)
                if match:
                    key, value = match.groups()
                    attributes[key] = value.strip() if value is not None else ""
    except IOError as e:
        logging.error(f"Could not read attributes file {file_path}: {e}")
    return attributes

def load_and_process_adoc_attributes(file_path: Path, initial_context: Dict[str, str]) -> dict:
    """
    Loads and processes an AsciiDoc attributes file, handling ifndef, ifeval,
    and iterative attribute expansion.
    """
    if not file_path or not file_path.exists():
        logging.error(f"Attributes file not found at {file_path}")
        return {}

    attributes = initial_context.copy()
    lines = file_path.read_text(encoding='utf-8').splitlines()
    
    # Pre-compile regex patterns for efficiency
    attr_re = re.compile(r'^:([\w-]+):(?:\s+(.*))?$')
    ifndef_re = re.compile(r'^ifndef::([\w-]+)\[\]$')
    ifeval_re = re.compile(r'^ifeval::\["\{([\w-]+)\}" == "([^"]+)"\]$')
    endif_re = re.compile(r'^endif::\[\]$')
    
    # --- First Pass: Parse file and handle conditionals ---
    in_active_block = True
    active_block_stack = []

    for line in lines:
        line = line.strip()
        if not line or line.startswith('//'):
            continue

        # Handle endif
        if endif_re.match(line):
            if active_block_stack:
                in_active_block = active_block_stack.pop()
            continue

        # Handle ifndef
        match = ifndef_re.match(line)
        if match:
            key = match.group(1)
            active_block_stack.append(in_active_block)
            in_active_block = in_active_block and (key not in attributes)
            continue
            
        # Handle ifeval
        match = ifeval_re.match(line)
        if match:
            key, value = match.groups()
            active_block_stack.append(in_active_block)
            is_match = attributes.get(key) == value
            in_active_block = in_active_block and is_match
            continue

        if not in_active_block:
            continue

        # Handle normal attribute definitions
        match = attr_re.match(line)
        if match:
            key, value = match.groups()
            # Handle attributes with no value (e.g., :showtitle:)
            attributes[key] = value.strip() if value is not None else ""

    # --- Second Pass: Iteratively resolve attribute values ---
    unresolved = True
    # Limit iterations to prevent infinite loops from bad definitions
    for _ in range(10): 
        if not unresolved:
            break
        unresolved = False
        for key, value in attributes.items():
            if isinstance(value, str) and '{' in value:
                # Find all {placeholder} occurrences
                placeholders = re.findall(r'\{([\w-]+)\}', value)
                if not placeholders:
                    continue
                
                unresolved = True
                new_value = value
                for placeholder in placeholders:
                    if placeholder in attributes:
                        new_value = new_value.replace(f'{{{placeholder}}}', str(attributes[placeholder]))
                attributes[key] = new_value
    
    logging.info(f"Successfully processed and expanded attributes from {file_path}")
    return attributes


def resolve_attributes(text: str, attributes: dict) -> str:
    """Iteratively replaces AsciiDoc attributes in a string."""
    if not attributes: return re.sub(r"\{([A-Za-z0-9_-]+)\}", "", text)
    # Iteratively replace attributes until no placeholders are left
    for _ in range(10): # Safety break for circular references
        if '{' not in text:
            break
        # Create a temporary copy for this iteration's replacements
        temp_text = text
        for key, value in attributes.items():
            temp_text = temp_text.replace(f'{{{key}}}', str(value))
        # If no changes were made in a full pass, we're done
        if temp_text == text:
            break
        text = temp_text

    # Remove any remaining (unresolved) attributes
    text = re.sub(r"\{([A-Za-z0-9_-]+)\}", "", text)
    return text

def extract_adoc_payload(content: str, max_len: int = 4000) -> str:
    """Converts the entire AsciiDoc content to plain text for analysis."""
    lines = content.splitlines()
    header_end = 0
    if lines and re.match(r"^\s*=\s+.+", lines[0]):
        header_end = 1
        while header_end < len(lines) and (lines[header_end].strip().startswith(':') or not lines[header_end].strip()):
            header_end += 1
    text = "\n".join(lines[header_end:])
    
    text = re.sub(r'^==+\s+.*$', '', text, flags=re.MULTILINE)
    text = re.sub(r'^[\*\.\-]+\s+', '', text, flags=re.MULTILINE)
    text = re.sub(r'\[\[.*?\]\]', '', text)
    text = re.sub(r'<<.*?>>', '', text)
    text = re.sub(r'image::\S+\[.*?\]', '', text)
    text = re.sub(r'\|===', '', text)
    text = re.sub(r'\|', ' ', text)
    text = re.sub(r'----', '', text)
    text = re.sub(r'//.*$', '', text, flags=re.MULTILINE)
    text = re.sub(r'`([^`]+)`', r'\1', text)
    text = re.sub(r'\*([^*]+)\*', r'\1', text)
    text = re.sub(r'_([^_]+)_', r'\1', text)
    text = re.sub(r'xref:\S+\[(.*?)\]', r'\1', text)
    text = re.sub(r'\n{2,}', '\n', text)
    return text.strip()[:max_len]

def process_adoc_file(path: Path, config: ScriptConfig, args: argparse.Namespace, attributes: dict, html_log_entries: list = None, brands=[]):
    file_type = "AsciiDoc"
    logging.info(f"Processing {file_type}: {path}")
    try:
        raw_text = path.read_text(encoding="utf-8")
        if config.DESC_RE.search(raw_text) and not args.force_overwrite:
            msg = "File already has a description."
            logging.info(f"SKIPPED: {msg} ({path})")
            add_html_log_entry(html_log_entries, path, file_type, "SKIPPED", msg)
            return
        
        # Attributes are resolved first for AsciiDoc
        resolved_text = resolve_attributes(raw_text, attributes)
        payload = extract_adoc_payload(resolved_text)
        if not payload.strip():
            msg = "Empty content payload after extraction."
            logging.warning(f"SKIPPED: {msg} ({path})")
            add_html_log_entry(html_log_entries, path, file_type, "WARNING", msg)
            return

        prompt = config.PROMPT_TMPL.format(blacklist=", ".join(config.BANNED_LITERALS), content=payload)
        draft = call_ollama(args.model, prompt, args.ollama_url)

        draft_after_branding, update_status = post_process_description(draft, str(path), brands)
        corrected_draft = validate_and_correct_grammar(draft_after_branding, args.model, args.ollama_url, config)
        desc = sanitize_and_finalize(corrected_draft, config)

        if not desc or len(desc) < 100:
            logging.warning(f"First pass description too short ({len(desc)} chars). Retrying for {path}")
            retry_prompt = config.PROMPT_TMPL_RETRY.format(blacklist=", ".join(config.BANNED_LITERALS), content=payload)
            draft = call_ollama(args.model, retry_prompt, args.ollama_url)
            
            draft_after_branding_retry, update_status = post_process_description(draft, str(path), brands)
            corrected_draft_retry = validate_and_correct_grammar(draft_after_branding_retry, args.model, args.ollama_url, config)
            desc = sanitize_and_finalize(corrected_draft_retry, config)

            if not desc or len(desc) < 100:
                msg = f"Generated description still too short after retry ({len(desc)} chars)."
                logging.error(f"SKIPPED: {msg} ({path})")
                add_html_log_entry(html_log_entries, path, file_type, "ERROR", msg)
                return
        
        if args.dry_run:
            logging.info(f"[DRY RUN] Would update {path}")
            add_html_log_entry(html_log_entries, path, file_type, "DRY_RUN", desc)
            return desc

        lines = raw_text.splitlines()
        existing_desc_idx = next((i for i, line in enumerate(lines) if config.DESC_RE.match(line)), -1)
        status = ""
        if existing_desc_idx != -1:
            lines[existing_desc_idx] = f":description: {desc}"
            status = "REPLACED"
        else:
            title_idx = next((i for i, line in enumerate(lines) if config.TITLE_RE.match(line)), -1)
            lines.insert(title_idx + 1 if title_idx != -1 else 0, f":description: {desc}")
            status = "ADDED"
        
        if update_status:
            status = update_status

        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
        logging.info(f"{status}: Description for {path} ({len(desc)} chars)")
        add_html_log_entry(html_log_entries, path, file_type, status, desc)
        return desc
    except IOError as e:
        logging.error(f"Failed to process AsciiDoc file {path}: {e}", exc_info=args.verbose)
        add_html_log_entry(html_log_entries, path, file_type, "ERROR", str(e))


# ==============================================================================
# process_xml_file (MODIFIED)
# This function uses a pure string/regex approach to perform the precise
# namespace modifications requested.
# ==============================================================================
def process_xml_file(path: Path, config: ScriptConfig, args: argparse.Namespace, html_log_entries: list = None, brands=[]):
    ITS_NS = "http://www.w3.org/2005/11/its"
    ns = {'db': 'http://docbook.org/ns/docbook', 'xi': 'http://www.w3.org/2001/XInclude'}

    try:
        # MODIFIED: Payload parser now set to resolve_entities=False to prevent AI hyper-focus
        base_url = path.resolve().parent.as_uri() + "/"
        parser_payload = ET.XMLParser(resolve_entities=False, load_dtd=True)
        payload_tree = ET.parse(str(path), parser=parser_payload, base_url=base_url)
        
        root = payload_tree.getroot()
        file_type = "DocBook XML"
        try:
            version = root.get("version")
            if version:
                file_type = f"DocBook {version}"
        except Exception: pass
        
        logging.info(f"Processing {file_type}: {path}")

        if root.tag == f"{{{ns['db']}}}set":
            logging.info(f"Detected a DocBook <set> file. Building a table of contents for the payload.")
            titles = []
            includes = root.findall('.//xi:include', namespaces=ns)
            for include in includes:
                href = include.get('href')
                if not href: continue
                
                included_path = path.parent / href
                if not included_path.is_file(): continue
                
                try:
                    # Use a non-resolving parser here too for consistency
                    included_tree = ET.parse(str(included_path), ET.XMLParser(resolve_entities=False))
                    title_element = included_tree.find('.//db:info/db:title', namespaces=ns)
                    if title_element is not None:
                        titles.append(''.join(title_element.itertext()).strip())
                except Exception as e:
                    logging.warning(f"Could not parse or find title in {included_path}: {e}")
            payload = "\n".join(f"- {title}" for title in titles)
        else:
            abstract_element = root.find('.//db:info/db:abstract', namespaces=ns)
            if abstract_element is not None:
                logging.info("Found <abstract> tag. Using it as the primary payload.")
                payload = "\n".join(text.strip() for text in abstract_element.itertext() if text.strip())
            else:
                payload_tree.xinclude() # xinclude might still be useful for structure
                payload = "\n".join(text.strip() for text in payload_tree.getroot().itertext() if text.strip())

        raw_text = path.read_text(encoding="utf-8")
        if re.search(r'<meta\s+name="description"[^>]*>', raw_text, re.IGNORECASE) and not args.force_overwrite:
            msg = "File already has a <meta name='description'> tag."
            logging.info(f"SKIPPED: {msg} ({path})")
            add_html_log_entry(html_log_entries, path, file_type, "SKIPPED", msg)
            return

        if not payload.strip():
            msg = "Empty content payload after extraction."
            logging.warning(f"SKIPPING: {msg} ({path})")
            add_html_log_entry(html_log_entries, path, file_type, "WARNING", msg)
            return

        # --- AI Generation and Processing ---
        prompt = config.PROMPT_TMPL.format(blacklist=", ".join(config.BANNED_LITERALS), content=payload[:4000])
        draft = call_ollama(args.model, prompt, args.ollama_url)
        
        draft_after_branding, update_status = post_process_description(draft, str(path), brands)
        corrected_draft = validate_and_correct_grammar(draft_after_branding, args.model, args.ollama_url, config)
        desc = sanitize_and_finalize(corrected_draft, config)

        if not desc or len(desc) < 100:
            logging.warning(f"First pass description too short ({len(desc)} chars). Retrying for {path}")
            retry_prompt = config.PROMPT_TMPL_RETRY.format(blacklist=", ".join(config.BANNED_LITERALS), content=payload[:4000])
            draft = call_ollama(args.model, retry_prompt, args.ollama_url)
            
            draft_after_branding_retry, update_status = post_process_description(draft, str(path), brands)
            corrected_draft_retry = validate_and_correct_grammar(draft_after_branding_retry, args.model, args.ollama_url, config)
            desc = sanitize_and_finalize(corrected_draft_retry, config)

            if not desc or len(desc) < 100:
                msg = f"Generated description still too short after retry ({len(desc)} chars)."
                logging.error(f"SKIPPED: {msg} ({path})")
                add_html_log_entry(html_log_entries, path, file_type, "ERROR", msg)
                return
        
        # MODIFIED: Entities are resolved at the end for DocBook files.
        desc = resolve_entities_in_string(desc, brands)
        
        if args.dry_run:
            logging.info(f"[DRY RUN] Would update {path}")
            add_html_log_entry(html_log_entries, path, file_type, "DRY_RUN", desc)
            return desc

        # --- Direct String/Regex Modification ---
        file_content = path.read_text(encoding='utf-8')

        # 1. Ensure 'its' namespace is on the root element.
        try:
            parser_tag_find = ET.XMLParser(resolve_entities=False)
            tree_tag_find = ET.parse(str(path), parser_tag_find)
            root_tag_local_name = ET.QName(tree_tag_find.getroot().tag).localname
            
            root_tag_pattern = re.compile(fr"<{root_tag_local_name}[^>]*>", re.DOTALL)
            root_tag_match = root_tag_pattern.search(file_content)
            
            if root_tag_match:
                original_root_tag = root_tag_match.group(0)
                if 'xmlns:its' not in original_root_tag:
                    replacement_marker = f"<{root_tag_local_name}"
                    new_root_tag = original_root_tag.replace(
                        replacement_marker,
                        f'{replacement_marker} xmlns:its="{ITS_NS}"',
                        1
                    )
                    file_content = file_content.replace(original_root_tag, new_root_tag, 1)
            else:
                 logging.warning(f"Could not find root tag regex match for '{root_tag_local_name}' in {path}.")
        except ET.XMLSyntaxError as e:
            logging.warning(f"LXML parse failed for {path} while finding root tag: {e}. Skipping 'its' namespace injection.")

        # 2. Actively remove all xmlns attributes from the <info> tag.
        # Use word boundary to avoid matching <informaltable>
        info_tag_pattern = re.compile(r"<\binfo\b[^>]*>", re.IGNORECASE)
        info_tag_match = info_tag_pattern.search(file_content)
        
        # If no <info> tag exists, create one.
        if not info_tag_match:
            root_tag_pattern = re.compile(fr"<{root_tag_local_name}[^>]*>", re.DOTALL)
            root_tag_match = root_tag_pattern.search(file_content)
            if root_tag_match:
                original_root_tag = root_tag_match.group(0)
                # Add an info block right after the root tag's opening
                insertion_point = f"{original_root_tag}\n  <info></info>"
                file_content = file_content.replace(original_root_tag, insertion_point, 1)
                # Find the newly created info tag
                info_tag_match = info_tag_pattern.search(file_content)

        if info_tag_match:
            original_info_tag = info_tag_match.group(0)
            xmlns_pattern = re.compile(r'\s+xmlns(?::\w+)?="[^"]+"')
            cleaned_info_tag = xmlns_pattern.sub('', original_info_tag)
            
            if original_info_tag != cleaned_info_tag:
                file_content = file_content.replace(original_info_tag, cleaned_info_tag, 1)
                info_tag_to_modify = cleaned_info_tag
            else:
                info_tag_to_modify = original_info_tag
        else:
            msg = "No <info> block found and could not create one. Cannot process file."
            logging.warning(f"SKIPPED: {msg} ({path})")
            add_html_log_entry(html_log_entries, path, file_type, "WARNING", msg)
            return

        # 3. Add or Replace the meta tag.
        existing_meta_pattern = re.compile(r'<meta\s+name="description".*?/>|<meta\s+name="description".*?>.*?</meta>', re.IGNORECASE | re.DOTALL)
        new_meta_string = f'<meta name="description" its:translate="yes">{html.escape(desc)}</meta>'

        status = ""
        final_text = ""

        if existing_meta_pattern.search(file_content):
            status = "REPLACED"
            final_text = existing_meta_pattern.sub(new_meta_string, file_content, 1)
        else:
            status = "ADDED"
            insertion_text = f"{info_tag_to_modify}\n    {new_meta_string}"
            final_text = file_content.replace(info_tag_to_modify, insertion_text, 1)
        
        if update_status:
            status = update_status
            
        path.write_text(final_text, encoding="utf-8")
        logging.info(f"{status}: Description for {path} ({len(desc)} chars)")
        add_html_log_entry(html_log_entries, path, file_type, status, desc)
        return desc

    except (ET.XMLSyntaxError, IOError) as e:
        logging.error(f"Failed to process XML file {path}: {e}", exc_info=args.verbose)
        add_html_log_entry(html_log_entries, path, "DocBook XML", "ERROR", str(e))
        return

# =========================
# Main Execution
# =========================

def main():
    ap = argparse.ArgumentParser(description="Generate meta descriptions for AsciiDoc and DocBook files.")
    ap.add_argument("root", help="Path to the root directory of your documentation files.")
    ap.add_argument("--model", default="llama3.1:8b", help="Ollama model to use.")
    ap.add_argument("--ollama-url", default=os.environ.get("OLLAMA_URL", "http://127.0.0.1:11434"), help="Ollama base URL.")
    ap.add_argument("--type", default="all", choices=['adoc', 'xml', 'all'], help="Choose the type of files to process (default: all).")
    ap.add_argument("--force-overwrite", action="store_true", help="Overwrite existing descriptions.")
    ap.add_argument("--dry-run", action="store_true", help="Preview changes without writing to files.")
    ap.add_argument("--html-log", help="Path to an HTML file to log all actions.")
    ap.add_argument("--report-title", default="Description Generation Report", help="Set a custom title for the HTML report.")
    ap.add_argument("-v", "--verbose", action="store_true", help="Enable verbose DEBUG level logging.")
    ap.add_argument("--attributes-file", help="Path to an .adoc file with AsciiDoc attributes.")
    ap.add_argument("--banned-terms", help="Comma-separated list of case-insensitive terms to ban from the description.")
    ap.add_argument("--entities-file", required=False, help="Optional path to an entities file (.adoc or .ent) to extract acronyms/brands.")
    # NEW: Flag for conditional builds
    ap.add_argument("-a", "--build-attributes", action="append", help="Set an AsciiDoc attribute for conditional processing, e.g., 'build-type=product'. Can be used multiple times.")

    args = ap.parse_args()
    args.report_title = args.report_title.replace('_', ' ')

    # --- Setup ---
    logging.basicConfig(level=logging.DEBUG if args.verbose else logging.INFO, format='[%(levelname)-7s] %(message)s')
    root = Path(args.root).resolve()
    if not root.is_dir():
        logging.critical(f"Error: Root path not found: {root}"); sys.exit(1)
    
    config = ScriptConfig()
    if args.banned_terms:
        config.BANNED_LITERALS.update(x.strip() for x in args.banned_terms.split(","))

    system_info = get_system_info()
    
    # --- MODIFIED: Conditional attribute loading ---
    attributes = {}
    if args.attributes_file:
        attributes_path = Path(args.attributes_file)
        if args.build_attributes:
            # For complex, conditional files, use the advanced parser
            logging.info(f"Using advanced attribute parser with context: {args.build_attributes}")
            build_context = {}
            for attr in args.build_attributes:
                if '=' in attr:
                    key, value = attr.split('=', 1)
                    build_context[key] = value
            attributes = load_and_process_adoc_attributes(attributes_path, build_context)
        else:
            # For simple files, use the standard key-value parser
            logging.info("Using standard attribute parser.")
            attributes = load_adoc_attributes(attributes_path)

    # MODIFIED: Removed acronyms_text loading
    brands = load_brand_config_from_entities(args.entities_file)
    html_log_entries = [] if args.html_log else None
    
    logging.info(f"Starting in GENERATE mode. Root: {root}")
    if args.dry_run: logging.warning("Dry run enabled. No files will be modified.")

    files_to_scan = []
    if args.type in ['adoc', 'all']:
        files_to_scan.extend(root.rglob("*.adoc"))
    if args.type in ['xml', 'all']:
        files_to_scan.extend(root.rglob("*.xml"))
    
    logging.info(f"Found {len(files_to_scan)} initial files.")
    
    final_file_list = [p for p in files_to_scan if not should_skip(p, config)]
    logging.info(f"Processing {len(final_file_list)} files after skipping nav files and partials.")

    changed_files_count = 0
    start_time = time.monotonic()

    for path in sorted(final_file_list):
        result = None
        if path.suffix.lower() == '.adoc':
            # MODIFIED: Removed acronyms_text from call
            result = process_adoc_file(path, config, args, attributes, html_log_entries, brands)
        elif path.suffix.lower() == '.xml':
            # MODIFIED: Removed acronyms_text from call
            result = process_xml_file(path, config, args, html_log_entries, brands)
        
        if result and not args.dry_run:
            changed_files_count += 1

    duration = time.monotonic() - start_time
    if args.dry_run and html_log_entries:
        changed_files_count = sum(1 for e in html_log_entries if e['status'] == 'DRY_RUN')

    if args.html_log:
        generate_html_report(args.html_log, args.root, args.model, duration, len(final_file_list), changed_files_count, html_log_entries, system_info, args.report_title)

    logging.info("--- Script Finished ---")
    logging.info(f"Total processing time: {duration:.2f} seconds.")
    logging.info(f"Total files scanned: {len(final_file_list)}")
    if args.dry_run:
        logging.info(f"Files that would be changed: {changed_files_count}")
    else:
        logging.info(f"Total files modified: {changed_files_count}")
    if args.html_log:
        logging.info(f"HTML report saved to '{args.html_log}'")

if __name__ == "__main__":
    main()