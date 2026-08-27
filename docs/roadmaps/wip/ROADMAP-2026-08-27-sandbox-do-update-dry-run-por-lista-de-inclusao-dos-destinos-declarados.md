---
status: wip
date: 2026-08-27
req: "docs/req/REQ-2026-08-17-update-dry-run-aborta-em-symlink-quebrado-ao-copiar-a-arvore-inteira-do-projeto.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: sandbox do update dry-run por lista de inclusao dos destinos declarados

> Created: 2026-08-27 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-17-update-dry-run-aborta-em-symlink-quebrado-ao-copiar-a-arvore-inteira-do-projeto.md`
ADR: `docs/adr/ADR-2026-08-27-sandbox-do-update-dry-run-copia-apenas-os-destinos-declarados-nao-a-arvore-do-projeto.md`

`update --dry-run` copia a árvore inteira do projeto para um sandbox e **aborta em symlink
pendurado**. KG sentiu em produção no CMDB: `.venv/bin/python -> python3.13` com o alvo removido pelo
Homebrew. **Um link que o trackfw nunca vai tocar derruba a operação.**

**Decisão:** inverter para **lista de inclusão** derivada dos destinos declarados. Fecha a classe —
qualquer coisa fora do conjunto declarado deixa de ter efeito.


## Acceptance Criteria

- [ ] AC1–ACn da REQ, integralmente
- [ ] 🔴 **O risco dominante é omissão:** se um target escrever caminho que o sandbox não copiou, o
      dry-run **mente por omissão**. A Wave 0 ataca isso.
- [ ] `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 make quality` exit 0 (exit code medido)

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Completude da lista de destinos declarados
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-27-modelo-de-ameaca-do-sandbox-por-inclusao.md`

**A pergunta que decide a entrega:** a lista de destinos declarados está **completa**? Se faltar um
caminho, o `--dry-run` passa a **mentir por omissão** — pior que abortar, porque abortar é visível.

**Enumere por busca**, não pela lista que o código expõe: quais caminhos cada target escreve, nos 3
CLIs, incluindo os que ele escreve **condicionalmente** (identidade, escopo, presença de arquivo) e os
que **lê** para decidir. Um caminho lido-para-decidir e não copiado muda o resultado do dry-run
silenciosamente.

**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
test -f docs/seguranca/2026-08-27-modelo-de-ameaca-do-sandbox-por-inclusao.md
grep -q "Completude de enumera" docs/seguranca/2026-08-27-modelo-de-ameaca-do-sandbox-por-inclusao.md
grep -q "Residual declarado" docs/seguranca/2026-08-27-modelo-de-ameaca-do-sandbox-por-inclusao.md
```

## Wave 1 — Sandbox por inclusão

> Dependências: ML-0A auditado. **ML único:** os 3 stacks saem byte-idênticos.

### ML-1A — Lista de inclusão nos 3 CLIs
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

`internal/generators/update.go:2121` (`copyProjectTree`) e o equivalente Python
(`pypi/trackfw/commands/update.py:535`) — confirmar se o Node tem o mesmo padrão.

**Critérios de aceite:** repro do CMDB passa · symlink pendurado fora do conjunto é irrelevante ·
`make quality` exit 0 medido

---

## Wave 2 — Gate

### ML-2A — Paridade e falsificação nas duas direções
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

Direção A: destino declarado que **deixa** de ser copiado ⇒ detectado. Direção B: sandbox voltando a
copiar a árvore inteira ⇒ detectado.

---

## Wave 3 — Barreira

### ML-3A — Reverificação
**Status:** ⬜ Pendente · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

---

## Notas
- **Fora de escopo:** tudo listado no *Escopo negativo* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
