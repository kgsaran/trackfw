# Version is hardcoded here and must match internal/version/version.go and pypi/pyproject.toml.
# importlib.metadata.version() is intentionally NOT used: it would return the installed
# package version, not the source version — causing parity check failures during development
# when a different version is installed. The pyproject.toml is the canonical distribution
# metadata; this constant serves the CLI runtime (used by cli.py for the version command).
__version__ = "8.0.0-rc1"
