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
- [ ] AC1 — remoção, estreitamento, neutralização e chave de desligamento detectados, mesmo em lenient e com baseline
- [ ] AC2 — âncora em `origin/main` (ADR D1)
- [ ] AC3 — falsificação nas duas direções (adulterado acusa; nunca instalado não acusa)
- [ ] AC4 — mensagem distingue "nunca instalado" de "instalado e removido"
- [ ] AC5 — regras de `git_branch_guard` ancoradas
- [ ] AC6 — contrato em `docs/cli-parity.md` com `trackfw-contract`
- [ ] AC7 — `make quality` EXIT=0 e CI verde, inclusive `windows-full-suites`
- [ ] AC8 — cada teste novo declara o que afirma e reprova sem a correção
- [ ] AC9 — medição de volta dos três braços com o binário da branch

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
**Status:** ⬜ Pendente
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
   disco.
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

### ML-1B — Contrato em `docs/cli-parity.md`
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Dependencies:** ML-1A (mesmo executor, sequencial: o contrato descreve o comportamento entregue)
**Files affected:** `docs/cli-parity.md`
**Acceptance criteria:**
- [ ] Seção da regra com anotação `trackfw-contract`, estados da âncora e mensagens
- [ ] `make parity-rest` EXIT=0

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
