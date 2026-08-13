---
name: fronteira-de-escrita-dos-auditores-corrigida-no-gerador
description: Auditores (Hefesto/Hades/Atena) agora podem escrever docs designadas — contradição corrigida no gerador, não no ~/.claude; exige `trackfw agents update` para valer
metadata:
  type: feedback
---

Os três agentes **auditores** — `code-quality` (Hefesto), `security` (Hades), `ux` (Atena) — **podem
e devem** escrever os próprios artefatos: relatório/parecer, entrada em
`docs/agents-working-context.md`, e **documentação designada pelo orquestrador** (`docs/cli-parity.md`,
README). Recusar isso é **erro de escopo na direção oposta**, e o texto do agente agora diz isso
explicitamente. Eles continuam proibidos de modificar **código de produto** (`internal/`, `npm/src/`,
`pypi/trackfw/` e testes).

**Why:** em 2026-08-12 Hefesto recusou um ML de documentação após tê-lo executado em 4 PRs na mesma
sessão. A causa não era indisciplina: o asset se contradizia em 3 pontos — `tools:` não concedia
`Write`/`Edit` mas o arquivo ordenava append no working context; *"You do not modify code"* colidia
com *"Do not edit code without a requirement…"* e com *"refuse to implement…"*. Prova de que a
ambiguidade gerava roteamento imprevisível: sob a mesma redação, Hades escrevia pareceres sem
reclamar enquanto Hefesto recusava. Corrigido no **PR #165**.

**How to apply:** despachar normalmente MLs de documentação para os auditores. **Dois cuidados que
permanecem:**

1. **Corrigir sempre no gerador**, `{internal,npm/src,pypi/trackfw}/integrations/assets/agents/` —
   `~/.claude/agents/` é **artefato gerado** e é sobrescrito a cada `trackfw update`. Foi KG quem
   apontou isso.
2. **A correção só vale depois de `trackfw agents update`** (sem `--force`, se o arquivo nunca foi
   editado à mão). Se um auditor recusar de novo, **verifique primeiro se o agente instalado foi
   atualizado** antes de suspeitar do prompt.

**Nota de rede fina:** o `check-integration-assets.sh` verifica **paridade entre stacks**, não
**conteúdo** — dá para alterar o texto de um agente sem gate nenhum acusar, desde que os 3 stacks
mudem juntos.

Relacionado: [[feedback-zeus-subagent-type]] — mesma família, roteamento de especialista que falha em
silêncio.
