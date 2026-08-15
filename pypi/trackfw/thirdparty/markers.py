"""D3 objective-refusal criterion: 6 literal heading markers, 5-step
normalization pipeline (with the fenced-block-removal amendment), and the
D6 checksum primitive. Native port of internal/thirdparty/markers.go.

Unlike Go's RE2-based `regexp` package, Python's `re` DOES support
backreferences — but this module deliberately still implements
`_remove_fenced_blocks` as an explicit line-scanner with state, not a
single backreference regex, to stay byte-for-byte faithful to the
CommonMark closing rule the Go implementation encodes (same delimiter
character, closer with at least as many repeats as the opener). See
vault/notes/go-regexp-re2-sem-backreference-fenced-block-removal-2026-08-15.md
— that note explicitly says a backreference port is fine in Python; the
scanner form is kept here anyway for parity of algorithm, not because
Python needs it.
"""

from __future__ import annotations

import hashlib
import re
import unicodedata
from urllib.parse import urlsplit, urlunsplit

# literal_markers is the objective, literal list of headings whose presence
# causes a third-party artifact to be refused by default (D3). This is a
# tripwire, not a filter against a competent adversary.
LITERAL_MARKERS: list[str] = [
    "git authority",
    "mode lock",
    "governance prerequisite",
    "reporting boundary",
    "scope boundary",
    "dispatch contract",
]

# Matches HTML comments; step 1 of the D3 normalization pipeline
# NEUTRALIZES them (strips only the delimiters, keeping the inner content
# in place to be scanned) — D3-ter(b) amendment, see
# _neutralize_html_comments below.
_HTML_COMMENT_PATTERN = re.compile(r"<!--(.*?)-->", re.DOTALL)


def _neutralize_html_comments(text: str) -> str:
    """Strips only the HTML comment delimiters ("<!--" and "-->"), keeping
    whatever text was between them in place to be scanned by the later
    steps of the pipeline.

    D3-ter(b) amendment: the previous wholesale removal contradicted D3's
    own written justification for this step ("an LLM reads HTML comments
    in the token stream") — a marker hidden as
    ``<!-- ## Git authority -->`` passed clean. Reproduced by the architect
    against this exact module before the fix (returned ``[]``). Mirrors
    internal/thirdparty/markers.go's neutralizeHTMLComments."""
    return _HTML_COMMENT_PATTERN.sub(r"\1", text)

# Detects a fence-opening/closing line: optional leading whitespace
# followed by three or more backticks or tildes.
_FENCE_PREFIX_PATTERN = re.compile(r"^\s*(```+|~~~+)")

# Matches a single, already-collapsed Markdown heading line (level 1-6):
# "#" through "######" followed by whitespace and the heading body.
# Applied per-line after step 5, on text that no longer contains internal
# runs of whitespace.
_HEADING_LINE_PATTERN = re.compile(r"^#{1,6}\s+(.*)$")

# Collapses runs of internal whitespace, step 5 of the D3 pipeline.
_WHITESPACE_PATTERN = re.compile(r"\s+")


def _remove_fenced_blocks(text: str) -> str:
    """Strips PROPERLY-CLOSED fenced code blocks (``` or ~~~), step 2 of
    the D3 pipeline (architect's amendment to the original hades-tf
    opinion): lines inside a CLOSED fence are not read as headings,
    otherwise documentation that merely quotes the marker list would be
    refused by its own criterion. A fence is closed by a line starting
    with the same delimiter character (backtick or tilde), with at least
    as many repeats as the opener — the CommonMark rule.

    D3-ter(a) amendment: an opener with NO matching closer before EOF is
    NOT a fence for this check — the buffered lines (including the
    opener) are replayed as ordinary text instead of being dropped.
    Before this amendment, an unclosed fence swallowed the rest of the
    document as "fenced" content, silently hiding any marker after it —
    reproduced by the architect against this exact module before the fix.
    Mirrors internal/thirdparty/markers.go's removeFencedBlocks (line-
    scanner with explicit state)."""
    lines = text.split("\n")
    out: list[str] = []
    buffered: list[str] = []  # lines consumed since the current fence opener; replayed verbatim if it never closes
    closer = ""  # fence delimiter run that closes the current block, "" if not in a fence
    for line in lines:
        if closer == "":
            match = _FENCE_PREFIX_PATTERN.match(line)
            if match:
                closer = match.group(1)
                buffered = [line]  # keep the opener in case this fence never closes
                continue
            out.append(line)
            continue
        # Inside a (possibly-never-closing) fence: buffer the line, then
        # check if it closes the block.
        buffered.append(line)
        trimmed = line.strip()
        delim_char = closer[0]
        if trimmed.startswith(delim_char * len(closer)) and trimmed.strip(delim_char) == "":
            closer = ""
            buffered = []  # closed properly: the buffered fenced content is discarded, as before
    if closer != "":
        # Reached EOF still "inside" a fence that never closed (D3-ter(a)):
        # not a fence at all — replay every buffered line, including the
        # opener, as ordinary text to be scanned.
        out.extend(buffered)
    return "\n".join(out)


def check_markers(content: bytes) -> list[str]:
    """Applies the D3 objective-refusal criterion to content and returns
    the literal marker names (from LITERAL_MARKERS) that matched as a
    heading. The normalization pipeline, in fixed order (amended by
    D3-ter):
      1. neutralize HTML comments (strip delimiters, keep inner content —
         D3-ter(b));
      2. remove PROPERLY-CLOSED fenced code blocks (``` and ~~~) — content
         inside a closed fence is never read as a heading; an unclosed
         fence is NOT a fence for this purpose and its content is scanned
         normally (D3-ter(a));
      3. NFKC normalize;
      4. casefold — deliberately str.lower() (simple lowercase), NOT
         str.casefold() (full Unicode casefold): unified across the 3
         CLIs by D3-ter(c) so this module never silently diverges from
         Go/Node's strings.ToLower/toLowerCase() on a normalization step
         that feeds a security check. There is no known exploit against
         the 6 ASCII literal markers either way — total-width and
         Cyrillic homoglyphs remain a documented gap of this step, not a
         bug: see the ADR's "o que este critério NÃO cobre" section;
      5. collapse internal whitespace + strip (applied per line, so
         newlines are preserved as line separators);
      6. match only lines matching ^#{1,6}\\s+ against the literal marker
         list.
    """
    text = content.decode("utf-8", errors="replace")

    # 1. Neutralize HTML comments — strip only the delimiters, keep the
    # inner content in place to be scanned (D3-ter(b)).
    text = _neutralize_html_comments(text)

    # 2. Remove fenced code blocks — lines inside a fence are not headings.
    text = _remove_fenced_blocks(text)

    # 3. NFKC normalize.
    text = unicodedata.normalize("NFKC", text)

    # 4. Casefold — simple lowercase (D3-ter(c)), see the docstring above.
    text = text.lower()

    matched: list[str] = []
    seen: set[str] = set()
    for line in text.split("\n"):
        # 5. Collapse internal whitespace + strip.
        collapsed = _WHITESPACE_PATTERN.sub(" ", line).strip()

        # 6. Match only heading lines against the literal marker list.
        match = _HEADING_LINE_PATTERN.match(collapsed)
        if not match:
            continue
        body = match.group(1)
        for marker in LITERAL_MARKERS:
            if body == marker and marker not in seen:
                matched.append(marker)
                seen.add(marker)
    return matched


def checksum(raw: bytes) -> str:
    """SHA-256 hex digest of the raw bytes, before any normalization (D6).
    Mirrors Go's Checksum (markers.go), itself a replica of the unexported
    contentHash in internal/integrations/manager.go."""
    return hashlib.sha256(raw).hexdigest()


def redact_url(raw_url: str) -> str:
    """Returns raw_url with its query string — and userinfo, if present —
    replaced by the literal marker "[redacted]" (D6-bis). Used before
    persisting a third-party artifact's source URL to disk (the
    quarantine record and the provenance entry): a pre-signed URL can
    carry a bearer token in its query string, which would otherwise
    become a permanent secret in the git history the moment either file
    is committed. The full, unredacted URL is used only in memory, for
    the network fetch itself (D7) — never for anything persisted.
    Mirrors internal/thirdparty/markers.go's RedactURL.

    Rebuilds netloc from hostname/port rather than reusing
    urlsplit().netloc directly, which is what actually strips userinfo —
    urlsplit() never re-serializes credentials on its own."""
    parsed = urlsplit(raw_url)
    netloc = parsed.hostname or ""
    if parsed.port:
        netloc = f"{netloc}:{parsed.port}"
    query = "[redacted]" if parsed.query else ""
    return urlunsplit((parsed.scheme, netloc, parsed.path, query, parsed.fragment))
