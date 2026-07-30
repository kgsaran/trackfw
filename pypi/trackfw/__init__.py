try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "5.0.0"
except Exception:
    __version__ = "5.0.0"
