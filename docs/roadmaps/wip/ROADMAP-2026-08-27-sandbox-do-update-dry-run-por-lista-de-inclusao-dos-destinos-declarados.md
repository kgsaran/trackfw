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
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
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
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-0A

`internal/generators/update.go:2121` (`copyProjectTree`) e o equivalente Python
(`pypi/trackfw/commands/update.py:535`) — confirmar se o Node tem o mesmo padrão.

**Critérios de aceite:** repro do CMDB passa · symlink pendurado fora do conjunto é irrelevante ·
`make quality` exit 0 medido

---

### Auditoria do ML-0A e do ML-1A — aprovadas; a Wave 0 inverteu a ordem do trabalho

**A Wave 0 achou SEIS gaps, três HIGH — e dois deles são defeito HOJE, independente do sandbox:**

```
A (HIGH)  .windsurf/hooks.json                    escrito por InjectWindsurfHooks, NAO declarado
B (HIGH)  .amazonq/cli-agents/q_cli_default.json  escrito por InjectAmazonQHooks,  NAO declarado
C (HIGH)  .github/copilot-instructions.md         SINAL de deteccao; sem ele o dry-run diz missing
                                                  onde o run real diz updated
E (MED)   trackfw.yaml                            lido PARA CONTEUDO no sandbox (agent_conventions,
                                                  agent_models); sem ele o hash de CLAUDE.md diverge
D (MED)   codex-project-agents                    bypassa runFileTarget — fora do alcance do sandbox
F (LOW)   Python                                  faltava scripts/trackfw-git-branch-guard.sh
```

Confirmei A e B por leitura (`update.go:1881-1890` não os continha). **A linha `skipped (...)` que o
`trackfw update` imprimiu para o KG hoje já omitia dois caminhos que ele de fato escreve.**

**Isso inverteu a ordem do trabalho:** com sandbox por inclusão, declaração incompleta deixa de
**abortar** e passa a **mentir por omissão** — pior, porque abortar é visível. Mandei corrigir a
declaração **antes** de inverter o sandbox.

**Achado que evitou trabalho:** o **Node não tinha o defeito** — usa `fs.existsSync`, que segue o
symlink e devolve `false` no link quebrado. A classe de abort era só de Go e Python.

#### Medição minha, com a repro do KG

```
fixture com .venv/bin/python -> python3.99 (alvo inexistente)
  trackfw update --dry-run  ->  exit 0 · updated=1 missing=4     <- era o abort do CMDB

symlink quebrado DENTRO do conjunto (CLAUDE.md -> /nao-existe)
  trackfw update --dry-run  ->  exit 0 · missing=5               <- tratado como ausente, sem abort

declaracoes corrigidas nos 3 CLIs (update.go:1891-1892, update.py:94)
make quality (CI-exata, minha)  exit 0
```

**Correção minha antes do commit:** a seção nova do `cli-parity.md` que documenta o resíduo D veio
**sem anotação de contrato**, e o `check-parity-contract-coverage.sh` reprovou. Anotei como `gap` com
o motivo — *não há gate porque não há comportamento a fixar; a seção documenta a **ausência** de
garantia*. É a segunda vez em dois dias que esse checker pega documentação sem verificação: antes um
`gate=` apontando para stub vazio, agora uma seção sem anotação nenhuma.

---

## Wave 2 — Gate

### ML-2A — Paridade e falsificação nas duas direções
**Status:** 🔄 Em andamento · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

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
