---
layout: home

hero:
  name: "trackfw"
  text: "ADR → REQ → ROADMAP → kanban"
  tagline: "Governança de entrega de software para times humanos e agentes de IA."
  actions:
    - theme: brand
      text: Começar agora
      link: /guide/getting-started
    - theme: alt
      text: Ver no GitHub
      link: https://github.com/kgsaran/trackfw

features:
  - icon: 🔗
    title: Cadeia de governança
    details: Cada linha de código rastreada até uma decisão (ADR), um requisito (REQ) e um microlote de implementação (ROADMAP).
  - icon: 🤖
    title: Nativo para agentes de IA
    details: '"trackfw context --format=json" emite o contexto de governança completo para LLMs. Claude Code, Gemini CLI e Cursor já usam esse trilho.'
  - icon: 🌐
    title: Multi-stack e multi-linguagem
    details: Go, Java, Node.js, Python. Frontend React/Vue/Angular. Hooks para husky, lefthook, GitHub Actions e GitLab CI.
  - icon: 📦
    title: Sem lock-in
    details: Arquivos Markdown versionados no próprio repositório. Sem servidor, sem banco de dados, sem conta obrigatória.
---

## Instalação

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

## Em 30 segundos

```bash
trackfw init          # wizard de configuração do projeto
trackfw adr new       # nova decisão arquitetural
trackfw req new       # novo requisito
trackfw roadmap new   # novo roadmap vinculado à REQ
trackfw status        # visão geral do projeto
```
