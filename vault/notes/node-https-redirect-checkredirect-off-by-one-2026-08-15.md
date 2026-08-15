# Go's `net/http.CheckRedirect` counts completed requests, not followed redirects — `maxRedirects=3` only follows 2 hops

> 2026-08-15 · ML-2A do `ROADMAP-2026-08-15-instalacao-de-skills-de-terceiro-via-url-para-agentes-especialistas`
> (`npm/src/thirdparty/fetch.js`, porte de `internal/thirdparty/fetch.go`)

## O que o Go faz

`internal/thirdparty/fetch.go` define:

```go
const maxRedirects = 3

var fetchClient = &http.Client{
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= maxRedirects {
			return fmt.Errorf("stopped after %d redirects", maxRedirects)
		}
		...
	},
}
```

`net/http`'s `CheckRedirect` is invoked **before following each redirect**, and `via` is the
slice of **requests already completed** (the original request plus every redirect already
followed) — not the count of redirects the caller is about to follow.

Tracing it: `via` has length 1 after the original request returns a redirect (`1 >= 3`? no →
follow). After the 2nd request (1st redirect followed) returns a redirect, `via` has length 2
(`2 >= 3`? no → follow). After the 3rd request (2nd redirect followed) returns a redirect,
`via` has length 3 (`3 >= 3`? **yes** → refuse). So **only 2 redirects are ever followed**
before the 3rd is refused, even though the constant is named `maxRedirects = 3` and the doc
comment says "at most 3 redirect hops". This is exactly what
`TestFetch_RefusesFourthRedirect` in `internal/thirdparty/fetch_test.go` asserts: a server that
redirects endlessly is refused, and the naming makes it easy to misread as "3 hops allowed"
when the real behavior is "the 3rd redirect attempt is refused" (2 hops followed).

## Why this matters for porting

Node's `https` module (or any HTTP client) does not expose an equivalent hook with the same
`via`-counts-completed-requests semantics by default — a naive Node port using "redirects
followed so far, starting at 0, refuse when `>= maxRedirects`" would follow **3** hops instead
of 2, diverging from the Go reference's actual behavior in the boundary case the Go test suite
explicitly covers.

## The rule

When porting a Go HTTP client's `CheckRedirect`-style redirect cap to Node/Python, count
**completed requests** (starting at 1 after the first response, or 0 before any request and
incrementing after each), not "redirects followed". Check the cap **after** incrementing, right
before deciding whether to follow the next `Location` header. `npm/src/thirdparty/fetch.js`
does this via a `requestsCompleted` parameter threaded through recursive `fetchOnce` calls.

## How to recognize the divergence

- The Go doc comment/constant name (`maxRedirects = 3`, "at most 3 redirect hops") does NOT
  match the observable behavior (2 hops followed, 3rd refused) — this is a property of how
  `net/http.Client.CheckRedirect`'s `via` accounting works, not a bug in the trackfw code.
- A port that "fixes" the off-by-one to match the doc comment's literal wording (3 hops
  followed) will pass its own unit tests but silently diverge from the Go binary's actual
  network behavior on a redirect chain of exactly 3-4 hops.

## Related

- `internal/thirdparty/fetch.go` — `fetchClient.CheckRedirect`, `maxRedirects`.
- `internal/thirdparty/fetch_test.go` — `TestFetch_RefusesFourthRedirect`.
- `npm/src/thirdparty/fetch.js` — `fetchOnce`, `MAX_REDIRECTS` (doc comment states the
  off-by-one explicitly to prevent a future "fix").
- For whoever ports Python (ML-2B): `urllib`/`requests` redirect handling has its own counting
  convention (usually "redirects followed", not "requests completed") — apply the same
  `requestsCompleted`-style accounting there too, don't assume `max_redirects=3` means 3 hops.
