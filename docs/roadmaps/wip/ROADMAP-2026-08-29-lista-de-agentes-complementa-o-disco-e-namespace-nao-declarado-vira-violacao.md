---
status: wip
date: 2026-08-29
req: "docs/req/REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md"
squad: "hades-tf, apolo-tf, artemis-tf"
---

# Roadmap: A lista `agents:` complementa o disco, e namespace não declarado vira violação

> Created: 2026-08-29 | Status: wip

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Context

REQ: `REQ-2026-08-29-namespace-de-agente-nao-declarado-em-agents-fica-invisivel-e-o-validate-reporta-limpo-sem-olhar.md`
ADR: `ADR-2026-08-29-lista-de-agentes-complementa-o-disco-em-vez-de-substitui-lo-e-namespace-nao-declarado-vira-violacao.md`

Em `roadmap_namespacing: by_agent`, a lista `agents:` **substitui** o disco. Diretório não declarado
fica invisível — e o `validate` reporta `No violations found` sobre o que nunca enumerou. No projeto
cmdb, `docs/roadmaps/zeus/` e `docs/requisições/zeus/` estavam fora de tudo.

A regra está duplicada em **6 funções** só no `validator.go` e o modo aparece em 9 arquivos no Go,
11 sítios no Node e 24 no Python.

## Acceptance Criteria

Consolidado — AC1 a AC11 da REQ.

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Modelo de ameaça e enumeração da superfície
**Status:** ✅ Concluído
**Agente:** `hades-tf`
**Files affected:** apenas este roadmap. Nenhum arquivo de produto.
**Actions:**
1. **Completude de enumeração — vá pelo CONSUMIDOR, não pelo padrão de texto.** Liste **todos** os
   pontos, nos 3 runtimes, que resolvem diretório de estado ou de REQ em modo `by_agent`. Já
   conhecidos em Go: `validator.go` (`validateWIPLimit:221`, `GetStatus:912`, `resolveStateDirs:1020`,
   `resolveREQFiles:1071`, `validateFolderStatusCoherence:1959`, `validateFilenameUniqueness:2036`),
   mais `validator_traceid.go`, `commands/barrier.go`, `generators/roadmap.go`, `generators/req.go`,
   `generators/context.go`, `serve/api_board.go`, `serve/api_metrics.go`. Confirme e complete para
   Node e Python.
   > **A Wave 0 da REQ do pin declarou enumeração fechada sobre um padrão de busca que eu dei, e
   > perdeu metade da superfície.** Não repita: derive os pontos de quem **consome** o caminho, e
   > justifique por que a lista fecha.
2. **Modelo de ameaça.** A união amplia o que a ferramenta lê. Quem se aproveita disso? Cubra no
   mínimo: diretório com nome que escapa do `roadmap_dir` (`..`, caminho absoluto, symlink apontando
   para fora — lembre que ontem achamos escrita fora do projeto por symlink); nome de diretório com
   separador ou caractere de controle; diretório oculto (`.git`, `.DS_Store`, `node_modules`);
   e o caso de `roadmap_dir` e `req_dir` apontando para o mesmo lugar.
3. **Alvos de falsificação nas duas direções.** O que quebra se regredir (volta a substituir o disco)
   **e** se regredir para o lado oposto (passa a enumerar qualquer coisa, inclusive o que não é
   namespace; ou a violação vira tão barulhenta que o usuário desliga a regra — ver `ADR-2026-08-17`).
4. **Residual declarado.** O que o desenho aceita não cobrir.
**Critérios de aceite:**
- [ ] As quatro seções com evidência medida, não asserção
- [ ] A enumeração cobre os 3 runtimes e justifica o fechamento
- [ ] Nenhuma linha de implementação escrita

**Gates da wave:**
```bash
test -f docs/adr/ADR-2026-08-29-lista-de-agentes-complementa-o-disco-em-vez-de-substitui-lo-e-namespace-nao-declarado-vira-violacao.md
```

## Wave 1 — Resolvedor canônico e união (ML único)
> Dependências: Wave 0 aprovada. **ML único e sequencial**: a mesma regra nos 3 runtimes. Agentes em
> paralelo produziram divergência de comportamento três vezes no ciclo anterior.

### ML-1A — Um resolvedor por runtime, união de `agents:` com o disco
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** os pontos que o ML-0A enumerar, nos 3 runtimes, e os testes correspondentes.
**Actions:**
1. Criar **um** resolvedor canônico por runtime que devolva a **união** entre `agents:` e os
   diretórios existentes, para `roadmap_dir` **e** `req_dir` (AC1, AC2).
2. Substituir **todos** os pontos duplicados por chamadas a ele (AC6). O `grep` por `len(agents) == 0`
   e equivalentes só pode casar dentro do resolvedor.
3. Modo `flat` **intocado** (AC8).
**Critérios de aceite:**
- [ ] AC1, AC2, AC3, AC6, AC8
- [ ] AC7 — não-regressão: `validate`, `status` e `context` sobre este repositório produzem saída
      idêntica à de antes. Compare byte a byte.
- [ ] `go build ./...` → 0 · `go test ./...` → 0 · `npm test --prefix npm` → 0 ·
      `PYTHONPATH=pypi python3 -m pytest pypi/tests` → 0

## Wave 2 — Violação de namespace não declarado (ML único)
> Dependências: Wave 1 concluída. A violação só é segura de emitir depois que a união existe.

### ML-2A — Violação nomeando o namespace não declarado
**Status:** ⬜ Pendente
**Agente:** `apolo-tf`
**Files affected:** validator dos 3 runtimes e testes.
**Actions:**
1. Namespace em disco e ausente de `agents:` → **violação**, nomeando o diretório e instruindo a
   acrescentá-lo (AC4).
2. Mensagem byte-idêntica nos 3 (AC9).
3. Decidir e documentar o tratamento de diretório oculto e de nome que não parece namespace — o
   ML-0A traz a lista.
**Critérios de aceite:**
- [ ] AC4, AC5, AC9
- [ ] Os artefatos do namespace não declarado continuam sendo **enumerados** mesmo com a violação
      ativa — a união não depende da declaração
- [ ] Suítes dos 3 verdes

## Wave 3 — Gate e contrato
> Dependências: Waves 1 e 2 concluídas.

### ML-3A — Gate falsificável e `docs/cli-parity.md`
**Status:** ⬜ Pendente
**Agente:** `artemis-tf`
**Files affected:** `scripts/check-agent-namespace-union.sh` (novo), `docs/cli-parity.md`, `Makefile`.
**Actions:**
1. Gate cobrindo AC1, AC4 e AC5 nos 3 runtimes, com projeto de sonda em `by_agent` e um namespace
   não declarado. Falsificação nas duas direções.
2. Guarda de vacuidade; contagem de cenários.
3. Seção em `docs/cli-parity.md` anotada com `gate=`.
4. Registrar no `Makefile`.
**Critérios de aceite:**
- [ ] AC10, AC11
- [ ] Gate exit 0 com contagem; vacuidade provada
- [ ] **Rodar no ambiente empobrecido** antes de declarar pronto: locale `C` e `en_US.UTF-8`, e sem
      `node`/`python3` no PATH quando aplicável. Ver
      `vault/notes/ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29.md`
- [ ] `check-parity-contract-coverage.sh` → 0

> **Formato do bloco de gate:** cada linha é um **comando independente** — não é script, não há
> estado entre linhas. Ver `vault/notes/gates-da-wave-sao-um-comando-por-linha-2026-08-29.md`.

## Barreira final
Revisão `hefesto-tf` e `hades-tf` sobre o diff entregue, auditoria do arquiteto e
`trackfw barrier --wave 3`. **Só declarar concluído com o CI verde**, não com o verde local.
