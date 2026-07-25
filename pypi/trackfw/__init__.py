try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "2.16.0"
except Exception:
    __version__ = "2.16.0"
