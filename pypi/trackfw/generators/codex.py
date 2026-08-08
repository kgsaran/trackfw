"""Repo-scoped OpenAI Codex integration."""

SKILLS = {
    "trackfw-governance": """---
name: trackfw-governance
description: Inspect or maintain ADR, REQ, ROADMAP, traceability, lifecycle state, and trackfw validation. Use for governance questions and artifact changes.
---
Run `trackfw context --format=json`, preserve ADR → REQ → ROADMAP, and finish with `trackfw validate --json`.
""",
    "trackfw-plan": """---
name: trackfw-plan
description: Plan a trackfw-governed implementation before code changes. Use for ADRs, REQs, roadmaps, microbatches, acceptance criteria, and validation commands.
---
Inspect artifacts and code. Create required decisions and requirements before a decision-complete roadmap. Keep it analyzing until coding starts.
""",
    "trackfw-implement": """---
name: trackfw-implement
description: Implement work governed by an existing trackfw roadmap. Use for features and fixes that must follow lifecycle and validation gates.
---
Identify the matching REQ and roadmap, move it to wip, implement one microbatch at a time, run tests and `trackfw validate --json`, then update lifecycle state.
""",
    "trackfw-review": """---
name: trackfw-review
description: Review a change for governance traceability, correctness, security, regressions, missing tests, and roadmap acceptance.
---
Stay read-only unless fixes are requested. Report findings by severity and verify the linked artifacts, tests, and validation output.
""",
    "trackfw-release": """---
name: trackfw-release
description: Prepare and verify a trackfw release. Use for versioning, changelog, packaging, parity, and publication gates.
---
Run all quality and parity gates, derive SemVer from commits, and stop before tags or publication unless explicitly authorized.
""",
}

AGENTS = {
    "trackfw-architect.toml": """name = "trackfw_architect"
description = "Architecture and implementation-planning specialist for governed changes."
sandbox_mode = "read-only"
developer_instructions = \"\"\"Map architecture, contracts, constraints, and traceability. Require ADRs for unresolved material decisions. Produce decision-complete plans.\"\"\"
""",
    "trackfw-backend.toml": """name = "trackfw_backend"
description = "Backend implementation specialist for APIs, domain logic, integrations, Go, Java, Node.js, and Python."
developer_instructions = \"\"\"Implement only the assigned backend scope, preserve contracts and traceability, and run focused tests.\"\"\"
""",
    "trackfw-frontend.toml": """name = "trackfw_frontend"
description = "Frontend specialist focused on accessibility, i18n, UX consistency, and browser behavior."
developer_instructions = \"\"\"Implement only the assigned frontend scope and run focused build and UI tests.\"\"\"
""",
    "trackfw-qa.toml": """name = "trackfw_qa"
description = "Read-only QA specialist for regressions, flaky tests, critical flows, and acceptance coverage."
sandbox_mode = "read-only"
developer_instructions = \"\"\"Trace critical flows and report reproducible failures and missing coverage.\"\"\"
""",
    "trackfw-security.toml": """name = "trackfw_security"
description = "Read-only security reviewer for trust boundaries, secrets, injection, permissions, and unsafe defaults."
sandbox_mode = "read-only"
developer_instructions = \"\"\"Perform evidence-backed threat analysis and report concrete mitigations by severity.\"\"\"
""",
    "trackfw-reviewer.toml": """name = "trackfw_reviewer"
description = "Owner-level reviewer for correctness, governance, maintainability, security, and missing tests."
sandbox_mode = "read-only"
developer_instructions = \"\"\"Return findings first, ordered by severity, with file references and governance evidence.\"\"\"
""",
}
