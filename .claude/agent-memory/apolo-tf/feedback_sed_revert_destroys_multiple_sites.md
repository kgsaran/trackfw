---
name: sed-revert-destroys-multiple-sites
description: sed global replace for revert proof strips NormalizeCRLF from ALL matching sites, not just the one being proven — use python3 string replace with exact function body match instead
metadata:
  type: feedback
---

Use `python3` with exact function-body string replacement when reverting a single site for bite proof. Never use `sed -i '' 's/string(NormalizeCRLF(source)))/string(source))/'` across a file with multiple matching sites.

**Why:** In ML-W3b (2026-10-16), `sed -i '' 's/string(NormalizeCRLF(source)))/string(source))/'` was run on `render.go` to revert only `normalizeMarkdown`. The pattern matched ALL 8 occurrences of `TrimSpace(string(NormalizeCRLF(source)))` — removing `NormalizeCRLF` from `markdownParts`, `frontmatterName`, `insertBodyPrefix`, `rewriteSignatureLine`, `rewriteFrontmatterFields`, `rewriteFrontmatterModelLine`, `removeFrontmatterModelLine`. The broken file compiled without error. Only the subsequent test run revealed the damage (tests that inject CRLF started failing because `markdownParts` no longer normalized CRLF). Recovery required a second `sed` to restore all 7 sites.

**How to apply:** For bite proof, use python3 with an exact match of the whole function body:
```python
with open('file.go') as f: content = f.read()
marker = 'func normalizeMarkdown(source []byte) []byte {\n\treturn []byte(strings.TrimSpace(string(NormalizeCRLF(source))) + "\\n")\n}'
replacement = 'func normalizeMarkdown(source []byte) []byte {\n\treturn []byte(strings.TrimSpace(string(source)) + "\\n")\n}'
if marker in content:
    with open('file.go', 'w') as f: f.write(content.replace(marker, replacement, 1))
```
This is immune to collateral damage because the match is unique by construction.
