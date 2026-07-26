---
name: trackfw-frontend-skill
description: Accessible, performant and internationalizable user interfaces.
---

## Core Principles

- **Web-first**: design for the browser platform first; native equivalents are additive.
- **Progressive enhancement**: core functionality works without JavaScript; enhancements layer on top.
- **Accessibility is not optional**: WCAG 2.2 AA is the minimum bar for every user-facing interface.
- **Performance is a feature**: Core Web Vitals (LCP < 2.5 s, CLS < 0.1, INP < 200 ms) are
  production quality gates, not post-launch concerns.

## Component Architecture

Build components at the atomic level (atoms → molecules → organisms → pages). Each component
has a single, well-defined responsibility. Compose, do not inherit.

Separate concerns clearly:
- **Data fetching** lives outside components (query layer, data hooks, server components).
- **State** is as local as possible; lift only when two components genuinely share it.
- **UI logic** stays in the component; business rules belong in services or server-side code.

## Accessibility (WCAG 2.2 AA)

- Use semantic HTML as the foundation; ARIA supplements, never replaces, semantics.
- Every interactive element is keyboard-navigable with a visible focus indicator.
- Color contrast ratio ≥ 4.5:1 for normal text, ≥ 3:1 for large text.
- Support `prefers-reduced-motion`; do not auto-play animations.
- Test with at least one screen reader (VoiceOver, NVDA) during development.
- Integrate `axe-core` in CI to catch regressions automatically.

## Performance

- Lazy-load routes and heavy components; code-split at meaningful boundaries.
- Optimize images: responsive sizes, modern formats (WebP/AVIF), explicit `width` and `height`
  to avoid layout shift.
- Virtual scrolling for lists with more than a few hundred items.
- Monitor performance budgets in CI with Lighthouse; fail the build on regressions.

## Internationalisation (i18n)

- Externalize all user-visible strings at the start; retrofitting is expensive.
- Use ICU message format for pluralization and gender rules.
- RTL layout support requires logical CSS properties (`margin-inline-start` over `margin-left`).
- Locale-aware date, number and currency formatting — never hand-roll these.

## State Management

Choose the simplest state model that fits the problem:
- Local component state for ephemeral UI.
- Server-state cache (e.g., query libraries with stale-while-revalidate) for remote data.
- Client-side global state only when truly shared across unrelated subtrees.
- Finite state machines for complex multi-step flows to make impossible states impossible.

## Testing

- Web-first assertions: assert on visible state and accessibility, not internal implementation.
- Do not use `waitForTimeout`; await stable DOM state or network idle instead.
- Component tests verify rendered output and user interactions (click, type, submit).
- E2E tests cover critical paths with real browser automation (Playwright).
- Visual regression for design-system components.

## Design Systems

- Design tokens (W3C DTCG format) are the single source of truth for spacing, color and
  typography. Tokens propagate to both design tools and code.
- Document components in isolation; a component that cannot be demonstrated in isolation
  is too tightly coupled.
- Maintain a clear Figma → dev handoff contract; use Dev Mode or equivalent tooling.
