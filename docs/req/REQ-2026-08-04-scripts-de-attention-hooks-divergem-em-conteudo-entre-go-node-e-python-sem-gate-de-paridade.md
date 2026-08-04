---
status: Open
date: 2026-08-04
author: "kg.saran@gmail.com"
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-08-04-scripts-de-attention-hooks-divergem-em-conteudo-entre-go-node-e-python-sem-gate-de-paridade.md"
---

# REQ: scripts de attention hooks divergem em conteudo entre Go Node e Python (sem gate de paridade)

> Date: 2026-08-04 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Achado durante a auditoria da REQ-2026-08-04-discover-init-... (paridade de *geração* dos scripts):
os três runtimes geram `scripts/trackfw-attention-signal.sh` e `scripts/trackfw-attention-cleanup.sh`
com **conteúdo diferente entre si**. Confirmado empiricamente rodando os três binários reais
(`trackfw-go`, `node npm/bin/trackfw`, `python3 -m trackfw`) contra fixtures independentes e
comparando byte-a-byte:

| Divergência | Go | Node.js | Python |
|---|---|---|---|
| Comentário "no-op fora da raiz" | `# Script e intencionalmente no-op quando executado fora do diretorio raiz do projeto trackfw.` | `# Script is intentionally a no-op when executed outside the project root` | `# Script é intencionalmente no-op fora da raiz do projeto (onde trackfw.yaml reside)` |
| Linha em branco após `ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}` | Ausente | Presente | Presente |
| Estilo do `sed` em `TOOL_ESC`/`MSG_ESC` (só no signal script) | Um `sed` com `;` (`sed 's/\\/\\\\/g; s/"/\\"/g'`) | Dois `-e` (`sed -e 's/\\/\\\\/g' -e 's/"/\\"/g'`) | Igual ao Go |

Nenhuma das três diferenças muda o **comportamento** do script (comentário é só documentação; a
linha em branco é cosmética; as duas formas de `sed` produzem exatamente o mesmo resultado). Mas é
uma quebra de contrato: `docs/cli-parity.md` já pina literalmente vários outros artefatos gerados
(mensagens de erro do `barrier`, JSON do `update`, etc.) exatamente para que os três CLIs produzam a
mesma saída observável — e não existe hoje nenhum gate cobrindo o conteúdo desses dois scripts
especificamente (confirmado: nenhuma menção a "attention" em `docs/cli-parity.md`, e nenhum script
de paridade em `scripts/` os cobre — diferente do que `scripts/check-integration-assets.sh` faz para
`internal/integrations/assets`). Foi exatamente a ausência desse gate que permitiu a divergência
crescer sem ser detectada.

Localização exata dos três literais-fonte:
- Go: `internal/generators/scaffold.go` (`signalScript`/`cleanupScript`, dentro de
  `GenerateAttentionScripts`, ~linhas 682-740 — ver REQ/roadmap da correção de paridade de geração
  para a numeração pós-fix)
- Node.js: `npm/src/generators/hooks.js:60` (`SIGNAL_SCRIPT`) e `:97` (`CLEANUP_SCRIPT`)
- Python: `pypi/trackfw/generators/init_gen.py:779` e `:815` (dentro de `_generate_attention_scripts`,
  definida em `:852`)

## Acceptance Criteria
- [ ] Um texto canônico único escolhido para cada um dos dois scripts (signal e cleanup) — mesmo
      comportamento observável dos três hoje, só resolvendo comentário, espaçamento e estilo do `sed`
      para uma única forma
- [ ] Os três literais-fonte (Go/Node/Python, localizações acima) atualizados para ficarem
      **byte-idênticos** ao texto canônico
- [ ] Novo gate de paridade cobrindo o conteúdo desses dois scripts nos três runtimes — seguindo o
      padrão de `scripts/check-integration-assets.sh` (comparação byte-a-byte) ou estendendo o gate
      de `discover --init`/`init` já testado nesta sessão para comparar Go×Node×Python diretamente em
      vez de só confirmar existência+conteúdo-consigo-mesmo em cada runtime isoladamente
- [ ] Gate documentado em `docs/cli-parity.md` (nova entrada, seguindo o padrão de
      `check-integration-assets.sh`)
- [ ] Gate integrado a `make quality`
- [ ] `trackfw init`, `trackfw update` e `trackfw discover --init` continuam funcionando sem
      regressão em nenhum dos três runtimes (os scripts são só texto — nenhuma lógica de execução
      deve mudar)

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: N/A — correção de conteúdo de texto + gate de regressão, sem decisão arquitetural nova.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/backlog/ROADMAP-2026-08-04-scripts-de-attention-hooks-divergem-em-conteudo-entre-go-node-e-python-sem-gate-de-paridade.md`
