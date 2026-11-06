---
status: wip
date: 2026-11-06
req: "docs/req/REQ-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado-e-trata-glob-e-leitura-recursiva-de-forma-diferente.md"
squad: ""
---

# Roadmap: guard de credencial atribui ao payload o achado que veio de arquivo referenciado e trata glob e leitura recursiva de forma diferente

> Created: 2026-11-06 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado-e-trata-glob-e-leitura-recursiva-de-forma-diferente.md -->
Defeito medido e decisão: ver REQ e ADR-2026-11-06 (guard de credencial). Varredura de issues abertas: nenhuma com o mesmo mecanismo.

REQ: docs/req/REQ-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado-e-trata-glob-e-leitura-recursiva-de-forma-diferente.md

## Acceptance Criteria
- [ ] AC1–AC4 da REQ

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model (hades-tf)
**Status:** ✅ Concluído
**Files affected:** `docs/seguranca/2026-11-06-wave0-guard-credencial-origem-do-achado.md`
**Acceptance criteria:**
- [x] riscos de imprimir o caminho no stderr e no sinal de atenção analisados (caminho forjado, caractere de controle, comando de workflow, vazamento de caminho sensível)
- [x] a limitação de diretório (D2) analisada quanto ao que ela deixa passar, e se o texto da documentação ensina o contorno
- [x] a montagem de token em tempo de execução nas fixtures analisada quanto a enfraquecer o que os testes afirmam
**Veredito:** APROVA COM RESSALVAS. As ressalvas entraram no ADR (D1 sem caminho no sinal de atenção, D2 com a lista exata de comandos, D4 glob relativo) e nos itens obrigatórios do ML-1A e do ML-1B, na seção final do threat model.

## Wave 1 — Implementação (ML-1A e ML-1B em paralelo)
> Dependencies: Wave 0 auditada. Arquivos disjuntos.

### ML-1A — Mensagem com a origem do achado e documentação da limitação (apolo-tf) — AC1, AC2
**Status:** ✅ Concluído
**Files affected:** `internal/guard/credential.go`, `internal/guard/credential_test.go`, `internal/commands/guard.go`, `docs/cli-parity.md`, `internal/generators/testdata/guard-sh-reference/credential-guard-project.sh`, `internal/generators/testdata/guard-sh-reference/credential-guard-global.sh`, `internal/generators/guard_parity_helper_test.go`
**Acceptance criteria:**
- [x] achado da segunda camada sai como `possible <tipo> detected in file <caminho> referenced by the command`, com o caminho sanitizado; achado no payload continua `in tool payload`; os dois escopos, os dois modos e o sinal de atenção
- [x] teste fixa as duas formas, e um caso com caractere de controle no nome do arquivo
- [x] `--help` e `docs/cli-parity.md` documentam que diretório não é lido recursivamente, com o exemplo medido e a lista exata de comandos cobertos (`cat`, `head`, `tail`, `jq`, `grep`)
- [x] D4: glob relativo resolvido contra a `cwd` da ferramenta; teste com `cwd` do payload diferente da pasta do processo
- [x] os 8 itens obrigatórios do threat model da Wave 0 para o ML-1A
- [x] `go test ./internal/guard/ ./internal/commands/` verde

**Auditoria:** os scripts de referência `.sh` e o sha256 deles entraram pela mesma mudança de contrato da mensagem. O D4 foi estendido ao campo `cwd` da raiz (schema do Kiro), que é o mesmo mecanismo. Ficou uma divergência: a mensagem do sinal de atenção no oráculo `.sh` ainda diz `tool payload`. Vai para o ML-1C.

### ML-1B — Fixtures montam o token em tempo de execução (artemis-tf) — AC3
**Status:** ✅ Concluído
**Files affected:** `internal/guard/credential_patterns_test.go`, `internal/generators/credential_guard_test.go`, `vault/notes/guard-credencial-comentarios-de-teste-disparam-o-guard-2026-11-06.md`, `vault/notes/index.md`
**Acceptance criteria:**
- [x] nenhum dos dois arquivos contém literal que case com os padrões do guard
- [x] os testes continuam afirmando a detecção sobre o valor montado; contra-braço mostra que a detecção ainda reprova sem o padrão
- [x] os 4 itens obrigatórios do threat model da Wave 0 para o ML-1B
- [x] `go test ./internal/guard/ ./internal/generators/` verde
**Auditoria:** o executor chamou de "pré-existentes" 15 falhas em `internal/generators` que vinham do ML-1A em curso; com os dois MLs juntos, a suíte passa. O executor escreveu uma nota de vault fora do escopo, e a própria nota trazia o cabeçalho PEM literal num bloco de código; o arquiteto removeu o literal.

### ML-1C — Oráculo `.sh` com a mensagem de atenção nova (apolo-tf) — AC1
**Status:** ✅ Concluído
**Files affected:** `internal/generators/testdata/guard-sh-reference/credential-guard-project.sh`, `internal/generators/testdata/guard-sh-reference/credential-guard-global.sh`, `internal/generators/guard_parity_helper_test.go`, `internal/generators/credential_guard_test.go`
**Acceptance criteria:**
- [x] a mensagem de atenção dos dois scripts de referência distingue a origem como o Go (`in a file referenced by the command`, sem caminho)
- [x] um teste de paridade exercita o sinal de atenção com achado de arquivo no modo warn, comparando Go com `.sh`
- [x] `go test ./internal/generators/` verde

## Wave 2 — Red team
> Dependencies: Wave 1 completa.

### ML-2A — Red team (hades-tf)
**Status:** ✅ Concluído
**Files affected:** `docs/seguranca/2026-11-06-redteam-guard-credencial-origem-do-achado.md`
**Acceptance criteria:**
- [x] cada risco da Wave 0 atacado com medição
- [x] veredito escrito — **BLOQUEIA** (achado A-1)

**Auditoria:** índice limpo, só o relatório novo na árvore. A-1 confirmado por leitura: `credGlobToken` faz `filepath.Join(baseCwd, pattern)` sem escapar os metacaracteres da própria `cwd`. Uma `cwd` com `[`, `*` ou `?` vira parte do padrão: `d[1]` não casa nada (falso negativo), `d[1` dá `ErrBadPattern` e volta ao token cru (falso negativo), `a*b` casa diretórios irmãos (falso positivo com atribuição errada — o defeito que D1 corrige). Antes do D4 o glob rodava relativo à cwd do processo e a `cwd` nunca entrava no padrão. Mesma causa (D4), mesma REQ: vira ML-2B. O guard instalado bloqueou o red team cinco vezes por literal PEM no texto dos próprios comandos; os dados do A-1 não dependem desses passos.

### ML-2B — Corretivo do A-1: escapar a `cwd` antes de juntar ao glob (apolo-tf) — AC5
**Status:** ⬜ Pendente
**Files affected:** `internal/guard/credential.go`, `internal/guard/credential_test.go`, `docs/cli-parity.md`
**Acceptance criteria:**
- [ ] `credGlobToken` escapa `[`, `*`, `?` de `baseCwd` (e `\` fora do Windows) antes do `filepath.Join`; só o token do comando é interpretado como padrão
- [ ] testes: `cwd=d[1]` com `*.txt` detecta; `cwd=d[1` detecta; `cwd=a*b` com token em `axb/` não detecta; arg nomeado com `cwd=d[1]` continua detectando; vetores `*`/`?` com skip no Windows, `[` roda em todas as plataformas
- [ ] contra-braço: sem o escape, os três primeiros testes reprovam
- [ ] `docs/cli-parity.md`: remover "não verificada" da cwd raiz do Claude Code (coberta por `TestRunCredential_D4_GlobRootLevelCwd`) e declarar que a `cwd` é tratada como literal
- [ ] `go test ./internal/guard/` verde; `make quality` verde
