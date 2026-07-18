try:
    from importlib.metadata import version
    __version__ = version("trackfw")
except Exception:
    __version__ = "2.12.4"
