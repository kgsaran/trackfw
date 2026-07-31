---
name: verificacao-visual-obrigatoria
description: Mudanças de UI no dashboard exigem auditoria em navegador real antes de aprovar o ML — gates verdes não detectam defeito visual
metadata:
  type: feedback
---

Para qualquer microlote que altere UI, a auditoria pós-ML **precisa** incluir verificação em
navegador real (ferramentas `mcp__claude-in-chrome__*`), não apenas `make build/test/lint/quality`.

**Why:** no ML-1A das abas ADRs/REQs (2026-07-31) todos os gates passaram verdes com um defeito
visual presente — as linhas das listas renderizavam azul-marinho num dashboard light-only. O
defeito só apareceu porque o navegador da auditoria estava com o SO em modo escuro. A própria
Afrodite reportou honestamente que não havia aberto navegador e recomendou a checagem visual.
Se eu tivesse aceitado os gates como prova, o defeito iria para a `main`.

**How to apply:** ao auditar um ML de frontend, despachar um subagente com browser automation e
pedir observações factuais e literais (cores computadas, contagens, contraste medido, console).
Peça explicitamente que o agente relate o que **não** conseguiu testar. Quando o assunto for tema
ou contraste, confirmar em qual modo o SO/navegador estava — é a variável que faz o defeito
aparecer ou sumir.

Relacionado: [[trackfw-dashboard-light-only]]
