---
status: wip
date: 2026-10-02
req: "docs/req/REQ-2026-10-02-driver-de-falsificacao-paralelo-espera-para-sempre-por-chunk-que-nao-termina-limite-de-tempo-por-chunk-com-fail-nomeado.md"
squad: "hades-tf, ares-tf, hefesto-tf"
---

# Roadmap: driver de falsificação paralelo — limite de tempo por chunk com FAIL nomeado

> Created: 2026-10-02 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-02-driver-de-falsificacao-paralelo-espera-para-sempre-por-chunk-que-nao-termina-limite-de-tempo-por-chunk-com-fail-nomeado.md
Issue: #504 (o PR fecha). Base: `main` em `0bf66679`.

Sítio único: `scripts/run-gates-falsify-parallel.sh`. O laço de disparo (~:143-146) cria um subshell
por chunk (`( TRACKFW_ROOT_DIR=… bash "$chunk" ) >"$log" 2>&1 &`), e o laço de coleta (~:149-162) faz
`wait "$pid"` sem limite. Roda com o `bash` do PATH (`#!/usr/bin/env bash`; usa `mapfile`, que é
bash 4+). Só o `make` local usa este driver; o CI usa `run-gates-falsify-shard.sh`.

## Acceptance Criteria
- [x] AC1 — Wave 0 auditada
      ✅ `docs/seguranca/2026-10-02-wave0-limite-por-chunk.md`: APROVA COM AJUSTES. Chunk mais lento medido: 137 s ociosa, 205 s com carga; 1200 s mantido (5,8× de margem). Kill por grupo de processos (`set -m`), 8 ajustes absorvidos no ML-1A.
- [ ] AC2 — chunk sintético travado: FAIL nomeado, árvore impressa, nenhum processo sobrevivente
- [ ] AC3 — suíte real verde sem override (347/0)
- [ ] AC4 — o teste reprova no driver de `0bf66679`
- [ ] AC5 — bash declarado
- [ ] AC6 — `make quality` EXIT=0 e CI verde

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — O que o limite pode esconder ou quebrar
**Status:** ✅ Concluído
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-02-wave0-limite-por-chunk.md` (único arquivo escrito)
**Actions:**
1. **Completude:** há outro ponto de espera sem limite no driver (guarda de conjunto, `cat` dos logs, `trap` de limpeza)? E fora dele, no caminho do `make quality` local?
2. **Ameaça e quebra:** (a) chunk lento legítimo virando FAIL: meça o tempo por chunk sob o `make quality`, com a máquina ociosa e com carga (por exemplo, `go test ./...` rodando junto), e diga se 1200 s é margem suficiente; (b) processo da árvore que sobrevive ao kill. Como matar a árvore inteira no macOS sem `setsid`? Compare grupo de processos (`set -m` / `kill -- -PGID`) com descida recursiva por `pgrep -P`, e diga qual pega neto que fez `setsid`/`nohup`. (c) A guarda de conjunto segue acusando rótulo ausente de chunk morto por limite? (d) Windows/MSYS: `ps` e `kill` de grupo funcionam lá?
3. **Falsificação nas duas direções:** o limite dispara cedo demais (frouxo para a velocidade da máquina) e não dispara (o travamento volta a ser silêncio).
4. **Resíduo declarado.**
**Acceptance criteria:**
- [x] As quatro seções com evidência (comando + saída)
- [x] Veredito explícito, com o mecanismo de kill recomendado
- [x] Nenhuma linha de implementação

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-02-wave0-limite-por-chunk.md
grep -q "Veredito" docs/seguranca/2026-10-02-wave0-limite-por-chunk.md
```

## Wave 1 — Implementação
> Dependencies: Wave 0 auditada

### ML-1A — Limite por chunk no driver e teste com chunk sintético
**Status:** ⬜ Pendente
**Squad:** ares-tf
**Files affected:** `scripts/run-gates-falsify-parallel.sh`, script de teste novo (nome e ponto de ligação no `Makefile`/`parity-rest` a decidir lendo como os outros `check-*.sh` de autoteste são ligados), `docs/cli-parity.md` se descrever o driver
**Actions:** conforme a REQ, com o mecanismo e os 8 ajustes da Wave 0 (§ Veredito do parecer):
1. `set -m` antes do disparo; `PGID=$!` **imediatamente** após o `&` (não por `ps -o pgid=` depois: há janela de corrida medida).
2. Disparo com `</dev/null` explícito (sob `set -m`, o job assíncrono deixa de ganhar `/dev/null` implícito; medido).
3. No estouro: árvore de processos do grupo, `kill -TERM -- -$PGID`, espera curta e `kill -9 -- -$PGID`, e nova varredura do PGID depois do `wait`.
4. `trap` de `INT TERM HUP` no driver que mata os grupos vivos; **não** usar `EXIT`, que substituiria o `trap 'rm -rf "$WORKDIR"' EXIT` existente.
5. Nenhum `exit` no caminho do estouro antes da guarda de conjunto: ela tem de rodar e acusar os rótulos do chunk morto.
6. `TRACKFW_FALSIFY_CHUNK_TIMEOUT` validado contra `^[1-9][0-9]*$` (valor inválido aborta com mensagem; `$(( abc ))` avalia a 0).
7. Declarar no cabeçalho que o driver requer bash 4+ (`mapfile`).
8. Chunk sintético do teste com `sleep 999999` ou laço (no macOS não existe `sleep infinity` nem `timeout`).
9. O autoteste roda no macOS e no Linux; no Git Bash/MSYS (`uname` com `MINGW`/`MSYS`), `t.Skip`/saída declarada, porque `kill -- -PGID` não existe lá (resíduo R4 da Wave 0). `TRACKFW_FALSIFY_CHUNK_TIMEOUT` (padrão 1200), override denunciado no stderr. No estouro: árvore de processos do chunk, kill da árvore, `FAIL [falsify-driver/chunk-timeout] chunk <N> excedeu <T>s`, rc≠0, e a guarda de conjunto continua rodando.
**Acceptance criteria:**
- [ ] Teste com chunk sintético (dorme para sempre, com um neto em background): rc≠0 em até T+10 s, linha FAIL, árvore impressa, 0 processos sobreviventes
- [ ] O mesmo teste contra o driver de `0bf66679` (cópia, com teto externo) não termina dentro do teto: prova de que morde
- [ ] Suíte real sem override: 347 OK / 0 FAIL
- [ ] Uma frase por teste
**Gates da wave:**
```bash
bash -n scripts/run-gates-falsify-parallel.sh
make build
```

## Wave 2 — Revisão e gate completo
> Dependencies: Wave 1 auditada

### ML-2A — Revisão de qualidade e `make quality`
**Status:** ⬜ Pendente
**Squad:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-02-revisao-limite-por-chunk.md`
**Actions:** revisão do diff; `make quality` completo **com a máquina ociosa** (conferir `ps` antes).
**Acceptance criteria:**
- [ ] `make quality` EXIT=0, com a linha `suite completa … 347 OK, 0 FAIL` citada do log
- [ ] Veredito explícito

**Gates da wave:**
```bash
test -s docs/qualidade/2026-10-02-revisao-limite-por-chunk.md
```
