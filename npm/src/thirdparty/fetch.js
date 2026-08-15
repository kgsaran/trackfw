'use strict'

// Node port of internal/thirdparty/fetch.go — the network fetch primitive
// used by the two-phase third-party artifact gate (skills/agents installed
// from a URL). See:
// docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md
// (D7).

const https = require('node:https')
const { URL } = require('node:url')

// MAX_CONTENT_SIZE is the network fetch cap for third-party text artifacts
// (D7): 2 MiB — mirrors internal/thirdparty/fetch.go:maxContentSize.
const MAX_CONTENT_SIZE = 2 * 1024 * 1024

// MAX_REDIRECTS mirrors internal/thirdparty/fetch.go:maxRedirects (3). Note
// the Go reference's own net/http.Client.CheckRedirect semantics: the check
// runs BEFORE following a redirect and compares the count of requests
// ALREADY COMPLETED (via.length, which includes the request that just
// returned the redirect) against maxRedirects — so in practice only
// (maxRedirects - 1) redirects are ever followed before the next one is
// refused. Reproduced here byte-for-byte via requestsCompleted, not
// "fixed" to what the doc comment alone might suggest.
const MAX_REDIRECTS = 3

const TIMEOUT_MS = 30 * 1000

// ALLOWED_CONTENT_TYPES mirrors internal/thirdparty/fetch.go:allowedContentTypes.
const ALLOWED_CONTENT_TYPES = new Set(['text/plain', 'text/markdown', 'text/x-markdown'])

// contentTypeAllowed mirrors internal/thirdparty/fetch.go:contentTypeAllowed
// — tolerates an optional "; charset=..." (or any other parameter) suffix.
function contentTypeAllowed(contentType) {
  const raw = String(contentType || '')
  const semi = raw.indexOf(';')
  const base = (semi >= 0 ? raw.slice(0, semi) : raw).trim().toLowerCase()
  return ALLOWED_CONTENT_TYPES.has(base)
}

// api is the module.exports object itself, referenced internally via
// `api.requestOnce(...)` so tests can substitute the single HTTP round
// trip without a real TLS socket — mirrors the thirdPartyFetch
// package-var substitution pattern already used at the command layer
// (npm/src/commands/thirdparty.js). Added in ML-4C to close a coverage
// gap hefesto-tf found: the resp.statusCode !== 200 branch below existed
// but was never exercised by any test.
const api = {}

function requestOnce(rawURL) {
  return new Promise((resolve, reject) => {
    const req = https.get(rawURL, { timeout: TIMEOUT_MS }, res => {
      const chunks = []
      let total = 0
      let tooLarge = false
      res.on('data', chunk => {
        if (tooLarge) return
        total += chunk.length
        if (total > MAX_CONTENT_SIZE) {
          tooLarge = true
          res.destroy()
          return
        }
        chunks.push(chunk)
      })
      res.on('end', () => {
        resolve({ statusCode: res.statusCode, headers: res.headers, body: Buffer.concat(chunks), tooLarge })
      })
      res.on('error', err => reject(new Error(`failed to read response body: ${err.message}`)))
    })
    req.on('timeout', () => req.destroy(new Error('fetch failed: request timed out after 30s')))
    req.on('error', err => reject(new Error(`fetch failed: ${err.message}`)))
  })
}

// fetchOnce follows redirects with the same accounting as Go's
// fetchClient.CheckRedirect (see MAX_REDIRECTS doc above). requestsCompleted
// starts at 0 and is incremented after every completed HTTP request.
async function fetchOnce(rawURL, requestsCompleted) {
  let parsed
  try {
    parsed = new URL(rawURL)
  } catch (err) {
    throw new Error(`invalid URL "${rawURL}": ${err.message}`)
  }
  if (parsed.protocol !== 'https:') {
    throw new Error(`refused: URL scheme must be https, got "${parsed.protocol.replace(/:$/, '')}"`)
  }

  const res = await api.requestOnce(rawURL)
  requestsCompleted += 1

  if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
    if (requestsCompleted >= MAX_REDIRECTS) {
      throw new Error(`fetch failed: stopped after ${MAX_REDIRECTS} redirects`)
    }
    let next
    try {
      next = new URL(res.headers.location, rawURL).toString()
    } catch (err) {
      throw new Error(`invalid redirect location: ${err.message}`)
    }
    const nextScheme = new URL(next).protocol
    if (nextScheme !== 'https:') {
      throw new Error(`redirect to non-https URL refused: ${next}`)
    }
    return fetchOnce(next, requestsCompleted)
  }

  if (res.statusCode !== 200) {
    throw new Error(`fetch failed: HTTP ${res.statusCode} for ${rawURL}`)
  }

  const contentType = res.headers['content-type']
  if (!contentTypeAllowed(contentType)) {
    throw new Error(`refused: unsupported Content-Type "${contentType || ''}" (allowed: text/plain, text/markdown, text/x-markdown)`)
  }

  if (res.tooLarge) {
    throw new Error(`refused: content exceeds ${MAX_CONTENT_SIZE} bytes`)
  }

  return res.body
}

// fetch downloads the content at rawURL under the D7 network policy:
// https-only (validated before the first request), a 30s timeout, at most
// (MAX_REDIRECTS - 1) redirect hops (each revalidated for https), a 2 MiB
// size cap, and a Content-Type allowlist. Mirrors
// internal/thirdparty/fetch.go:Fetch.
async function fetch(rawURL) {
  return fetchOnce(rawURL, 0)
}

api.fetch = fetch
api.contentTypeAllowed = contentTypeAllowed
api.requestOnce = requestOnce
api.MAX_CONTENT_SIZE = MAX_CONTENT_SIZE
api.MAX_REDIRECTS = MAX_REDIRECTS

module.exports = api
