package generators

import (
	"bytes"
	"encoding/json"
)

// marshalJSONNoEscape encodes v as indented JSON with HTML escaping disabled.
//
// json.MarshalIndent and json.Marshal HTML-escape '<', '>' and '&' to
// '<', '>' and '&'. The D11 fail-closed hook line (ML-6B /
// ADR-2026-10-04 D11) contains '>' in '2>${null-/dev/null}' which must
// survive as a literal '>' in the emitted JSON file — the shell receives the
// exact bytes written to disk, so '>' would give the shell a literal
// backslash-u sequence instead of a redirect operator and break the polyglot.
//
// The returned slice has no trailing newline, matching json.MarshalIndent's
// contract. Callers that need a POSIX-terminated file should append('\n').
func marshalJSONNoEscape(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	// Encode always appends a trailing '\n'; strip it so the returned slice
	// length matches what json.MarshalIndent would produce.
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
