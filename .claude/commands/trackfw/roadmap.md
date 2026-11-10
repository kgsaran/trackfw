Generate an implementation roadmap in microbatches for a project REQ.

## Steps

1. **List available REQs**
   Use Glob to list `docs/req/*.md`. If no file found, inform:
   > No REQ found in `docs/req/`. Create one first with `/trackfw:req`.

2. **Select the REQ**
   - If `$ARGUMENTS` was provided: use it as a filter (case-insensitive substring) to find the file
   - If not provided or the filter does not find exactly one: list the available files and ask the user which to use
   - Read the full contents of the selected REQ file

3. **Generate the roadmap**
   Based on the REQ content, generate a roadmap following **strictly** this format:

````markdown
---
status: backlog
date: <YYYY-MM-DD>
req: "docs/req/<selected-file>.md"
squad: ""
---

# Roadmap: <title derived from REQ>

> Created: <YYYY-MM-DD> | Status: backlog

## Context / Diagnosis
<summary of the problem, motivation and scope extracted from the REQ>

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pending
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Wave gates:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
# each line runs as a separate sh -c — see docs/cli-parity.md rule 5
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

## Wave 1 — <descriptive name> (<N> MLs in parallel)
> Dependencies: Independent

### ML-1A — <title>
**Status:** ⬜ Pending
**Files affected:**
- `exact/path/to/file`
**Actions:**
- Detailed description of the action with exact values, keys and commands
**Acceptance criteria:**
- [ ] build without errors
- [ ] tests green
**Validation commands:** `<project build and test command>`

### ML-1B — <title> (if independent of ML-1A)
...

## Wave 2 — <name> (depends on Wave 1)
> Dependencies: Wave 1 complete
...
````

   **Mandatory principles:**
   - MLs within the same Wave are **independent** (distinct files, no conflict)
   - Each ML must be detailed enough for execution by an agent without extra context
   - Maximize parallelism: group in parallel everything that does not share files
   - Sequential Waves only when there is a real dependency on a result
   - Measurable acceptance criteria in each ML

4. **Save the file**
   - Compute the slug: title in lowercase, spaces → hyphens, remove special characters
   - Create the file at `docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`
   - Fill `req:` with the full relative path of the selected REQ
   - Use today's date

5. **Confirm**
   Report the path of the created file and a summary of Waves and total MLs generated.
