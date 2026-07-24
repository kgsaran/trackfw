try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "2.15.1"
except Exception:
    __version__ = "2.15.1"
