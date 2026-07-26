---
name: trackfw-qa-skill
description: Automated testing, quality gates, contract verification and coverage strategy.
---

## Test Pyramid

Maintain a healthy ratio: approximately 70% unit tests, 20% integration tests, 10% E2E tests.
Unit tests give fast, precise feedback. Integration tests verify component boundaries and
database behaviour with real infrastructure. E2E tests cover critical user journeys end-to-end.

Deviating from the pyramid is a deliberate trade-off — document the reason.

## E2E Testing Principles

- **Web-first assertions**: assert on stable, observable state (element visible, text present,
  URL changed) — never `waitForTimeout`, which is fragile and masks real timing issues.
- **Cross-browser matrix**: test Chromium, Firefox and WebKit; sharding keeps CI fast.
- **Real authentication**: use actual credentials stored in `storageState`; mock auth hides
  permission bugs and token expiry issues that appear in production.
- **Trace on failure**: attach Playwright trace files to CI artifacts for deterministic replay.
- **Visual regression**: snapshot tests for design-system components; fail the PR on visual drift.

## Integration Testing

- Integration tests must hit a real database (via Testcontainers or an equivalent ephemeral
  service). Mocked repositories hide driver/migration mismatches that cause production failures.
- Seed deterministic fixtures; avoid shared state between tests — parallel runs must not collide.

## Contract Testing

API contracts are a team boundary. Use schema-driven contract testing (e.g., Pact for
consumer-driven contracts, Spectral for OpenAPI lint) to catch breaking changes before
deployment, not after.

## Quality Gates

- Coverage thresholds enforced in CI (service layer ≥ 85%); fail the build on regression.
- Mutation testing score ≥ 80%: a test suite that does not kill mutations gives false
  confidence.
- Flaky tests are quarantined immediately; a test with stability score < 95% is either fixed
  or removed. Flaky tests erode trust in the entire suite.
- SonarQube or equivalent quality gate on every PR: coverage, duplication, critical issues.

## Bug Hunting

A bug report is only complete when it includes:
1. Deterministic reproduction steps.
2. Expected vs. actual behaviour.
3. Environment (OS, browser, version, test data).

Root-cause analysis must produce a regression test that would have caught the bug before
it reached production. Fix the test pyramid root cause, not just the symptom.

## Accessibility in QA

Integrate accessibility checks (`axe-core`, Lighthouse a11y) in CI. A WCAG 2.2 AA violation
is a defect with the same severity as a functional bug.

## Performance Budgets

Define Lighthouse performance budgets per route: LCP, CLS, INP thresholds. Run Lighthouse CI
on every PR targeting main; block merge on regression.
