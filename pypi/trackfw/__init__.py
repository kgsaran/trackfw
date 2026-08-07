try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "6.5.0"
except Exception:
    __version__ = "6.5.0"
