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
**Status:** ⬜ Pendente
**Files affected:** `docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md` (novo; único arquivo que pode ser escrito)
**Actions:**
1. **Mapa CLI → mecanismo de home no Windows**, com evidência por CLI (código-fonte oficial, documentação oficial com URL, ou medição): Claude Code (medido: `USERPROFILE`), Codex, Gemini, Cursor, Copilot, Windsurf, Kiro — cada um com o caminho exato do arquivo global que o `update harness` escreve (`internal/generators/update.go:630-678`). Inclua variáveis de override (ex.: `CLAUDE_CONFIG_DIR`, `CODEX_HOME`).
2. **População completa** dos sítios que leem/escrevem fiação global: grep de `homedir.Dir()` e `os.UserHomeDir()` em `internal/` (produção), classificando cada um como artefato do trackfw (fica em `homedir.Dir()`) ou fiação de CLI (passa ao resolvedor por CLI).
3. **Threat model:** quem esvazia a correção — override de config dir, `USERPROFILE` vazio/ausente, perfil roaming/redirecionado, Git Bash sem `HOME`, CI.
4. **Testes afetados:** quais testes isolam só `HOME` e passariam a escrever no home real do runner (risco de poluir o `USERPROFILE` do CI/dev).
5. **Alvos de falsificação** nas duas direções e **resíduo declarado**.
6. **Veredito** sobre o ADR com ajustes numerados.
**Acceptance criteria:**
- [ ] Seis seções com evidência (arquivo:linha, URL ou comando + saída)
- [ ] Nenhuma linha de implementação; nenhum arquivo além do parecer
- [ ] Nenhum `git commit`/`push` bruto

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md
grep -q "Veredito" docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md
git diff --quiet HEAD -- internal/
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditado e ajustes no ADR. MLs detalhados após o parecer.

### ML-1A — Resolvedor de home por CLI, usado pelo `update harness` e pelo `validate`
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente

### ML-1B — Aviso de divergência e resto inerte
**Owner:** `apolo-tf`
**Status:** ⬜ Pendente

### ML-1C — Medição de CI antes/depois (`windows-probe.yml`), contrato e README
**Owner:** `ares-tf` / `apolo-tf`
**Status:** ⬜ Pendente

## Wave 2 — Revisão e fechamento
### ML-2A — Revisão de qualidade, red-team, `make quality` e CI
**Owner:** `hefesto-tf` e `hades-tf` (em sequência)
**Status:** ⬜ Pendente
**Acceptance criteria:**
- [ ] Pareceres em `docs/qualidade/` e `docs/seguranca/`; corretivos aplicados
- [ ] `make quality` EXIT=0 com máquina ociosa; CI verde, inclusive `windows-full-suites`
