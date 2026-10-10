---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-09-02-remover-a-entrada-pretooluse-do-settings-nao-e-detectado-por-nenhuma-regra-de-integridade-de-guard.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: Remover a entrada `PreToolUse` do settings não é detectado por nenhuma regra de integridade de guard

> Created: 2026-09-22 | Status: wip

## Context
REQ: docs/req/REQ-2026-09-02-remover-a-entrada-pretooluse-do-settings-nao-e-detectado-por-nenhuma-regra-de-integridade-de-guard.md
ADR: docs/adr/ADR-2026-10-10-a-fiacao-do-guard-e-ancorada-no-origin-main-por-tupla-e-a-remocao-e-violacao-fora-do-lenient-e-do-baseline.md

Medido em 2026-10-10 (worktree de `origin/main` `39ad84de`, com e sem harness global): apagar
`hooks.PreToolUse` do `.claude/settings.json`, ou trocar os dois `trackfw guard` por `true`, deixa o
`trackfw validate` byte a byte idêntico (169 warnings). Desenho proposto no ADR (Proposed): âncora
na cópia do arquivo de hook em `origin/main`, comparação por tupla (arquivo, evento, matcher, guard),
regra nos carve-outs de lenient e baseline, e as 2 regras de `git_branch_guard` ancoradas (AC5).

Varredura de issues abertas em 2026-10-10: nenhuma aberta. REQs com o mesmo mecanismo: nenhuma em
aberto (a `REQ-2026-09-01-trackfw-escreve-e-audita-guard-global-...` trata de caminho de home, outra
causa).

## Acceptance Criteria
- [x] AC0 — Wave 0 auditado (parecer do `hades-tf` em `docs/seguranca/`)
      ✅ Evidência: `docs/seguranca/2026-10-10-wave0-fiacao-do-guard-ancorada.md` — aprovado com ajustes A1–A4; A2, A3, A4 incorporados ao ADR; A1 divergido (D7: chaves de desligamento viram detecção)
- [x] AC1 — remoção, estreitamento, neutralização e chave de desligamento detectados, mesmo em lenient e com baseline
      ✅ Evidência: AC9 — binário da branch em worktree de `origin/main`: base 0; `PreToolUse` apagado 4; `true` 7; `disableAllHooks` 1; `echo` 6 violações
- [x] AC2 — âncora em `origin/main` (ADR D1)
      ✅ Evidência: ref derivado por `deriveOriginDefaultBranch()`; `TestGuardWiringRemoved_SemTrackfwYamlNaRef_AindaCompara`, `RefIlegivel_FalhaFechada`
- [x] AC3 — falsificação nas duas direções (adulterado acusa; nunca instalado não acusa)
      ✅ Evidência: adulterados acusam (AC9); `ArquivoAusenteNaRef_Silencio`, `MigracaoLegitima_Silencio`
- [x] AC4 — mensagem distingue "nunca instalado" de "instalado e removido"
      ✅ Evidência: violação diz "was present in origin/main and is absent from disk … investigate"; ausente nos dois → silêncio
- [x] AC5 — regras de `git_branch_guard` ancoradas
      ✅ Evidência: `git_branch_guard_hook_resolvable` e `git_branch_guard_script_integrity` em `credentialGuardAnchoredRules`
- [x] AC6 — contrato em `docs/cli-parity.md` com `trackfw-contract`
      ✅ Evidência: seção `guard_wiring_removed` com `trackfw-contract` em `docs/cli-parity.md`
- [ ] AC7 — `make quality` EXIT=0 e CI verde, inclusive `windows-full-suites`
- [x] AC8 — cada teste novo declara o que afirma e reprova sem a correção
      ✅ Evidência: nomes conferidos por `go test -list`; mordida M1–M5 + mutação independente do arquiteto
- [x] AC9 — medição de volta dos três braços com o binário da branch
      ✅ Evidência: base 0, `PreToolUse` apagado 4, `true` 7, `disableAllHooks` 1, `echo` 6 (binário da branch, worktree de `origin/main`)

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model do ADR da fiação ancorada
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-10-10 · aprovado com ajustes A1–A4 (`docs/seguranca/2026-10-10-wave0-fiacao-do-guard-ancorada.md`)
**Files affected:** `docs/seguranca/2026-10-10-wave0-fiacao-do-guard-ancorada.md` (novo; único arquivo que pode ser escrito)
**Actions:**
1. **Completude da população (ADR D3).** Enumerar todo arquivo de hook de projeto em que o gerador
   (`internal/generators/agentfiles.go`) escreve um guard, para os dois guards, e cruzar com
   `credentialGuardHookFiles` e `credentialGuardRequiredEntries`
   (`internal/validator/validator_credential_guard.go`). Buscar também pelo literal do comando
   emitido (`guardCredentialCmdPSPOSIX`, `guardGitBranchCmdPSPOSIX` e equivalentes cmd.exe), não só
   pelos nomes já citados.
2. **Threat model.** Quem desliga o guard sem que a regra proposta acuse, e como. Obrigatório cobrir:
   (a) arquivos que o CLI lê e que não são rastreados (ex.: `.claude/settings.local.json`) e chaves
   de desligamento de hooks de cada CLI — verificar na documentação oficial, não presumir;
   (b) casamento do comando do guard: o que conta como fiação viva (`echo trackfw guard credential`,
   comando com prefixo/sufixo, matcher com regex equivalente);
   (c) os 4 estados do `loadOriginMainAnchor` aplicados a arquivos de hook — quais o adversário
   alcança editando só a working tree ou a branch;
   (d) interação com o dedup do gerador quando o guard global está instalado
   (`agentfiles.go:368,395`): confirmar que `trackfw update` não remove fiação existente;
   (e) se `credentialGuardAnchoredRules` + isenção de baseline + `lenientCarveoutRules` bastam para
   a regra não ser silenciada (ler `validator.go:597` e `filterBaselineTagged`).
3. **Alvos de falsificação nas duas direções**, por tupla: o que reprova quando a detecção regride
   (adulteração passa) e quando regride ao contrário (projeto que nunca instalou é acusado; `update`
   legítimo é acusado).
4. **Resíduo declarado:** o que o desenho aceita não cobrir, com nome.
5. **Veredito sobre o ADR:** ajustes numerados (A1, A2, …) que o arquiteto incorpora antes do
   Wave 1.
**Acceptance criteria:**
- [x] As cinco seções respondidas com evidência (comando + resultado, ou arquivo:linha), não asserção
      ✅ Evidência: fontes oficiais citadas por CLI; arquivo:linha para `filterBaselineTagged`, `applyLenientWithCarveout`, `migrateGuardHookMatcher`; A4 conferido pelo arquiteto (`validator_credential_guard_integrity.go:306`)
- [x] Nenhuma linha de implementação escrita; nenhum arquivo além do parecer alterado
      ✅ Evidência: `git status --short` só com o parecer; `git diff --quiet HEAD -- .claude/settings.json internal/` EXIT=0
- [x] Toda medição feita em worktree ou scratch próprio; `.claude/settings.json` da árvore real intacto
      ✅ Evidência: `git worktree list` só com a árvore principal após o ML

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-10-wave0-fiacao-do-guard-ancorada.md
grep -q "Veredito" docs/seguranca/2026-10-10-wave0-fiacao-do-guard-ancorada.md
git diff --quiet HEAD -- .claude/settings.json internal/
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditado e ajustes incorporados ao ADR. Arquivos exatos e nome final da regra
> confirmados após o parecer.

### ML-1A — Regra `guard_wiring_removed` e ancoragem do `git_branch_guard`
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-10-10 — `go build ./...` EXIT=0, `go test ./internal/validator/` EXIT=0 (27 testes da regra, nomes conferidos por `go test -list`); AC9 medido pelo arquiteto com o binário da branch (após o corretivo ML-1C)
**Files affected:** `internal/validator/validator_guard_wiring.go` (novo), `internal/validator/validator_guard_wiring_test.go` (novo), `internal/validator/validator_credential_guard_integrity.go` (`credentialGuardAnchoredRules`), `internal/validator/validator.go` (registro da regra e `lenientCarveoutRules`)
**Actions:**
1. Ler, por arquivo de hook da população (ADR D3), a cópia em `origin/main` reaproveitando o
   discriminante de 4 estados do `loadOriginMainAnchor`; `origin/main` ilegível → falha fechada.
2. Extrair tuplas (arquivo, evento, matcher, guard) das duas cópias; guard reconhecido pelo comando
   canônico, não por substring.
3. Tupla em `origin/main` e ausente no disco → violação nomeando arquivo, evento, matcher, guard e
   o remédio ("investigar"). Tupla ausente nas duas → silêncio.
4. **A2 do ADR:** comando comparado por classe de equivalência (D2-legacy, D11-legacy, D11-revised —
   as formas de `migrateHookCommand`); matcher por cobertura de alternativas `|` (disco ⊇
   `origin/main`); matcher que não seja alternância literal → não cobre.
5. **A4 do ADR:** derivar o ref com `deriveOriginDefaultBranch()`, sem reaproveitar
   `currentOriginMain.ref`; `origin` presente e ref vazio/ilegível → falha fechada nomeada.
6. **D7 do ADR:** chave de desligamento ativa no disco e inativa/ausente em `origin/main` é violação:
   `disableAllHooks` (`.claude/settings.json`, `.github/hooks/trackfw-attention.json`),
   `hooksConfig.enabled: false` (`.gemini/settings.json`), `"enabled": false` em entrada de guard
   (`.kiro/hooks/trackfw-attention.json`), `[features] hooks = false` (`.codex/config.toml`, só se
   rastreado). `.claude/settings.local.json` com `disableAllHooks: true` → violação verificada só no
   disco. (decisão do usuário em 2026-10-10: violação, mesma severidade, não warning)
7. Registrar a regra em `credentialGuardAnchoredRules` e `lenientCarveoutRules`; acrescentar
   `git_branch_guard_hook_resolvable` e `git_branch_guard_script_integrity` a
   `credentialGuardAnchoredRules` (AC5).
**Acceptance criteria:**
- [ ] Testes com repositório git real (bare como `origin`), um por tupla adulterada: chave apagada,
      matcher apagado, matcher estreitado, comando neutralizado → violação
- [ ] Uma violação por chave de desligamento (D7), incluindo `.claude/settings.local.json`
- [ ] Controle: arquivo ausente em `origin/main` → silêncio; arquivo idêntico → silêncio
- [ ] Controle A2: `origin/main` com matcher `Bash` e forma de comando legada, disco com
      `Bash|PowerShell` e forma D11-revised → silêncio
- [ ] Controle A4: repositório sem `trackfw.yaml` em `origin/main` e com arquivo de hook → a regra
      ainda compara
- [ ] Violação sobrevive a lenient e a baseline (teste)
- [ ] Cada teste novo com uma frase do que afirma e prova de mordida (reprova sem a correção)
**Comandos de validação:**
```bash
go build ./...
go test ./internal/validator/
```

### ML-1C — Corretivo da auditoria do ML-1A
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — auditado em 2026-10-10 — `go build ./...` EXIT=0, `go test ./internal/validator/` EXIT=0 (27 testes da regra, nomes conferidos por `go test -list`); AC9 medido pelo arquiteto com o binário da branch
**Origem:** auditoria do arquiteto, 2026-10-10. AC9 medido pelo arquiteto com o binário da branch
(worktree de `origin/main`): base 0 violações da regra; `PreToolUse` apagado 4; `trackfw guard` → `true`
7; `disableAllHooks` 1; `echo trackfw guard credential` 6.
**Files affected:** `internal/validator/validator_guard_wiring.go`, `internal/validator/validator_guard_wiring_test.go`, `docs/cli-parity.md`
**Achados:**
1. Mensagem com prefixo duplicado: `present in origin/origin/main` (o ref já traz `origin/`).
2. `guardWiringParseTomlFeaturesHooks` falha aberto em forma não reconhecida — `features = { hooks =
   false }` desliga os hooks do Codex sem acusar. O ADR só aceita resíduo para `config.toml` **não
   rastreado**.
3. Sem teste para D7 em Gemini (`hooksConfig.enabled`), Copilot (`disableAllHooks`), Kiro
   (`"enabled": false`), Codex (`config.toml`), nem para o comando `echo trackfw guard credential`.
4. Sem prova de mordida para os 17 testes.
**Acceptance criteria:**
- [x] Mensagens sem `origin/origin/`; teste que afirma o texto do ref
      ✅ Evidência: `grep '"origin/%s'` vazio em `validator_guard_wiring.go`; `TestGuardWiringRemoved_RefTexto_SemDuplicacao`
- [x] `config.toml` rastreado com chave `hooks` sob `features` em forma não reconhecida → violação (falha fechada); teste com tabela inline, chave pontilhada e comentário
      ✅ Evidência: estado triplo `tomlFeaturesHooksState` (só `true` explícito habilita); `TestGuardWiringRemoved_Codex_Toml{Section,InlineTable,DottedKey,CommentInline}_Dispara`, controles `TomlNaoRastreado_Silencio`, `TomlHooksTrue_Silencio`
- [x] Um teste por CLI da D7 e um para o `echo`
      ✅ Evidência: `Gemini_HooksConfigDisabled`, `Copilot_DisableAllHooks`, `Kiro_EntryEnabled_False`, `Codex_Toml*`, `ComandoNeutralizadoEcho`, `ComandoNeutralizadoEchoSimples`
- [x] Prova de mordida por teste novo, com os nomes conferidos por `go test -list`
      ✅ Evidência: 5 mutações reportadas (M1–M5); mutação independente do arquiteto em cópia de scratch — sem `guard_wiring_removed` em `credentialGuardAnchoredRules`, `TestGuardWiringRemoved_ViolaçaoSobreviveAoLenient` reprova (`must not be toleratable via baseline`). Ressalva: a tabela de descrições do relatório divergia dos testes reais; o código foi lido e confere

### ML-1B — Contrato em `docs/cli-parity.md`
**Owner:** `apolo-tf`
**Status:** ✅ Concluído — seção `guard_wiring_removed` com `trackfw-contract` em `docs/cli-parity.md`. `make parity-rest` EXIT=2 local por `scripts/check-required-status-checks.py` (sintaxe `str | None`) sob Python 3.9.6 — script idêntico ao de `origin/main` (medido); é ambiente, verificado no `make quality`/CI do Wave 2
**Dependencies:** ML-1A (mesmo executor, sequencial: o contrato descreve o comportamento entregue)
**Files affected:** `docs/cli-parity.md`
**Acceptance criteria:**
- [x] Seção da regra com anotação `trackfw-contract`, estados da âncora e mensagens
      ✅ Evidência: seção `guard_wiring_removed` em `docs/cli-parity.md`
- [x] `make parity-rest` EXIT=0
      ✅ Evidência: EXIT=0, 2024 linhas, com Python 3.12.15 + PyYAML + packaging (venv de scratch); o EXIT=2 anterior era o Python 3.9.6 do sistema, sem suporte a `str | None`

## Wave 2 — Barreira final
> Dependencies: Wave 1.

### ML-2A — Revisão de qualidade e red-team
**Owner:** `hefesto-tf` e `hades-tf` (em sequência, não em paralelo — custo de CPU)
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] Parecer do `hades-tf` reimplementando a partir da leitura e tentando burlar a regra
- [ ] Defeitos achados viram ML corretivo neste roadmap
- [ ] AC9: três braços repetidos com o binário da branch — base silenciosa, os dois adulterados acusam
- [ ] `make quality` EXIT=0 com máquina ociosa; CI do PR verde, inclusive `windows-full-suites`
