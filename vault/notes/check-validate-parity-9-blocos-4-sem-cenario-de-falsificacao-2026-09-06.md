# `check-validate-parity.sh` tem 9 blocos cross-CLI; 4 não têm cenário de falsificação em `check-gates-falsify.sh` — a lacuna do ML-1C não era isolada

**Data:** 2026-09-06
**Onde:** `scripts/check-validate-parity.sh`, `scripts/check-gates-falsify.sh`
**Achado por:** artemis-tf (QA), ML-1E (ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio)

## Contexto

O ML-1C acrescentou 3 blocos a `check-validate-parity.sh` (script ilegível project-scope, script
ilegível GLOBAL-scope, FIFO) sem nenhum cenário em `check-gates-falsify.sh` provando que cada um
reprovaria se a produção regredisse. O ML-1D achou isso incidentalmente (investigando um FAIL
isolado do Cenário 80). O ML-1E fechou os 3 (Cenários 186-188) e, por pedido do handoff, enumerou
o RESTO do arquivo — a suspeita era que a lacuna não fosse exclusiva do ML-1C.

## Achado

`check-validate-parity.sh` tem **9 blocos** de comparação cross-CLI (cada um termina num
`echo "Validate JSON parity checks passed (...)"` ou equivalente). Cobertura em
`check-gates-falsify.sh` (buscado por `check-validate-parity.sh"` e por nome literal de fixture):

| # | Bloco | Linhas | Tem cenário? |
|---|---|---|---|
| 1 | Contrato bare ADR/REQ (`adr_accepted_when_req_done`/`blocked_by_draft_adr` + "differs between runtimes") | 42-217 | Parcial (Cenário 4, via `wip_has_req`) |
| 2 | GVP — `git_branch_guard_script_integrity` GLOBAL, mensagem (ML-3A) | 219-303 | **NENHUM** |
| 3 | SIU project — script ilegível projeto (ML-1C) | 305-391 | Cenário 186 |
| 4 | SIU global — script ilegível global (ML-1C) | 393-463 | Cenário 187 |
| 5 | FIFO (ML-1C) | 465-546 | Cenário 188 |
| 6 | GVMT — `git_branch_guard_hook_resolvable` GLOBAL "missing type" (ML-4B) | 548-640 | **NENHUM** |
| 7 | `branch_has_wip_roadmap` aceita `done/` | 642-821 | Cenário 79 |
| 8 | `credential_guard_hook_resolvable` cross-CLI (22 fixtures) | 823-1381 | Bem coberto, exceto `claude-invalid-json`/`claude-unreadable`/`claude-utf16` (ML-1A/1B) |
| 9 | `git_branch_guard_hook_resolvable` cross-CLI (2 próprios + 3 compartilhados) | 1383-1514 | **NENHUM** para `gbg-claude-relativo`/`gbg-cursor-relativo-present`; os 3 compartilhados herdam a mesma ausência do bloco 8 |

## Como foi medido, não só grepado

Para cada bloco sem cenário aparente, busquei o nome literal do arquivo/regra/fixture em
`check-gates-falsify.sh` (`grep -n`) e, quando havia candidato próximo (ex.: Cenário 82 no contexto
de "missing-type"), li o corpo do cenário para confirmar QUAL fixture ele realmente sabota — Cenário
82 sabota `cg-claude-notype` (bloco 8, PROJETO), não o bloco 6 GVMT (GLOBAL), apesar do comentário do
Cenário 82 mencionar "missing-type" também. Confiar só na palavra-chave no comentário teria dado um
falso "coberto".

## Por que isso importa

Os blocos GVP (2) e GVMT (6) comparam **texto de mensagem byte-a-byte entre 3 CLIs escritas à mão**
— exatamente a classe de defeito mais fácil de divergir silenciosamente (um `—` vs `--`, como o
ML-1A já pegou uma vez nesta mesma REQ). Sem cenário de falsificação, uma divergência de texto
nesses 2 blocos passaria despercebida indefinidamente — o gate estaria verde por não ter sido
testado, não por a produção estar correta.

Os 2 fixtures próprios do bloco 9 (`gbg-claude-relativo`, `gbg-cursor-relativo-present`) e os 3
compartilhados entre os blocos 8/9 (JSON inválido, ilegível, UTF-16 — adicionados pelo ML-1A/1B
desta MESMA REQ) têm o mesmo problema: os blocos passam hoje, mas nenhum cenário prova que
reprovariam se a detecção regredisse especificamente para `git_branch_guard_hook_resolvable`
(função compartilhada com `credential_guard_hook_resolvable`, mas a MENSAGEM final e o `ruleName`
diferem por wrapper).

## Decisão

**Não corrigido nesta ML** — o handoff pediu enumeração, não correção ("Não corrija os que
faltarem: reporte a contagem"). Decisão de virar ML nova nesta frente (mesma REQ, mesma causa raiz
— regra "mesma causa, mesma REQ") é do `trackfw_architect`.

## Ver também

- [[credential-guard-marker-const-compartilhada-entre-3-consumidores-quebra-cenario-80-2026-09-06]]
  — achado irmão do ML-1D que motivou esta enumeração (uma const compartilhada por 3 call sites
  quebrou um cenário EXISTENTE; aqui o problema é a AUSÊNCIA total de cenário nos blocos afetados)
- `docs/roadmaps/wip/ROADMAP-2026-09-06-fecha-o-fail-open-do-guard-config-ilegivel-deixa-de-ser-silencio.md` — ML-1E
