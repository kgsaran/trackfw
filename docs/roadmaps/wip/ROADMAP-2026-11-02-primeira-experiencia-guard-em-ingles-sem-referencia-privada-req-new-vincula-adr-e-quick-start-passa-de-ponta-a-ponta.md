---
status: wip
date: 2026-11-02
req: "docs/req/REQ-2026-11-02-primeira-experiencia-guard-em-ingles-sem-referencia-privada-req-new-vincula-adr-e-quick-start-passa-de-ponta-a-ponta.md"
squad: "hades-tf, apolo-tf"
---

# Roadmap: Primeira experiência: guard em inglês sem referência privada, req new vincula ADR e Quick start passa de ponta a ponta

> Created: 2026-11-02 | Status: wip

## Context
REQ: docs/req/REQ-2026-11-02-primeira-experiencia-guard-em-ingles-sem-referencia-privada-req-new-vincula-adr-e-quick-start-passa-de-ponta-a-ponta.md

Os defeitos estão medidos na REQ. Varredura de issues abertos em 2026-11-02: nenhum aberto.

## Acceptance Criteria
- [x] AC1–AC6 da REQ (com AC3b e AC4b)

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-11-02. As prescrições P1 a P8 foram aceitas. O T6 (`req new` sobrescreve uma REQ existente sem aviso) foi medido pelo arquiteto com o binário 11.2.5: a marca acrescentada à REQ sumiu depois de um segundo `req new "Login"`, com saída 0. É perda de dado no mesmo comando, então entra no ML-1B, sem reabrir a REQ-2026-09-09. O R1–R3 (português em cerca de 30 arquivos fora dos guards, parte deles contrato parseado, como a legenda de status do roadmap) foi levado ao KG como decisão de escopo.
**Files affected:** `docs/seguranca/2026-11-02-wave0-primeira-experiencia.md`
**Actions:**
1. Completude da enumeração:
   - todas as mensagens emitidas pelo binário em português ou com `§`, por grep de mecanismo e não só pelo literal `bruto bloqueado`, em `internal/guard/`, `internal/commands/`, `internal/validator/` e `internal/generators/`;
   - os testes, goldens, `testdata/` e scripts que afirmam o texto atual, para que a troca não quebre um gate de identidade de bytes. Cite `internal/generators/testdata/guard-sh-reference/` e o shim byte-identity.
2. Modelo de ameaça:
   - o `--adr` aceita caminho com `../`, symlink ou nome que casa com mais de um ADR?
   - a escrita do vínculo pode corromper um frontmatter existente?
   - a tradução pode tirar da mensagem do guard uma orientação que o agente usa para se corrigir?
3. Alvos de falsificação nos dois sentidos para o AC1 a AC4.
4. Residual declarado.
**Acceptance criteria:**
- [x] As quatro seções respondidas com evidência (comando e resultado)
- [x] Prescrições em seção intitulada `Prescrições`
- [x] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-02-wave0-primeira-experiencia.md
grep -q '^## Prescrições' docs/seguranca/2026-11-02-wave0-primeira-experiencia.md
```

## Wave 1 — Implementação (sequencial: ML-1A, depois ML-1B)
> Dependencies: Wave 0 auditada. Os dois MLs editam arquivos diferentes do pacote `internal/commands`. Em paralelo, um quebraria a compilação do teste do outro no meio da edição, então rodam em sequência.

### ML-1A — Mensagens do guard em inglês, sem referência privada (apolo-tf) — AC1, AC2
**Status:** ✅ Concluído — auditado em 2026-11-02. As 20 constantes `reason*` e as 3 sentinelas do fixture estão em inglês, em paridade exata e sem `§`; os `Long` de `branch prune` e `agents models` e o erro do `commit` estão limpos. Testes novos: `TestReasonOrientationSubstrings` (A), `TestNoPrivateSectionReferenceInternal` (B), `TestReasonNoPortugueseStopwords` (C-guard) e `TestBranchPruneLong_`/`TestAgentModelsLong_NoPrivateSectionReference` (C-commands). Os quatro contra-braços reprovam, e os textos foram restaurados com `cmp`. Precisou de dois despachos, porque a lista da Wave 0 omitia três testes, as sentinelas e o pin SHA-256. Fica como residual o `§` em comentários Go, que não são emitidos ao usuário.
**Files affected:** `internal/guard/gitbranch.go`, `internal/guard/payload.go`, teste novo `internal/guard/messages_test.go`, `internal/commands/commit.go`, `internal/commands/commit_test.go`, `internal/commands/branch_prune.go`, `internal/commands/agents_models.go`, teste novo `internal/commands/no_private_reference_test.go`, `internal/generators/testdata/guard-sh-reference/git-branch-guard.sh` (só linhas `REASON=` e as sentinelas `_TRACKFW_(TRUNC|NUL|AWK)_REASON=`; P8 ampliado), `internal/generators/git_branch_guard_dedup_test.go`, `internal/generators/guard_parity_helper_test.go` (cópias `c22*Reason` e pin SHA-256 do fixture), `internal/generators/git_branch_guard_stdin_drain_test.go`, `internal/generators/git_branch_guard_test.go`. A Wave 0 não listou esses três testes nem as sentinelas; o primeiro despacho deixou 5 testes de `internal/generators` reprovando e foi completado num segundo.
**Actions:**
1. Traduzir as constantes `reason*` e todas as demais mensagens emitidas listadas pela Wave 0, mantendo a orientação.
2. Remover `Ver CLAUDE.md §1`.
3. Teste que varre as mensagens e reprova com `§` ou texto em português (AC2), com contra-braço.
**Acceptance criteria:**
- [x] `go test ./internal/guard/ -count=1` e os pacotes dos fixtures tocados verdes
- [x] contra-braço do AC2 medido, restaurado e conferido com `cmp`

### ML-1B — `req new --adr` e Quick start de ponta a ponta (apolo-tf) — AC3, AC4
**Status:** ✅ Concluído — auditado em 2026-11-02. O `--adr` valida antes de qualquer escrita (P5), resolve por nome exato via `ResolveADRFiles` (P6) e grava no `adr:` e no `ADR:`. Na auditoria, o primeiro guard de sobrescrita (recusar com erro) quebrou o contrato de idempotência do ML-4A. O arquiteto decidiu: REQ existente fica intacta, o vínculo é reparado linha a linha e o comando sai 0; com `--adr` diferente, recusa. O AC3b foi reescrito com essa decisão. Os testes foram conferidos por nome em `-v`: 19 PASS, incluindo os dois do ML-4A sem mudança de asserção. Contra-braços: voltar à reescrita do template reprova 5 braços; sem `--adr`, o Quick start reprova pela `req_has_adr`. O comentário desatualizado em `req_chain_ml4a.go` foi corrigido pelo arquiteto (só comentário). Residuais: CRLF normalizado quando o reparo escreve; o assistente interativo só descobre a REQ existente depois dos formulários; nome de ADR com espaço passa no `--adr`, mas a regra lê só o primeiro token.
**Files affected:** `internal/commands/req.go`, `internal/commands/req_test.go`, `internal/generators/req.go`, `internal/generators/req_chain_ml4a.go` (comentário), `internal/commands/req_chain_ml4a_test.go` (comentário), teste novo `internal/commands/quickstart_e2e_test.go`, `docs/cli-parity.md`
**Actions:**
1. Flag `--adr`, resolvido por `ResolveADRFiles`. Grava o `adr:` do frontmatter e a linha `ADR:`; ADR inexistente ou ambíguo dá erro sem criar arquivo.
2. Teste de ponta a ponta do Quick start em `t.TempDir()`, com `validate` terminando em 0 violações.
3. T6 da Wave 0: `req new` com REQ de mesmo nome já existente recusa, com saída diferente de zero e o arquivo intacto. Hoje ele sobrescreve sem aviso.
**Acceptance criteria:**
- [x] `go test ./internal/commands/ -count=1` verde
- [x] contra-braço: sem o vínculo, o teste de ponta a ponta reprova

**Gates da wave:**
```bash
go build ./...
go vet ./internal/guard/ ./internal/commands/
go test ./internal/guard/ -count=1
go test ./internal/commands/ -count=1
```

### ML-1C — Padrão do assistente de identidade (apolo-tf) — AC4b
**Status:** ✅ Concluído — auditado em 2026-11-02. `identitySelect` passa a começar em `defaultIdentitySelection()` = `neutral`; o `huh` posiciona o cursor por `selectValue` em `Accessor` (`field_select.go` do huh v1.0.0). Testes: `TestDefaultIdentitySelectionIsNeutral` e `TestIdentitySelectDefaultCursorIsNeutral` (cursor lido por `Hovered()`, sem TTY). Contra-braço com o valor inicial vazio: o cursor fica em "greek" e os dois reprovam. O defeito foi achado pelo arquiteto ao gravar o GIF num terminal real; os textos em português do assistente ficam com a REQ de idioma.
**Files affected:** `internal/commands/identity_wizard.go` e o teste dele em `internal/commands/`
**Actions:**
1. O seletor abre em `neutral`.
2. Teste que afirma o valor inicial, com contra-braço.
**Acceptance criteria:**
- [x] `go test ./internal/commands/ -count=1` verde
- [x] contra-braço: com o valor inicial vazio, o teste reprova

## Wave 2 — README e GIF (arquiteto)
> Dependencies: Wave 1 auditada. O GIF precisa do binário com o AC1 e o AC3.

### ML-2A — Primeira tela do README, Quick start e `demo.tape` (trackfw_architect) — AC5, AC6
**Status:** ✅ Concluído — 2026-11-02, pelo arquiteto.
- **README:** a primeira tela tem três pontos, o GIF e o Quick start com a mesma sequência de `TestQuickStartEndToEnd`; a ressalva do Windows virou uma linha com link. O antigo Quick start virou "Quick start, continued" (`status`, `ship`, `serve`). A seção "Demo" antiga, com a transcrição de um assistente de junho, foi removida.
- **Afirmações ajustadas ao produto:** o guard bloqueia `git commit`/`git push`/criação de branch brutos e **sinaliza** credenciais (o README diz "detection, not prevention"); nada de "stop leaking".
- **GIF:** a URL é absoluta (`raw.githubusercontent.com`), para renderizar no PyPI e no npm, que usam o README. O `demo.tape` é portátil (sem caminho da máquina, `HOME` isolado em `mktemp -d`) e o `init` roda fora de câmera. Os assistentes de `adr new` e `req new` são respondidos em cena.
- **Auditoria visual em quadros extraídos com `ffmpeg`:** REQ criada com `--adr`, `✓ No violations found.`, guard com `exit=2` e mensagem em inglês, e `status` com 1 ADR, 1 REQ e 1 roadmap em `wip`.
- **Achados ao gravar num terminal real:** AC4b (cursor de identidade em "greek"), corrigido no ML-1C; os textos em português do assistente do `init` e do `req new` ("Escopo dos ADRs desta REQ", "Próximo passo") ficam com a REQ de idioma.
**Files affected:** `README.md`, `demo.tape`, `docs/demo.gif`
**Actions:**
1. Reescrever a primeira tela e o Quick start do README; o Quick start traz a mesma sequência do teste do ML-1B.
2. Reescrever o `demo.tape` e gerar o `docs/demo.gif` com `vhs` e o binário da branch.
3. Auditoria visual em quadros extraídos com `ffmpeg`.
**Acceptance criteria:**
- [x] quadros conferidos: guard barrando `git push` com a mensagem em inglês; `validate` sem violação; nenhum `Usage:` e nenhum caminho absoluto
- [x] Quick start do README idêntico à sequência do teste

**Gates da wave:**
```bash
test -f docs/demo.gif
grep -q 'req new' README.md
```

## Wave 3 — Red team (hades-tf)
> Dependencies: Wave 2 concluída.

### ML-3A — Red team (hades-tf)
**Status:** ✅ Concluído — auditado em 2026-11-02. T1–T10 fechados; T11 e P7 parciais no Windows. Nenhum CRITICAL ou HIGH. O MEDIUM (o README na `main` descreve o `--adr` antes de ele estar no binário publicado) se resolve com o bump para 11.3.0 neste PR e a tag logo depois do merge. Os LOW 2, 5, 6, 7 e 8 viram o ML-3B. Ficam como residual R4, o espaço no nome do ADR, o CRLF, o modo warn do credential e o write-scope sem lease, todos já documentados.
**Files affected:** `docs/seguranca/2026-11-02-redteam-primeira-experiencia.md`
**Actions:**
1. Reimplementar, a partir da leitura, os alvos de falsificação da Wave 0 contra o diff.
2. Seguir o README novo num repositório limpo, como usuário novo.
**Acceptance criteria:**
- [x] Parecer com achados classificados e evidência

**Gates da wave:**
```bash
test -f docs/seguranca/2026-11-02-redteam-primeira-experiencia.md
```

### ML-3B — Correções do red team (apolo-tf)
**Status:** ✅ Concluído — auditado em 2026-11-02. Item 5: a mensagem passa a orientar a edição do vínculo, sem mencionar remoção; as duas asserções reprovam no contra-braço. Item 8: `homedir.Dir()`, com o teste `TestADRPortablePath_GlobalADR_UsesHOMEFirstResolution`, que no macOS não discrimina (os dois leem `$HOME`); residual declarado, porque a diferença só aparece no Windows. Item 6: `USERPROFILE` isolado no Windows, sem contra-braço possível no macOS. Item 2: nome do ADR pela data e 0 warnings; data trocada reprova. Item 7: `TestCredentialAndWriteScopeNoPortugueseStopwords` por `go/ast`; "bloqueado" num literal reprova. O roadmap do teste do Quick start ainda é localizado por glob (residual menor).
**Files affected:** `internal/generators/req.go`, `internal/commands/req.go`, `internal/commands/req_test.go`, `internal/commands/quickstart_e2e_test.go`, `internal/guard/messages_test.go`
**Actions:**
1. Item 5: a mensagem do AC3b com `--adr` conflitante não manda remover o arquivo; orienta a editar o `adr:` e a linha `ADR:` da REQ existente. Teste fixa o texto.
2. Item 8: `adrPortablePath` usa `homedir.Dir()` em vez de `os.UserHomeDir()`, com teste próprio de HOME divergente.
3. Item 6: `quickstartSetup` isola também o `USERPROFILE` no Windows.
4. Item 2: o teste do Quick start calcula o nome do ADR pela data e afirma 0 warnings.
5. Item 7: o teste de stopwords em português cobre os literais emitidos por `credential.go` e `writescope.go`.
**Acceptance criteria:**
- [x] `go test ./internal/commands/ ./internal/generators/ ./internal/guard/ -count=1` verde
- [x] contra-braço por item, medido e restaurado

**Gates da wave:**
```bash
go build ./...
go test ./internal/commands/ -count=1
go test ./internal/guard/ -count=1
```

### ML-3C — Release 11.3.0 no mesmo PR (trackfw_architect)
**Status:** ✅ Concluído — 2026-11-02. Atende o MEDIUM do red team: o README novo ensina o `--adr`, então o binário publicado precisa trazê-lo junto. A tag sai logo depois do merge. `make check-required-full`: D\R, R\W e D\W vazios.
**Files affected:** `CHANGELOG.md`, `docs/visao-projeto/VISION.md`, `internal/version/version.go`, `npm/package-lock.json`, `npm/package.json`, `pypi/pyproject.toml`
**Actions:**
1. Bump para 11.3.0 nos arquivos de versão e seção `[11.3.0]` no CHANGELOG.
**Acceptance criteria:**
- [x] `go run ./cmd/trackfw --version` responde `trackfw 11.3.0`
- [x] `make check-required-full` sem divergência
