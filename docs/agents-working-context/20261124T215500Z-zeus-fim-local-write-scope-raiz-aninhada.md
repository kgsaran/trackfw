# Zeus — REQ-2026-11-23 pronta localmente; aguarda PR para o CI

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** fix/guard-write-scope-re-enraizado-trackfw-yaml-aninhado (HEAD 827c06f4, pushed)
- **Mudou:**
  - ADR-2026-11-24 (D1–D6), com uma emenda no ADR-2026-10-23. O que entrou na Wave 2:
    - fluxo NTFS e 8dot3, inclusive a forma com hash;
    - D3 vale também para a detecção de raiz e para o wrapper de shell;
    - D5: leases de todos os ancestrais;
    - D6: congelados comparados também na grafia normalizada.
  - Wave 1 (ML-1A..1E) e Wave 2 (ML-2A..2G) concluídas. Hefesto e Hades revisaram duas vezes; a segunda rodada não achou escape.
  - `make quality` rc=0 e barrier da Wave 2 `passed`.
- **Pendente:**
  - ML-3A: o CI só roda em `pull_request`, então precisa de PR (draft) quando o KG pedir.
  - Fechamento só depois do merge.
- **Decisões do KG em aberto:**
  - se o `mode: warn` de projeto deixa de rebaixar o hook global (implementado como bloqueio sempre e reversível);
  - se MultiEdit e OpenCode (R-5/R-6) ganham REQ própria.
- **Windows:** não medido em Windows real. Os vetores no macOS provam o casamento de strings.
