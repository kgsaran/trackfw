---
status: wip
date: 2026-10-06
req: "docs/req/REQ-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md"
squad: ""
---

# Roadmap: credential guard nao cobre Windsurf e Amazon Q, e a premissa da ADR-2026-08-05 que excluiu o Windsurf caducou

> Created: 2026-10-06 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md -->
REQ: docs/req/REQ-2026-10-06-credential-guard-nao-cobre-windsurf-e-amazon-q-e-a-premissa-da-adr-2026-08-05-que-excluiu-o-windsurf-caducou.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] AC1 da REQ — eventos de hook do Windsurf e do Amazon Q remedidos com fonte oficial e data (ML-0B)
- [x] AC2 da REQ — adendo à ADR-2026-08-05 com a decisão por CLI (ML-1A)
- [x] AC3 da REQ — `init`/`update`/`update harness` emitem o credential guard para cada CLI decidido "instalar"; `validate` deixa de silenciar; teste nas duas direções (ML-1B)
- [x] AC5 da REQ — o guard lê o payload de cada CLI (ML-1C)
- [ ] AC4 da REQ — prova de disparo real ou impossibilidade declarada com motivo (ML-2A)
- [x] `make quality` verde (arquiteto, sem `~/.local/bin` no PATH)

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model e remedição (2 MLs, mesmo despacho do hades-tf)
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-08-wave0-credential-guard-windsurf-amazonq.md` (novo)
**Actions:**
1. Enumeration completeness — todos os sítios que emitem, migram ou auditam o credential guard por CLI (gerador de projeto, `update`, `update harness`/`buildHarnessTargetIDs`, `harnessCatalogTargetOrder`, `validate`, detectores globais, `check-*` scripts, `docs/cli-parity.md`). Fechar a lista por grep do literal `trackfw guard credential` e de `credential-guard`, não só pelos arquivos da REQ.
2. Threat model — quem esvazia esta Wave 0 sem quebrar regra escrita, e como.
3. Falsification targets in both directions — por superfície: o que quebra se o guard deixar de ser emitido, e o que quebra se for emitido num evento que não bloqueia (falsa sensação de proteção) ou que bloqueia tudo (deny-all).
4. Declared residual.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

### ML-0B — Remedição dos eventos de hook (AC1 da REQ)
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** mesmo documento do ML-0A, seção "Remedição"
**Actions:** por CLI (Windsurf, Amazon Q CLI), com URL da documentação oficial e data de acesso: quais eventos existem antes de ler arquivo, escrever arquivo e executar comando; que payload chega no stdin (comando? caminho? conteúdo escrito?); exit code que bloqueia vs só avisa; se há escopo global (arquivo de usuário) além do de projeto. Confrontar com o que o gerador emite hoje.
**Acceptance criteria:**
- [x] Tabela por CLI × evento com fonte e data; cada célula "bloqueia/avisa/não existe" tem citação
- [x] Recomendação por CLI: instalar (em qual evento e com qual nome de guard) ou manter fora (motivo medido)

      Auditoria (2026-10-08): 1º parecer reprovado — payload de escrita inventado (`content`) nos dois CLIs; ML corretivo
      remediu com payload real (Windsurf `edits[].new_string`, Amazon Q `fs_write` tag `command`). Veredito mantido.
      Achados novos, conferidos pelo arquiteto com binário desta árvore em projeto `mode: block`: JWT + `> /dev/null` no
      conteúdo de `Write`/`Edit` do Claude Code e `fs_write` do Amazon Q → RC 0 (isenção efêmera aplicada ao payload
      inteiro); `cat "arquivo"` (aspas) passa na 2ª camada; Windsurf `command_line` não é lido. Mesma causa → ML-1C.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-08-wave0-credential-guard-windsurf-amazonq.md
grep -q "Veredito" docs/seguranca/2026-10-08-wave0-credential-guard-windsurf-amazonq.md
```

## Wave 1 — Decisão e implementação
> Dependencies: Wave 0 auditada. O detalhe dos MLs é fechado a partir do parecer da Wave 0.

### ML-1A — Adendo à ADR-2026-08-05 (AC2)
**Status:** ✅ Concluído
**Squad:** zeus-tf
**Files affected:** `docs/adr/ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-credenciais-reais-por-subagentes.md`
**Acceptance criteria:**
- [x] Adendo datado revendo a premissa do Windsurf e avaliando o Amazon Q, decisão por CLI citando o ML-0B

### ML-1B — Gerador, update, harness e validate (AC3)
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/generators/agentfiles.go`, `internal/generators/update.go`, `internal/validator/validator_credential_guard.go`, `internal/validator/validator_guard_binary_probe.go` (se preciso), testes desses pacotes, `.windsurf/hooks.json`/`.amazonq/...` deste repo se existirem, `docs/cli-parity.md`. **Não toca `internal/guard/`** (ML-1C, paralelo).
**Eventos:** Windsurf `pre_run_command` + `pre_write_code`; Amazon Q `preToolUse` matchers `execute_bash` + `fs_write`. Nunca `pre_read_code`/`fs_read`. Harness `windsurf-credential-guard` (`~/.codeium/windsurf/hooks.json`); Amazon Q só projeto.
**Acceptance criteria:**
- [x] Teste: `init`/`update` emitem a linha D11 revista do credential guard no evento decidido, por CLI decidido
- [x] Teste: `validate` acusa o arquivo sem o credential guard (antes silenciado) e não acusa o arquivo correto
- [x] Falsificação nas duas direções, com a frase de reconciliação por teste novo
- [x] Configs deste repo e `docs/cli-parity.md` atualizados
      Auditoria (2026-10-08): 18 testes novos conferidos por nome; artefato real medido com o binário (init
      `--ai-tools windsurf,amazonq`, HOME isolado): guard em pre_run_command/pre_write_code e execute_bash/fs_write, ausente
      em pre_read_code; tirar pre_write_code → validate acusa; `update` repõe → 0 achados. Reprovado 1x: o validate não
      olhava o global do Windsurf (violação que o `update` nunca corrige) → ML-1D.

### ML-1C — O guard lê o payload de cada CLI (AC5)
**Status:** ✅ Concluído
**Squad:** apolo-tf (paralelo ao ML-1B: arquivos disjuntos)
**Por que o escopo original não previa:** achados da Wave 0 (ver ML-0B). A REQ dizia "não muda o guard"; a medição
mostrou que o guard não entende o payload dos CLIs que esta REQ cobre, e que a mesma causa já abria o Claude Code.
**Files affected:** `internal/guard/credential.go` e testes em `internal/guard/`. Nada fora de `internal/guard/`.
**Acceptance criteria:**
- [x] 2ª camada extrai o comando por JSON parse de `tool_input.command` e `tool_info.command_line` (e `command` no topo, se algum CLI usar); `cat "arquivo"` e `cat 'arquivo'` detectados
- [x] Isenção efêmera (`> /dev/null` etc.) avaliada só sobre o comando de shell extraído; payload de escrita (Write/Edit do Claude, `pre_write_code`, `fs_write`) com JWT + `> /dev/null` → 2 em `block`, nos escopos projeto e global
- [x] `echo <JWT> > /dev/null` num comando de shell continua isento (direção oposta)
- [x] Testes com payload real de Claude Code, Codex, Windsurf e Amazon Q; falsificação nas duas direções; frase de reconciliação por teste novo

### ML-1D — Corretivo da auditoria da Wave 1
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Por que:** (1) o gerador pulava o guard de projeto do Windsurf com o global instalado, mas o validate exigia no
projeto; e "global instalado" contava só pre_run_command, deixando pre_write_code descoberto num global parcial;
(2) faltou o teste com payload do Codex pedido no AC5.
- [x] Validate respeita o global completo (pre_run_command E pre_write_code), mesma definição do gerador; 3 testes + falsificação
- [x] 3 testes de payload do Codex (cat, cat com aspas, echo > /dev/null isento)
- [x] `make quality` (arquiteto, máquina ociosa, sem `~/.local/bin`): EXIT=0, falsify 347 OK / 0 FAIL

## Wave 2 — Prova e red-team
> Dependencies: Wave 1 auditada.

### ML-2A — Prova de disparo real (AC4)
**Status:** ⬜ Pendente
**Squad:** zeus-tf
**Acceptance criteria:**
- [ ] Disparo real medido em ao menos um CLI, ou impossibilidade declarada (sem conta de Windsurf/Amazon Q, decisão do usuário)

### ML-2B — Red-team do diff
**Status:** ⬜ Pendente
**Squad:** hades-tf
**Acceptance criteria:**
- [ ] Parecer sobre o diff da Wave 1 contra o threat model do ML-0A; achados corrigidos nesta REQ
