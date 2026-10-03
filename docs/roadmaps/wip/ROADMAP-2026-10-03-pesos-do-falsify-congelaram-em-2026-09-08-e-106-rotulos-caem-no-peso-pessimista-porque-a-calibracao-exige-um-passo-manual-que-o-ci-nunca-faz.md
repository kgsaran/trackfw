---
status: wip
date: 2026-10-03
req: "docs/req/REQ-2026-10-03-pesos-do-falsify-congelaram-em-2026-09-08-e-106-rotulos-caem-no-peso-pessimista-porque-a-calibracao-exige-um-passo-manual-que-o-ci-nunca-faz.md"
squad: "hades-tf, ares-tf, hefesto-tf"
---

# Roadmap: pesos do falsify congelaram em 2026-09-08 e 106 rotulos caem no peso pessimista porque a calibracao exige um passo manual que o CI nunca faz

> Created: 2026-10-03 | Status: wip

## Context
REQ: docs/req/REQ-2026-10-03-pesos-do-falsify-congelaram-em-2026-09-08-e-106-rotulos-caem-no-peso-pessimista-porque-a-calibracao-exige-um-passo-manual-que-o-ci-nunca-faz.md
ADR: docs/adr/ADR-2026-10-03-pesos-do-falsify-se-recalibram-a-partir-do-ci-que-grava-o-arquivo-de-tempos-em-todo-run.md
Issue: #403 (label `req-aberta`). Fecha #403.

Medido em 2026-10-03:
- 106 rótulos sem peso calibrado (`python3 scripts/gen-falsify-chunks.py scripts/check-gates-falsify.sh <dir> 4`
  emite 106 avisos "sem peso calibrado").
- Os 4 shards levam 57–113 s no CI; o `parity-other-gates`, em paralelo, leva 150–200 s (runs
  37157607943, 37158139889, 37145130352).
- A marca de tempo já existe nos chunks (`gen-falsify-chunks.py:620-635`). O shard copia o log para
  `$OUTPUT_DIR` (`run-gates-falsify-shard.sh:135`), e o workflow sobe `falsify-shard-out/` como artefato
  `falsify-shard-<n>` (`quality.yml:785`).

## Acceptance Criteria
- [ ] AC1–AC9 da REQ, cada um com evidência apontável

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat model
> Dependências: nenhuma. Bloqueia toda implementação.

### ML-0A — Threat model da instrumentação no CI e da recalibração
**Status:** ⬜ Pendente
**Squad:** hades-tf
**Files affected:** `docs/seguranca/2026-10-03-wave0-recalibracao-pesos-falsify.md` (único arquivo)
**Actions:**
1. **Completude:** todos os consumidores de `falsify-scenario-weights.json` e de `FALSIFY_TIMING_FILE`
   (grep em `scripts/`, `Makefile` e `.github/`).
2. **Threat model:**
   - a marca ligada altera stdout/stderr do chunk, a coleta de rótulos (`ECHO_LABEL_PAT`) ou o rc do shard?
   - um `gh run download` de run de fork ou de outro repositório injeta pesos?
   - pesos adulterados (zero, negativo, NaN, gigante) tiram cenário da cobertura, ou só desequilibram?
     A guarda `check-falsify-shard-coverage.sh` pega isso?
   - artefato com 7 dias de retenção, ou run sem os 4 shards.
3. **Alvos de falsificação nas duas direções.**
4. **Resíduo declarado.**
**Acceptance criteria:**
- [ ] As quatro seções com evidência (comando e saída)
- [ ] Veredito por cenário: coberto, requer ajuste (qual), ou resíduo

**Gates da wave:**
```bash
test -s docs/seguranca/2026-10-03-wave0-recalibracao-pesos-falsify.md
```

## Wave 1 — Instrumentação, alvo de recalibração e linha de resumo
> Dependências: Wave 0 auditada. Um ML só: o workflow, o `Makefile` e os scripts formam um caminho único.

### ML-1A — CI grava as marcas, `make falsify-recalibrate`, resumo da fração
**Status:** ⬜ Pendente
**Squad:** ares-tf
**Files affected:**
- `.github/workflows/quality.yml` (só o job `parity-falsify-shard`)
- `Makefile`
- `scripts/falsify-recalibrate.sh` (novo)
- `scripts/gen-falsify-chunks.py` (só a linha de resumo)
- um autoteste do script novo, ligado no `Makefile` como os `check-*.sh` existentes
**Actions:**
1. ADR D1: no step do shard, `FALSIFY_TIMING_FILE: ${{ github.workspace }}/falsify-shard-out/timing_${{ matrix.shard }}.log`.
   O diretório precisa existir antes da primeira marca; confira quem o cria
   (`run-gates-falsify-shard.sh:96`) e se a variável chega ao processo do chunk.
2. ADR D2: `scripts/falsify-recalibrate.sh <run-id>`:
   - `gh run download <id> --repo kgsaran/trackfw --pattern 'falsify-shard-*' --dir <tmp>`;
   - exige `timing_<n>.log` não vazio para cada n em 0..FALSIFY_SHARD_COUNT-1 (o valor lido do
     `quality.yml`, sem duplicar a constante);
   - concatena, roda `gen-falsify-scenario-weights.py` e escreve `scripts/falsify-scenario-weights.json`;
   - exit ≠ 0 com mensagem quando falta algum shard.
   - Alvo `make falsify-recalibrate RUN=<id>`.
3. ADR D4: no `gen-falsify-chunks.py`, uma linha final em stderr: `N de M rotulos sem peso calibrado (X%)`.
   O exit code não muda.
4. Autoteste do script: `gh` falso via PATH, com os dois braços:
   - artefatos completos → json escrito;
   - um shard sem marcas → reprova, e o json não é tocado.
**Acceptance criteria:**
- [ ] AC3 e AC5 com o autoteste nos dois braços; cada braço declara o que afirma, e o autoteste reprova
  sem a correção
- [ ] `actionlint` (se disponível) ou `python3 -c "import yaml..."` sobre o `quality.yml`
- [ ] `python3 scripts/gen-falsify-chunks.py scripts/check-gates-falsify.sh <tmp> 4` imprime a linha de
  resumo, com exit inalterado
- [ ] `scripts/check-falsify-shard-coverage.sh` verde (**não** rodar `make quality`)

**Gates da wave:**
```bash
bash scripts/check-falsify-recalibrate.sh
python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/quality.yml'))"
```

## Wave 2 — Recalibrar a partir do run do PR
> Dependências: Wave 1 commitada e empurrada; o run de CI do PR concluído com os 4 shards verdes.

### ML-2A — Recalibração e medição de volta
**Status:** ⬜ Pendente
**Squad:** ares-tf
**Files affected:** `scripts/falsify-scenario-weights.json`
**Actions:**
1. `make falsify-recalibrate RUN=<id do run do PR>`; o arquiteto informa o id.
2. AC2: listar os 4 `timing_<n>.log` no artefato.
3. AC4: rodar o gerador de novo e contar os avisos "sem peso calibrado".
4. AC7: tabela de peso, rótulos e linhas por chunk (4 e 8 chunks), antes e depois.
**Acceptance criteria:**
- [ ] Rótulos sem peso = 0, ou o resto nomeado
- [ ] Tabela antes/depois com o desvio máximo/mínimo

## Wave 3 — Barreira
> Dependências: Wave 2 auditada.

### ML-3A — Qualidade e gate completo
**Status:** ⬜ Pendente
**Squad:** hefesto-tf
**Files affected:** `docs/qualidade/2026-10-03-revisao-recalibracao-pesos-falsify.md`
**Acceptance criteria:**
- [ ] Revisão do diff; `make quality` EXIT=0 com a máquina ociosa e o log conferido

**Gates da wave:**
```bash
test -s docs/qualidade/2026-10-03-revisao-recalibracao-pesos-falsify.md
```
