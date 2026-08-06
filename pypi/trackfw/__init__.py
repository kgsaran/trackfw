try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "6.4.1"
except Exception:
    __version__ = "6.4.1"
