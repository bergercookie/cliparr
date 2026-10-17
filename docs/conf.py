"""Sphinx configuration. The docs are plain Markdown so they also read well on GitHub."""

project = "Cliparr"
copyright = "Cliparr contributors"

extensions = ["myst_parser"]
myst_enable_extensions = ["colon_fence", "deflist"]
myst_heading_anchors = 3

exclude_patterns = ["_build", "requirements.txt"]

html_theme = "furo"
html_title = "Cliparr"
