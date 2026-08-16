"""D7 network policy for third-party artifact fetches: HTTPS-only
(validated before the first request), 30s timeout, at most 3 redirect
hops (each revalidated for https), a 2 MiB size cap, and a Content-Type
allowlist. Native port of internal/thirdparty/fetch.go.

Deliberately uses its own urllib opener rather than any shared HTTP
client elsewhere in this codebase, mirroring the Go package's fetchClient
doc comment: changes here must never alter third-party artifact download
behavior.

Redirect counting is intentionally NOT urllib's own max_redirections
mechanism (which counts "redirects visited", i.e. len(via)+1 relative to
Go): Go's `net/http.Client.CheckRedirect` receives `via` = requests
ALREADY COMPLETED (the original request plus every redirect already
followed), and refuses once `len(via) >= maxRedirects` — which means only
2 redirects are ever followed before the 3rd is refused, even though the
constant is literally named `maxRedirects = 3`. See
vault/notes/node-https-redirect-checkredirect-off-by-one-2026-08-15.md
(written while porting Node, explicitly flagging this for the Python
port too). This module manually loops over hops, incrementing a
requests-completed counter and checking the cap BEFORE following the next
redirect, to reproduce that exact off-by-one rather than the more
"natural" reading of the doc comment.
"""

from __future__ import annotations

import urllib.error
import urllib.request
from urllib.parse import urljoin, urlsplit

# 2 MiB — deliberately small, since this is text, not a binary release
# asset (the plugin subsystem that downloaded binary assets was removed;
# see docs/adr/ADR-2026-08-15-remocao-do-subsistema-de-plugins-em-vez-de-gate-de-binario-de-terceiro.md).
MAX_CONTENT_SIZE = 2 << 20

# Named to match Go's maxRedirects=3 constant, but see the module
# docstring: because the counter here mirrors Go's `via`-counts-completed-
# requests accounting, only 2 hops are actually followed before the 3rd
# redirect attempt is refused (TestFetch_RefusesFourthRedirect in Go's
# fetch_test.go exercises exactly this boundary with a 5+-hop chain,
# which does not distinguish 2 from 3 hops followed — but the accounting
# must still match Go's for any shorter, deterministic chain).
MAX_REDIRECTS = 3

TIMEOUT_SECONDS = 30.0

ALLOWED_CONTENT_TYPES = {"text/plain", "text/markdown", "text/x-markdown"}

_REDIRECT_CODES = {301, 302, 303, 307, 308}


class ThirdPartyFetchError(RuntimeError):
    """Raised for any refusal or failure under the D7 network policy."""


class _NonFollowingRedirectHandler(urllib.request.HTTPRedirectHandler):
    """Never auto-follows a redirect: redirect_request() returning None
    makes http_error_302 (and its 301/303/307/308 aliases) return None,
    which — since no other installed handler claims the http_error_30x
    event — falls through to HTTPDefaultErrorHandler and raises
    urllib.error.HTTPError with the redirect response's status/headers
    intact. fetch() below catches that HTTPError and drives the redirect
    loop itself, so it controls the exact off-by-one hop accounting
    described in the module docstring."""

    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def _content_type_allowed(content_type: str) -> bool:
    """Checks the Content-Type header against the D7 allowlist, tolerating
    an optional "; charset=..." (or any other parameter) suffix."""
    base = (content_type or "").strip()
    if ";" in base:
        base = base.split(";", 1)[0]
    return base.strip().lower() in ALLOWED_CONTENT_TYPES


def fetch(raw_url: str) -> bytes:
    """Downloads the content at raw_url under the D7 network policy."""
    parsed = urlsplit(raw_url)
    if parsed.scheme != "https":
        raise ThirdPartyFetchError(f"refused: URL scheme must be https, got {parsed.scheme!r}")

    opener = urllib.request.build_opener(_NonFollowingRedirectHandler)
    current_url = raw_url
    requests_completed = 0  # mirrors the length of Go's CheckRedirect `via` slice
    response = None

    while True:
        request = urllib.request.Request(current_url, method="GET")
        try:
            response = opener.open(request, timeout=TIMEOUT_SECONDS)
        except urllib.error.HTTPError as error:
            if error.code in _REDIRECT_CODES:
                requests_completed += 1
                if requests_completed >= MAX_REDIRECTS:
                    raise ThirdPartyFetchError(f"stopped after {MAX_REDIRECTS} redirects") from error
                location = error.headers.get("Location")
                if not location:
                    raise ThirdPartyFetchError(
                        f"fetch failed: redirect with no Location header for {current_url}"
                    ) from error
                next_url = urljoin(current_url, location)
                if urlsplit(next_url).scheme != "https":
                    raise ThirdPartyFetchError(f"redirect to non-https URL refused: {next_url}") from error
                current_url = next_url
                continue
            raise ThirdPartyFetchError(f"fetch failed: HTTP {error.code} for {current_url}") from error
        except urllib.error.URLError as error:
            raise ThirdPartyFetchError(f"fetch failed: {error.reason}") from error
        break

    with response:
        status = getattr(response, "status", None) or response.getcode()
        if status != 200:
            raise ThirdPartyFetchError(f"fetch failed: HTTP {status} for {current_url}")

        content_type = response.headers.get("Content-Type", "")
        if not _content_type_allowed(content_type):
            raise ThirdPartyFetchError(
                f"refused: unsupported Content-Type {content_type!r} "
                "(allowed: text/plain, text/markdown, text/x-markdown)"
            )

        body = response.read(MAX_CONTENT_SIZE + 1)

    if len(body) > MAX_CONTENT_SIZE:
        raise ThirdPartyFetchError(f"refused: content exceeds {MAX_CONTENT_SIZE} bytes")

    return body
