---
name: atribuicao-de-papeis-ml
description: Hefesto (code quality) e Hades (security) não modificam código de produto — nem cópia mecânica; MLs de implementação vão para especialistas
metadata:
  type: feedback
---

Ao distribuir microlotes, lembrar que `hefesto-tf` (code quality) e `hades-tf` (security) são
papéis de **revisão**: auditam, reportam e devolvem o fix para quem é dono do código. Não editam
código de produto.

**Why:** em 2026-07-31 atribuí o ML-2A (copiar três assets estáticos de `internal/serve/static/`
para `npm/` e `pypi/`) ao Hefesto. Ele recusou corretamente, citando o próprio limite de papel:
"You do not modify code". Perdi um ciclo de despacho. Mesmo sendo `cp` mecânico, continua sendo
execução de microlote de implementação.

**How to apply:** MLs que escrevem em arquivos de produto vão para implementadores (Afrodite,
Apolo, Ártemis, Poseidon, Ares, Métis, Dédalo, Prometeu). Hefesto e Hades entram como **barreira**
depois — Hefesto para qualidade/duplicação/cobertura, Hades para superfície de ataque. Continuidade
também importa: quem fez o ML-1A é a escolha natural para o ML corretivo e para o espelhamento,
porque já tem o contexto.

Relacionado: [[verificacao-visual-obrigatoria]]
