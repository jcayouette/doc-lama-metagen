# ============================================================================
# doc-lama-metagen.py - Setup and Usage Guide for openSUSE Leap 15.6
# ============================================================================
#
# PREREQUISITES:
# - pyenv installed on your system
# - Python 3.7+ (dataclasses module required)
#
# SETUP INSTRUCTIONS:
# -------------------
#
# 1. Install pyenv (if not already installed):
#    curl https://pyenv.run | bash
#    
#    Add to ~/.bashrc:
#    export PATH="$HOME/.pyenv/bin:$PATH"
#    eval "$(pyenv init -)"
#    eval "$(pyenv virtualenv-init -)"
#    
#    Reload shell:
#    source ~/.bashrc
#
# 2. Install Python 3.13.7 (or any version 3.7+):
#    pyenv install 3.13.7
#
# 3. Set local Python version in your project directory:
#    cd /path/to/kubewarden-product-docs
#    pyenv local 3.13.7
#
# 4. Verify Python version:
#    python3 --version
#    # Should output: Python 3.13.7
#
# 5. Create a virtual environment:
#    python3 -m venv venv
#
# 6. Activate the virtual environment:
#    source venv/bin/activate
#
# 7. Install required dependencies:
#    pip install requests lxml psutil
#
# USAGE:
# ------
#
# Basic command structure:
#    python3 ../doc-lama-metagen/doc-lama-metagen.py <pages_directory> --entities-file <entities_file>
#
# Example for version 1.32:
#    source venv/bin/activate
#    python3 ../doc-lama-metagen/doc-lama-metagen.py \
#        docs/version-1.32/modules/en/pages/ \
#        --entities-file docs/version-1.32/modules/en/partials/variables.adoc
#
# NOTES:
# ------
# - The script generates AI-powered meta descriptions for AsciiDoc files
# - Requires an active internet connection (calls AI service)
# - Processing time varies based on number of files (~2 seconds per file)
# - Creates .pyenv-version file in project directory (can be committed to git)
# - Virtual environment (venv/) directory should be added to .gitignore
#
# TROUBLESHOOTING:
# ----------------
# - If "ModuleNotFoundError: No module named 'dataclasses'": Python < 3.7
# - If "ModuleNotFoundError: No module named 'requests'": Run pip install
# - If "IsADirectoryError": --entities-file needs a file path, not directory
# - Deactivate venv: deactivate
#
# ============================================================================
