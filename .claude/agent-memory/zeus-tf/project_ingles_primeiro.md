---
name: project-ingles-primeiro
description: Decisão do KG em 2026-11-02 — trackfw é inglês primeiro; marcadores de status passam a ser bilíngues; português no produto é dívida com REQ em backlog
metadata:
  type: project
---

Em 2026-11-02, o KG escolheu a opção A: o produto trackfw é **inglês primeiro**.

- As mensagens do CLI e o conteúdo gerado no consumidor (por exemplo, o `CLAUDE.md` do `init`) devem ser em inglês.
- Os marcadores de status (`⬜ Pendente · 🔄 Em andamento · ✅ Concluído`) são contrato lido pelo `barrier` e pelo `validate`. Eles passam a aceitar inglês e português, sem quebrar o acervo existente.
- A documentação interna em `docs/` (ADRs, REQs, pareceres, vault) continua em português.
- A REQ está em backlog: `REQ-2026-11-02-idioma-do-produto-ingles-primeiro-…`.

**Why:** o README é em inglês e o objetivo é adoção (ver [[project-direcao-consolidar-e-adocao]]). Seguindo o README como usuário novo em 2026-11-02, o guard bloqueou em português citando `CLAUDE.md §1`, uma seção da convenção pessoal do KG que vazou para o produto.

**How to apply:** texto novo emitido pelo binário ou gerado no consumidor sai em inglês. Nunca cite seção de documento que o `init` não gera. Ao revisar o diff de um executor, reprove string nova em português dentro de `internal/`.
