---
status: wip
date: 2026-09-03
squad: apolo-tf
req: "docs/req/REQ-2026-09-03-as-217-falhas-reais-de-windows-colapsam-em-poucas-causas-e-tres-delas-exigem-decisao-antes-de-codigo.md"
---

# Roadmap: Fechar os grupos de falha de Windows por causa raiz

> Criado em: 2026-09-03 | Status: wip

## Context

REQ: docs/req/REQ-2026-09-03-as-217-falhas-reais-de-windows-colapsam-em-poucas-causas-e-tres-delas-exigem-decisao-antes-de-codigo.md

## Diagnóstico

Contagem medida no run `33810452454` da `main`, o primeiro com a Wave 0 e o `eol` dentro:

```
          ANTES  AGORA  delta
Go          86     64    -22
Node        56     52     -4
Python     104    101     -3
          ────   ────   ────
TOTAL      246    217    -29
```

**As 217 são defeito real.** Eu estimei 73 desmascaradas e foram 29 — errei por 2,5x, e o ML-1C
tinha avisado.

## Acceptance Criteria

- [ ] O mecanismo do grupo B identificado e escrito, ou virado REQ com o que foi eliminado
- [ ] As 3 ADRs `Accepted` antes do código do grupo que cada uma governa
- [ ] Falsificação nas duas direções e controle POSIX em cada grupo
- [ ] 🔴 Recontagem no CI **por wave**, com o delta atribuído ao grupo
- [ ] 🔴 Nenhuma correção reduz contagem **escondendo** defeito
- [ ] `make quality` verde e os 9 checks obrigatórios verdes ao fim de cada wave

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — O desconhecido, sozinho
> Dependências: nenhuma. **Bloqueia a estimativa**, não as outras waves.

### ML-0A — Grupo B: por que o `bash` do Python devolve exit 1 uniforme
**Status:** ⬜ Pendente · **Agente:** `artemis-tf`
**~56 testes, 26% do total, mecanismo DESCONHECIDO.** É o maior risco isolado do lote.

`pypi/tests/test_credential_guard.py`, `test_git_branch_guard.py`,
`test_credential_guard_sabotage.py`, `test_git_branch_guard_dedup.py` — todos os testes de
guard-script retornam 1, com stderr vazio, **inclusive o caso que deveria sair 0 na segunda linha**
(`[ -f trackfw.yaml ] || exit 0`).

🔴 **O discriminante já existe e mata a teoria ambiental:** o **Node roda o mesmo script pelo mesmo
`bash`**, com a mesma chamada `spawnSync('bash',[script])`, **e passa** —
`credential_guard.test.js` dá `22 passed, 2 failed` internamente, e os 2 são bit de execução. **O
defeito é do lado Python.**

Suspeitos **não verificados**: `HOME` de sessão herdado pelo filho; tradução de newline por
`text=True`.

**Critérios de aceite:**
- [ ] 🔴 O mecanismo está **escrito com a medição**, ou o relatório lista **o que foi eliminado e
      como** — "não sei ainda" é resultado válido; **hipótese como causa, não**
- [ ] O caso `exit 0` (segunda linha do script) é medido **isoladamente** — é o que discrimina
      "script errado" de "invocação errada"
- [ ] Comparação **lado a lado** com o braço Node, que passa: mesma chamada, mesmo script, resultado
      diferente. **A diferença é o achado**
- [ ] Nenhuma correção aplicada nesta wave — é **investigação**

## Wave 1 — As três decisões (arquiteto, sequenciais, NÃO paralelizam)
> Dependências: nenhuma. Não esperam a Wave 0.

### ML-1A — ADR: o trackfw escreve separador POSIX nos artefatos que autora?
**Status:** ⬜ Pendente · **Agente:** `trackfw_architect`
**Resolve TRÊS grupos de uma vez** (~45 testes): `tildeify` devolvendo `~\...`, `provenanceKey`
nativo no Node, e caminho em JSON lido por CLI de agente.
Evidência que tende a **sim**: a chave de proveniência **já é** `/` por decisão documentada; `~` é
POSIX-ismo que nenhum shell do Windows expande; um `command` bash com `\` é mastigado pelo shell.

### ML-1B — ADR: o parser de frontmatter deve tolerar CRLF?
**Status:** ⬜ Pendente · **Agente:** `trackfw_architect`
O parser é **cego a CRLF** e emitiu frontmatter **duplicado** em `TestRenderOpenCodeAgent`. ~14
testes. 🔴 A alternativa — declarar `eol` sobre os assets — **foi medida e recusada** no ML-1C:
esconde o defeito em vez de curá-lo.

### ML-1C — ADR: caminho POSIX ancorado num config lido por CLI de agente é "absoluto"?
**Status:** ⬜ Pendente · **Agente:** `trackfw_architect`
`filepath.IsAbs("/opt/…")` é **falso** no Windows → `classifyHookAnchorage` classifica ancorado como
relativo → **o validator deixa de emitir violation de guard ausente**. ~14 testes, **e é de
segurança**: a detecção de hook de guard **enfraquece no Windows**.

## Wave 2 — Separador, nos 3 CLIs
> Dependências: ML-1A `Accepted`.

### ML-2A — Separador POSIX em artefato autorado
**Status:** ⬜ Pendente · **Agente:** `apolo-tf`
**Files affected — os 3 stacks:** `npm/src/lib/update-engine.js:172-181`,
`pypi/trackfw/commands/update_harness.py::_tildeify`, `internal/integrations/manager.go`,
`npm/src/validator/index.js:3153` (`provenanceKey` sem normalização), `npm/src/serve/api_chain.js`
⚠️ `npm/src/validator/index.js` é classificado como **binário** pelo `file` — `grep` sem `-a` o pula
**em silêncio**; 2 REQs deste repo têm premissa falsa por isso.
**Critérios:** falsificação nas duas direções · controle POSIX com números · os 3 CLIs dão o **mesmo**
resultado · recontagem no CI com o delta atribuído a este grupo.

## Wave 3 — `IsAbs`, sozinho e sequencial
> Dependências: ML-1C `Accepted` **e** a branch `fix/validate-detecta-hook-de-guard-...` fechada.

### ML-3A — Caminho POSIX ancorado deixa de ser classificado como relativo
**Status:** ⬜ Pendente · **Agente:** `apolo-tf` + barreira `hades-tf`
🔴 **É segurança e COLIDE com outra branch.** Não paralelizar com nada.
`internal/validator/validator_credential_guard.go`, `validator_git_branch_guard.go`, e pares nos
outros 2 CLIs.

## Wave 4 — Resíduo (paralelo, arquivos disjuntos)
> Dependências: Wave 2.

### ML-4A — Bit de execução em NTFS
**Status:** ⬜ Pendente · **Agente:** `artemis-tf` · ~22 testes, **decisão já tomada** no vault:
`goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01`. **Não relitigar** — guard de
plataforma no assert.

### ML-4B — `WinError 32`, `.sh` sem `bash`, `stale_wip` off-by-one
**Status:** ⬜ Pendente · **Agente:** `artemis-tf` · ~15 testes, todos de teste, todos disjuntos.
O `stale_wip` é **truncamento**, não fuso horário — a hipótese de TZ foi **falsificada** na triagem.

## Verificação que só o CI fecha

A contagem por runtime, **medida após cada wave** e com o delta **atribuído** ao grupo. Sem
atribuição não se sabe qual correção funcionou — e nesta REQ eu já errei uma estimativa por 2,5x.

## Barreira final

`hefesto-tf` e `hades-tf`. O Hades é **obrigatório** na Wave 3 (segurança) e na Wave 2 (caminho em
config lido por CLI que executa bash).
