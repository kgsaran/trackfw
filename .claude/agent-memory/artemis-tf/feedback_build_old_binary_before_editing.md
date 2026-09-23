---
name: build-old-binary-before-editing
description: Build the test binary from the current (buggy) code BEFORE applying any fix, then apply fix and build new binary — enables load-bearing proof without git stash
metadata:
  type: feedback
---

Build the old binary as the very first Bash call after reading the file, before any Edit — `go test -c ./internal/<pkg>/ -o <scratch>/pkg-old.test`. Then apply the fix and build `pkg-new.test`. Run both against the adversarial scenario to prove RC differs.

**Why:** if you edit first, you need `git show HEAD:...` into a scratch package to reconstruct the old binary, which is slower and more error-prone. The advisor flagged this sequencing error before the first edit was made.

**How to apply:** in any ML that requires load-bearing proof against old vs. new behavior in a Go test file, build the old binary as step 1 (before reading the file for editing), not as step 3 after the fix.
