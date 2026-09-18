---
status: done
date: 2026-08-27
req: "docs/req/REQ-2026-08-17-update-dry-run-aborta-em-symlink-quebrado-ao-copiar-a-arvore-inteira-do-projeto.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: sandbox do update dry-run por lista de inclusao dos destinos declarados

> Created: 2026-08-27 | Status: done

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
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-1A

Direção A: destino declarado que **deixa** de ser copiado ⇒ detectado. Direção B: sandbox voltando a
copiar a árvore inteira ⇒ detectado.

---

### Auditoria do ML-2A — aprovada; a prova compara dry-run contra o run REAL

**O modo de falha mudou de natureza com esta REQ**, e o gate acompanhou: antes o dry-run **abortava**
(barulhento); com sandbox por inclusão, destino não copiado passa a **mentir por omissão**
(silencioso). Um gate que só verificasse exit code passaria com a mentira dentro.

```
FAIL [sandbox/gap-e/dry-vs-real/go]: dry=skipped real=updated
     — trackfw.yaml may be missing from sandbox        <- Direcao A
FAIL [sandbox/dangling-outside-set/exit-zero/go]: go exited 1   <- Direcao B

check-update-parity     5 casos novos, 17 guards de vacuidade, 3 runtimes
make quality (CI-exata, minha)  exit 0, 176 cenarios
validate                        16 warnings, 0 violations
```

Os casos do **Gap C** e do **Gap E** comparam `--dry-run` contra o **run real** num fixture
descartável — é o que prova ausência de mentira, e não apenas que o dry-run "parece certo".

---

## Wave 3 — Barreira

### ML-3A — Reverificação
**Status:** ✅ Concluído · **Agente:** `hades-tf` (`subagent_type: hades-tf`)

---

## Wave 4 — Correção dos resíduos R-novo-1 e R-novo-2 (pós-barreira)

> Dependências: ML-3A concluído e aprovado pela barreira.

### ML-1B — Recursão de diretório em Go e Python + documentação de R-novo-2
**Status:** ✅ Concluído · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`) · **Dep.:** ML-3A

**Arquivos afetados:**
- `internal/generators/update.go` — `copyPath`: recursa diretório (Go)
- `pypi/trackfw/commands/update.py` — `_copy_path`: recursa diretório (Python)
- `scripts/check-update-parity.sh` — Scenario 14: dry-run vs real para diretório já correto
- `docs/cli-parity.md` — R-novo-2 documentado como residual com anotação `trackfw-contract: gap`

**Critérios de aceite:**
- [x] Go e Python: `copyPath`/`_copy_path` recursam conteúdo de diretório declarado
- [x] Scenario 14 no gate: `dry=skipped real=skipped` com guard de vacuidade — OK nos 3 runtimes
- [x] Node.js sem regressão
- [x] R-novo-2 documentado em `cli-parity.md` com anotação de contrato
- [x] `go build ./...` — EXIT:0
- [x] `go test ./internal/generators/...` — OK
- [x] `bash scripts/check-update-parity.sh` — EXIT:0 (176 + 1 novo = 177 cenários passando incluindo Scenario 14)
- [x] `bash scripts/check-parity-contract-coverage.sh` — EXIT:0

---

### Auditoria do ML-3A e do ML-1B — **APROVADO**; entrega completa

**A barreira aprovou e respondeu a pergunta do `trackfw.yaml` hostil de um jeito que vale registrar:**

> `agent_conventions` hostil flui verbatim para o `CLAUDE.md` gerado — **e isso é correto**. O dry-run
> agora **prevê** a injeção (`updated`), enquanto antes (Gap E) dizia `skipped` em silêncio
> **enquanto o run real injetava**. O fix tornou o dry-run **honesto sobre a injeção**; não abriu
> superfície nova.

A superfície já existia; o defeito era o dry-run **esconder** que ela seria usada.

**Dois resíduos novos, e eu tratei os dois de forma diferente:**

**R-novo-1 — corrigido.** Confirmei por leitura que `copyPath` fazia
`if info.IsDir() { return os.MkdirAll(dst, 0755) }` — criava o diretório **vazio**, sem recursar. Para
`.claude/commands/trackfw`, o dry-run dizia `updated` onde o real diria `skipped`. **É divergência
dry-run × run real**, a classe que esta REQ existe para eliminar — menos grave que a omissão porque
exagera em vez de esconder, mas ainda é o dry-run mentindo. O Node já fazia certo e serviu de
referência.

```
copyPath agora: MkdirAll + os.ReadDir + copyPath recursivo, reusando o mesmo
tratamento de symlink do caminho de arquivo (nao um WalkDir paralelo com regras proprias)

cenario 14: dry=skipped real=skipped, com DOIS guards de vacuidade
  guard 1: o fixture ficou nao-vazio
  guard 2: real=skipped
make quality (CI-exata, minha)  exit 0
validate                        16 warnings, 0 violations
```

**R-novo-2 — declarado, não corrigido.** Caminho declarado ilegível (chmod 000, socket, fifo) aborta
o dry-run em vez de reportar `failed` por target. Semanticamente defensável — o arquivo é
**necessário** para o target. Documentado no `cli-parity.md` com anotação de contrato.

**Consequência que ele declarou sem eu pedir:** a superfície de abort **ampliou** com a recursão —
agora inclui arquivos **dentro** de diretório declarado. É paridade com o Node, que já recursava e já
tinha esse comportamento. Prefiro a consequência declarada à descoberta depois.

**Entrega completa.**

---

## Notas
- **Fora de escopo:** tudo listado no *Escopo negativo* da REQ.
- Commits, branch e PR são exclusivos do `trackfw_architect`.
