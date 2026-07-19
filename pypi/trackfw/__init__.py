try:
    from importlib.metadata import version
    __version__ = version("trackfw") or "2.12.4"
except Exception:
    __version__ = "2.12.4"
