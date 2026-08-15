'use strict'

// Node port of internal/thirdparty/markers.go — the D3 objective-refusal
// marker check and the Checksum primitive. See:
// docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md
// (D3, D6).

const crypto = require('node:crypto')

// LITERAL_MARKERS mirrors internal/thirdparty/markers.go:literalMarkers — an
// objective, literal tripwire list, not a filter against a competent
// adversary (see the ADR's "o que este critério NÃO cobre" section).
const LITERAL_MARKERS = [
  'git authority',
  'mode lock',
  'governance prerequisite',
  'reporting boundary',
  'scope boundary',
  'dispatch contract',
]

// HTML_COMMENT_PATTERN removes HTML comments, step 1 of the D3 pipeline.
const HTML_COMMENT_PATTERN = /<!--[\s\S]*?-->/g

// FENCE_PREFIX_PATTERN detects a fence-opening/closing line: optional
// leading whitespace followed by three or more backticks or tildes.
//
// Unlike the Go reference — which is forced into an explicit line-scanner
// because Go's RE2-based regexp package rejects backreferences at runtime
// (see vault/notes/go-regexp-re2-sem-backreference-fenced-block-removal-2026-08-15.md)
// — JS RegExp does support backreferences. This port still uses the same
// explicit line-scanner algorithm as the Go reference (removeFencedBlocks
// below), NOT because JS requires it, but because the scanner is what
// implements the actual CommonMark closing rule correctly (same delimiter
// character, closer repeat count >= opener repeat count, unclosed fence
// never re-opens a heading read). A single backreference regex here would
// diverge from the Go reference on: an unclosed fence, a closing run
// shorter than the opener, and an indented fence — porting the algorithm,
// not the regex, is what keeps behavior byte-identical to Go.
const FENCE_PREFIX_PATTERN = /^\s*(```+|~~~+)/

// trimChar mirrors Go's strings.Trim(s, cutset) for a single-char cutset:
// strips ONLY leading/trailing runs of ch, never occurrences in the middle.
function trimChar(str, ch) {
  let start = 0
  let end = str.length
  while (start < end && str[start] === ch) start++
  while (end > start && str[end - 1] === ch) end--
  return str.slice(start, end)
}

// removeFencedBlocks strips fenced code blocks (``` or ~~~), step 2 of the
// D3 pipeline: lines inside a fence are not read as headings, otherwise
// documentation that merely quotes the marker list would be refused by its
// own criterion. A fence is closed by a line starting with the same
// delimiter character, with at least as many repeats as the opener — the
// CommonMark rule. Mirrors internal/thirdparty/markers.go:removeFencedBlocks.
function removeFencedBlocks(text) {
  const lines = text.split('\n')
  const out = []
  let closer = '' // fence delimiter run that closes the current block, '' if not in a fence
  for (const line of lines) {
    if (closer === '') {
      const match = FENCE_PREFIX_PATTERN.exec(line)
      if (match) {
        closer = match[1]
        continue // drop the opening fence line itself
      }
      out.push(line)
      continue
    }
    // Inside a fence: drop the line; check if it closes the block.
    const trimmed = line.trim()
    const delimChar = closer[0]
    if (trimmed.startsWith(delimChar.repeat(closer.length)) && trimChar(trimmed, delimChar) === '') {
      closer = ''
    }
  }
  return out.join('\n')
}

// HEADING_LINE_PATTERN matches a single, already-collapsed Markdown heading
// line (level 1-6). Applied per-line after step 5, on text that no longer
// contains internal runs of whitespace.
const HEADING_LINE_PATTERN = /^#{1,6}\s+(.*)$/

// WHITESPACE_PATTERN collapses runs of internal whitespace, step 5.
const WHITESPACE_PATTERN = /\s+/g

// checkMarkers applies the D3 objective-refusal criterion to content and
// returns the literal marker names (from LITERAL_MARKERS) that matched as a
// heading. The normalization pipeline, in fixed order, mirrors
// internal/thirdparty/markers.go:CheckMarkers exactly:
//  1. remove HTML comments;
//  2. remove fenced code blocks (``` and ~~~);
//  3. NFKC normalize;
//  4. casefold (String.prototype.toLowerCase(), matching Go's
//     strings.ToLower — not a true Unicode casefold, deliberately, to stay
//     byte-identical to the Go reference);
//  5. collapse internal whitespace + strip, applied per line;
//  6. match only lines matching ^#{1,6}\s+ against the literal marker list.
//
// content may be a Buffer or a string; Buffers are decoded as UTF-8,
// mirroring Go's string(content) over raw bytes.
function checkMarkers(content) {
  let text = Buffer.isBuffer(content) ? content.toString('utf8') : String(content)

  // 1. Remove HTML comments.
  text = text.replace(HTML_COMMENT_PATTERN, '')

  // 2. Remove fenced code blocks — lines inside a fence are not headings.
  text = removeFencedBlocks(text)

  // 3. NFKC normalize.
  text = text.normalize('NFKC')

  // 4. Casefold.
  text = text.toLowerCase()

  const matched = []
  const seen = new Set()
  for (const line of text.split('\n')) {
    // 5. Collapse internal whitespace + strip.
    const collapsed = line.replace(WHITESPACE_PATTERN, ' ').trim()

    // 6. Match only heading lines against the literal marker list.
    const match = HEADING_LINE_PATTERN.exec(collapsed)
    if (!match) continue
    const body = match[1]
    for (const marker of LITERAL_MARKERS) {
      if (body === marker && !seen.has(marker)) {
        matched.push(marker)
        seen.add(marker)
      }
    }
  }
  return matched
}

// checksum returns the SHA-256 hex digest of the raw bytes, before any
// normalization. Mirrors internal/thirdparty/markers.go:Checksum. raw may be
// a Buffer or a string (encoded as UTF-8).
function checksum(raw) {
  const buf = Buffer.isBuffer(raw) ? raw : Buffer.from(String(raw), 'utf8')
  return crypto.createHash('sha256').update(buf).digest('hex')
}

module.exports = { checkMarkers, checksum, literalMarkers: LITERAL_MARKERS, removeFencedBlocks }
