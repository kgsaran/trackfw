---
name: writescope-lowercase-opencode-bypass
description: OpenCode lowercase tool names ("write","edit","shell") bypass write-scope Step 4; frozen paths never checked; five independent prerequisites to fix (measured 2026-11-19)
metadata:
  type: project
---

**Fact:** `writeScopeWriteTools` (writescope.go:45–48) contains only PascalCase keys: `"Write"`, `"Edit"`, `"NotebookEdit"`. Step 4 of write-scope does an exact case-sensitive lookup and returns allow if the tool name is not found. OpenCode sends lowercase tool names (`"write"`, `"edit"`, `"shell"`), so Step 4 returns allow without ever reaching the frozen-paths check.

**Why:** Measured 2026-11-19 with branch binary v11.7.0 (1f8e82c0): `tool_name="write"` + `agent_id` + `.agents/hooks.json` → rc=0. `tool_name="Write"` + same → rc=2.

**Five independent prerequisites to actually protect OpenCode config paths (all measured 2026-11-19):**
1. Add `.opencode/plugins/`, `.opencode/opencode.json` etc. to `writeScopeSubagentFrozenRelPaths`
2. Accept lowercase tool names in `writeScopeWriteTools` OR have the plugin map to PascalCase before calling guard
3. Plugin forwards `agent_id` only for subagent calls (V2: `event.agent` is non-optional, present even in main session; V1: `input` has no `agent` field at all)
4. Path value must be **absolute** — write-scope rejects relative paths with rc=2 "not an absolute path"
5. Path field key must be `file_path` — not `filePath` (V1) nor `path` (V2). Wrong key → rc=2 "tool_input.file_path absent" (blocks ALL Write calls — false positive)

**Note on v11.5.0 probes:** bin 11.5.0 (pre-ML-FA2) lacked `.agents/hooks.json` in `writeScopeSubagentFrozenRelPaths`, so probes with that path on 11.5.0 returned rc=0. The measurements above were done with the working-tree build at `1f8e82c0` (v11.7.0 equivalent) which includes the ML-FA2 entry.

**Credential guard on Write/Edit (additional gap):** Claude Code wires credential guard to `Bash|PowerShell`, `Read`, AND `Write|Edit` matchers (agentfiles.go:414–417). An OpenCode plugin that only calls credential guard for shell calls misses credentials in `content`/`newString`. Confirmed by `validate --json` warnings for all three matchers separately.

**How to apply:** when auditing any new CLI that uses lowercase tool names or non-standard path keys, verify (a) Step 4 behavior, (b) path field name, and (c) credential guard matchers against the agentfiles.go source. The fix for point 2 (lowercase) can be done inside the plugin without changing shared code (map to PascalCase in the plugin).

Related: [[write-scope-global-hooks-not-protected]]
