---
status: wip
date: 2026-09-22
req: "docs/req/REQ-2026-09-01-trackfw-escreve-e-audita-guard-global-em-caminho-que-o-agente-real-nunca-le-instalacao-fantasma-sob-home-diferente-de-userprofile.md"
squad: "hades-tf, apolo-tf, hefesto-tf"
---

# Roadmap: `trackfw` escreve e audita guard global em caminho que o agente real nunca lê — instalação fantasma sob `$HOME` ≠ `%USERPROFILE%`

> Created: 2026-09-22 | Status: wip

## Context
REQ: docs/req/REQ-2026-09-01-trackfw-escreve-e-audita-guard-global-em-caminho-que-o-agente-real-nunca-le-instalacao-fantasma-sob-home-diferente-de-userprofile.md
ADR: docs/adr/ADR-2026-10-14-no-windows-o-guard-global-de-cada-cli-de-agente-e-gravado-e-auditado-no-home-que-aquele-cli-le.md
Medição: docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md

## Acceptance Criteria
- [ ] AC0 — Wave 0 auditado (mapa CLI → home no Windows, por evidência)
- [ ] AC2–AC5 — CI reproduz e fecha; resolvedor único por CLI; aviso de divergência; falsificação sem depender de Windows
- [ ] AC6–AC8 — contrato/README, reconciliação dos testes, `make quality` + CI

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Mapa de home por CLI e threat model
**Owner:** `hades-tf`
**Status:** ✅ Concluído — auditado em 2026-10-14 · aprova com ressalvas; A1–A8 no ADR D6 (A1: `CLAUDE_CONFIG_DIR` entra; A8: Windsurf resíduo)
**Files affected:** `docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md` (novo; único arquivo que pode ser escrito)
**Actions:**
1. **Mapa CLI → mecanismo de home no Windows**, com evidência por CLI (código-fonte oficial, documentação oficial com URL, ou medição): Claude Code (medido: `USERPROFILE`), Codex, Gemini, Cursor, Copilot, Windsurf, Kiro — cada um com o caminho exato do arquivo global que o `update harness` escreve (`internal/generators/update.go:630-678`). Inclua variáveis de override (ex.: `CLAUDE_CONFIG_DIR`, `CODEX_HOME`).
2. **População completa** dos sítios que leem/escrevem fiação global: grep de `homedir.Dir()` e `os.UserHomeDir()` em `internal/` (produção), classificando cada um como artefato do trackfw (fica em `homedir.Dir()`) ou fiação de CLI (passa ao resolvedor por CLI).
3. **Threat model:** quem esvazia a correção — override de config dir, `USERPROFILE` vazio/ausente, perfil roaming/redirecionado, Git Bash sem `HOME`, CI.
4. **Testes afetados:** quais testes isolam só `HOME` e passariam a escrever no home real do runner (risco de poluir o `USERPROFILE` do CI/dev).
5. **Alvos de falsificação** nas duas direções e **resíduo declarado**.
6. **Veredito** sobre o ADR com ajustes numerados.
**Acceptance criteria:**
- [x] Seis seções com evidência (arquivo:linha, URL ou comando + saída)
      ✅ `docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md`; Windsurf divergente e `os.UserHomeDir()` só-`USERPROFILE` conferidos pelo arquiteto
- [x] Nenhuma linha de implementação; nenhum arquivo além do parecer
      ✅ `git diff --quiet HEAD -- internal/` EXIT=0
- [x] Nenhum `git commit`/`push` bruto

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md
grep -q "Veredito" docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md
git diff --quiet HEAD -- internal/
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditado (✅). ML-1A → ML-1B sequenciais (mesmos arquivos); ML-1C depois.

### ML-1A — Pacote `internal/agentconfig`: resolvedor por CLI usado pelo escritor e pelo auditor
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Files affected:** novo `internal/agentconfig/` (+ testes); os 8 sítios do ADR D6/A2; testes que isolam só `HOME` (tabela da seção 4 do parecer); `scripts/check-update-parity.sh` (definir `USERPROFILE`)
**Acceptance criteria:**
- [ ] Claude Code: `CLAUDE_CONFIG_DIR` (raiz) → senão, no Windows, `os.UserHomeDir()` → senão `homedir.Dir()`; os demais CLIs inalterados (D2), Windsurf intocado (A8)
- [ ] `update harness` e `validate` chamam a MESMA função por CLI (D3); erro de resolução → finding (A7)
- [ ] Resolvedor injetável; teste com `HOME`≠`USERPROFILE` simulado reprova sem a correção **em macOS/Linux** (AC5); teste de `CLAUDE_CONFIG_DIR`
- [ ] Controle: sem divergência e sem `CLAUDE_CONFIG_DIR`, saída idêntica à de hoje
- [ ] `go build ./...`, `go vet`, `GOOS=windows go vet`, testes dos pacotes tocados EXIT=0; `make parity-rest` EXIT=0

### ML-1B — Aviso de divergência (D4) e resto inerte (D5)
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] Predicado A3; aviso informativo nomeia os dois caminhos, CLIs medidos × inferidos (A4) e o Windsurf (A8); silêncio sem divergência
- [ ] `update harness` relata o guard antigo sob `HOME` sem apagá-lo (D5)
- [ ] Testes nas duas direções; mordida mostrada

### ML-1C — Medição de CI antes/depois, contrato e README
**Owner:** `ares-tf` / `apolo-tf`
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] `windows-probe.yml` (ou repro equivalente) com `HOME`≠`USERPROFILE`: `main` reproduz, a branch fecha (AC2)
- [ ] `docs/cli-parity.md` (`trackfw-contract`) e seção Windows do README

## Wave 2 — Revisão e fechamento
### ML-2A — Revisão de qualidade, red-team, `make quality` e CI
**Owner:** `hefesto-tf` e `hades-tf` (em sequência)
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] Pareceres em `docs/qualidade/` e `docs/seguranca/`; corretivos aplicados
- [ ] `make quality` EXIT=0 com máquina ociosa; CI verde, inclusive `windows-full-suites`
