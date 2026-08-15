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

# Matches HTML comments, removed in step 1 of the D3 normalization
# pipeline.
_HTML_COMMENT_PATTERN = re.compile(r"<!--.*?-->", re.DOTALL)

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
    """Strips fenced code blocks (``` or ~~~), step 2 of the D3 pipeline
    (architect's amendment to the original hades-tf opinion): lines inside
    a fence are not read as headings, otherwise documentation that merely
    quotes the marker list would be refused by its own criterion. A fence
    is closed by a line starting with the same delimiter character
    (backtick or tilde), with at least as many repeats as the opener — the
    CommonMark rule. Mirrors internal/thirdparty/markers.go's
    removeFencedBlocks exactly (line-scanner with explicit state)."""
    lines = text.split("\n")
    out: list[str] = []
    closer = ""  # fence delimiter run that closes the current block, "" if not in a fence
    for line in lines:
        if closer == "":
            match = _FENCE_PREFIX_PATTERN.match(line)
            if match:
                closer = match.group(1)
                continue  # drop the opening fence line itself
            out.append(line)
            continue
        # Inside a fence: drop the line; check if it closes the block.
        trimmed = line.strip()
        delim_char = closer[0]
        if trimmed.startswith(delim_char * len(closer)) and trimmed.strip(delim_char) == "":
            closer = ""
    return "\n".join(out)


def check_markers(content: bytes) -> list[str]:
    """Applies the D3 objective-refusal criterion to content and returns
    the literal marker names (from LITERAL_MARKERS) that matched as a
    heading. The normalization pipeline, in fixed order:
      1. remove HTML comments;
      2. remove fenced code blocks (``` and ~~~) — content inside a fence
         is never read as a heading;
      3. NFKC normalize;
      4. casefold (str.casefold(), not str.lower() — the ADR-mandated
         normalization step; total-width and Cyrillic homoglyphs are a
         documented gap of this step, not a bug: see the ADR's "o que
         este critério NÃO cobre" section);
      5. collapse internal whitespace + strip (applied per line, so
         newlines are preserved as line separators);
      6. match only lines matching ^#{1,6}\\s+ against the literal marker
         list.
    """
    text = content.decode("utf-8", errors="replace")

    # 1. Remove HTML comments.
    text = _HTML_COMMENT_PATTERN.sub("", text)

    # 2. Remove fenced code blocks — lines inside a fence are not headings.
    text = _remove_fenced_blocks(text)

    # 3. NFKC normalize.
    text = unicodedata.normalize("NFKC", text)

    # 4. Casefold.
    text = text.casefold()

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
