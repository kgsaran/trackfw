---
status: wip
date: 2026-11-23
req: "docs/req/REQ-2026-11-23-guard-write-scope-re-enraizado-por-trackfw-yaml-aninhado-permite-ao-subagente-desligar-os-proprios-hooks.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: Guard write-scope re-enraizado por trackfw.yaml aninhado permite ao subagente desligar os próprios hooks

> Created: 2026-11-23 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-23-guard-write-scope-re-enraizado-por-trackfw-yaml-aninhado-permite-ao-subagente-desligar-os-proprios-hooks.md -->
REQ: docs/req/REQ-2026-11-23-guard-write-scope-re-enraizado-por-trackfw-yaml-aninhado-permite-ao-subagente-desligar-os-proprios-hooks.md

Achado F-5 do red team da REQ-2026-11-22 (`docs/seguranca/2026-11-22-red-team-onboarding.md`, F-5). O guard
write-scope (`internal/guard/writescope.go`) resolve a raiz pelo primeiro ancestral do **alvo** que tem `trackfw.yaml`
(`findProjectRoot`, ~:504; Step 7, ~:165). O contrato vem do ADR-2026-10-23 (adendo, "D3, contrato do ML-3A"): a raiz é a
do alvo, para que um agente num worktree seja julgado pelas regras do checkout principal. O efeito não declarado: um
subagente cria `<sub>/trackfw.yaml` (fora dos congelados) e passa a escrever em `<sub>/...` sob uma raiz sem regras —
por exemplo `.claude/settings.json`, onde ficam os hooks. O mesmo ADR já declara como residual que escrita por Bash
escapa da D3 e da D4; a Wave 0 mede o que esta REQ fecha diante disso.

Varredura de issues abertos em 2026-11-24: nenhum.

## Acceptance Criteria
- [ ] AC1: criar um `trackfw.yaml` aninhado não muda a raiz nem as regras que o guard aplica a um subagente (braço e
  contra-braço no binário)
- [ ] AC2: decisão sobre `trackfw.yaml` novo escrito por subagente fora da raiz registrada em ADR
- [ ] AC3: mesmo mecanismo varrido nos outros guards e nos comandos que resolvem a raiz pelo `trackfw.yaml` mais próximo
- [ ] AC4: `make quality` rc=0 e CI verde

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model, medição e varredura (hades-tf)
**Status:** ✅ Done — F-5 reproduzido no binário pelo hades e pelo arquiteto (baseline 2, planta 0, braço 0,
contra-braço 2). Variantes do mesmo mecanismo: diretório `trackfw.yaml`, `TRACKFW.YAML` em FS sem caixa, FIFO (trava).
Mesma causa no guard de credencial (`RunCredentialGlobal`: pre=2, braço=0; `RunCredential` lê só o yaml do cwd, por
leitura). Comandos leem o yaml do cwd sem subir e não decidem permissão de subagente. Recomendação (a)+(d)+(e) e piso
de credencial, registrada em ADR-2026-11-24 (D1–D4).
**Files affected:** `docs/seguranca/2026-11-24-wave0-write-scope-raiz-aninhada-threat-model.md` (novo; único arquivo)
**Actions:**
1. Reproduce F-5 on the binary with an isolated HOME: the two writes (`.claude/trackfw.yaml`, then
   `.claude/settings.json`) as a subagent payload (`agent_id` present), arm and counter-arm (main session; existing
   nested project).
2. Enumeration completeness: every place that resolves a project root by the nearest `trackfw.yaml` — in `internal/guard/`
   (write-scope Step 7 and Step 11a, lease roots, credential, git-branch, opencode/antigravity plugins) and in commands
   (`config`, `validate`, `serve`, `barrier`, `ship`, `push`, `commit`). For each: can a nested `trackfw.yaml` change who
   is allowed to do what?
3. Candidate designs with the measurement for each: (a) every `trackfw.yaml` is frozen for subagents regardless of
   location; (b) root anchored on the outermost `trackfw.yaml` or on the session `cwd` root; (c) a nested root only
   counts if it predates the session or is committed; plus the legitimate case to preserve (agent in a worktree writing
   to the main checkout, ADR-2026-10-23) and monorepos with real nested projects.
4. Threat model with the Bash residual: what this REQ closes given that a subagent can already write through Bash
   (ADR-2026-10-23 residual); recommend the D for the ADR.
5. Falsification targets in both directions and declared residual.
**Acceptance criteria:**
- [x] The sections above answered with evidence (commands and output), not a one-line assertion
- [x] No implementation line written for this ML

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-24-wave0-write-scope-raiz-aninhada-threat-model.md
```

## Wave 1 — Implementation (2 MLs em paralelo)
> Dependencies: Wave 0 audited; ADR-2026-11-24 written. ML-1A and ML-1B touch disjoint files and run in parallel; each
> runs only `go test ./internal/guard/` (no `make quality` in this wave).

### ML-1A — Write-scope monotônico: D1, D2, D3 do ADR-2026-11-24 (apolo-tf)
**Status:** ✅ Done — D1 antes da raiz (`hasTrackfwYAMLComponent`, só com `agent_id`); D2 por `frozenEntry{root,rel}` com
`buildFrozenListForRoot` de cada ancestral; D3 via `findProjectRootSafe` + `ReadRegularFile` (não regular nega). 16
testes + 1 de FIFO (arquivo com `//go:build !windows`, porque `syscall.Mkfifo` não compila no Windows); braço antes:
9 vetores rc=0 e FIFO travado; depois, todos passam. Auditoria do arquiteto: build/vet rc=0; `go test ./internal/guard/`
só reprova nos 3 testes do contrato antigo de credencial (ML-1C). O agente tentou um `git stash`, bloqueado pelo hook
de worktree compartilhado; nada executado.
**Files affected:** `internal/guard/writescope.go`; `internal/guard/writescope_nested_root_test.go` (novo);
`internal/guard/writescope_nested_root_fifo_unix_test.go` (novo); `.claude/agent-memory/apolo-tf/MEMORY.md`;
`.claude/agent-memory/apolo-tf/feedback_fifo_test_needs_build_tag.md` (novo)
**Actions:**
1. D1: for payloads with `agent_id`, deny (rc=2) any write whose path has a component equal to `trackfw.yaml` under
   `strings.EqualFold`, after path normalization and BEFORE the project-root lookup (Step 7), so it also applies outside
   any project. Normalization strips trailing `.` and spaces from every component (Windows creates `trackfw.yaml` from
   `trackfw.yaml.`); apply that normalization as a pure function covered by unit tests on every OS.
2. D2: the frozen list is the union of the FULL `buildFrozenList` of every ancestor that has `trackfw.yaml`, from the
   target up to the filesystem root, each entry resolved against its own root. Never a union of only
   `write_scope.frozen_paths`: the protection of `.claude/settings.json` comes from the hard-coded subagent list.
3. D3: every `trackfw.yaml` read by write-scope goes through `fileutil.ReadRegularFile`. Only `os.IsNotExist` is absent;
   `ErrNotRegularFile` and any other error deny. `findProjectRoot` must not accept a directory or FIFO as a root.
   Never `if err != nil { return nil, nil, nil }`.
4. Keep the target's root (ADR-2026-10-23 worktree contract) and every existing test green.
**Tests (AC1 vectors from the Wave 0, each test with one sentence stating what it asserts):**
- Write `<proj>/.claude/trackfw.yaml` (subagent, no lease) → rc=2; also under lease `allow=[".claude/"]`
- Write `<proj>/x/trackfw.yaml/y` (subagent) → rc=2 (component, not basename)
- Write `<proj>/P/trackfw.yaml` under exact-file lease `allow=["P/trackfw.yaml"]` → rc=2
- Write `<proj>/.claude/TRACKFW.YAML` (subagent) → rc=2 (string comparison, runs on every FS)
- Write `trackfw.yaml` in a directory with no project → rc=2
- Outer yaml WITHOUT `write_scope`, nested yaml written by the test into the fixture (not via the guard), then Write
  `<proj>/.claude/settings.json` → rc=2
- Directory named `trackfw.yaml` created in the fixture, then Write `.claude/settings.json` → rc=2
- FIFO named `trackfw.yaml` (skip on Windows) → rc=2 without hanging (test with timeout)
- Symlink `trackfw.yaml → real.yaml` at the root → protection kept (rc=2 on `.claude/settings.json`)
- Worktree counter-arm (agent cwd in worktree, target in main checkout) → rc=2, as today
- Monorepo with pre-existing nested yamls → rc=2 on both `svc/.claude/settings.json` and `.claude/settings.json`
- Main session (no `agent_id`) writing `<proj>/sub/trackfw.yaml` → rc=0 (D1 is subagent-only)
- Normalization unit test: `trackfw.yaml.`, `trackfw.yaml ` and `TRACKFW.YAML` all match; `trackfw.yaml.bak` does not
**Acceptance criteria:**
- [x] all vectors above pass; arm reproduced as rc=0 before the change and rc=2 after (report both)
- [x] `go test ./internal/guard/` rc=0; `go vet ./internal/guard/` rc=0; `go build ./...` rc=0 (após ML-1C)
- [x] no file outside "Files affected" changed

### ML-1B — Guard de credencial: modo mais estrito da cadeia, D4 do ADR-2026-11-24 (apolo-tf)
**Status:** ✅ Done — `credFindNearestProjectRoot` (projeto pelo ancestral mais próximo com yaml regular) e
`credWalkStrictestMode(startDir, callerDefault)` nos dois escopos; nenhum `"block"` literal na função compartilhada;
attention e `roadmap_dir` agora pela raiz do projeto. 8 testes; braço antes rc=0 em 4 vetores, depois rc=2; contra-braço
`warn` de projeto mantido. Três testes antigos afirmam o contrato que o ADR mudou (ML-1C).
**Files affected:** `internal/guard/credential.go`; `internal/guard/credential_nested_mode_test.go` (novo)
**Actions:**
1. `RunCredential` (project scope): detect the project by the nearest ancestor of the cwd with `trackfw.yaml` (today it
   only checks `<cwd>/trackfw.yaml`, so a cwd in a subdirectory turns the guard off). Keep the Antigravity payload-cwd
   rule (ADR-2026-11-18 D3).
2. Both scopes: the effective mode is the strictest of the caller's default and the `credential_guard.mode` of every
   `trackfw.yaml` from the cwd up to the filesystem root (`block` > `warn`). Defaults stay per caller: `warn` in
   `RunCredential`, `block` in `RunCredentialGlobal`. TRAP: `credReadModeWithDefault` has two callers; never put a
   literal `"block"` floor inside the shared function.
3. Reads go through `fileutil.ReadRegularFile`; a non-regular `trackfw.yaml` counts as `block`.
**Tests (each with one sentence stating what it asserts):**
- global, cwd=sub with planted `mode: warn` → blocks (was rc=0)
- global, no yaml anywhere → blocks (unchanged)
- project, root `mode: block`, cwd=sub without yaml → blocks (was off)
- project, root `mode: block`, cwd=sub with planted `mode: warn` → blocks
- project counter-arm, root `mode: warn` only → warns, rc=0 (the default `warn` is kept)
- project, no yaml in any ancestor → rc=0 (not a project)
- Antigravity payload cwd still used
**Acceptance criteria:**
- [x] all tests above pass; `go test ./internal/guard/` rc=0; `go build ./...` rc=0 (após ML-1C)
- [x] no file outside "Files affected" changed

### ML-1C — Quem afirma o contrato antigo do modo global (apolo-tf)
**Status:** ✅ Done — três testes reescritos e renomeados para D4 (`..._WarnYamlDoesNotLowerGlobalBlock`,
`..._WarnYamlBlocksEvenWithoutRoadmapDir`, `..._GlobalScope_WarnCRLFStillBlocks`); `cli-parity.md` com D4; wrapper de
projeto sobe ancestrais (script, gerador e referência de integridade idênticos). `guard`, `discover`, `validator`
verdes. Ficaram: 3 testes de `internal/generators/credential_guard_test.go` com o contrato antigo, a exceção do teste
de FIFO no `check-symlink-privilege-guard.sh` (reprova o `make parity-rest`) e o ramo `warn` do global, agora
inalcançável (ML-1D).
**Files affected:** `internal/guard/credential_test.go`; `internal/guard/credential_mode_test.go`; `docs/cli-parity.md`;
`scripts/trackfw-credential-guard.sh`; `internal/generators/scaffold.go`;
`internal/validator/validator_credential_guard_integrity_reference.go`
**Actions:**
1. `TestRunCredentialGlobal_WarnModeFromYamlWritesJSON` (`credential_test.go:548`),
   `TestRunCredentialGlobal_WarnNoDirNoFileCreated` (`:647`) and `TestReadCredentialGuardMode_EndToEnd_GlobalScope`
   (`credential_mode_test.go:253`) assert that `mode: warn` lowers the global hook. Rewrite each to assert the D4
   contract (global stays `block`), keeping whatever else they pin that is still true.
2. `docs/cli-parity.md` (~6640): the credential mode section says the yaml mode is respected in global scope; state D4.
3. The wrapper `scripts/trackfw-credential-guard.sh` exits 0 when `trackfw.yaml` is not in the cwd, which turns the
   guard off in a subdirectory (same cause). Replace with an ancestor walk, in the wrapper and in its generator, keeping
   the reference copy and the generated copy byte-identical (`internal/discover/discover_test.go:452`).
4. Search `scripts/check-gates-falsify.sh` and `scripts/check-*.sh` for scenarios that assert the old global mode or the
   wrapper's cwd check; list what you found (update only if they assert the old contract).
**Acceptance criteria:**
- [x] `go test ./internal/guard/ ./internal/discover/` rc=0; `go build ./...` rc=0; `make parity-rest` rc=0 (após ML-1D)

### ML-1D — Restante do contrato antigo e código morto do modo global (apolo-tf)
**Status:** ✅ Done — `RunCredentialGlobal` bloqueia sem ler yaml (ramo `warn` e `credReadModeWithDefault` removidos,
sem chamador); 2 testes de `generators` reescritos e renomeados, o caso global da paridade virou teste próprio; exceção
FIFO no `check-symlink-privilege-guard.sh`. `go build`/`vet` rc=0, `make parity-rest` rc=0. `go test ./...` só reprova
em `TestGate_RealTree` (`internal/winskips`): o teste de FIFO do ML-1A falta em `.github/windows-skips.json` (ML-1E).
**Files affected:** `internal/generators/credential_guard_test.go`; `scripts/check-symlink-privilege-guard.sh`;
`internal/guard/credential.go`; `internal/validator/validator_credential_guard_global_reference.go`
**Actions:**
1. `TestGlobalCredentialGuardScript_RespectsExplicitProjectModeWarn`,
   `TestGlobalCredentialGuardScript_WritesAttentionOnlyWhenRoadmapsDirExists` and the global subtest of
   `TestCredentialGuardScript_SecondLayer_AttentionMessageParityNoPath` assert the old contract; rewrite to D4.
2. `EXCEPTIONS["internal/guard/writescope_nested_root_fifo_unix_test.go"]="FIFO_BUILD_TAG_NOT_WINDOWS"` beside the four
   existing FIFO entries.
3. `RunCredentialGlobal`: with default `block`, the effective mode is always `block`; remove the unreachable `warn`
   branch (attention write, roadmap dir) and make the code say what it does, keeping D4 documented.
4. The comment in `validator_credential_guard_global_reference.go` that quotes the old project wrapper line.
5. Run `go test ./... -count=1` once to find any other site that asserts the old contract; list it, change only files
   above.
**Acceptance criteria:**
- [x] `go test ./... -count=1` rc=0 (após ML-1E); `make build && make parity-rest` rc=0

### ML-1E — Registro do teste de FIFO no inventário de skips do Windows (apolo-tf)
**Status:** ✅ Done — entrada classe `c` no `windows-skips.json`; comentário corrigido. Auditoria: `go test ./... -count=1`
rc=0 na árvore real.
**Files affected:** `.github/windows-skips.json`; `internal/guard/credential_mode_test.go`
**Actions:**
1. Add the entry for `internal/guard/writescope_nested_root_fifo_unix_test.go:TestD3_FIFONamedTrackfwYAML_DenyWithoutHanging`
   following the format and classification of the existing FIFO entries in the same file.
2. `credential_mode_test.go:188`: the comment cites the removed `credReadModeWithDefault`; point it to
   `credWalkStrictestMode`.
**Acceptance criteria:**
- [x] `go test ./internal/winskips/ ./internal/guard/ -count=1` rc=0

**Wave gates:**
```bash
go build ./...
go vet ./internal/guard/
go test ./internal/guard/
```

## Wave 2 — Review and red team (2 MLs em paralelo)
> Dependencies: Wave 1 audited.

### ML-2A — Revisão de qualidade (hefesto-tf)
**Status:** ✅ Done — veredito REPROVA. MÉDIO-1 (confirmado pelo arquiteto): `credFindNearestProjectRoot` pula yaml não
regular ou ilegível e, se for o único da cadeia, o guard de projeto sai com 0 (viola D3). MÉDIO-2: cinco frases de
reconciliação do escopo global citam leitura de yaml que não ocorre. BAIXO-1 a 4: `cli-parity`, `::$DATA`, comentário de
teste, wrapper `findProjectRoot` em produção. Corretivo: ML-2C.
**Files affected:** `docs/seguranca/2026-11-24-revisao-write-scope-raiz-aninhada.md` (novo)
**Actions:** review ML-1A and ML-1B against ADR-2026-11-24 D1–D4: each test's sentence matches what it asserts; no
`err != nil → absent`; no `"block"` literal in the shared function; dead code; Windows normalization.
**Acceptance criteria:**
- [x] review written; blocking findings fixed by a corrective ML (ML-2C)

### ML-2B — Red team (hades-tf)
**Status:** ✅ Done — APROVA COM RESSALVAS. D1–D3 passam contra os binários antes (`3ae8d29c`) e depois (`7613c035`);
FIFO antes travava >30 s, depois rc=2 em 0 s. Abertos: R-8b (`trackfw.yaml::$DATA` contorna D1, rc=0 nos dois) e
F-LEASE-MONO (lease só na raiz do alvo e na do cwd). D4 verificado por leitura e pelos 8 testes, não falsificado no
binário. Revisão do arquiteto: o wrapper de credencial testa `-f` e repete o MÉDIO-1 no shell; a lista de congelados
compara o caminho cru (mesma causa do R-8b). ADR-2026-11-24 emendado (D1 ADS/8dot3, D3 detecção de raiz, D5, D6).
**Files affected:** `docs/seguranca/2026-11-24-red-team-write-scope-raiz-aninhada.md` (novo)
**Actions:** falsify D1–D4 from the contract on the built binary with an isolated HOME: every Wave 0 vector, the six
"Formas" of the Wave 0, and new variants; both directions (worktree, monorepo, main session must keep working).
**Acceptance criteria:**
- [x] report written with arm and counter-arm; residual declared

### ML-2C — Corretivo da Wave 2 (apolo-tf)
**Status:** ✅ Done — itens 1–7 entregues; 19 testes novos com frase, braço antes rc=0 nos vetores de D1/D5/D6/MÉDIO-1 e
do wrapper. Auditoria do arquiteto: um teste de D6 sem asserção e um `break` silencioso em `collectLeaseRoots` diante de
yaml ancestral não regular (D3) voltaram ao apolo e foram corrigidos (asserção rc=0; erro → rc=2, teste direto da
função). O agente marcou o status no roadmap, fora da lista; mantido. Gates rodados pelo arquiteto: build, vet, testes
dos 4 pacotes, `make build`, `make parity-rest`, todos rc=0.
**Files affected:** `internal/guard/credential.go`; `internal/guard/credential_nested_mode_test.go`;
`internal/guard/credential_mode_test.go`; `internal/guard/writescope.go`; `internal/guard/writescope_nested_root_test.go`;
`internal/guard/writescope_ml2c_test.go` (novo); `internal/generators/credential_guard_test.go`;
`internal/generators/scaffold.go`; `scripts/trackfw-credential-guard.sh`;
`internal/validator/validator_credential_guard_integrity_reference.go`; `docs/cli-parity.md`;
`docs/adr/ADR-2026-10-23-regras-de-escrita-dos-agentes-sao-aplicadas-pelo-harness-e-nao-pelo-texto-do-handoff.md`
(linha "Emendado por ADR-2026-11-24", escrita pelo arquiteto para o D5)
**Actions:**
1. MÉDIO-1 (D3, detecção de raiz): `credFindNearestProjectRoot` para em qualquer `trackfw.yaml` que exista (inclusive
   diretório, FIFO, ilegível) e `RunCredential` nega (rc=2) quando esse yaml não é arquivo regular legível. Só
   `os.IsNotExist` continua subindo.
2. Wrapper (D3): nas três cópias byte-idênticas, `[ -f "${d%/}/trackfw.yaml" ]` vira
   `[ -e "${d%/}/trackfw.yaml" ] || [ -L "${d%/}/trackfw.yaml" ]`.
3. D1 (R-8b, 8dot3): `normalizePathComponent` corta no primeiro `:` e depois remove `.` e espaço finais;
   `hasTrackfwYAMLComponent` também casa `TRACKF~<dígitos>.YAM` (EqualFold).
4. D6: `frozenHit` casa também a grafia com cada componente normalizado pela mesma função.
5. D5: leases consultados em todo ancestral do alvo com `trackfw.yaml`, além da raiz do cwd; todo lease encontrado
   autoriza; inválido nega.
6. MÉDIO-2: corrigir as frases dos 4 testes citados; `TestReadCredentialGuardMode_EndToEnd_GlobalScope_WarnCRLFStillBlocks`
   é removido ou passa a exercitar o escopo de projeto (onde o CRLF importa).
7. BAIXO-1: `cli-parity.md` diz que o escopo global nunca grava attention. BAIXO-3: comentário de
   `TestD3_DirectoryNamedTrackfwYAML_Deny`. BAIXO-4: `findProjectRoot` sai do código de produção.
**Acceptance criteria:**
- [x] cada teste novo com braço antes (rc=0 ou falha) e depois, e uma frase de reconciliação
- [x] `go build ./...`, `go vet ./internal/guard/ ./internal/generators/` rc=0
- [x] `go test ./internal/guard/ ./internal/generators/ ./internal/discover/ ./internal/validator/ -count=1` rc=0
- [x] `make build && make parity-rest` rc=0
- [x] nenhum arquivo fora de "Files affected" alterado (exceto o status do próprio ML no roadmap)

### ML-2D — Re-revisão estreita do delta do ML-2C (hefesto-tf)
**Status:** ✅ Done — APROVA COM RESSALVAS; MÉDIO-1/2 e BAIXO-1..4 fechados. Novos BAIXO-5..9 (mensagem com nome antigo,
teste 8dot3 com regex própria, contra-braço do D5 que não ativa a subida, falso positivo de diretório não declarado,
frase do teste CRLF). BAIXO-8 resolvido no ADR pelo arquiteto; os outros no ML-2F.
**Files affected:** `docs/seguranca/2026-11-24-revisao-ml-2c-write-scope.md` (novo)
**Acceptance criteria:**
- [x] revisão escrita; nenhum achado bloqueante aberto

### ML-2E — Red team estreito do delta do ML-2C (hades-tf)
**Status:** ✅ Done — APROVA COM RESSALVAS; nenhum escape. 22 vetores com antes e depois (D3 diretório e chmod 000,
wrapper com diretório e symlink quebrado, D1 ADS/8dot3/Unicode, D6, D5 com lease inválido e monorepo, regressões).
Ressalva: nome 8dot3 com hash (`TR1A2B~1.YAM`) escapa da regex; vai para o ML-2F (decisão: corrigir, não declarar).
**Files affected:** `docs/seguranca/2026-11-24-red-team-ml-2c-write-scope.md` (novo)
**Acceptance criteria:**
- [x] relatório com braço e contra-braço para D3-raiz, wrapper, R-8b, 8dot3, D5 e D6

### ML-2F — Ressalvas da re-revisão (apolo-tf)
**Status:** ✅ Done — regex 8dot3 com a forma de hash; mensagem com `findProjectRootSafe:`; teste 8dot3 usa
`trackfwYAML8dot3RE`; contra-braço do D5 com alvo em `svc/` e lease só em `outer/`; frase do CRLF cita `TrimSpace`;
`cli-parity` diz rc=2 antes do modo. Desvio aceito: o vetor `TR1A2B~1.YAM` ficou no teste unitário da regex de produção,
não ponta a ponta (o caminho ponta a ponta é o mesmo de `TestD1_8dot3TrackfwYAML_Deny`). Gates rodados pelo arquiteto:
build, vet, `go test ./internal/guard/` rc=0.
**Files affected:** `internal/guard/writescope.go`; `internal/guard/writescope_ml2c_test.go`;
`internal/guard/credential_mode_test.go`; `docs/cli-parity.md`
**Actions:**
1. Regex 8dot3 vira `(?i)^[^~\/:]{1,6}~[0-9]+\.yam$` (ADR D1, forma com hash); vetor `TR1A2B~1.YAM` → rc=2.
2. BAIXO-5: mensagem de erro com `findProjectRoot:` passa a citar `findProjectRootSafe:`.
3. BAIXO-6: o teste de 8dot3 usa `trackfwYAML8dot3RE` (ou `hasTrackfwYAMLComponent`), não uma regex própria.
4. BAIXO-7: contra-braço do D5 com alvo `svc/main.go` e lease externo que o permite, cwd em `svc/` → rc=0 (a subida
   precisa estar ativa).
5. BAIXO-9: frase do teste CRLF cita o mecanismo real.
6. `cli-parity.md`: a frase "yaml não regular conta como block" passa a dizer que nega (rc=2) antes da escolha do modo.
**Acceptance criteria:**
- [x] `go build ./...`; `go vet ./internal/guard/`; `go test ./internal/guard/ -count=1` rc=0
- [x] nenhum arquivo fora de "Files affected" alterado

### ML-2G — Teste do wrapper no inventário de skips do Windows (apolo-tf)
**Status:** ✅ Done — o `make quality` (ML-3A) reprovou em `TestGate_RealTree`: o teste
`TestCredentialGuardScript_DirectoryYaml_CallsTrackfw` pula no Windows (exige `sh`) e faltava no inventário. Entrada
classe `c`, no modelo de `TestCredentialGuardScript_GeneratedScript_WindowsPathRedirectDetectsJWTInFile`.
**Files affected:** `.github/windows-skips.json`
**Acceptance criteria:**
- [x] `go test ./internal/winskips/ -count=1` rc=0

**Wave gates:**
```bash
test -s docs/seguranca/2026-11-24-revisao-write-scope-raiz-aninhada.md
test -s docs/seguranca/2026-11-24-red-team-write-scope-raiz-aninhada.md
go build ./...
go test ./internal/guard/ ./internal/generators/ ./internal/discover/ ./internal/validator/ -count=1
```

## Wave 3 — Fechamento
> Dependencies: Wave 2 audited.

### ML-3A — `make quality` e CI (zeus-tf)
**Status:** ⬜ Pending
**Files affected:** `docs/roadmaps/wip/ROADMAP-2026-11-23-guard-write-scope-re-enraizado-por-trackfw-yaml-aninhado-permite-ao-subagente-desligar-os-proprios-hooks.md`
**Actions:** `make quality` with an idle machine; push; CI green on all jobs (Windows included).
**Acceptance criteria:**
- [ ] `make quality` rc=0; CI green
