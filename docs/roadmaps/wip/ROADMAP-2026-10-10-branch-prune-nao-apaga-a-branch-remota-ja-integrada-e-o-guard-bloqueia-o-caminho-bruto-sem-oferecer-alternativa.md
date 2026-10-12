---
status: wip
date: 2026-10-10
req: "docs/req/REQ-2026-10-10-branch-prune-nao-apaga-a-branch-remota-ja-integrada-e-o-guard-bloqueia-o-caminho-bruto-sem-oferecer-alternativa.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: branch prune não apaga a branch remota já integrada e o guard bloqueia o caminho bruto sem oferecer alternativa

> Created: 2026-10-10 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-10-branch-prune-nao-apaga-a-branch-remota-ja-integrada-e-o-guard-bloqueia-o-caminho-bruto-sem-oferecer-alternativa.md
ADR: docs/adr/ADR-2026-10-12-branch-prune-apaga-a-branch-remota-so-pelo-sinal-do-forge-e-condicionada-ao-tip-avaliado.md

Branch compartilhada com a REQ-2026-10-12 (#556, prazo das operações de rede). **Esta REQ implementa
depois**: a remoção remota exige fetch concluído dentro do prazo (ADR D4), e as duas tocam
`internal/commands/branch_prune*.go`.

## Acceptance Criteria
- [ ] AC1 — Wave 0 auditado (parecer do `hades-tf`)
- [ ] AC3/AC4/AC5 — `--remote` opt-in; só `merged_pr` com tip remoto contido; lease no SHA avaliado; `keep`/`review`/padrão/protegida nunca apagadas
- [ ] AC6 — mensagem do guard aponta para `trackfw branch prune --remote --apply`
- [ ] AC7 — dry-run com `--remote` neste repositório
- [ ] AC8/AC9/AC10 — contrato, reconciliação dos testes, `make quality` + CI

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation. Roda em paralelo com o Wave 0 do ROADMAP-2026-10-12.

### ML-0A — Threat model da remoção remota
**Owner:** `hades-tf`
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-12-wave0-prune-remoto.md` (novo; único arquivo que pode ser escrito)
**Actions:**
1. **Completude da população.**
   - Enumerar onde vive a mensagem do guard para `git push` (`internal/guard/gitbranch.go`,
     `internal/generators/testdata/guard-sh-reference/git-branch-guard.sh`, gerador do script,
     `scripts/`), por grep do literal `git push bruto bloqueado`.
   - Enumerar as formas de remoção remota que o guard precisa reconhecer: `--delete`, `-d`, `:<ref>`,
     `+:<ref>`, `refs/heads/` explícito e múltiplos refspecs.
2. **Threat model (AC1 da REQ):** o que faz a remoção remota apagar trabalho não integrado ou de
   terceiro.
   - Obrigatório cobrir: branch remota de outro autor; ref que avançou depois do fetch (TOCTOU); PR
     aberto de mesmo nome; branch protegida; remoto que não é `origin`; resposta do forge truncada
     (A7); ausência de `gh`; fork com o mesmo nome de branch; `refs/remotes/origin/*` forjado
     localmente com refspec estreitado (nota do vault
     `git-fetch-self-heals-forged-origin-head-and-tracking-refs-2026-08-19`).
3. **Lease na remoção (ADR D5).**
   - Na documentação oficial do git, confirmar que `--force-with-lease=<ref>:<sha>` vale para a forma
     de remoção (`:<ref>` / `--delete`).
   - Propor o teste Go (repositório bare em `t.TempDir()`) que o ML-1A deve conter, com os dois braços:
     tip igual apaga; tip avançado recusa e mantém.
   - 🔴 Não tente medir com `git commit`/`git push` brutos no shell: o guard bloqueia, e não se contorna.
4. **Branch protegida (ADR D6).** Qual consulta ao forge informa a proteção, se ela exige permissão
   de administrador (a nota de `check-required-full` mediu 404 sem token de mantenedor), e o que
   fazer quando ela falha.
5. **Alvos de falsificação nas duas direções**, por caso: o que reprova quando a regra regride
   (apaga o que não devia) e quando regride ao contrário (não apaga a integrada; `--remote` sem
   `--apply` escreve no remoto).
6. **Resíduo declarado** e **veredito sobre o ADR** com ajustes numerados (A1, A2, …).
**Acceptance criteria:**
- [ ] As seis seções respondidas com evidência (comando + resultado, ou arquivo:linha, ou documentação citada), não asserção
- [ ] Nenhuma linha de implementação escrita; nenhum arquivo além do parecer alterado
- [ ] Nenhuma escrita em remoto real; nenhum `git commit`/`push` bruto

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-12-wave0-prune-remoto.md
grep -q "Veredito" docs/seguranca/2026-10-12-wave0-prune-remoto.md
git diff --quiet HEAD -- internal/
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditado e ajustes no ADR; **Wave 1 do ROADMAP-2026-10-12 concluído** (prazo
> no fetch e no `gh`). ML-1A e ML-1B tocam arquivos disjuntos e podem rodar em paralelo.

### ML-1A — `branch prune --remote`
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Files affected (provisório):** `internal/commands/branch_prune.go`, `internal/commands/branch_prune_forge.go`, novo `internal/commands/branch_prune_remote.go`, testes.
**Acceptance criteria:**
- [ ] Sem `--remote`: nenhuma chamada que escreva no remoto (teste com executor que registra os argumentos)
- [ ] `--remote` sem `--apply`: só relata
- [ ] Remoção só com `merged_pr` sobre o tip remoto e com lease no SHA avaliado; teste em bare repo com os dois braços
- [ ] `keep`/`review`/padrão/protegida nunca apagadas, um teste por caso; fetch falho/estourado ou forge degradado/truncado → nenhuma remoção remota
- [ ] `go build ./...` EXIT=0; `go test ./internal/commands/` EXIT=0

### ML-1B — Mensagem do guard para remoção remota
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Files affected (provisório):** `internal/guard/gitbranch.go`, referência `.sh` e gerador conforme o Wave 0, testes.
**Acceptance criteria:**
- [ ] Cada forma de remoção listada no Wave 0 recebe a mensagem que nomeia `trackfw branch prune --remote --apply`; `git push` comum mantém a mensagem atual
- [ ] Guard continua bloqueando a forma bruta (teste)
- [ ] `go test ./internal/guard/ ./internal/generators/` EXIT=0

### ML-1C — Contrato
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Files affected:** `docs/cli-parity.md` (depois do ML-1B do ROADMAP-2026-10-12, que também escreve nele)
**Acceptance criteria:**
- [ ] Seção do `branch prune --remote` com `trackfw-contract`; `make parity-rest` EXIT=0

## Wave 2 — Revisão e fechamento
> Dependencies: Wave 1 dos dois roadmaps. A revisão é única para a branch (ML-2A do ROADMAP-2026-10-12).

### ML-2A — Medição de volta e fechamento
**Owner:** `zeus-tf`
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] AC7: `trackfw branch prune --remote` (dry-run) neste repositório lista as remotas integradas e nenhuma com trabalho não integrado
- [ ] Pareceres do ML-2A do ROADMAP-2026-10-12 cobrem esta REQ; corretivos aplicados
- [ ] `make quality` EXIT=0 com máquina ociosa; CI do PR verde, inclusive `windows-full-suites`
