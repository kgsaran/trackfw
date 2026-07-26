---
name: trackfw-code-quality-skill
description: Maintainability metrics, quality gates, static analysis and refactoring discipline.
---

## Quality as a Constraint

Quality gates are binary: the build either passes or fails. "Mostly passing" is not a gate.
Define thresholds before writing a line of code; retroactive enforcement is culturally harder.

Core gates:
- **Coverage**: ≥ 85% on the service/domain layer; measured per PR, not only on trunk.
- **Duplication**: < 5% code duplication across the codebase.
- **Cognitive complexity**: ≤ 15 per method/function (Sonar metric).
- **Cyclomatic complexity**: ≤ 10 per method (McCabe); lower is better.
- **Reliability rating**: A or B; no unresolved blocker or critical bugs.

## Static Analysis

Static analysis is the fastest feedback loop. Run it before tests, not after:
- Linters enforce style and catch common mistakes without executing the code.
- Security-focused analyzers (Semgrep, CodeQL) detect injection patterns, misuse of
  cryptographic APIs and permission escalation.
- Language-specific tools provide deeper analysis than generic linters.

Apply the same analysis categories regardless of language:
- Dead code detection: exported symbols with no callers are maintenance overhead.
- Type coverage: avoid `any` / untyped in strongly-typed languages.
- Copy-paste detection: duplicated blocks are candidates for extraction.

## Refactoring Discipline

Refactor in isolation from feature work; separate commits, separate PRs when possible.
Never refactor and change behaviour in the same commit.

Common high-value refactoring moves:
- **Extract Method**: reduces cognitive complexity and enables unit testing of the extracted logic.
- **Replace Conditional with Polymorphism**: eliminates `switch` statements that grow with
  every new type.
- **Extract Class (God Class)**: a class with Weighted Methods per Class > 47 or > 7
  public methods is doing too much.
- **Introduce Parameter Object**: more than 3 parameters to a function signal that a value
  object is missing.

## Architecture Fitness Functions

Architecture rules are code, not documentation:
- Enforce layer dependency direction (domain must not import infrastructure).
- Forbid cross-context imports between bounded contexts.
- Enforce naming conventions per layer (handler/controller, service, repository).
- Run as part of the CI test suite; a failing fitness function blocks merge.

## Conventional Commits

Every commit message follows the Conventional Commits specification:
`<type>(<scope>): <short description>`

Types: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `build`, `ci`, `perf`, `style`.

The commit message explains **why**, not what — the diff shows what.
Breaking changes are marked with `!` after the type or with `BREAKING CHANGE:` in the footer.
Use commitlint in pre-commit hooks to enforce the format automatically.

## Mutation Testing

Mutation testing measures the effectiveness of the test suite by introducing deliberate
defects and checking whether tests catch them. A kill rate ≥ 80% is the target.

A test suite with high coverage but low mutation score is testing execution paths, not
correctness. Mutation testing surfaces untested assertions.

## Technical Debt Management

- Quantify debt as remediation effort (hours), not subjective "bad code" labels.
- Prioritize debt that sits on the critical path of active development.
- A technical debt item with no owner and no resolution date is not managed — it is ignored.
- Budget 10–20% of sprint capacity for debt reduction when the codebase is under active
  development; higher when the debt is blocking feature velocity.
