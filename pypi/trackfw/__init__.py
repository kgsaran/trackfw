try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "3.0.0"
except Exception:
    __version__ = "3.0.0"
