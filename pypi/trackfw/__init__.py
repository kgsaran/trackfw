try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "3.1.0"
except Exception:
    __version__ = "3.1.0"
