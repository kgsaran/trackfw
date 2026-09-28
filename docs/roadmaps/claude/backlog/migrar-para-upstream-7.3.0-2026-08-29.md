---
name: migrar-para-upstream-7.3.0-2026-08-29
title: "Migrar para a base do upstream 7.3.0"
status: backlog
date: 2026-08-29
req: docs/requisições/claude/REQ-2026-08-29-migrar-para-upstream-7.3.0.md
branch: chore/migrar-upstream-7.3.0
---

# Roadmap: migrar para o upstream 7.3.0

> Created: 2026-08-29 | Status: done

REQ: `docs/requisições/claude/REQ-2026-08-29-migrar-para-upstream-7.3.0.md`
ADR: `docs/adr/ADR-2026-08-29-adotar-upstream-como-base.md`

## Diagnóstico / Contexto

Cópia por ZIP da v2.12.2 contra um upstream na v7.3.0, sem ancestral comum. Medição e decisão de
rota na REQ e na ADR.

Superfície medida antes de começar: **237 arquivos em conflito**, **985 novos** do upstream, **228**
só locais.

## Critérios de Aceite

- [x] Ancestralidade estabelecida; `merge upstream/<tag>` funciona sem flag
- [x] Produto na 7.3.0, governança local intacta, governança do upstream fora
- [x] Fix de UTF-8 reaplicado e verificado
- [x] Build verde e `validate` executando

---

## Wave 1 — O merge

### ML-1 — Merge de históricos com resolução por área
**Status:** ✅ Concluído
**Ações:**
1. `git merge v7.3.0 --allow-unrelated-histories --no-commit`.
2. Resolver em bloco: produto e doc de produto vêm do upstream; governança e configuração locais
   permanecem.
3. Remover do índice a governança do upstream que entrou como arquivo novo — `docs/adr/ADR-*` dele,
   `docs/req/`, `docs/roadmaps/` flat, `analises/`, `pesquisa/`, `qualidade/`, `seguranca/`,
   `analise-cmdb/`.
**Critérios de aceite:**
- [x] Nenhum marcador de conflito em arquivo versionado
- [x] `docs/adr/` só com as ADRs locais
- [x] `trackfw.yaml`, `.gitattributes` e `docs/roadmaps/.trackfw-log` preservados

### ML-2 — `CLAUDE.md` mesclado
**Status:** ✅ Concluído
**Ações:** base do upstream, mais a seção de dogfooding deste repositório — `req_dir` em português,
`roadmap_namespacing: by_agent` e os agentes.
**Critérios de aceite:**
- [x] Instruções de produto correspondem à 7.3.0
- [x] Seção de dogfooding presente

### ML-3 — Fix de UTF-8 reaplicado
**Status:** ✅ Concluído
**Ações:** reaplicar `_force_utf8_output` sobre o `cli.py` da 7.3.0, com o teste correspondente.
**Critérios de aceite:**
- [x] `--help`, `status` e `validate` com rc=0 em console cp1252
- [x] Saída sem CRLF

---

## Wave 2 — Verificação

### ML-4 — Estado pós-migração
**Status:** ✅ Concluído
**Ações:**
1. `trackfw version` nos três runtimes; `go build ./...`; `validate`.
2. Conferir a ancestralidade com `git merge-base`.
3. Regravar o baseline se o validator da 7.3.0 divergir do antigo.
4. Registrar o que a migração remove — `plugins` e os cinco aliases.
**Critérios de aceite:**
- [x] `merge-base` devolve commit
- [x] Versão 7.3.0 nos três; build verde
- [x] Remoções registradas

---

## Wave 3 — O que a rota C revelou (não estava no plano)

Merge de históricos não-relacionados **adiciona, mas nunca apaga**. Todo arquivo que o upstream
deletou entre a 2.12.2 e a 7.3.0 sobreviveu em silêncio. Quem denunciou foi o `go build`:
`internal/generators/codex.go:142: undefined: injectCodexHooks` — o `codex.go` é da 2.12.2 e o
`hooks.go` que definia a função veio da 7.3.0, que não tem codex.

### ML-5 — Poda do produto morto
**Status:** ✅ Concluído
**Ações:** remover 110 arquivos que o upstream apagou — os 6 geradores legados
(`amazonq/codex/copilot/cursor/gemini/windsurf`, REQ de remoção no `done/` do upstream), o
subsistema de plugins nos 3 runtimes (ADR-2026-08-15), os 80 templates de agente e o
`internal/server/` (virou `internal/serve/`).
**Critérios de aceite:**
- [x] `go build ./...` verde
- [x] Nenhum arquivo de produto só-local fora da lista de divergências deliberadas

### ML-6 — Isolamento de HOME no Windows
**Status:** ✅ Concluído
**Ações:** `internal/homedir` com `Dir()` preferindo `$HOME`, substituindo `os.UserHomeDir()` em 19
call sites de produção.
**Motivo:** os testes do upstream isolam a home com `t.Setenv("HOME", ...)` em 97 call sites, mas no
Windows `os.UserHomeDir()` lê `%USERPROFILE%` — não isola nada. Uma rodada de `go test ./...` nesta
máquina escreveu ADR, `integrations-manifest.json` e dois scripts de guard **dentro da home real**, e
tocou os seis arquivos de config global de agente. Corrigir os 97 call sites de teste geraria
conflito em todo merge futuro; 19 de produção é o custo menor.
**Critérios de aceite:**
- [x] `internal/commands` cai de 613s para 74s (parou de fazer I/O na home real)
- [x] Nenhum `os.UserHomeDir()` restante em produção

### ML-7 — Testes locais superados
**Status:** ✅ Concluído
**Ações:** remover os testes escritos contra as portas locais de `req move`, `req list`, `adr list`,
`roadmap new` — o upstream implementou as mesmas features com contratos diferentes e 18 de 33
asserções ficaram vermelhas. Mantidos: `pypi/tests/test_cli_encoding.py` (divergência de UTF-8) e
`internal/generators/roadmap_move_test.go` (passa inteiro).
**Nota:** o `adr list` da 7.3.0 lê só `> Date: ... | Status:` e ignora frontmatter YAML. As 7 ADRs
locais têm as duas formas, então os três runtimes concordam — verificado.
**Critérios de aceite:**
- [x] Superfície local fora de `docs/` reduzida a 5 arquivos
- [x] `internal/thirdparty` verde (dependia de `docs/seguranca/`, restaurado)

### ML-8 — Referências de REQ em caminho completo
**Status:** ✅ Concluído
**Ações:** o `referenceExists` da 7.3.0 faz `os.Stat(ref)` relativo ao cwd; as 6 REQs legadas
escreviam só o basename. Reescritas para caminho completo.
**Critérios de aceite:**
- [x] 11 violações de link → 0

---

## Passivo aberto — bloqueio estrutural no Windows

`trackfw validate` **sai 1 nesta máquina** e não há como evitar. A regra
`credential_guard_hook_resolvable` testa `info.Mode()&0111 == 0`, e o `os.Stat` do Go no Windows
devolve `-rw-rw-rw-` para todo arquivo — verificado empiricamente, inclusive depois de `chmod +x`.
O baseline não resolve: `filterBaselineTagged` isenta `credentialGuardAnchoredRules` da supressão
por decisão de segurança do upstream (`validator.go:577`). 9 das 15 violações de bit de execução
foram congeladas; as 6 de credential-guard nunca podem ser.

É a terceira divergência estrutural Windows da 7.3.0, junto com o UTF-8 e o `homedir`.

## Suítes de teste — a 7.3.0 é vermelha no Windows

Medido contra worktree pristina em `v7.3.0`, mesma máquina:

| runtime | 7.3.0 pristina | migrado |
|---|---|---|
| Go | 6 pacotes FAIL / 8 ok | 6 pacotes FAIL / 8 ok |
| npm | 329 falhas / 614 passes | 297 falhas / 631 passes |
| pypi | 223 falhas / 1264 passes | 213 falhas / 1307 passes |

A migração não introduziu regressão de teste — em npm e pypi ficou melhor que o upstream puro.
