---
name: trackfw-tooling-skill
description: AI agent design, skill composition, MCP configuration and context economy.
---

## Agent Design Principles

An agent is effective when its scope, tools and constraints are explicit. Vague instructions
produce vague behaviour:
- **Lock the role**: every agent declares its role at the top of its system prompt and does
  not switch roles mid-session without an explicit handoff.
- **Declare tools explicitly**: list only the tools the agent needs for its role; an agent
  with write access that only needs to read is a liability.
- **Explicit memory scope**: use project-scoped memory for context that belongs to the
  repository; use global memory only for cross-project preferences.
- **Restrict scope**: an agent must know what it is forbidden to do (create branches, open PRs,
  modify configuration outside its domain). Prohibitions are as important as permissions.

## Skill Design

Skills extend agents with domain expertise loaded on demand:
- **Single responsibility**: one skill per role or domain. A skill that covers both backend
  and frontend is too broad to be useful without loading the entire file.
- **Stack-agnostic content**: skills contain principles, patterns and vocabulary that define
  the role. Stack-specific choices (library names, project configuration) belong in the
  project's CLAUDE.md, not in a shared skill.
- **Right size**: 40–90 lines is the target. A 5-line skill teaches nothing; a 300-line
  skill inflates context on every invocation. Prefer principles over tool lists.
- **Load on demand**: skills are not always loaded. Design them as self-contained documents
  that provide value when read in isolation, without assuming other skills are present.

## MCP (Model Context Protocol)

MCP enables agents to interact with external tools and services through a standardised protocol:
- Define tool schemas precisely: name, description, input schema, output schema.
- Group related tools in a tool set to reduce context size when only a subset is needed.
- Prefer tool sets over individual tool registrations when the agent needs more than five tools.
- Validate MCP server responses before acting on them; a malformed response must not
  silently corrupt state.
- Document which MCP servers a configuration depends on; an agent that fails silently because
  a server is unavailable is hard to debug.

## Prompt Engineering

- **Specificity over length**: a short, specific prompt outperforms a long, vague one.
  Remove filler instructions that repeat the model's defaults.
- **Reusable prompt files**: extract repeated prompt patterns into versioned, reviewable files.
  Treat prompts as code: they have bugs, they drift, they need tests.
- **Context economy**: every token in context is a cost. Load only the context that is
  relevant to the current task. Unload when done.
- **Structured output**: when the downstream consumer is code (not a human), request JSON
  or a defined schema. Freeform prose is hard to parse reliably.

## Validating Recommendations

Before recommending a tool, configuration option, or API:
1. Check the official documentation for the current version.
2. Verify that the feature or flag exists in the version the project is using.
3. Test in a minimal example before including in a recommendation.

A memory that says "feature X exists" is not the same as "feature X exists now." Models have
knowledge cutoffs; documentation does not. When in doubt, read the source.

## Human-in-the-Loop

Define checkpoints where human review is mandatory:
- Irreversible operations (production deploys, data deletions, secret rotation).
- Operations with significant blast radius (schema migrations, dependency upgrades).
- Decisions that bifurcate the architecture (new service boundary, new data model).

Agents that can proceed without human input should declare that capability explicitly in their
design. Agents that are uncertain must ask, not guess.

## Observability for AI Systems

- Log prompts, tool calls, token counts and latency for every agent interaction in production.
- Monitor for response quality drift: a pipeline that produces correct output today may not
  produce correct output after a model update.
- Evaluation datasets: maintain a set of representative inputs with expected outputs; run
  them on every model or prompt change.
