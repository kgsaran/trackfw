---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-09-05-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md"
squad: "hades-tf, ares-tf, prometeu-tf, apolo-tf, artemis-tf, hefesto-tf"
---

# Roadmap: os hooks de guard nao executam no Windows na maioria dos CLIs de agente e o validate reporta instalado

> Created: 2026-09-22 | Reescrito: 2026-10-04 | Status: wip

## Context
REQ: docs/req/REQ-2026-09-05-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md
ADR: docs/adr/ADR-2026-10-04-o-guard-de-hook-e-um-subcomando-go-do-trackfw-e-a-linha-de-hook-e-a-mesma-string-em-todo-shell.md
(emenda a ADR-2026-09-05: substitui D6/D7)

**Desenho:** os dois guards de segurança viram `trackfw guard git-branch` e `trackfw guard credential`,
em Go. Toda config de hook emitida usa essa string, sem variável, aspas ou caminho, e ela é
idêntica em `sh`, Git Bash, PowerShell e `cmd.exe`. Por isso não existe escolha entre `.sh` e `.ps1`:
as configs são versionadas e servem ao time misto. Os `.sh` viram invólucro
`exec trackfw guard <nome> "$@"`.

**Fatos medidos em 2026-10-04 que o roadmap anterior não tinha:**
- `scripts/trackfw-git-branch-guard.sh`: 756 linhas, e não 561; `scripts/trackfw-credential-guard.sh`: 152.
- O conteúdo do guard vem da constante `gitBranchGuardScript` (`internal/generators/scaffold.go`),
  usada por `GenerateGitBranchGuardScript`, `GenerateGlobalGitBranchGuardScript` e
  `scaffold_doctor.go:282`; a ADR-2026-10-02 D4 fala em 4 cópias.
- Emissão em `internal/generators/agentfiles.go`: `Inject{Claude,Codex,Gemini,Kiro,Copilot,Cursor,Windsurf,AmazonQ}Hooks`,
  ou seja, **8 CLIs**, e não 6.
- Regras do validate envolvidas: `credential_guard_hook_resolvable`, `credential_guard_script_integrity`,
  `credential_guard_mode_downgrade`, `git_branch_guard_hook_resolvable`, `git_branch_guard_script_integrity`.
- Os guards já chamam `trackfw` (44 e 8 ocorrências) e o git-branch usa `jq`, com fallback `awk` (ADR-2026-10-02).

**Fora deste roadmap:** a REQ-2026-09-01 (`$HOME` ≠ `%USERPROFILE%`), que tem outra causa (ver o escopo
negativo da REQ), a jornada de instalação no Windows e os `attention-*`.

## Acceptance Criteria
- [ ] AC1–AC9 da REQ, cada um fechado pelo ML que o cita
- [ ] `make quality` EXIT=0 com a máquina ociosa, no fim
- [ ] Medição na VM Windows: o guard dispara e bloqueia em PowerShell, `cmd.exe` e Git Bash

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Medição e threat model
> Dependências: nenhuma. Bloqueia toda implementação.
> Ordem: ML-0B ∥ ML-0C (arquivos e máquinas disjuntos) → ML-0A (o threat model consome as duas medições).

### ML-0B — Medição na VM: o `trackfw` no PATH de cada shell, por canal
**Status:** ✅ Concluído
**Squad:** ares-tf
**Files affected:** `docs/portabilidade/2026-10-04-trackfw-no-path-dos-shells-do-windows-por-canal.md` (único arquivo escrito no repositório)
**Actions:**
1. **Na VM Windows** (UTM; acesso via `utmctl exec` / sshd, como em `docs/portabilidade/`), com
   `Get-ExecutionPolicy` = `Restricted` (registre o valor), instale o trackfw por **cada canal**, um de
   cada vez: `npm i -g trackfw@9.2.0`, `pip install trackfw==9.2.0` e o `.exe` do release v9.2.0 do GitHub.
2. Para cada canal, rode `trackfw --version` e registre stdout, stderr e exit code em:
   `powershell -NoProfile -Command "trackfw --version"`, `pwsh -NoProfile -Command …` (se instalado),
   `cmd /c trackfw --version`, Git Bash `bash -lc "trackfw --version"`. Registre também qual arquivo
   cada shell resolve (`Get-Command trackfw | fl` e `where trackfw`). 🔴 A pergunta decisiva é se o
   PowerShell resolve o shim `trackfw.ps1` do npm e se a `ExecutionPolicy` o bloqueia.
3. **Falha aberta por binário velho:** com o 9.2.0, que não tem `guard`, rode
   `echo '{}' | trackfw guard git-branch` nos 4 shells e registre o exit code.
4. **Git Bash e `jq`:** registre se o Git for Windows instalado traz `jq` (`command -v jq`) e o que o
   `trackfw-git-branch-guard.sh` atual faz com `{"tool_input":{"command":"git push origin main"}}`
   no stdin, com e sem `jq` (exit code e stderr).
5. **Custo de startup:** mediana de 20 execuções de `trackfw --version` por canal, no PowerShell.
6. **Controle POSIX (linha de base do AC5), no macOS:**
   `go test ./internal/generators/ -run 'Guard|Credential' -count=1 2>&1 | tail -3` e
   `go test ./internal/commands/ -count=1 2>&1 | tail -3`; registre a saída literal e o commit (`git rev-parse HEAD`).
**Acceptance criteria:**
- [x] Tabela canal × shell com exit code e arquivo resolvido, medida e não inferida
- [x] Veredito explícito: "o shim do npm sob Restricted no PowerShell bloqueia: sim/não"
- [x] Exit code do binário velho, por shell
- [x] Nenhum arquivo do repositório alterado além do documento

### ML-0C — Remedição dos schemas de hook dos CLIs de agente
**Status:** ✅ Concluído
**Squad:** prometeu-tf
**Files affected:** `docs/portabilidade/2026-10-04-remedicao-do-schema-de-hook-dos-clis-de-agente.md` (único arquivo)
**Actions:**
1. Para cada um dos 8 CLIs emitidos por `internal/generators/agentfiles.go` (Claude Code, Codex,
   Gemini, Kiro, Copilot, Cursor, Windsurf, Amazon Q), releia a **documentação oficial ou o código-fonte
   atual** (com URL e data) e responda: com que shell o `command` do hook roda no Windows, se existe
   campo por plataforma ou por shell (ex.: o Copilot tem `bash`/`powershell`/`command`; o Claude Code
   tem algum campo `shell`?), e qual exit code bloqueia a ferramenta.
2. Compare com `docs/portabilidade/2026-09-05-contrato-de-execucao-de-hook-por-cli-de-agente-no-windows.md`
   e o adendo da ADR-2026-09-05: o que mudou, linha a linha.
3. Responda se a string nua `trackfw guard git-branch`, sem caminho nem variável, é aceita como
   `command` por cada CLI, e qual é o cwd do processo do hook. O guard precisa achar `trackfw.yaml`.
**Acceptance criteria:**
- [x] 8 linhas na tabela, cada uma com fonte e data
- [x] As diferenças em relação a 2026-09-05 explicitadas, ou "sem mudança" com fonte
- [x] Nenhum arquivo além do documento

### ML-0A — Threat model do guard em Go
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-04-wave0-guard-em-go.md` (único arquivo)
**Actions:** (lê os documentos de ML-0B e ML-0C, a ADR-2026-10-04 e a ADR-2026-10-02)
1. **Completude:** enumere **todo** sítio que emite, copia ou referencia os guards: grep por
   `trackfw-git-branch-guard`, `trackfw-credential-guard`, `gitBranchGuardScript` e
   `credentialGuardScript` em `internal/` (exceto testdata de corpus), `scripts/`, `npm/`, `pypi/`,
   `plugins/` e templates. Feche a lista ou diga o que falta.
2. **Threat model** da troca de confiança da D5 da ADR: `trackfw` resolvido pelo PATH (sequestro de
   PATH, shim do npm), binário sem o subcomando (falha aberta, inclusive **neste repositório** durante
   o desenvolvimento, já que o binário instalado é o 9.2.0), `trackfw` fora do PATH do processo do
   CLI de agente, cwd diferente da raiz, e a perda da `*_script_integrity` como proteção do caminho
   padrão. Para cada um: falha aberta ou fechada, e mitigação proposta.
3. **Alvos de falsificação nas duas direções**, por superfície: guard Go, invólucro `.sh`, emissão por
   CLI, migração no `update` e o relato do `validate`.
4. **Resíduo declarado.**
**Acceptance criteria:**
- [x] As quatro seções com evidência (comando e saída)
- [x] Lista de sítios fechada, com o veredito por sítio
- [x] Um veredito por risco da D5: "bloqueia o desenho" ou "mitigação X, entra no ML Y"

      ✅ Parecer: nenhum risco bloqueia o desenho. O arquiteto confirmou na VM o achado (b-1): `powershell -Command` transforma o exit 2 em 1 (PS 5.1.26100), e o sufixo `; exit $LASTEXITCODE` preserva o 2 em PS, sh, bash e Git Bash. Decisões no adendo da ADR-2026-10-04 (D2 revista, D7–D9).

**Gates da wave:**
```bash
test -s docs/portabilidade/2026-10-04-trackfw-no-path-dos-shells-do-windows-por-canal.md
test -s docs/portabilidade/2026-10-04-remedicao-do-schema-de-hook-dos-clis-de-agente.md
test -s docs/seguranca/2026-10-04-wave0-guard-em-go.md
```

> 🔴 **Barreira:** se o ML-0B mostrar que o shim do npm é bloqueado e o ML-0A não achar mitigação, a
> Wave 1 não é liberada: a decisão volta ao KG. As Waves 1–3 abaixo serão ajustadas com os achados da
> Wave 0 antes do despacho.

## Wave 1 — O guard em Go
> Dependências: Wave 0 auditada.
> Ordem: ML-1D ∥ ML-1A (VM × código, disjuntos) → ML-1B → ML-1C. Entre 1A, 1B e 1C, em sequência. 1A e 1B compartilham `internal/commands/guard.go` e o
> parser de payload; 1C exercita os dois.

### ML-1D — Medições residuais na VM (antes do ML-2A; em paralelo ao ML-1A)
**Status:** ✅ Concluído
**Squad:** ares-tf
**Files affected:** `docs/portabilidade/2026-10-04-trackfw-no-path-dos-shells-do-windows-por-canal.md` (seção nova "ML-1D")
**Actions:** (1) Mark-of-the-Web do shim do npm: `Get-Item "$(npm prefix -g)\trackfw.ps1" -Stream Zone.Identifier`; e, sob `Set-ExecutionPolicy RemoteSigned -Scope Process` com **cmd como pai**, o `trackfw.ps1` roda e o sufixo `; exit $LASTEXITCODE` devolve 2 com um `.exe` que sai 2? (2) O que um `.exe` recebe em `cmd /c "probe.exe git-branch; exit $LASTEXITCODE"` (argv literal). (3) Stdin vindo do PowerShell (`'payload' | probe.exe`): há BOM UTF-8 ou UTF-16? O pipe fecha dentro de 2 s?
**Acceptance criteria:**
- [x] Os três itens com o comando e a saída literal; `%ERRORLEVEL%` lido em arquivo `.cmd`, linha a linha (na mesma linha ele expande antes de executar)

      ✅ (1) O shim do npm não tem MoTW; sob `RemoteSigned` roda e devolve 2. 🔴 Sob `Restricted` **com** o sufixo, sai **0** (a PSSecurityException não atualiza `$LASTEXITCODE`): falha aberta também no Copilot, que sem o sufixo negaria. Vai para o ML-2B. (2) No `cmd`, o binário recebe `["git-branch;","exit","$LASTEXITCODE"]`: o sufixo fica proibido para Kiro e Amazon Q, como previsto. (3) A forma do Cursor entrega o stdin com BOM UTF-8 (`EF BB BF`): o parser precisa descartá-lo, senão nega todo comando do Cursor. Correção autorizada no ML-1B (`payload.go`), não é melhoria.

### ML-1A — `trackfw guard git-branch`
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/guard/` (pacote novo: `payload.go`, `gitbranch.go` e testes),
`internal/commands/guard.go` (novo), registro em `internal/commands/root.go`
**Actions:** (contrato completo: adendo da ADR-2026-10-04, D7–D9, e `docs/seguranca/2026-10-04-wave0-guard-em-go.md` §2(e)(f) e §3)
portar `scripts/trackfw-git-branch-guard.sh`, ou seja, a constante `gitBranchGuardScript`,
**com comportamento igual**: leitura do stdin com orçamento, extração do comando nas chaves e na
prioridade da ADR-2026-10-02 (falha fechada em chave indecodificável e em NUL, chave duplicada: vence
a última; leitura por `map[string]json.RawMessage`), dreno em janelas de 2 s, subida até
`trackfw.yaml`, `--command` no lugar de argv, todo erro sob `guard` → exit 2 (D7), no-op fora de projeto, as mesmas mensagens `REASON`, exit 2 + stderr e a mesma saída
`hookSpecificOutput`. Nenhuma evasão declarada no cabeçalho passa a ser fechada. Sem `jq`, sem `awk`:
`encoding/json`.
**Acceptance criteria:**
- [x] `go build ./...` sem erro
- [x] `go test ./internal/guard/... -count=1` verde, com testes que nomeiam cada regra do `.sh`
- [x] Os três gates da Wave 0: (i) subcomando/flag inválido → exit 2; (ii) `"Command"` não sobrescreve `"command"`; (iii) `(payload; sleep 3)` libera e `(payload; sleep 6)` nega
- [x] Relatório com a frase por teste novo (Regra Dura de Reconciliação)

      ✅ Auditoria: a primeira entrega reprovou. O gate (iii) divergia (`sleep 3`: sh rc=0, Go rc=2), porque o dreno reiniciava o prazo a cada leitura, e o teste que dizia cobrir o caso usava EOF imediato. Um corretivo trocou o dreno para janela fixa, com uma goroutine só, e 4 dos 7 testes de tempo passaram a reprovar no código antigo. Remedido pelo arquiteto com o binário: `sleep 3` → 0 e `sleep 6` → 2, igual ao sh; `go test -race ./internal/guard/... -count=3` verde. Nota no vault: `powershell-command-converte-exit-2-e-bash-read-t-e-janela-fixa-2026-10-04`.

### ML-1B — `trackfw guard credential`
**Status:** ✅ Concluído
**Squad:** apolo-tf
**Files affected:** `internal/guard/credential.go` (+ teste), `internal/commands/guard.go`
**Actions:** (a) em `internal/guard/payload.go`, descartar um BOM UTF-8 inicial (`EF BB BF`) do stdin antes de decodificar, nos dois guards, com teste (ML-1D, item 3); (b) portar `scripts/trackfw-credential-guard.sh` com comportamento igual (cwd only, sem subida; lê o stdin inteiro **antes** de olhar o projeto; sem timeout, resíduo declarado na D9): padrões JWT/AWS,
isenção de destino efêmero, `credential_guard.mode` `warn`/`block` do `trackfw.yaml`, o arquivo
`.trackfw-credential-guard.json` em `roadmap_dir` com a mesma normalização de caminho e CRLF.
**Acceptance criteria:**
- [x] `go build ./...` e `go test ./internal/guard/... -count=1` verdes
- [x] Frase por teste novo

      ✅ Auditoria: a primeira entrega reprovou. Com `cat *.txt`, o sh dava rc=2 e o Go rc=0 (falha aberta): o `set -- $CMD_LINE` do .sh expande glob, e o Go não. O corretivo portou o glob e a variante global como `--global` (default block, sem checagem de projeto, `docs/roadmaps` fixo e sem mkdir), decisão do arquiteto. Remedido pelo arquiteto com o binário: o glob dá 2 nos dois lados; o global com JWT, fora de projeto, também dá 2 nos dois lados; `-race` verde.

### ML-1C — Paridade `.sh` ↔ Go pelo corpus existente (AC4)
**Status:** ✅ Concluído
**Squad:** artemis-tf
**Files affected:** `internal/generators/git_branch_guard_test.go`, `credential_guard_test.go`,
`git_branch_guard_dedup_test.go`, `git_branch_guard_stdin_drain_test.go` (só testes)
**Actions:** primeiro, congelar o `.sh` atual (756 linhas) e o credential como fixtures em
`internal/generators/testdata/guard-sh-reference/`, com o sha256 afirmado num teste: depois do ML-2A o
`.sh` vira invólucro, e sem a fixture a paridade compararia Go com Go. Depois, fazer os cenários que hoje executam `bash <script>` rodarem também contra o binário
`trackfw guard <nome>` compilado no teste, comparando exit code, stderr e stdout. Divergência
reprova. Contar e reportar quantos cenários rodam nos dois braços.
**Acceptance criteria:**
- [x] Os dois braços rodam em todo cenário — 244 chamadas de `assertGuardParity` no pacote (contador temporário, medido pelo apolo-tf; o modo com/sem jq conta 2 por subteste). Número não reconferido de forma independente pelo arquiteto.
- [x] Falsificação: trocar `reasonPush` reprova C01; trocar `reasonNUL` reprova C22 (a normalização do C22 não engole o caso)
- [x] `go test ./internal/generators/ -count=1` verde — medido pelo arquiteto em 2026-10-05: `ok … 61.020s`; `go test ./internal/guard/...` ok

      ✅ Fechado em 2026-10-05 (dois corretivos do apolo-tf):
      **A**: `quoteAwareSplit` trata LF fora de aspas como separador de segmento; `git push` na segunda linha bloqueia (teste `TestMatchSubcommand_MultilineNewlineBlocksSecondSegment`).
      **B**: valor não-string em `command` é tratado como ausente (`payload.go`), igual ao awk.
      **C**: linhas diagnósticas do awk removidas só do stderr do bash. C22: com jq, o próprio `.sh` devolve a REASON de NUL, igual ao Go. Sem jq, o awk do `.sh` diverge só no texto (rc=2 nos dois). Normalização estreita acionada só pelo `nul_in_value` (2 acionamentos, ambos C22).

**Gates da wave:**
```bash
go build ./...
go test ./internal/guard/... -count=1
go test ./internal/generators/ -count=1
```

## Wave 2 — Emissão, migração e relato
> Dependências: Wave 1 auditada. ML-2A ∥ ML-2B (arquivos disjuntos; o contrato entre eles é a string
> exata `trackfw guard git-branch` / `trackfw guard credential`).

### ML-2A — Toda config emite `trackfw guard <nome>`; `.sh` vira invólucro; `update` migra
**Status:** ⬜ Pendente
**Squad:** apolo-tf
**Files affected:** `internal/generators/agentfiles.go`, `internal/generators/scaffold.go`,
`internal/generators/update.go`, `internal/generators/scaffold_doctor.go`, `scripts/trackfw-git-branch-guard.sh`,
`scripts/trackfw-credential-guard.sh`, `docs/cli-parity.md` (+ testes de generators), e os sítios a mais que a Wave 0 enumerar
**Pré-condição (gate):** `trackfw guard --help` sai 0 com o binário do PATH (`make install` da branch) e o ML-1D está auditado.
**Actions:** os 8 `Inject*Hooks` e os caminhos globais emitem a linha da D2 revista
(`trackfw guard <nome>; exit $LASTEXITCODE` para Claude, Codex, Gemini, Cursor, Copilot e Windsurf; `trackfw guard <nome>` para Kiro e Amazon Q);
atualizar `scripts/check-git-branch-guard-hook-schema.sh` para exercitar o binário; migrar as configs deste
repositório (`.claude/settings.json`, `.codex/hooks.json`, `.gemini/settings.json`); o Copilot passa a usar o
campo `command` (AC1); `migrateHookCommand` troca as formas antigas pela nova no `trackfw update`;
`gitBranchGuardScript` e a constante do credential viram `#!/usr/bin/env bash` +
`exec trackfw guard <nome> "$@"`.
**Acceptance criteria:**
- [ ] Teste por CLI afirmando a string emitida; teste de schema do Copilot (AC1, nas duas direções)
- [ ] `update` sobre uma config antiga produz a nova (teste)
- [ ] `go test ./internal/generators/ -count=1` verde

### ML-2B — `validate` relata se o hook pode executar (AC7)
**Status:** 🔄 Em andamento
**Squad:** apolo-tf
**Files affected:** `internal/validator/validator_credential_guard*.go`, `internal/validator/validator_git_branch_guard*.go` (+ testes)
**Actions:** `*_hook_resolvable` compara a linha **exata** da D2 revista (hoje é `strings.Contains`, que aceitaria qualquer sufixo); versão mínima do `trackfw` resolvido; no Windows, `trackfw` resolvido para `.ps1` com a política efetiva `Restricted` é violation com orientação `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned` (ML-1D: com o sufixo, sai 0 = falha aberta); `trackfw.exe`/`trackfw.cmd`/`trackfw.bat` na raiz do projeto é violation (o `cmd.exe` procura no cwd antes do PATH); uma config que ainda aponta para o `.sh`
recebe um aviso de que não executa no Windows fora do Git Bash; `trackfw` resolvido sem o subcomando
`guard` vira violation. A mitigação do binário velho segue o que a Wave 0 decidir.
**Decisão do arquiteto (2026-10-05):** a "versão mínima" da ADR é atendida pela sonda do subcomando
(`<trackfw resolvido> guard --help` sai 0). O `guard` estreia numa única versão, então todo binário que o
tem já atende o mínimo, e uma constante de versão reprovaria os builds de desenvolvimento (que reportam 9.2.0).
**Acceptance criteria:**
- [ ] Falsificação nas duas direções por regra
- [ ] `go test ./internal/validator/ -count=1` verde

**Gates da wave:**
```bash
go build ./...
go test ./internal/generators/ ./internal/validator/ -count=1
```

## Wave 3 — Prova no Windows e no POSIX
> Dependências: Wave 2 auditada.

### ML-3A — Guard disparando e bloqueando na VM (AC3) e controle POSIX (AC5)
**Status:** ⬜ Pendente
**Squad:** ares-tf
**Files affected:** `docs/portabilidade/2026-10-04-trackfw-no-path-dos-shells-do-windows-por-canal.md` (seção nova), `vault/notes/` (nota, se a causa for não óbvia)
**Actions:** com o binário da branch na VM: `git push` bruto bloqueado (exit 2) e comando inofensivo
liberado, em `powershell -NoProfile -Command`, `pwsh`, `cmd /c` e Git Bash; e pelo menos em um CLI
de agente de PowerShell e no Kiro, se estiverem instalados na VM. No macOS: repetir a linha de base do
ML-0B e comparar.
**Acceptance criteria:**
- [ ] Matriz shell × (bloqueia / libera) com saída literal
- [ ] Linha de base POSIX idêntica, ou diferença explicada pela mudança

### ML-3B — README e docs do usuário
**Status:** ⬜ Pendente
**Squad:** prometeu-tf
**Files affected:** `README.md` (seção de hooks/Windows)
**Actions:** trocar "planejado" por estado medido, por CLI, conforme a matriz do ML-3A.
**Acceptance criteria:**
- [ ] Cada afirmação aponta para uma linha da matriz

## Wave 4 — Red team e qualidade
> Dependências: Wave 3 auditada. ML-4A ∥ ML-4B (somente leitura + parecer).

### ML-4A — Red team do diff
**Status:** ⬜ Pendente
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-04-red-team-guard-em-go.md`
**Acceptance criteria:**
- [ ] Cada risco da Wave 0 reconfirmado contra o código entregue

### ML-4B — Revisão de qualidade
**Status:** ⬜ Pendente
**Squad:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-04-guard-em-go.md`
**Acceptance criteria:**
- [ ] Parecer sobre duplicação remanescente entre `.sh` e Go, e sobre a cobertura do pacote `internal/guard`

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-04-red-team-guard-em-go.md
make quality
```
