---
status: wip
date: 2026-08-15
req: "docs/req/REQ-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados.md"
squad: ""
---

# Roadmap: trackfw validate deve detectar scripts de hook ausentes ou desatualizados

> Created: 2026-08-15 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados.md -->
REQ: docs/req/REQ-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] `trackfw validate` detecta `trackfw-git-branch-guard.sh` ausente/não-executável
      (mesma cobertura que `trackfw-credential-guard.sh` já tem via
      `credential_guard_hook_resolvable`).
- [ ] `trackfw validate` detecta `trackfw-git-branch-guard.sh` desatualizado (conteúdo
      diverge do que a versão atual do binário geraria), mesma cobertura que
      `credential_guard_script_integrity` já tem para credential-guard.
- [ ] `make quality` passa sem novas divergências de paridade.

## Diagnóstico / Contexto
Confirmado por teste direto do binário nesta sessão (2 experimentos, ver REQ vinculada
para o detalhe completo):
1. Remover `scripts/trackfw-credential-guard.sh` → `trackfw validate` silencioso. **Não é
   bug** — este repo usa credential-guard em escopo global, sem entrada de projeto para
   checar.
2. Remover `scripts/trackfw-git-branch-guard.sh` (que TEM entrada de projeto registrada
   em `.claude/settings.json`) → `trackfw validate` também silencioso. **Gap real** —
   `credential_guard_hook_resolvable` (`internal/validator/validator_credential_guard.go`)
   tem o marker de script hardcoded só para `trackfw-credential-guard.sh`, nunca
   examinando entradas de `trackfw-git-branch-guard.sh`.

Duas regras existentes a generalizar/espelhar:
- `credential_guard_hook_resolvable` (existência/executabilidade) —
  `internal/validator/validator_credential_guard.go`.
- `credential_guard_script_integrity` (conteúdo bate com o template do binário atual) —
  `internal/validator/validator_credential_guard_integrity.go`.

Ambas seguem o padrão de mensagem "... run `trackfw update` to regenerate it" — pedido
explícito do usuário: preservar esse padrão para git-branch-guard também, para que
qualquer gap detectado sempre instrua o comando de correção, nunca só reporte o problema.

## Wave 1 — Go (implementação de referência, 1 ML)
> Dependências: nenhuma

### ML-1A — Generalizar `credential_guard_hook_resolvable` + `credential_guard_script_integrity` para git-branch-guard
**Status:** ✅ Concluído
**Arquivos afetados:**
- `internal/validator/validator_credential_guard.go` (ou arquivo novo
  `validator_git_branch_guard.go`, espelhando o padrão — decisão do ML: preferir
  generalizar a função existente para aceitar uma lista de
  `{marker string, hookFiles []credentialGuardHookFile}` em vez de duplicar 100% da
  lógica, já que a resolução de caminho por CLI — `$CLAUDE_PROJECT_DIR/`,
  `$GEMINI_PROJECT_DIR/`, prefixo Codex, caminho relativo — é idêntica para os 2
  scripts)
- `internal/validator/validator_credential_guard_integrity.go` (mesma decisão de
  generalizar vs. espelhar)
- `internal/validator/validator.go` (registrar as regras — nomes sugeridos:
  `git_branch_guard_hook_resolvable`, `git_branch_guard_script_integrity` — ou, se
  generalizado, os nomes de regra continuam por-script dentro da mesma função)
- `internal/config/config.go` (se a severidade das novas regras precisar de entrada
  própria em `rules:` — seguir o mesmo padrão de `credential_guard_hook_resolvable`)
- Testes correspondentes
**Ações:**
1. Ler `validator_credential_guard.go` e `validator_credential_guard_integrity.go`
   INTEIROS antes de tocar em qualquer linha — entender `resolveCredentialGuardHookPath`,
   `collectCredentialGuardCommands`, `credentialGuardHookFiles`, e o template de
   comparação de conteúdo usado pela regra de integrity.
2. Generalizar (preferencial) ou duplicar a lógica de `validateCredentialGuardHookResolvable`
   para também escanear por `"trackfw-git-branch-guard.sh"` como marker, usando a MESMA
   lista `credentialGuardHookFiles` (os arquivos de hook de projeto são os mesmos 6,
   independente de qual script é referenciado) e a mesma `resolveCredentialGuardHookPath`
   (não duplicar essa função — reusar).
3. Cobrir escopo GLOBAL de verdade — dois sub-passos, não confundir:
   (a) dedup (já existe, preservar): não reportar ausência de projeto quando o global
   está instalado;
   (b) checagem real do global (gap principal, não existe hoje): `~/.trackfw/scripts/
   trackfw-credential-guard.sh` e `~/.trackfw/scripts/trackfw-git-branch-guard.sh`
   existem, são executáveis e íntegros sempre que algum hook de projeto delega para o
   global. `globalCredentialGuardInstalledClaude()` (`internal/generators/agentfiles.go`)
   hoje só responde "está instalado" para decidir dedup na geração — não valida
   existência/integridade do alvo. Criar a checagem que falta no `validator`, reusando
   a mesma lógica de comparação byte-a-byte da integrity rule, apontada para
   `~/.trackfw/scripts/` em vez de `scripts/` do projeto.
4. Generalizar ou espelhar `validateCredentialGuardScriptIntegrity` para comparar
   `scripts/trackfw-git-branch-guard.sh` no disco contra `gitBranchGuardScript`
   (const já existente em `internal/generators/scaffold.go`) — mesma técnica de
   comparação byte-a-byte já usada para `credentialGuardScript`.
5. Registrar as duas regras novas/estendidas em `internal/validator/validator.go`
   (função `Validate`/`ValidateTagged`), com mensagens de violação seguindo
   exatamente o padrão existente: "... references trackfw-git-branch-guard.sh
   resolved to %q, but the script does not exist — run `trackfw update` to regenerate
   it" (ausência) e equivalente para desatualização/integridade.
6. Severidade: seguir o mesmo mecanismo de `rules:` em `trackfw.yaml` que
   `credential_guard_hook_resolvable`/`credential_guard_script_integrity` já usam — não
   inventar um mecanismo de configuração novo.
**Critérios de aceite:**
- [ ] `go build ./...` sem erros
- [ ] `go test ./internal/validator/...` verde, incluindo: hook de git-branch-guard
      registrado + script ausente → violação; script presente mas 1 byte alterado →
      violação de integridade; script presente e íntegro → silêncio; escopo global
      dedupa igual ao credential-guard
- [ ] `go vet ./...` sem warnings
- [ ] Teste manual: reproduzir os 2 experimentos da REQ (remover
      `scripts/trackfw-git-branch-guard.sh`; depois alterar 1 byte no script existente)
      contra `bin/trackfw validate --json` e confirmar que agora aparece a violação/aviso
**Comandos de validação:** `go build ./... && go test ./internal/validator/... && go vet ./...`

## Wave 2 — Node.js e Python (2 MLs em paralelo — arquivos distintos por stack)
> Dependências: Wave 1 completa

### ML-2A — Node.js
**Status:** 🔄 Em andamento
**Arquivos afetados:** módulo Node equivalente (localizar via grep por
`credential_guard_hook_resolvable`/`credentialGuardScriptMarker` em `npm/src/`), teste
equivalente
**Ações:** replicar 1:1 a lógica do ML-1A em JS puro, lendo o Go real (já implementado
nesta branch) como fonte de verdade.
**Critérios de aceite:**
- [ ] testes do workspace Node verdes, mesmos casos do ML-1A
- [ ] mensagens de violação idênticas (byte-a-byte) às do Go
**Comandos de validação:** `npm test --workspace=npm` (ajustar nome real do workspace)

### ML-2B — Python
**Status:** 🔄 Em andamento
**Arquivos afetados:** módulo Python equivalente (localizar via grep em `pypi/trackfw/`),
teste equivalente
**Ações:** replicar 1:1 a lógica do ML-1A em Python puro, lendo o Go real como fonte de
verdade.
**Critérios de aceite:**
- [ ] `pytest pypi/tests -k git_branch_guard` verde, mesmos casos do ML-1A
- [ ] mensagens de violação idênticas ao Go
**Comandos de validação:** `python -m pytest pypi/tests -k git_branch_guard`

## Wave 3 — Validação cruzada (1 ML)
> Dependências: Wave 2 completa

### ML-3A — Paridade e teste manual end-to-end
**Status:** ⬜ Pendente
**Arquivos afetados:** nenhum novo
**Ações:**
1. Rodar `make quality` na raiz.
2. Reproduzir os 2 experimentos reais desta sessão contra cada um dos 3 binários
   (Go/Node/Python): remover `scripts/trackfw-git-branch-guard.sh` → deve aparecer
   violação/aviso nos 3; restaurar, alterar 1 byte → deve aparecer violação de
   integridade nos 3; restaurar ao original → silêncio nos 3.
**Critérios de aceite:**
- [ ] `make quality` verde
- [ ] os 2 experimentos confirmados nos 3 CLIs
**Comandos de validação:** `make quality`
