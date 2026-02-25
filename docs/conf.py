import importlib.util

project = "Kron"
author = "Kron Authors"

extensions = [
    "myst_parser",
]

source_suffix = {
    ".rst": "restructuredtext",
    ".md": "markdown",
}

master_doc = "index"

exclude_patterns = [
    "_build",
    "Thumbs.db",
    ".DS_Store",
]

# Prefer RTD theme when available (CI/RTD), but keep local docs buildable
# in environments where optional theme packages are unavailable.
if importlib.util.find_spec("sphinx_rtd_theme") is not None:
    html_theme = "sphinx_rtd_theme"
else:
    html_theme = "alabaster"

myst_heading_anchors = 3
