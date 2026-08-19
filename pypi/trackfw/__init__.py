try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "7.1.0"
except Exception:
    __version__ = "7.1.0"
