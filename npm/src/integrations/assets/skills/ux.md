---
name: trackfw-ux-skill
description: User-centred design, design systems, accessibility and usability validation.
---

## User-Centred Design

Design starts with understanding users, not with wireframes. Before sketching, establish:
- **Who** are the users? (personas, JTBD — Jobs to Be Done)
- **What** are they trying to accomplish? (user journey, task analysis)
- **Where** does the current experience fail? (pain points, drop-off analysis)

Design Thinking cycle: Empathize → Define → Ideate → Prototype → Test. The cycle is
iterative; return to earlier phases when testing invalidates assumptions.

## Prototyping

Prototypes are for learning, not for pride. Build the cheapest prototype that can answer the
design question:
- **Low-fidelity** (sketch, wireframe): validates information architecture and flow.
- **High-fidelity** (interactive prototype): validates micro-interactions and visual language.
- **Discardable prototypes**: isolated from production code; exist to prove a concept, not
  to ship. Remove them before the production implementation goes to review.

## Accessibility (WCAG 2.2 AA Minimum)

Accessibility is a legal requirement in many jurisdictions and a quality attribute in all:
- Color contrast ≥ 4.5:1 for body text; ≥ 3:1 for large text and UI components.
- All interactive elements are keyboard-operable with a visible focus indicator.
- ARIA landmarks structure the page for assistive technology; do not use ARIA to override
  broken semantics — fix the semantics first.
- `prefers-reduced-motion`: disable or reduce animations for users who opt out.
- Every form field has a programmatically associated label.
- Test with a real screen reader; automated tools (axe-core) catch ~30–40% of issues.

## Usability Heuristics

Apply Nielsen's 10 heuristics as a review lens, not a checklist:
- **Visibility of system status**: users always know what is happening.
- **Match with the real world**: language and concepts match the user's mental model.
- **User control and freedom**: support undo and clear exit paths.
- **Consistency**: same words, same icons, same placement across the product.
- **Error prevention**: design to make errors impossible before handling them gracefully.

## Design Systems

A design system is a product that serves the product team:
- **Design tokens** (W3C DTCG format): spacing, colour, typography as named variables.
  Tokens are the contract between design and code.
- **Atomic Design**: atoms (button, input) → molecules (form field) → organisms (card) →
  templates → pages. Each level composes from the level below.
- Document components in isolation with rendered examples, props API and accessibility notes.
- A component that cannot be demonstrated without application context is too tightly coupled.

## Validation and Metrics

A design is a hypothesis until tested with users:
- **A/B testing**: one variable, measurable success criterion, sufficient sample size.
  Do not run experiments without a pre-registered hypothesis.
- **Usability testing**: five users reveal the majority of critical usability problems.
  Recruit from the actual target audience, not convenience.
- **Quantitative signals**: task completion rate, time on task, error rate, NPS/CSAT.
- **Qualitative signals**: think-aloud sessions, session replays, support ticket themes.

## Responsive and Adaptive Design

- Mobile-first: design and test the constrained layout first; expand to wider viewports.
- Touch targets ≥ 48 dp (CSS equivalent) for all interactive elements on touch surfaces.
- Fluid typography with `clamp()` to eliminate breakpoint-driven font size jumps.
- Content-driven breakpoints, not device-driven; break where the layout breaks.
