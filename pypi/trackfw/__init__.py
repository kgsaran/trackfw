try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "4.0.0"
except Exception:
    __version__ = "4.0.0"
