---
layout: home

hero:
  name: "trackfw"
  text: "ADR → REQ → ROADMAP → kanban"
  tagline: "Software delivery governance for human teams and AI agents."
  actions:
    - theme: brand
      text: Get started
      link: /en/guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/kgsaran/trackfw

features:
  - icon: 🔗
    title: Governance chain
    details: Every line of code traced back to a decision (ADR), a requirement (REQ), and an implementation microbatch (ROADMAP).
  - icon: 🤖
    title: AI-agent native
    details: '"trackfw context --format=json" emits full governance context for LLMs. Claude Code, Gemini CLI, and Cursor already use this rail.'
  - icon: 🌐
    title: Multi-stack
    details: Go, Java, Node.js, Python. React/Vue/Angular frontends. Hooks for husky, lefthook, GitHub Actions, and GitLab CI.
  - icon: 📦
    title: No lock-in
    details: Markdown files versioned in your own repo. No server, no database, no mandatory account.
---

## Installation

::: code-group

```bash [Homebrew]
brew tap kgsaran/trackfw
brew install trackfw
```

```bash [npm]
npm install -g trackfw
```

```bash [Go]
go install github.com/kgsaran/trackfw/cmd/trackfw@latest
```

:::

## In 30 seconds

```bash
trackfw init          # project setup wizard
trackfw adr new       # new architecture decision record
trackfw req new       # new requirement
trackfw roadmap new   # new roadmap linked to a REQ
trackfw status        # project overview
```
