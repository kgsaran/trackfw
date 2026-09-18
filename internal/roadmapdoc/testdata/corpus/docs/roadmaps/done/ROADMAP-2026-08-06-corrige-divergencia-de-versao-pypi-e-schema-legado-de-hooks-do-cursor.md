---
status: done
date: 2026-08-06
req: "docs/req/REQ-2026-08-06-corrige-divergencia-de-versao-pypi-e-schema-legado-de-hooks-do-cursor.md"
squad: ""
---

# Roadmap: corrige divergência de versão pypi e schema legado de hooks do Cursor

> Created: 2026-08-06 | Status: done

## Context
REQ: `docs/req/REQ-2026-08-06-corrige-divergencia-de-versao-pypi-e-schema-legado-de-hooks-do-cursor.md`

Dois achados fora de escopo, documentados durante a auditoria do
`ROADMAP-2026-08-05-hooks-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`
(PR #141, mergeado):

1. `pypi/trackfw/__init__.py` tem fallback hardcoded `"6.3.1"` (usado quando
   `importlib.metadata.version("trackfw")` falha — caso real de `check-cli-parity.sh`, que roda via
   `PYTHONPATH=pypi python3 -m trackfw`, pacote não instalado via pip) enquanto `pypi/pyproject.toml`
   já declara `6.4.1` (mesma versão de Go `internal/version/version.go` e `npm/package.json`) — só o
   fallback do `__init__.py` ficou desatualizado. Isso bloqueia `check-cli-parity.sh`, que por sua vez
   bloqueia toda a cadeia `parity`/`quality` antes de alcançar os demais gates.
2. `InjectCursorHooks` (Go/Node/Python) escreve o mecanismo legado de attention-signal/cleanup em
   `preToolUse`/`postToolUse` no nível raiz de `.cursor/hooks.json` — schema que não corresponde a
   nenhum evento real do Cursor (confirmado via `cursor.com/docs/agent/hooks`: o schema real é
   `{"version":1,"hooks":{"<eventName>":[...]}}`, com eventos documentados `sessionStart`/
   `sessionEnd`/`beforeShellExecution`/`beforeMCPExecution`/`afterShellExecution`/`afterMCPExecution`/
   `beforeReadFile`/`afterFileEdit`/`beforeSubmitPrompt`/`preCompact`/`stop`/`beforeTabFileRead`/
   `afterTabFileEdit` — nenhum `preToolUse`/`postToolUse` genérico). O credential-guard já usa os
   eventos reais (`beforeShellExecution`/`afterShellExecution`, PR #141) — só o wiring legado ficou
   para trás.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] `pypi/trackfw/__init__.py` alinhado a `6.4.1` (mesma versão de `pyproject.toml`/Go/Node)
- [ ] `make quality`/`make parity` verdes de ponta a ponta
- [ ] Evento real do Cursor confirmado para attention-signal/cleanup (ou decisão documentada de que
      não há analog real e a funcionalidade é descontinuada para esse CLI especificamente)
- [ ] `.cursor/hooks.json` migrado, paridade Go/Node/Python mantida, sem regressão no wiring do
      credential-guard já existente (`beforeShellExecution`/`afterShellExecution`)
- [ ] `trackfw validate` sem violações novas

## Wave 1 — Dois fixes independentes (2 MLs em paralelo)
> Dependências: Independente (arquivos completamente distintos)

### ML-1A — Alinhar versão fallback do pacote Python
**Status:** ✅ Concluído
**Arquivos afetados:**
- `pypi/trackfw/__init__.py` (linhas 3 e 5, ambos os fallbacks `"6.3.1"`)
**Ações:**
- Trocar os dois literais `"6.3.1"` para `"6.4.1"` em `pypi/trackfw/__init__.py`.
- Confirmar que `pypi/pyproject.toml` (`version = "6.4.1"`) é de fato a fonte de verdade que deveria
  ter sido espelhada aqui — não hardcodar um valor arbitrário, replicar o que `pyproject.toml` já diz
  hoje.
- **Não** transformar isso numa leitura dinâmica de `pyproject.toml` neste ML — é fora de escopo
  (mudança estrutural maior); só alinhar o valor.
- Avaliar (registrar no relatório, não necessariamente corrigir se for mudança maior) se falta algum
  gate/lembrete de processo para essa string não desalinhar de novo a cada bump de versão (ex.: o
  protocolo de release em `CLAUDE.md` já lista os arquivos a atualizar? Conferir e, se
  `pypi/trackfw/__init__.py` não estiver na lista, propor adição — mas só documentar/propor, não é
  obrigatório mudar o protocolo neste ML).
**Critérios de aceite:**
- [x] `PYTHONPATH=pypi python3 -m trackfw version` imprime `trackfw 6.4.1`
- [x] `scripts/check-cli-parity.sh` passa
- [x] `python3 -m pytest pypi/` verde
**Comandos de validação:** `GO_BIN=bin/trackfw scripts/check-cli-parity.sh && python3 -m pytest pypi/`

### ML-1B — Migrar wiring legado de attention-signal/cleanup do Cursor para evento real
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/generators/agentfiles.go` (`InjectCursorHooks`)
- `internal/generators/agentfiles_test.go`
- `npm/src/generators/hooks.js` (`injectCursorHooks`)
- `npm/tests/generators.test.js`
- `pypi/trackfw/generators/hooks.py` (`inject_cursor_hooks`)
- `pypi/tests/test_generators_init.py`
- `docs/cli-parity.md` (atualizar a seção "Cursor wiring (ML-2E)" — não deixar desatualizada como
  aconteceu com a seção do Gemini no ciclo anterior; ver nota de auditoria do ML-5A daquele roadmap)
**Ações:**
- Investigar (WebSearch/WebFetch em `cursor.com/docs/agent/hooks`, confirmar versão mais atual) qual
  dos eventos documentados é o analog correto para o propósito original do attention-signal/cleanup:
  sinalizar quando o agente está prestes a fazer algo que precisa de atenção do usuário (hoje,
  noutros CLIs, isso é atrelado a pedir permissão/pergunta ao usuário) e limpar o sinal depois.
  Candidatos a avaliar com evidência, não assumir:
  - `beforeSubmitPrompt` (início de turno) + `stop` (fim de turno/resposta) — mais próximo
    semanticamente de "início de uma ação que pode precisar de atenção" → "encerramento", mas não é
    exatamente "pedir permissão" como nos outros CLIs.
  - Confirmar se existe algum evento de permissão/aprovação explícito no Cursor além de
    `beforeShellExecution`/`beforeMCPExecution` (que já são usados pelo credential-guard) — se não
    houver evento de "pergunta ao usuário" genérico, documentar essa limitação.
- Se um evento real for confirmado como analog aceitável: migrar o wiring legado (`preToolUse`/
  `postToolUse` → evento real), preservando o wiring do credential-guard já existente em
  `beforeShellExecution`/`afterShellExecution` (não regredir isso).
- Se NÃO houver analog real aceitável: **não forçar um wiring especulativo**. Documentar a limitação
  no roadmap e em `docs/cli-parity.md`, e decidir (registrar a decisão, não implementar
  silenciosamente) entre (a) remover o wiring legado inerte do Cursor por completo (attention-signal
  não funciona nesse CLI, é melhor não gerar um arquivo que promete funcionalidade que não existe) ou
  (b) manter documentado como limitação conhecida sem remover (se a remoção tiver custo/risco maior
  que o benefício) — decisão a ser tomada pelo orquestrador com base no que a investigação encontrar,
  não prescrita aqui.
- Atualizar a seção "Cursor wiring (ML-2E)" de `docs/cli-parity.md` para refletir o estado real
  pós-fix (não deixar duas seções conflitantes, como aconteceu com o Gemini no ciclo anterior).
**Critérios de aceite:**
- [ ] Decisão tomada e documentada com evidência (migrado para evento real, OU removido com
      justificativa, OU mantido como limitação conhecida com justificativa — qualquer uma das três é
      aceitável, desde que fundamentada)
- [ ] `.cursor/hooks.json` gerado em fixture não regride o wiring do credential-guard
      (`beforeShellExecution`/`afterShellExecution`) já existente
- [ ] Testes Go/Node/Python verdes, incluindo `scripts/check-agent-hooks-parity.sh` (gate criado no
      ciclo anterior, cobre Cursor)
- [ ] `docs/cli-parity.md` consistente (sem seções conflitantes sobre o mesmo assunto)
**Comandos de validação:** `go test ./internal/generators/... && npm run test --workspace=npm -- hooks && python3 -m pytest pypi/tests/ -k hooks && GO_BIN=bin/trackfw scripts/check-agent-hooks-parity.sh`

## Wave 2 — Confirmação final (1 ML)
> Dependências: Wave 1 completa

### ML-2A — `make quality` verde de ponta a ponta + encerramento
**Status:** ✅ Concluído
**Nota:** `make quality` passou de ponta a ponta na primeira execução pós-Wave 1 (exit 0, 102/102
cenários de `check-gates-falsify.sh`, incluindo o Cenário 44 do ciclo anterior que cobre
`check-agent-hooks-parity.sh`). `trackfw validate` limpo. Nenhum achado novo — não precisou expandir
para ML própria.
**Arquivos afetados:**
- Nenhum (só validação)
**Ações:**
- Rodar `make quality` completo e confirmar que passa de ponta a ponta agora que o bloqueio de versão
  foi removido.
- Atualizar `docs/agents-working-context.md` e a REQ (Acceptance Criteria + Linked Roadmap).
**Critérios de aceite:**
- [x] `make quality` verde
- [x] `trackfw validate` sem violações
**Comandos de validação:** `make quality && trackfw validate`
