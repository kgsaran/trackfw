# Medição do alvo `parity` — Wave 0 da REQ-2026-09-02

> Data da medição: 2026-09-03 · Autor: Ares (infraestrutura) · Escopo: **somente leitura**, nenhum
> arquivo do produto foi alterado, nenhuma operação de git foi executada.
>
> REQ: `docs/req/REQ-2026-09-02-job-parity-e-o-caminho-critico-do-ci-com-13m23s-e-a-premissa-que-justifica-manter-sequencial-esta-desatualizada-em-3x.md`
> Cobre **AC1** (local **e** no runner do CI), **AC2** e a decisão do **AC3**; descreve a forma
> exigida pelo **AC4** sem implementá-la; entrega o piso medido que o **AC6** precisa e o texto
> sugerido para o **AC7**. **Não** implementa nada — nenhuma mudança de CI foi feita.

## Veredito em uma linha

**Um único gate — `check-gates-falsify.sh` — responde por 78,36 % do tempo dos gates no CI
(609,74 s medidos no runner) e 82,71 % localmente.** Qualquer divisão em shards tem como piso o tempo
desse gate sozinho: o `parity` continuaria custando ~10m50 contra os 13m23 de hoje — ganho de
**~2m30 (≈19 %)**. E a execução concorrente medida ficou **até 3,4× mais lenta** que a sequencial. **A recomendação é NÃO dividir** — "não vale" é o resultado, e ele encerra a REQ pelo
AC6 se o orquestrador concordar. O ganho real está dentro do `check-gates-falsify.sh`, que é
**REQ própria** e está no escopo negativo desta.

## Método

- Lista de comandos extraída mecanicamente de `make -n parity | grep 'scripts/check-'` — **46
  invocações**, com os prefixos `GO_BIN=bin/trackfw` exatamente como o `Makefile` os passa. Nenhum
  comando foi redigitado.
- Cada gate executado **uma vez, sequencialmente**, medido com `Time::HiRes` em volta do processo.
- Binário `bin/trackfw` já compilado e **não obsoleto** (`find internal cmd -name '*.go' -newer
  bin/trackfw` → vazio). Compilação **não** entra nos tempos, exceto onde o próprio gate recompila
  (ver §3).
- Por gate, além do tempo: exit code, bytes de saída e **delta observado** em `git status
  --porcelain`, mtime de `bin/trackfw`, mtime de `docs/roadmaps/.trackfw-log`, arquivos novos em
  `scripts/` e em `~/.claude ~/.trackfw ~/.config`.
- Ambiente: macOS (Apple Silicon), 10 vCPU, árvore limpa em `fix/validate-detecta-hook-de-guard-na-forma-relativa-antiga` (`c0f6781`).

### Duas medições, não uma

- **Local** (§1): macOS/Apple Silicon, 10 vCPU, binário pré-compilado. Cobre os **46** gates de hoje.
- **CI real** (§1-bis): extraída do log bruto do run `33679814232` — o `Makefile` ecoa cada linha da
  receita e o Actions carimba timestamp em cada linha, então **o delta entre dois ecos consecutivos é
  a duração daquele gate no runner**. Cobre os **44** gates que existiam naquele commit. Não exigiu
  disparar CI nem tocar na árvore.

A medição de CI é o que o **AC1** pede ("ambiente comparável ao runner") e adianta boa parte da
evidência do **AC6** — o que o AC6 ainda exige é um run *depois* de uma mudança, para comparar.

### Reconciliação 44 × 45 × 46

- **46** = invocações no alvo `parity` **hoje** (46 scripts distintos; a 46ª é
  `scripts/check-pr-closing-keyword.sh --self-test`).
- **45** = o número escrito na REQ, correto quando ela foi redigida em 2026-09-02.
- **44** = o que o run `33679814232` (head `83305007`, 2026-09-02) realmente executou.
  `check-pr-closing-keyword.sh` entrou em `9c86b0a` e `check-artifact-closed-cycle.sh` em `d1f564e`,
  **ambos em 2026-09-03**, depois do run. Custo local dos dois somados: 5,21 s — não muda conclusão
  alguma.

## 1. Tabela de tempos (AC1)

Ordem decrescente. `bin/trackfw` / `.trackfw-log` = o gate **alterou o mtime** do recurso real do
repositório durante a execução.

| # | Gate | Tempo (s) | % do total | exit | bin/trackfw | .trackfw-log | working tree |
|---:|---|---:|---:|:--:|:--:|:--:|:--:|
| 1 | `check-gates-falsify.sh` | 1141.35 | 82.71% | 0 | **escreve** | **escreve** | limpo |
| 2 | `check-roadmap-barrier-contract.sh` | 49.99 | 3.62% | 0 | — | — | limpo |
| 3 | `check-release-tag-parity.sh` | 27.32 | 1.98% | 0 | — | — | limpo |
| 4 | `check-agent-namespace-union.sh` | 20.60 | 1.49% | 0 | — | — | limpo |
| 5 | `check-audit-surface.sh` | 20.17 | 1.46% | 0 | — | — | limpo |
| 6 | `check-doctor-parity.sh` | 13.42 | 0.97% | 0 | — | — | limpo |
| 7 | `check-branch-prune-parity.sh` | 10.03 | 0.73% | 0 | — | — | limpo |
| 8 | `check-ship-force-parity.sh` | 9.03 | 0.65% | 0 | — | — | limpo |
| 9 | `check-barrier.sh` | 8.96 | 0.65% | 0 | — | — | limpo |
| 10 | `check-push-force-parity.sh` | 8.31 | 0.60% | 0 | — | — | limpo |
| 11 | `check-identity-parity.sh` | 8.27 | 0.60% | 0 | — | — | limpo |
| 12 | `check-validate-parity.sh` | 7.33 | 0.53% | 0 | — | — | limpo |
| 13 | `check-update-parity.sh` | 6.17 | 0.45% | 0 | — | — | limpo |
| 14 | `check-doctor-remote-parity.sh` | 5.93 | 0.43% | 0 | — | — | limpo |
| 15 | `check-agent-models-parity.sh` | 5.41 | 0.39% | 0 | — | — | limpo |
| 16 | `check-artifact-closed-cycle.sh` | 4.72 | 0.34% | 0 | — | — | limpo |
| 17 | `check-ship-parity.sh` | 4.34 | 0.31% | 0 | — | — | limpo |
| 18 | `check-cli-parity.sh` | 3.80 | 0.28% | 0 | **escreve** | — | limpo |
| 19 | `check-agent-hooks-parity.sh` | 3.30 | 0.24% | 0 | — | — | limpo |
| 20 | `check-push-parity.sh` | 2.95 | 0.21% | 0 | — | — | limpo |
| 21 | `check-ci-workflow-pin-parity.sh` | 2.44 | 0.18% | 0 | — | — | limpo |
| 22 | `check-artifact-parity.sh` | 2.36 | 0.17% | 0 | — | — | limpo |
| 23 | `check-branch-new-parity.sh` | 1.58 | 0.11% | 0 | — | — | limpo |
| 24 | `check-commit-parity.sh` | 1.56 | 0.11% | 0 | — | — | limpo |
| 25 | `check-thirdparty-parity.sh` | 1.43 | 0.10% | 0 | — | — | limpo |
| 26 | `check-attention-scripts-parity.sh` | 1.32 | 0.10% | 0 | — | — | limpo |
| 27 | `check-roadmap-move-parity.sh` | 1.24 | 0.09% | 0 | — | — | limpo |
| 28 | `check-unknown-command-parity.sh` | 1.24 | 0.09% | 0 | — | — | limpo |
| 29 | `check-serve-address-parity.sh` | 1.19 | 0.09% | 0 | — | — | limpo |
| 30 | `check-rules-parity.sh` | 1.02 | 0.07% | 0 | — | — | limpo |
| 31 | `check-harness-hooks-parity.sh` | 0.97 | 0.07% | 0 | — | — | limpo |
| 32 | `check-install-version-pin.sh` | 0.60 | 0.04% | 0 | — | — | limpo |
| 33 | `check-pr-closing-keyword.sh` | 0.49 | 0.04% | 0 | — | — | limpo |
| 34 | `check-homedir-parity.sh` | 0.26 | 0.02% | 0 | — | — | limpo |
| 35 | `check-slash-parity.sh` | 0.20 | 0.01% | 0 | — | — | limpo |
| 36 | `check-integration-assets.sh` | 0.11 | 0.01% | 0 | — | — | limpo |
| 37 | `check-tty-detection.sh` | 0.11 | 0.01% | 0 | — | — | limpo |
| 38 | `check-ci-workflow-job-id-collision.sh` | 0.06 | 0.00% | 0 | — | — | limpo |
| 39 | `check-output-encoding-declared.sh` | 0.06 | 0.00% | 0 | — | — | limpo |
| 40 | `check-ref-separator-portability.sh` | 0.05 | 0.00% | 0 | — | — | limpo |
| 41 | `check-referential-integrity.sh` | 0.04 | 0.00% | 0 | — | — | limpo |
| 42 | `check-parity-contract-coverage.sh` | 0.04 | 0.00% | 0 | — | — | limpo |
| 43 | `check-static-assets.sh` | 0.04 | 0.00% | 0 | — | — | limpo |
| 44 | `check-python-writes-lf.sh` | 0.04 | 0.00% | 0 | — | — | limpo |
| 45 | `check-shell-posix-portability.sh` | 0.04 | 0.00% | 0 | — | — | limpo |
| 46 | `check-atomic-write-anti-divergence.sh` | 0.03 | 0.00% | 0 | — | — | limpo |
| | **TOTAL (46 gates)** | **1379.92** | **100%** | | | | |

**Soma dos 46 gates: 1379,92 s (23m00).** O relógio de parede do harness foi 1393,80 s — a
diferença de 13,9 s é o custo das sondas de recurso entre gates, o que confirma que nenhum tempo
ficou fora da conta.

Concentração:

| Fatia | Tempo | % |
|---|---:|---:|
| `check-gates-falsify.sh` sozinho | 1141,35 s (19m01) | **82,71 %** |
| os outros 45 gates somados | 238,57 s (3m59) | 17,29 % |

Os **3 mais lentos**: `check-gates-falsify.sh` (1141,35 s), `check-roadmap-barrier-contract.sh`
(49,99 s), `check-release-tag-parity.sh` (27,32 s). O 2º e o 3º **juntos** são 6,8 % do 1º.

> A nota de 2026-08-05 no `quality.yml` registrava "~4m15s total, `check-gates-falsify.sh` ~3m05s"
> — 72,5 % do total. Hoje o mesmo gate é 82,7 %: a população de gates triplicou (14 → 46) e mesmo
> assim **a concentração aumentou**. Os 32 gates novos custam menos que o crescimento do dominante.

## 1-bis. Tabela de tempos **no runner do CI** (AC1, ambiente comparável)

Fonte: log bruto do job `parity` (`actions/jobs/100413926297`) do run `33679814232`, 2026-09-02.
Duração de cada gate = intervalo entre o eco da sua linha de receita e o eco da seguinte.

| # | Gate | CI (s) | % CI | Local (s) | local/CI |
|---:|---|---:|---:|---:|---:|
| 1 | `check-gates-falsify.sh` | 609.74 | 78.36% | 1141.35 | 1.87x |
| 2 | `check-agent-namespace-union.sh` | 25.69 | 3.30% | 20.60 | 0.80x |
| 3 | `check-roadmap-barrier-contract.sh` | 19.69 | 2.53% | 49.99 | 2.54x |
| 4 | `check-doctor-parity.sh` | 18.56 | 2.39% | 13.42 | 0.72x |
| 5 | `check-cli-parity.sh` | 16.25 | 2.09% | 3.80 | 0.23x |
| 6 | `check-identity-parity.sh` | 10.19 | 1.31% | 8.27 | 0.81x |
| 7 | `check-release-tag-parity.sh` | 8.61 | 1.11% | 27.32 | 3.17x |
| 8 | `check-barrier.sh` | 7.57 | 0.97% | 8.96 | 1.18x |
| 9 | `check-validate-parity.sh` | 7.26 | 0.93% | 7.33 | 1.01x |
| 10 | `check-update-parity.sh` | 6.93 | 0.89% | 6.17 | 0.89x |
| 11 | `check-audit-surface.sh` | 5.70 | 0.73% | 20.17 | 3.54x |
| 12 | `check-agent-models-parity.sh` | 5.36 | 0.69% | 5.41 | 1.01x |
| 13 | `check-ci-workflow-pin-parity.sh` | 4.64 | 0.60% | 2.44 | 0.53x |
| 14 | `check-doctor-remote-parity.sh` | 4.28 | 0.55% | 5.93 | 1.39x |
| 15 | `check-branch-prune-parity.sh` | 3.06 | 0.39% | 10.03 | 3.28x |
| 16 | `check-artifact-parity.sh` | 2.95 | 0.38% | 2.36 | 0.80x |
| 17 | `check-ship-force-parity.sh` | 2.31 | 0.30% | 9.03 | 3.91x |
| 18 | `check-push-force-parity.sh` | 2.31 | 0.30% | 8.31 | 3.60x |
| 19 | `check-agent-hooks-parity.sh` | 1.86 | 0.24% | 3.30 | 1.77x |
| 20 | `check-ship-parity.sh` | 1.74 | 0.22% | 4.34 | 2.49x |
| 21 | `check-branch-new-parity.sh` | 1.70 | 0.22% | 1.58 | 0.93x |
| 22 | `check-thirdparty-parity.sh` | 1.63 | 0.21% | 1.43 | 0.88x |
| 23 | `check-push-parity.sh` | 1.50 | 0.19% | 2.95 | 1.97x |
| 24 | `check-roadmap-move-parity.sh` | 1.48 | 0.19% | 1.24 | 0.84x |
| 25 | `check-serve-address-parity.sh` | 1.42 | 0.18% | 1.19 | 0.84x |
| 26 | `check-attention-scripts-parity.sh` | 1.29 | 0.17% | 1.32 | 1.02x |
| 27 | `check-unknown-command-parity.sh` | 1.28 | 0.16% | 1.24 | 0.97x |
| 28 | `check-commit-parity.sh` | 0.86 | 0.11% | 1.56 | 1.81x |
| 29 | `check-harness-hooks-parity.sh` | 0.77 | 0.10% | 0.97 | 1.26x |
| 30 | `check-rules-parity.sh` | 0.50 | 0.06% | 1.02 | 2.04x |
| 31 | `check-homedir-parity.sh` | 0.24 | 0.03% | 0.26 | 1.08x |
| 32 | `check-slash-parity.sh` | 0.24 | 0.03% | 0.20 | 0.83x |
| 33 | `check-install-version-pin.sh` | 0.12 | 0.02% | 0.60 | 5.00x |
| 34 | `check-tty-detection.sh` | 0.09 | 0.01% | 0.11 | 1.22x |
| 35 | `check-parity-contract-coverage.sh` | 0.07 | 0.01% | 0.04 | 0.57x |
| 36 | `check-output-encoding-declared.sh` | 0.06 | 0.01% | 0.06 | 1.00x |
| 37 | `check-referential-integrity.sh` | 0.03 | 0.00% | 0.04 | 1.33x |
| 38 | `check-integration-assets.sh` | 0.03 | 0.00% | 0.11 | 3.67x |
| 39 | `check-python-writes-lf.sh` | 0.02 | 0.00% | 0.04 | 2.00x |
| 40 | `check-ci-workflow-job-id-collision.sh` | 0.02 | 0.00% | 0.06 | 3.00x |
| 41 | `check-ref-separator-portability.sh` | 0.02 | 0.00% | 0.05 | 2.50x |
| 42 | `check-atomic-write-anti-divergence.sh` | 0.02 | 0.00% | 0.03 | 1.50x |
| 43 | `check-shell-posix-portability.sh` | 0.02 | 0.00% | 0.04 | 2.00x |
| 44 | `check-static-assets.sh` | 0.00 * | 0.00% | 0.04 | — |
| | **TOTAL (44 gates do run)** | **778.11** | **100%** | **1374.71** | **1.77x** |

\* `0.00` = abaixo da resolução de milissegundos entre dois ecos consecutivos. Nos gates de dezenas
de milissegundos a razão local/CI não é interpretável (o ruído domina); ela só tem significado nos
gates acima de ~1 s.

**Decomposição real do job de 13m23 (803 s):**

Confirmada pela API do Actions (`actions/jobs/100413926297`, durações por step), não só pelo log:

| Fase | Tempo |
|---|---:|
| `Set up job` + `checkout` + `setup-go/node/python` + `npm ci` + 2× `pip install` | **20 s** |
| **`Run make parity`** (o `build` + os 44 gates) | **780 s** |
| todos os `Post ...` + `Complete job` | **0 s** |
| **total do job** | **803 s (13m23)** |

Dentro dos 780 s: `build` = 1,5 s (o cache do Go no runner é quente) e os **44 gates = 778,1 s**.
A conclusão que isso força: **não existe overhead de setup para otimizar.** 97,1 % do job são gates,
e 78,4 % dos gates são um só.

🔴 **O ponto que decide a REQ, agora medido e não estimado:** `check-gates-falsify.sh` custa
**609,74 s (10m10) no CI — 78,36 % dos gates e 75,9 % do job inteiro.** O 2º colocado no CI
(`check-agent-namespace-union.sh`, 25,69 s) é **23,7× menor**.

A máquina local é **1,77× mais lenta** que o runner no agregado (1374,71 s × 778,11 s para os mesmos
44 gates) e **1,87×** no `falsify` isoladamente — os fatores são próximos, então a forma da
distribuição local **não** engana: as duas medições concordam sobre quem é o dominante (82,7 % local
× 78,4 % CI).

## 2. Classificação de paralelismo (AC2)

### 2.1 Recursos compartilhados observados na execução sequencial

| Gate | Recurso compartilhado | Evidência |
|---|---|---|
| `check-cli-parity.sh` | **`bin/trackfw`** | mtime alterado; `scripts/check-cli-parity.sh:27` faz `go build -o "$GO_BIN"` incondicionalmente |
| `check-identity-parity.sh` | **`bin/trackfw`** | `scripts/check-identity-parity.sh:125` faz `go build -o "$GO_BIN"` |
| `check-gates-falsify.sh` | **`bin/trackfw`** | mtime alterado; linhas 340 e 406 executam cópias mutadas do `check-identity-parity.sh` com `GO_BIN="$ROOT_DIR/bin/trackfw"` — ou seja, **recompilam o binário real do repositório** |
| `check-ci-workflow-pin-parity.sh` | **working tree rastreada** | `scripts/check-ci-workflow-pin-parity.sh:39` escreve e depois apaga `internal/generators/zz_dump_ci_workflow_pin_parity_test.go` **dentro do repositório** |
| `check-serve-address-parity.sh` | **portas TCP do host** | `scripts/check-serve-address-parity.sh:98` fixa `PORT=46199` e incrementa — alocação determinística, não efêmera |
| `check-barrier.sh` | git (declarado hostil) | 12 invocações de `git`, sandbox por `mktemp` |
| `check-roadmap-barrier-contract.sh` | fixtures + git (declarado hostil) | 11 invocações de `git`, 6 `sed -i`, sandbox por `mktemp` |

Os demais 39 gates **não** alteraram `bin/trackfw`, não sujaram a working tree, não criaram arquivos
em `scripts/` e isolam trabalho por `mktemp`.

Achado que corrige uma suspeita: `check-gates-falsify.sh` **não** muta `scripts/` in loco — ele
`cp` os gates de `scripts/` para dentro de um `mktemp -d` (`WORK`, linha 23) e muta a cópia. A coluna
"arquivos novos em `scripts/`" ficou em **0 para todos os 46 gates**. A hostilidade dele é o
`bin/trackfw`, não o `scripts/`.

### 2.2 Falsificação concorrente (o que o AC2 exige)

Conjunto candidato = 46 − 5 (excluídos `check-gates-falsify.sh`, `check-cli-parity.sh`,
`check-identity-parity.sh`, `check-barrier.sh`, `check-roadmap-barrier-contract.sh`) = **41 gates**.
Soma sequencial desses 41: **167,55 s**.

**A — concorrência cruzada** (os 41 simultâneos, 5 rodadas):

| Rodada | Parede | Exit codes ≠ 0 |
|---:|---:|---|
| 1 | 564,01 s | nenhum |
| 2 | 558,22 s | nenhum |
| 3 | 398,99 s | nenhum |
| 4 | 375,16 s | nenhum |
| 5 | 412,43 s | nenhum |

Veredito **estável** nas 5 rodadas. E o dado que decide o AC3: **167,55 s sequencial × 375–564 s em
paralelo — de 2,2× a 3,4× mais lento**, em 10 vCPU. Paralelismo dentro de um host não acelera este
conjunto; ele o degrada.

**B — auto-concorrência** (o mesmo gate 5× simultâneo — o teste que pega caminho fixo em vez de
`mktemp`). **39 de 41 estáveis em 0. Dois REPROVADOS, 5/5 falhas, determinístico:**

| Gate reprovado | Causa raiz nomeada |
|---|---|
| `check-serve-address-parity.sh` | `Porta 46207 já está em uso` / `[Errno 48] Address already in use` — porta TCP determinística (`PORT=46199` + incremento). Duas instâncias disputam a mesma porta. |
| `check-ci-workflow-pin-parity.sh` | `vet: open internal/generators/zz_dump_ci_workflow_pin_parity_test.go: no such file or directory` — as instâncias escrevem **o mesmo arquivo dentro da árvore rastreada** e uma apaga o dump da outra. |

🔴 Estes dois passaram na concorrência **cruzada** e falharam na **auto**-concorrência. Se a
falsificação tivesse parado no teste cruzado, os dois teriam sido classificados como seguros. **É
exatamente o flake que o AC2 manda impedir**, e ele só aparece quando duas cópias do mesmo gate
coexistem — situação que uma matriz com re-execução de shard produz naturalmente.

### 2.3 Confundimentos que NÃO devem ser lidos como achado

Registro para que ninguém releia estes dados como interferência de gate:

- **Escritas em `~/.claude`** apareceram para 7 gates na varredura sequencial. Sonda dirigida
  (executando os gates isoladamente e listando os arquivos) mostrou que **tudo era ruído da própria
  sessão do agente** (`shell-snapshots/`, `projects/`, `todos/`). **Nenhum gate escreve em `$HOME`.**
- **`git_dirty_files=1` nas rodadas 3–4** é este próprio documento (arquivo novo, não rastreado).
  **`=4` na rodada 5** inclui, além dele, uma REQ e um roadmap criados por **outra sessão** no mesmo
  working tree enquanto a medição corria, mais a linha correspondente em `.trackfw-log`. Nenhum gate
  sujou a árvore.
- Por causa disso, o `.trackfw-log` marcado como "escrito" pelo `check-gates-falsify.sh` na tabela do
  §1 **não está corroborado estaticamente** — os `roadmap move` dele rodam em sandbox. Trate essa
  única célula como não confirmada; a escrita em `bin/trackfw`, essa sim, está corroborada por leitura
  do código.

## 3. Recomendação (AC3): **nenhum dos dois mecanismos. Não vale.**

### 3.1 `make -j` — descartado, por dois motivos independentes

1. **Não funcionaria sem reescrever o alvo.** As 46 invocações são **linhas de uma única receita**.
   O `make -j` paraleliza *targets*, não linhas de receita — `make -j parity` no `Makefile` de hoje
   dá **zero** de ganho. Obter qualquer paralelismo exigiria quebrar `parity` em 46 alvos `.PHONY`,
   o que é mudança no `Makefile` e some com a lista legível que hoje serve de contrato de cobertura.
2. **A medição o falsifica de qualquer forma.** Paralelismo dentro do host deixou o conjunto
   **2,2×–3,4× mais lento** (§2.2 A) em 10 vCPU. O runner do CI tem 4 vCPU: seria pior. Somados a
   isso, dois gates reprovam sob auto-concorrência e três escrevem `bin/trackfw`.

### 3.2 Matriz de shards — tecnicamente possível, economicamente injustificada

O piso de qualquer divisão é **o gate mais lento sozinho**: **609,74 s medidos no CI** (1141,35 s
locais), 78,4 % dos gates. Distribuir os outros 43 gates por N shards não move esse piso um segundo —
e eles somam 168,4 s no CI, menos de 3 minutos para repartir.

### 3.3 Ganho — agora com o piso **medido no CI**, não estimado

O piso de qualquer sharding é o gate dominante sozinho, e ele **não é mais uma estimativa**:
`check-gates-falsify.sh` = **609,74 s medidos no runner**.

| Cenário | Tempo no CI |
|---|---:|
| Hoje (`parity` sequencial) | **803 s — 13m23** |
| Melhor caso com shards: setup 20,7 s + `build` 1,5 s + shard do `falsify` 609,74 s | ~632 s |
| … + agendamento do shard e do job agregador (~10–20 s, típico do Actions) | **~645–655 s — ~10m50** |
| **Ganho** | **~150–160 s (~2m30), ≈ 19 %** |

E o check `parity` continuaria custando **quase 11 minutos**. É esse o teto: os outros 43 gates
somam **168,4 s no CI** — pouco menos de 3 minutos —, e é tudo o que existe para distribuir.

🔴 **Ressalva que permanece:** os números acima são medição real de *um* run (`33679814232`), não a
medição *de uma distribuição implementada*. O **AC6** continua exigindo um run comparável **depois**
da mudança, caso ela seja feita. O que esta seção elimina é a incerteza sobre o piso — que era a
única variável capaz de virar a recomendação.

**Análise de sensibilidade:** a recomendação só se inverteria se a fatia do `falsify` caísse muito.
Com ele em 78,4 %, sobram ~2m30. Ele precisaria cair para **~40 % do job** para o sharding render
~7 min — e a medição diz 75,9 % do job inteiro. Não há margem para a conclusão mudar.

### 3.4 Onde o ganho realmente está — e por que não é aqui

Enquanto 78,4 % do tempo de gate (75,9 % do job) estiver num gate só, **nenhuma topologia de shard resolve o problema**. O
lever é o `check-gates-falsify.sh` (359 cenários, 9604 linhas), e mexer nele é **REQ própria**, como a
própria REQ-2026-09-02 determina no escopo negativo — é o gate que falsifica os outros 45.

**Recomendação formal:** encerrar a REQ-2026-09-02 pelo AC6 ("não deu ganho suficiente para
justificar a complexidade"), cumprindo apenas o **AC7** — reescrever o comentário do `quality.yml`
com esta medição e esta data —, e abrir REQ separada para o `check-gates-falsify.sh`. **Nenhum gate
sai do `parity`**: reduzir cobertura não está em questão.

## 4. Se a decisão for dividir mesmo assim — como o agregador `parity` tem de ser (AC4)

Descrição, **não implementação**. Pré-condição verificada ao vivo hoje:

```
$ gh api repos/kgsaran/trackfw/branches/main/protection
contexts: ["go","node","python (3.10)","python (3.12)","package-smoke",
           "windows-integrations-resolve","parity",
           "governance-install-script","governance-go-install"]
enforce_admins: true
```

`parity` está lá **por nome exato**. E os dois `python (3.10)` / `python (3.12)` são a **prova viva**
do risco: uma matriz publica o check com o sufixo entre parênteses. Um `strategy.matrix` aplicado ao
job `parity` faria nascerem `parity (1)`, `parity (2)`… e o contexto exigido `parity` **nunca mais
apareceria** — check exigido ausente é PR bloqueado para sempre, sem mensagem que aponte a causa.

Forma obrigatória:

- O job da matriz recebe um id **diferente** — ex.: `parity-shard` — e nunca o id `parity`.
- Um job **novo, sem matriz, com id exatamente `parity`**, `needs: [parity-shard]` e
  `if: always()` (sem o `always()` ele é *skipped* quando um shard falha, e **check exigido skipped
  bloqueia igual a check ausente**).
- O corpo dele é um único step que **reprova a menos que** `needs.parity-shard.result == 'success'`
  — tratando `failure`, `cancelled` **e** `skipped` como reprovação. Resultados de matriz agregam num
  único valor: basta um shard vermelho para o agregado sair de `success`.
- Guarda de cobertura (AC5): a partição dos 46 tem de ser **derivada** da mesma lista, não redigitada,
  e o agregador precisa conferir a **união** dos gates executados contra os 46 — shard que não executa
  nada é reprovação, não sucesso.
- Partição obrigatória pelos achados do §2: `check-gates-falsify.sh`, `check-cli-parity.sh` e
  `check-identity-parity.sh` **não podem** coexistir em shards concorrentes (os três escrevem
  `bin/trackfw`), e `check-serve-address-parity.sh` / `check-ci-workflow-pin-parity.sh` não podem ter
  duas instâncias vivas ao mesmo tempo.
- `scripts/check-ci-workflow-job-id-collision.sh` **não** governa o `quality.yml` — ele trava os job
  ids dos workflows que o produto *gera* (`trackfw-gate.yml` / `trackfw-validate.yml`). Não há gate
  hoje protegendo o nome do job `parity` no `quality.yml`; se a divisão acontecer, **essa proteção
  precisa ser criada**.

## 5. Achados colaterais (reportados, não corrigidos)

1. **Premissa caduca no `quality.yml`** (linhas ~547–552): "~4m15s total … abaixo de dois dígitos de
   minutos … não é o gargalo". Medição de hoje: 23m00 locais, 13m23 no CI, ~90 % do tempo de parede
   do PR. É o **AC7**. Sugestão de conteúdo para a nota nova: *"Medido em 2026-09-03 — local
   (Apple Silicon) 23m00 para 46 gates; CI (run 33679814232) 13m23, dos quais
   `check-gates-falsify.sh` = 609,74 s / 78,4 %. Sequencial mantido deliberadamente: sharding tem
   piso de ~10m50 e renderia ~2m30 — ver docs/qualidade/2026-09-03-medicao-do-alvo-parity.md."*
2. **Suspeita de vacuidade — `check-referential-integrity.sh` (0,04 s, 54 linhas).** É o único gate
   rápido **sem contador e sem guarda de vacuidade**: o corpo é `for req in docs/req/*.md; do [[ -f
   "$req" ]] || continue`. Se o glob não casar (diretório renomeado, `req_dir` diferente, execução de
   outro cwd), ele não checa nada e **imprime `Referential integrity OK`, saindo 0**. Não foi
   corrigido — só reportado, conforme instrução. Os demais gates ≤0,11 s (`check-static-assets`,
   `check-shell-posix-portability`, `check-ref-separator-portability`,
   `check-output-encoding-declared`, `check-ci-workflow-job-id-collision`,
   `check-parity-contract-coverage`) **têm** guarda de vacuidade ou emitem contagem no relatório, e
   são rápidos por serem `grep` puro — rapidez ali não é sintoma.
3. **Contagem de gates vácuos.** A auditoria anterior deste repositório apontou **4** gates vácuos;
   entre os **46 do alvo `parity`** eu encontrei **1** suspeito (o item 2 acima). Não é divergência:
   os outros 3 ou já foram corrigidos, ou estão fora do `parity` (o `make quality` roda mais que este
   alvo). Não os procurei — está fora do escopo desta medição.
4. **`check-gates-falsify.sh` NÃO escreve na árvore rastreada.** O `zz_dumpguard` dele
   (linhas 167–187) é criado em `$module_dir` dentro do `mktemp -d`, e as leituras de
   `$ROOT_DIR/internal/generators/*.go` são só leituras (`cmp`, redirecionamento **para** o sandbox).
   A hostilidade dele continua sendo só o `bin/trackfw`.
5. **`check-ci-workflow-pin-parity.sh` escreve dentro de `internal/generators/`.** Hoje ele limpa
   atrás de si e a árvore ficou limpa em todas as execuções sequenciais. Ainda assim, um gate que
   grava na árvore rastreada é um risco de poluição se ele morrer no meio (`Ctrl-C`, timeout de job).

## 6. Reprodutibilidade

- Lista de comandos: `make -n parity | grep 'scripts/check-'` (46 linhas).
- Medição sequencial: um processo por gate, `Time::HiRes` em volta, sondas de mtime/`git status` entre
  gates.
- Falsificação: 5 rodadas de 41 gates simultâneos + 5 instâncias simultâneas de cada gate.
- Tempos de CI: `gh api repos/kgsaran/trackfw/actions/jobs/100413926297/logs --allow-escape-sequences`,
  delta entre ecos consecutivos das linhas da receita. Retenção do log: 90 dias.
- Árvore em `c0f6781`, `bin/trackfw` compilado e não obsoleto, nenhum arquivo do produto alterado.
- Este documento é o **único** arquivo criado. `docs/agents-working-context.md` **não** foi tocado, de
  propósito: há PR aberto e a tarefa é somente leitura — registrar o ciclo ali geraria conflito na
  árvore alheia. Fica para o orquestrador, junto do commit.
