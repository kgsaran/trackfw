You are the trackfw implementation orchestrator. Follow the flow below **without skipping steps**.

## Argument

`$ARGUMENTS` is optional. If provided, it is used as a filter (case-insensitive substring) over the REQ file names.

---

## Step 1 — Select the REQ

Use Glob to list `docs/req/*.md`.

- If **no file found**: inform that there are no REQs available and suggest creating one with `/trackfw:req`.
- If **`$ARGUMENTS` was provided** and filters to exactly one REQ: use it directly.
- In **all other cases** (no argument, or ambiguous argument): present the list of available REQs and ask the user which to implement.

Read the full contents of the selected REQ.

---

## Step 2 — Find or generate the Roadmap

Check whether a roadmap linked to the REQ exists by searching in `docs/roadmaps/` (backlog, wip, blocked, done, abandoned) for a file whose name contains the REQ slug.

**If the roadmap does not exist yet:**
- Inform the user: "No roadmap found for this REQ. Generating now..."
- Execute the full generation flow from `/trackfw:roadmap` (read the file `.claude/commands/trackfw/roadmap.md` to follow the exact instructions), passing the already-selected REQ — do not ask again.
- Save the generated roadmap at `docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`.

**If the roadmap exists and is already in `done/` or `abandoned/`:**
- Inform the user and ask whether to create a new roadmap or stop.

**If the roadmap exists in `backlog/` or `blocked/`:**
- Proceed to Step 3.

**If it is already in `wip/`:**
- Proceed directly to Step 4 (already in progress).

---

## Step 3 — Move roadmap to WIP

Run:
```bash
trackfw roadmap move <roadmap-name> wip
```

Confirm that the file was moved to `docs/roadmaps/wip/`.

---

## Step 4 — Read and present the plan

Read the roadmap (now in `wip/`). Present to the user:
- Roadmap title
- Total Waves and MLs
- Summarized list of MLs per Wave

Confirm: "Starting implementation. I will execute each ML in order and update the roadmap at each completion."

---

## Step 5 — Execute each ML in order

For each Wave (in sequence), execute the Wave's MLs:

### For each ML:

**5a. Announce:** inform which ML is being executed (e.g. "Executing ML-1A — Create client.go").

**5b. Implement:** execute the actions described in the ML using your tools (Read, Write, Edit, Bash). Follow exactly the files affected, actions and acceptance criteria listed in the roadmap.

**5c. Validate:** run the ML's validation commands. If it fails, fix before advancing.

**5d. Update the roadmap:** edit the roadmap file in `docs/roadmaps/wip/` replacing the ML status:
- `**Status:** ⬜ Pending` → `**Status:** ✅ Done`

**5e. Commit:**
```bash
git add <ml-files>
trackfw commit -m "feat(<scope>): <ML description>"
```

Only advance to the next Wave after all MLs of the current Wave are ✅.

---

## Step 6 — Finalize

When all MLs are ✅:

**6a.** Run `trackfw validate` — must pass with zero violations.

**6b.** Move the roadmap to done:
```bash
trackfw roadmap move <roadmap-name> done
```

**6c.** Make the final commit:
```bash
git add docs/roadmaps/
git commit -m "docs(trackfw): roadmap <name> → done"
```

**6d.** Inform the user:
```
✅ Implementation complete.
Roadmap: docs/roadmaps/done/<name>.md
Next step: open PR with gh pr create
```