---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-09-05-a-instalacao-em-windows-promete-o-que-nao-entrega-e-o-instalador-recusa-a-plataforma-para-a-qual-publicamos-binario.md"
squad: "hades_tf, apolo_tf, hefesto_tf"
---

# Roadmap: a instalacao em Windows promete o que nao entrega e o instalador recusa a plataforma para a qual publicamos binario

> Created: 2026-09-22 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-05-a-instalacao-em-windows-promete-o-que-nao-entrega-e-o-instalador-recusa-a-plataforma-para-a-qual-publicamos-binario.md -->
REQ: docs/req/REQ-2026-09-05-a-instalacao-em-windows-promete-o-que-nao-entrega-e-o-instalador-recusa-a-plataforma-para-a-qual-publicamos-binario.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] AC8: hooks de atenção nativos em Go, migrados sem perder hooks personalizados.
- [ ] AC5–AC7: jornadas Windows medidas, sem confundir execução direta com execução pelo runtime.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** 🔄 Em andamento
**Files affected:** docs/seguranca/2026-10-20-wave0-attention-go.md
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
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
test -s docs/seguranca/2026-10-20-wave0-attention-go.md
```

## Wave 1 — Critérios históricos e jornadas AC1–AC7
> Dependências: AC1–AC4 reconciliados na triagem de 17/10; AC5–AC7 somente após Wave 3. Não executar estes MLs antes do ML-1H.

### ML-1A — **AC1** — O README declara a dependência de **shell** para as funcionalidades que dependem
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — O README declara a dependência de **shell** para as funcionalidades que dependem
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — Existe seção de instalação para Windows, com o caminho real (qual artefato, onde
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — Existe seção de instalação para Windows, com o caminho real (qual artefato, onde
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — A contradição entre install.sh e o release é resolvida **numa direção declarada**:
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — A contradição entre install.sh e o release é resolvida **numa direção declarada**:
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — O limite de ARM64 fica **declarado** onde o usuário lê, não só no .goreleaser.yaml.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — O limite de ARM64 fica **declarado** onde o usuário lê, não só no .goreleaser.yaml.
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — 🔴 **Jornada verificada de ponta a ponta em Windows**, com o hook **disparando** —
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — 🔴 **Jornada verificada de ponta a ponta em Windows**, com o hook **disparando** —
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — 🔴 **Falsificação da AC5:** num ambiente **sem** Git Bash, a jornada tem de **falhar
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — 🔴 **Falsificação da AC5:** num ambiente **sem** Git Bash, a jornada tem de **falhar
- [ ] build passes
- [ ] tests green

### ML-1G — **AC7** — Caminhos com **espaço** e **acento** exercitados (C:\Program Files\...,
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC7** — Caminhos com **espaço** e **acento** exercitados (C:\Program Files\...,
- [ ] build passes
- [ ] tests green

---

## Wave 2 — Atenção nativa (retomada autorizada em 20/10)
> Dependências: Wave 0 auditada. ML-1H executado por apolo_tf, um único dono de comandos, geradores e testes. Sem paralelismo entre implementação e revisão, pois compartilham os arquivos.

### ML-1H — **AC8** — Hooks de atenção viram subcomando Go (`trackfw attention signal|cleanup`)
**Status:** ⬜ Pendente
**Por que entra aqui:** Regra Dura de Causa Raiz — mesma causa da REQ (dependência de `sh` na
jornada Windows). Não previsto no escopo original porque, em 2026-09-22, os guards também eram `.sh`;
a ADR-2026-10-04 levou só os **guards** para Go e deixou os hooks de atenção para trás (medido em
2026-10-18).
**Files affected:** `internal/commands/` (novo `attention.go` + teste), `internal/generators/agentfiles.go`
(linhas 297, 405, 634, 640, 805, 813, 984, 991 e a migração `migrateHookCommand`), `scripts/trackfw-attention-{signal,cleanup}.sh`
(vira invólucro fino `exec trackfw attention …`, como ADR-2026-10-04 D3 fez com os guards),
`docs/cli-parity.md` (contrato do subcomando), emenda na ADR-2026-10-04 estendendo D2 aos hooks de atenção.
**Actions:**
1. Wave 0 (hades-tf): o hook de atenção escreve arquivo dentro de `roadmap_dir` a partir de payload do
   agente — confinar o caminho (hoje o `.sh` rejeita `/`, `..`), escapar JSON pelo `encoding/json`,
   limitar `message` a 300 chars, no-op fora da raiz do projeto, **nunca** bloquear o agente (exit 0 sempre).
2. Portar `trackfw-attention-signal.sh` (40 linhas) e `-cleanup.sh` (18) para `trackfw attention signal|cleanup`
   com comportamento igual (mesmos campos no `.trackfw-attention.json`).
3. Geradores emitem exatamente `trackfw attention signal` e `trackfw attention cleanup`, sem polyglot, para Claude, Codex, Gemini, Kiro, Cursor e Copilot (campo command); `trackfw update` migra todas as formas legadas emitidas sem duplicar/remover hooks personalizados. Atenção é informativa e não usa a linha fail-closed dos guards, defeituosa sob zsh (nota de 20/10). Não alterar guards nem arquivos do PR #590.
4. Teste que reprova se a linha emitida contiver `.sh`, `$VAR` ou caminho; teste de paridade `.sh` × subcomando.
**Acceptance criteria:**
- [ ] `go build ./...` e `make quality` verdes
- [ ] nenhum gerador emite `trackfw-attention-*.sh` como comando de hook
- [ ] no `windows-probe.yml`, com `bash` fora do PATH, o banner aparece (AC5/AC6 desta REQ)

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

**Mudou de forma**: `install.sh` aceita Windows (#373) e o README tem a seção; segue aberto só o que não foi medido — hooks com Cursor/Windsurf/Gemini/Kiro/Copilot/Amazon Q, `pwsh`, caminhos com espaço/acento.

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.


## Plano executável da retomada — 2026-10-20

- Escopo desta rodada: ML-0A, ML-1H e auditoria. AC5–AC7 seguem pendentes até medição real Windows; não encerrar a REQ nem mover o roadmap para done sem essa evidência.
- Arquivos adicionais descobertos: internal/commands/root.go, internal/generators/scaffold.go, internal/generators/agentfiles_test.go, internal/generators/copilot_hooks_parity_test.go, internal/generators/scaffold_literal_copy_parity_test.go, testes de atenção em internal/generators, e scripts/ de gates que ainda afirmem a implementação antiga. Inventário fechado pela Wave 0 antes do código.
- Contrato: entrada JSON stdin; prioridade question > command > texto por tool_name; message no máximo 300 runas; mesmos campos tool/message/level/timestamp; projeto somente com trackfw.yaml no cwd; caminho roadmap_dir configurado contido fisicamente na raiz; rejeitar symlinks/junctions com utilitários existentes; cleanup apaga somente .trackfw-attention.json, nunca .trackfw-credential-guard.json. Erros de payload/config/I/O são no-op e exit 0; stdin limitado para não travar hook.
- Compatibilidade: wrappers .sh e literais scaffold delegam para Go, preservando encaminhamento stdin e argumentos, sem jq/python; paridade usa fixture congelada do legado e prova comportamento sem ferramentas auxiliares. Não regenerar configurações locais/globais do usuário nesta rodada.
- Validação ML-1H: go build ./...; go test ./internal/commands ./internal/generators ./internal/integrations; go vet ./...; make quality; trackfw validate --json. Qualquer falha nova bloqueia; falhas de ambiente devem ser provadas e registradas.

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands ./internal/generators ./internal/integrations
go vet ./...
```

## Wave 3 — Auditoria de qualidade e red-team
> Dependências: Wave 2 implementada e auditada. Revisões sequenciais para não compartilhar contexto em escrita.

### ML-3A — Qualidade (hefesto_tf)
**Status:** ⬜ Pendente
**Files affected:** docs/seguranca/2026-10-20-quality-attention-go.md
**Actions:** revisar escopo, migração idempotente, fonte única, cobertura real e controles negativos; não escrever código.
**Acceptance criteria:**
- [ ] Parecer escrito, achados bloqueantes resolvidos e gates conferidos.

### ML-3B — Red-team (hades_tf)
**Status:** ⬜ Pendente
**Files affected:** docs/seguranca/2026-10-20-red-team-attention-go.md
**Actions:** exercer payload inválido, contenção caminho/symlink, exclusão segura e execução das linhas emitidas em sh/bash/zsh; distinguir prova local de runtime Windows.
**Acceptance criteria:**
- [ ] Parecer escrito, nenhum achado bloqueante aberto.

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-20-quality-attention-go.md
test -s docs/seguranca/2026-10-20-red-team-attention-go.md
go test ./internal/commands ./internal/generators ./internal/integrations
```
