try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "2.14.0"
except Exception:
    __version__ = "2.14.0"
