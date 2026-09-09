try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "7.5.1"
except Exception:
    __version__ = "7.5.1"
