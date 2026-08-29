---
status: done
date: 2026-08-07
req: "docs/req/REQ-2026-08-07-atualiza-actions-do-github-para-versoes-com-node24-nativo.md"
squad: ""
---

# Roadmap: atualiza actions do GitHub para versões com node24 nativo

> Created: 2026-08-07 | Status: done

## Context
REQ: `docs/req/REQ-2026-08-07-atualiza-actions-do-github-para-versoes-com-node24-nativo.md`

O workflow de Release da tag `v6.5.0` emitiu avisos de depreciação em todos os jobs:
`actions/checkout@v4`, `actions/setup-go@v5`, `actions/setup-node@v4`, `actions/setup-python@v5` e
`goreleaser/goreleaser-action@v6` ainda declaram runtime `node20`, rodando via fallback forçado para
`node24`. Versões majors mais recentes já confirmadas (WebFetch nas páginas oficiais de release,
2026-08-07) com runtime `node24` nativo:

| Action | Atual | Nova | Runtime confirmado |
|---|---|---|---|
| `actions/checkout` | `@v4` | `@v7` (v7.0.1) | `node24` (confirmado em `action.yml` da main) |
| `actions/setup-go` | `@v5` | `@v7` (v7.0.0) | `node24` (changelog: "Upgrade Nodejs runtime from node20 to node 24" desde v6.0.0) |
| `actions/setup-node` | `@v4` | `@v7` (v7.0.0) | `node24` (migração ESM) |
| `actions/setup-python` | `@v5` | `@v7` (v7.0.0) | `node24` (changelog: migração ESM/node24 desde v6.0.0) |
| `goreleaser/goreleaser-action` | `@v6` | `@v7` (v7.2.3) | `node24` (changelog: "node 24, update deps, rm yarn, ESM") |
| `actions/upload-pages-artifact` | `@v3` | `@v5` | não confirmado explicitamente, bump preventivo |
| `actions/deploy-pages` | `@v4` | `@v5` | `node24` (changelog: "Update Node.js version to 24.x") |

Arquivos afetados: `.github/workflows/quality.yml`, `trackfw-validate.yml`, `trackfw-gate.yml`,
`release.yml`, `deploy-docs.yml`.

## Acceptance Criteria
- [x] Todas as 7 actions listadas atualizadas para a versão nova, em todos os workflows onde aparecem
- [x] `trackfw validate` sem violações novas
- [x] Confirmado via execução real de CI que os workflows rodam sem o aviso de depreciação — PR #145,
      10/10 checks verdes (`go`, `governance` ×3, `node`, `package-smoke`, `parity`, `python 3.10`,
      `python 3.12`, `windows-integrations-resolve`)

## Wave 1 — Bump de versões pinadas (1 ML)
> Dependências: Independente

### ML-1A — Atualizar as 7 actions nos 5 workflows
**Status:** ✅ Concluído

**Nota de auditoria:** confirmado que as 7 actions não têm breaking change relevante ao uso atual do
projeto (checado `action.yml`/changelogs das versões novas contra os inputs/triggers realmente usados
— `go.mod` sem diretiva `toolchain`, `package.json` sem campo `packageManager`, nenhum `pip-install`
usado, nenhum `with:` custom em upload-pages-artifact/deploy-pages). Diff confirmado pelo
orquestrador: 35 inserções/35 deleções, 100% linhas `uses:`, nenhum outro parâmetro tocado. YAML de
todos os 5 workflows validado com `yaml.safe_load`.

**Achado real na primeira execução de CI do PR #145**: `trackfw validate` falhou —
`go: go.mod requires go >= 1.25.2 (running go 1.22.12; GOTOOLCHAIN=local)`. Causa raiz: o bump de
`setup-go@v5` → `@v7` muda o comportamento de `GOTOOLCHAIN` de `auto` para `local` quando
`go-version` é pinado explicitamente (confirmado comparando o log da execução anterior bem-sucedida,
que mostrava `GOTOOLCHAIN='auto'`, com o log da falha). `trackfw-validate.yml` era o único dos 6 usos
de `setup-go` no repo com `go-version: "1.22"` hardcoded e desatualizado (`go.mod` já exige 1.25.2) —
funcionava por acidente via troca automática de toolchain do Go antes do bump. Corrigido alinhando ao
padrão já usado nos outros 5 usos (`go-version-file: go.mod`), que resolve a versão certa
automaticamente e fica imune a essa classe de bug em bumps futuros de `go.mod`. Reconfirmado com
10/10 checks verdes na execução seguinte.
**Arquivos afetados:**
- `.github/workflows/quality.yml`
- `.github/workflows/trackfw-validate.yml`
- `.github/workflows/trackfw-gate.yml`
- `.github/workflows/release.yml`
- `.github/workflows/deploy-docs.yml`
**Ações:**
- Trocar `uses: actions/checkout@v4` → `uses: actions/checkout@v7` em todas as ocorrências.
- Trocar `uses: actions/setup-go@v5` → `uses: actions/setup-go@v7`.
- Trocar `uses: actions/setup-node@v4` → `uses: actions/setup-node@v7`.
- Trocar `uses: actions/setup-python@v5` → `uses: actions/setup-python@v7`.
- Trocar `uses: goreleaser/goreleaser-action@v6` → `uses: goreleaser/goreleaser-action@v7`.
- Trocar `uses: actions/upload-pages-artifact@v3` → `@v5`.
- Trocar `uses: actions/deploy-pages@v4` → `@v5`.
- Antes de aplicar cada troca, ler o changelog/notas de release da nova major (já parcialmente
  levantado neste roadmap) para confirmar que não há breaking change relevante ao uso atual (ex.:
  mudança de input obrigatório, comportamento de output). Se encontrar algo, documentar no relatório
  antes de aplicar — não aplicar cegamente.
- Não mudar nenhum outro parâmetro/input das actions neste ML — só a versão pinada.
**Critérios de aceite:**
- [ ] `trackfw validate` sem violações novas
- [ ] Nenhum outro conteúdo dos workflows alterado além das versões pinadas (diff limpo, só linhas
      `uses:`)
- [ ] Push/PR de teste confirma que os workflows rodam verdes sem o aviso de depreciação de node20
**Comandos de validação:** `trackfw validate` + observar a execução real do CI no PR
