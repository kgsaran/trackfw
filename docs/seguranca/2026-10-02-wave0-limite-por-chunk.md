---
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-02-driver-de-falsificacao-paralelo-espera-para-sempre-por-chunk-que-nao-termina-limite-de-tempo-por-chunk-com-fail-nomeado.md"
ml: "ML-0A"
author: "hades-tf"
date: "2026-10-02"
---

# Wave 0 — Modelo de Ameaça: limite de tempo por chunk no driver de falsificação paralelo

> ML-0A do ROADMAP-2026-10-02 | Hades | Nenhuma linha de implementação

---

## 1. Completude dos sítios (pontos de espera sem limite)

O driver `scripts/run-gates-falsify-parallel.sh` tem os seguintes pontos de espera ou latência. A
questão é: qual deles é ilimitado?

### 1.1 Varredura dos pontos de espera

| linha | ponto | limite atual | risco |
|---|---|---|---|
| L131 | `$PY_BIN "$GEN" … | strip_cr > manifest.txt` | nenhum | gerador Python trava → driver pendura antes de lançar chunks |
| L132 | `cat manifest.txt >&2` | nenhum | arquivo regular, I/O normal, risco desprezível |
| L134 | `mapfile -t CHUNKS < <(find …)` | nenhum | `find` sobre um `mktemp -d`, risco desprezível |
| **L155** | **`wait "$pid"`** em laço de coleta | **NENHUM** | **⬛ o defeito: qualquer chunk travado bloqueia aqui** |
| L159/160 | `cat "$log" >&2` / `cat "$log"` | nenhum | arquivo regular (logs sempre regulares pelo `>"$log" 2>&1`), risco desprezível |
| L178 | `tail -n 1 "$log"` na guarda | nenhum | arquivo regular, risco desprezível |
| L186 | `grep … "$log"` na guarda | nenhum | arquivo regular, risco desprezível |
| L104 | `trap 'rm -rf "$WORKDIR"' EXIT` | nenhum | remoção local, risco desprezível |
| L225-226 | `cat "$WORKDIR"/chunk_*.log` no resumo | nenhum | arquivos regulares, risco desprezível |

**Sítio adicional fora do escopo declarado:**
- **`JOBS≤1` → `exec bash "$SCRIPT"` (L100):** ao executar o script original serialmente, o driver
  abandona todo o mecanismo de chunks e de guarda. Não há nenhum limite de tempo. Este caminho não
  é alterado por esta REQ (escopo declarado), mas é uma lacuna de completude: o timeout existe
  apenas no caminho paralelo. Declarado em R1.
- **Python gen (L131):** se `gen-falsify-chunks.py` travar (ex: lock de filesystem em TMPDIR), o
  driver pendura antes de lançar qualquer chunk. Este ponto ficou fora do escopo da REQ.
  Declarado em R2.

**Conclusão: o único ponto de espera ilimitado no caminho paralelo é `wait "$pid"` (linha 155).**
A lista está fechada para o caminho relevante.

---

## 2. Modelo de ameaça

O adversário aqui é o implementador apressado e o arquiteto otimista, não um atacante externo.

### 2.1 Chunk lento legítimo virando FAIL — margem de 1200 s

**Medição 1 — máquina ociosa, 8 chunks em paralelo:**

```
$ bash scratchpad/run_chunks_idle.sh 2>&1
Launched 8 chunks at Fri Oct  2 13:45:38 -03 2026
CHUNK chunk_0: rc=0 elapsed=91s  sentinel=YES OK=44 FAIL=0
CHUNK chunk_1: rc=0 elapsed=91s  sentinel=YES OK=63 FAIL=0
CHUNK chunk_2: rc=0 elapsed=102s sentinel=YES OK=54 FAIL=0
CHUNK chunk_3: rc=0 elapsed=137s sentinel=YES OK=41 FAIL=0
CHUNK chunk_4: rc=0 elapsed=137s sentinel=YES OK=55 FAIL=0
CHUNK chunk_5: rc=0 elapsed=137s sentinel=YES OK=24 FAIL=0
CHUNK chunk_6: rc=0 elapsed=137s sentinel=YES OK=20 FAIL=0
CHUNK chunk_7: rc=0 elapsed=137s sentinel=YES OK=46 FAIL=0
MAX_CHUNK_TIME=137s
DONE
```

Total: **347 OK / 0 FAIL**, todos os sentinels presentes. Máximo: **137 s** (máquina ociosa).

**Medição 2 — com carga (`go test ./internal/... -count=1` em loop), 8 chunks:**

```
$ bash scratchpad/run_chunks_loaded.sh 2>&1
Load go test loop PID=9009 started at Fri Oct  2 13:50:38 -03 2026
Launched 8 chunks at Fri Oct  2 13:50:41 -03 2026
CHUNK chunk_0: rc=0 elapsed=117s sentinel=YES OK=44 FAIL=0
CHUNK chunk_1: rc=0 elapsed=117s sentinel=YES OK=63 FAIL=0
CHUNK chunk_2: rc=0 elapsed=141s sentinel=YES OK=54 FAIL=0
CHUNK chunk_3: rc=0 elapsed=205s sentinel=YES OK=41 FAIL=0
CHUNK chunk_4: rc=0 elapsed=205s sentinel=YES OK=55 FAIL=0
CHUNK chunk_5: rc=0 elapsed=205s sentinel=YES OK=24 FAIL=0
CHUNK chunk_6: rc=0 elapsed=205s sentinel=YES OK=20 FAIL=0
CHUNK chunk_7: rc=0 elapsed=205s sentinel=YES OK=46 FAIL=0
MAX_CHUNK_TIME=205s
```

Total: 347 OK / 0 FAIL, todos os sentinels presentes. Máximo: **205 s** (com go test em loop).
Diferença em relação ao idle: **+68 s** (+50%).

**Medição 3 — JOBS=2 (chunks maiores, parcialmente carregada):**

```
$ bash scratchpad/run_chunks_jobs2.sh 2>&1
Launched 2 chunks at Fri Oct  2 13:52:48 -03 2026   # loaded run terminava ~13:54
CHUNK chunk_0: rc=0 elapsed=178s sentinel=YES OK=171 FAIL=0
CHUNK chunk_1: rc=0 elapsed=234s sentinel=YES OK=176 FAIL=0
MAX_CHUNK_TIME=234s
DONE
```

Total: 347 OK / 0 FAIL, todos os sentinels presentes. Máximo: **234 s** (JOBS=2, parcialmente
carregada — a loaded run da medição 2 encerrou ~78 s após o lançamento deste run; os dois
overlapparam). Note: o loop de carga de M2 também gerou um `go test` órfão (cleanup bugado), que
pode ter contribuído com carga residual.

A REQ documenta que o driver usa `JOBS=nproc` com teto 8; JOBS=2 só ocorre em máquina com
exatamente 2 núcleos disponíveis (não o cenário documentado: 10 CPUs / 4 vCPUs do CI). A issue
documenta que o pior medido com `make quality` foi chunk 3 em **~360 s** (6 min, com atividade de
build paralela de agentes — não apenas `go test`).
Margem conservadora: 1200 s / 360 s = **~3,3×**.

**Tabela de margem:**

| cenário | max medido | 1200 s / max |
|---|---|---|
| 8 chunks, idle | 137 s | ~8,8× |
| 8 chunks, go test em loop | 205 s | ~5,8× |
| 2 chunks, parcialmente carregado | 234 s | ~5,1× |
| pior da issue (6 min, agentes em paralelo) | 360 s | ~3,3× |

**1200 s é adequado.** A margem mínima observada é 3,3× sobre o pior caso real registrado.

**Ressalva — `sleep infinity` não existe no macOS:**

```
$ sleep infinity
usage: sleep number[unit] [...]
Unit can be 's' (seconds, the default), m (minutes), h (hours), or d (days).
```

O chunk sintético do AC2 não pode usar `sleep infinity`. Deve usar `sleep 999999` ou
`while :; do sleep 60; done`. Este é um ajuste obrigatório para ML-1A (ver §Veredito).

### 2.2 Processo da árvore que sobrevive ao kill

**Experimento `kill_exp4.sh`** (bash 5.3 do PATH; bash 3.2 não pode rodar o driver — ver §2.4):

**Descoberta central — A3 corrigida:** o grupo de processos sobrevive ao seu líder. Se o PGID for
capturado NO LANÇAMENTO (antes de `set +m`) e o kill usar esse valor, o órfão ainda está no grupo
e é alcançado.

```
--- A3-corrected: PGID recorded AT LAUNCH ---
  Launched PID=31679 PGID=31679 (captured at launch)
  After 2s: intermediate exited
  Processes still in PGID=31679: 31684  1  31679  sleep 300
  SIGTERM to -31679 sent
  Remaining sleep 300: NONE   ← órfão morto
```

**O vetor A3 só escaparia se o PGID fosse consultado depois de o pai ter saído** (quando
`ps -o pgid= -p <pid_morto>` retorna vazio). Este é exatamente o que o ajuste #2 previne.

**Método B: recursão `pgrep -P`** — não alcança o órfão re-parentado ao PID 1 (`kill_exp6.sh`):

```
--- B3: intermediate exits (orphan reparented to PID 1) ---
  Orphan under PID 1: 38743  1  38623  sleep 120
  After recursive kill: 38743  1  38623  sleep 120   ← sobrevive
```

**Tabela vetores × métodos (medida, macOS bash 5.3, kill_exp7.sh+kill_exp8.sh+kill_exp10.sh):**

| vetor | PGID group kill (`kill -TERM -- -PGID`) | recursive `pgrep -P` |
|---|---|---|
| grandchild simples (sleep 1201) | 0 sobreviventes ✓ | 0 sobreviventes ✓ |
| nohup (sleep 1202/1203, mesmo PGID) | 0 sobreviventes ✓ | 0 sobreviventes ✓ |
| pai intermediário sai (órfão, PGID original) | 0 sobreviventes ✓ | **SOBREVIVENTE** (re-parentado ao PID 1, fora da árvore pgrep) |
| `trap "" TERM` (sleep 1205) | SIGTERM: vivo; SIGKILL: 0 ✓ | não medido (requer KILL fallback também) |
| `setsid()` child (pais vivos, sleep 1209) | **SOBREVIVENTE** (PGID 47437 ≠ PGID lançado 47434) | 0 sobreviventes ✓ (pgrep atravessa cadeia de pais vivos) |

**Qual método escolher:** grupo kill. Argumento em três partes:
1. Órfãos implícitos: qualquer grandchild que não seja esperado fica re-parentado ao PID 1 mas
   mantém o PGID original — grupo kill alcança, recursão não (medido B3).
2. `setsid()` requer chamada explícita; está ausente do script e do binário Go (grep confirmado).
   Recursão alcança setsid com pai vivo, mas o vetor não existe na prática.
3. Grupo kill é uma única chamada `kill(2)` atômica; `pgrep -P` percorre a árvore iterativamente
   e pode ser ultrapassado por processos que continuam forkando.
Nota: double-fork escapa de ambos os métodos; também ausente do script e do Go.
`setsid()` é residual teórico — declarado em R3.

**Vetor A5+B5: `setsid()` (`kill_exp10.sh`, bash 5.3, python3 com subprocess):**

Launcher usa `set -m` para dar ao python seu próprio PGID (PGID=PID sob `set -m`). Python
externa faz `os.setsid()` em filho, que lança `sleep 1209` — em nova sessão (PGID≠launcher).

```
--- A5: group kill ---
  A5_PGID=47434 (outer python, lançado com set -m)
  processo setsid child (47437) PGID=47437  ← nova sessão
  sleep 1209 (47438)              PGID=47437  ← filho do setsid child
  After group kill -47434:
  47437  1  47437  python3...      ← SOBREVIVENTE (PGID diferente)
  47438  47437  47437  sleep 1209  ← SOBREVIVENTE (herdou PGID 47437)
```

```
--- B5: pgrep -P recursão (pais vivos) ---
  B5_ROOT=47459
  sleep 1209 (47463) ppid=47462  ← setsid parent vivo
  Recursive kill from B5_ROOT=47459
  After recursive kill: NONE      ← setsid alcançado via cadeia pai viva
```

**`setsid()` e grupo kill:** escapa. **`setsid()` e recursão (pais vivos):** alcançado.
Inversão exata do vetor "pai sai": grupo kill é melhor para pais que saem; recursão é melhor
para setsid com pais vivos. No entanto,
`check-gates-falsify.sh` não usa `setsid`, `nohup`, `disown` nem `set -m`, e o binário Go
também não (grep confirmado):

```
$ grep -nE '&[[:space:]]*$|nohup|setsid|disown|set -m' scripts/check-gates-falsify.sh
545:    cd "$module_dir" &&
598:    cd "$module_dir" &&
$ grep -rnE 'Setsid|Setpgid|Foreground|SysProcAttr' internal/ cmd/
(saída vazia — exit 1)
```

Sem background detachment no script nem no binário Go. `setsid()` requer chamada explícita —
ausente de ambos. O vetor A5 é residual teórico. Declarado em R3.
Nota: qualquer processo que double-fork (duas saídas do pai) escapa de ambos os métodos;
este padrão também está ausente do script e do Go.

**A4 (`trap "" TERM`):** SIGTERM sozinho não é suficiente. SIGKILL garante.

**Cenário C — driver morto por fora (Ctrl+C / `make` cancelado):**

Medição com o comportamento ATUAL do driver (sem `set -m`):

```
--- C-measurement: SIGTERM to driver PID only ---
  31721  31716  31674  sleep 300   ← chunk antes do kill
Sending SIGTERM to driver (PID only)...
  31721  1  31674  sleep 300       ← chunk SOBREVIVE (re-parentado ao PID 1)
```

```
--- C-measurement2: SIGINT to driver PGID ---
  31735  31730  31730  sleep 300   ← chunk no mesmo grupo do driver
Sending SIGINT to driver PGID...
  31735  1  31730  sleep 300       ← chunk SOBREVIVE ao SIGINT do grupo
```

**Bash ignora SIGINT para comandos assíncronos quando job control está desligado** (shell
não-interativo sem `set -m`). Um `( bash "$chunk" ) &` tem SIGINT configurado como SIG_IGN. O
chunk continua vivo mesmo quando o processo group do driver recebe SIGINT. Ctrl+C ou SIGTERM
externo ao driver deixa todos os chunks como órfãos — contribuindo para a hipótese de concorrência
com processos órfãos documentada na issue 504. A causa exata dos 3 travamentos não foi medida
(issue 504: "Nenhuma das três teve `ps` capturado no momento do travamento").

**Solução:** o driver deve instalar um `trap` para INT/TERM/HUP (não EXIT) que mata todos os PGIDs
dos chunks antes de sair. Este trap corrige um defeito existente (não compensa uma regressão de
`set -m`). Ajuste #3.

**Nota:** `kill -9` do driver (SIGKILL direto) não pode ser interceptado por `trap` — é residual
irrecuperável. Declarado em R3.

### 2.3 A guarda de conjunto segue acusando rótulo ausente de chunk morto por limite

**Experimento — log truncado (simulação de chunk morto no meio):**

```
$ bash scratchpad/test_guard.sh 2>&1

=== Testing guard on chunk_0 with COMPLETE log ===
GUARD OK: sentinel present
Labels emitted by chunk_0: 34 lines
Labels expected in manifest for chunk_0: 31
Absent labels: 0

=== Testing guard on chunk_0 with TRUNCATED log (20 lines) ===
GUARD FIRES: sentinel missing (last='OK   [falsify/sandbox-gap-e/direction-a-baseline]')
Labels emitted by truncated chunk_0: 19 lines
ABSENT: call-site-pin
ABSENT: call-site-pin/self-governed-clean
[... 13 mais ...]
Absent labels in truncated run: 15
```

**Confirmação da não-saída precoce (linha 164-166 do driver não tem `exit`):**

```bash
164: if [[ "$FAILED" -ne 0 ]]; then
165:   echo "run-gates-falsify-parallel: FALHOU ..." >&2
166: fi
```

O bloco `if FAILED` apenas emite mensagem, não sai. A guarda de conjunto (linhas 168-207) roda
**sempre**, mesmo que algum chunk já tenha falhado. Confirmado por grep do driver.

**Resultado:** um chunk morto por limite de tempo terá:
1. `FAILED=1` (via `wait "$pid"` retornando não-zero)
2. A guarda acusa: "nao chegou ao sentinela CHUNK_COMPLETE" (linha 180)
3. Os rótulos que o chunk não chegou a emitir são listados como "AUSENTE" (linha 194)
4. `COVERAGE_FAILED=1` → exit 1

Isso é **comportamento correto e fail-closed**: a guarda amplifica a falha, não a oculta.

### 2.4 Windows / MSYS (por leitura — não medido, sem VM)

Primitivas dependentes de kill:
- `kill -- -PGID` (sinal para processo group): **indisponível no Git Bash**. O MSYS mapeia `kill`
  para uma chamada Win32 mas o flag de grupo negativo não funciona com processos nativos Win32.
- `pgrep -P`: **ausente no Git Bash** por padrão.
- `ps -o pgid=`: `ps` do Git Bash não aceita `-o` (sem POSIX ps). Abortaria sob `set -euo pipefail`.
- `set -m`: tem semântica diferente em shells MSYS (job control); pode não criar grupos separados
  para processos Win32 nativos (ex: `go test` compila para Win32, não MSYS).
- `mapfile` (linha 134 do driver): **não existe no bash 3.2** (medido: `/bin/bash: mapfile: command
  not found`). O driver já requer bash 4+ pelo uso de `mapfile`. O AC5 da REQ deve declarar isso.

**Impacto no CI:** o job `parity-other-gates` (que vai rodar o novo self-test via `make parity-rest`)
roda em `ubuntu-latest`. O job `parity-falsify-shard` roda em `ubuntu-latest`. O job
`windows-full-suites` roda apenas `go test ./...` — a v8 removeu `npm test` e `pytest pypi/tests`
(confirmado em `.github/workflows/quality.yml` linhas 180+). NÃO roda `make parity-rest` nem
nenhum script de falsificação.
Portanto, o self-test de ML-1A NÃO deve ser ligado a nenhum target que seja executado no Windows.
**Se por engano for ligado**, `ps -o pgid=`, `kill -- -PGID` e `mapfile` falharão, quebrando AC6.

**Impacto local em Windows:** `make quality` local em Git Bash/MSYS não terá o timeout funcionando,
já que `kill -- -PGID` não opera. O piso de observabilidade fica sem o mecanismo de kill.
Declarado em R4.

---

## 3. Alvos de falsificação nas duas direções por sítio

### 3.1 Superfície: valor do limite (`TRACKFW_FALSIFY_CHUNK_TIMEOUT`)

**FP — dispara cedo demais (chunk lento legítimo vira FAIL):**
- Sabotagem: valor padrão muito baixo (ex: 60 s) ou conversão numérica silenciosa de um valor
  não-numérico (`$(( abc ))` avalia para 0 em bash, fazendo o chunk falhar imediatamente).
- Sabotagem: `timeout 0 bash "$chunk"` mata o chunk antes de iniciar.
- Gate de entrada: AC3 (suíte real sem override: 347 OK / 0 FAIL). Se o padrão for muito baixo,
  AC3 reprova com timeouts falsos.
- Confirmação: medição mostra máximo de 137 s (idle) / 205 s (go test em loop). Valor 1200 s
  oferece margem de 5,8× sobre o máximo medido e 3,3× sobre o pior caso documentado na issue
  (360 s, agentes em paralelo).

**FN — limite nunca dispara (travamento silencioso permanece):**
- Sabotagem: valor padrão ausente ou muito alto (ex: 9999999 s). A variável não é
  usada na lógica de wait.
- Sabotagem: implementação do watchdog usa `sleep $T` mas nunca verifica se o chunk ainda está vivo;
  ou `wait "$pid"` sem um watchdog paralelo.
- Gate de entrada: AC2 (chunk sintético com `TRACKFW_FALSIFY_CHUNK_TIMEOUT=5` deve terminar
  em ≤T+10 s com rc≠0 e linha FAIL).
- AC4: driver de `0bf66679` (sem limite) NÃO termina com o chunk sintético dentro do teto externo.
- **Precondição de vacuidade:** AC2 deve primeiro confirmar que o chunk sintético está vivo (marcador
  único detectável por `ps` — ex: `sleep 999999`, não `sleep infinity` que falha no macOS) e
  que seu contador de processos ≥ 1 ANTES de medir o timeout. Se o chunk morrer por outro motivo
  antes do limite (ex: `sleep infinity` falha com erro de uso, saindo imediatamente com rc=1),
  AC2 passaria sem ter medido nada — exatamente o falso-verde de ajuste #1.

### 3.2 Superfície: kill da árvore do chunk

**FP — kill mata processos errados (outros chunks ou o próprio driver):**
- Sabotagem: matar o PGID do driver em vez do PGID do chunk morto.
- Sabotagem: `set -m` não desligado após o background → chunks seguintes também ficam em grupos
  isolados, mas o driver não registra seus PGIDs.
- Gate: AC2 — "depois da saída, nenhum processo da árvore do chunk segue vivo (conferido por `ps`)".
  Se outros chunks forem mortos, AC3 (347 OK) reprova.

**FP — regressão de stdin: `set -m` passa stdin do driver para chunks:**
- `set -m` desativa o redirecionamento implícito de stdin para `/dev/null` que bash usa para jobs
  assíncronos. Sem `</dev/null` explícito na linha de launch, cada chunk herda o stdin do driver
  (medido: 3 bytes lidos vs 0 sem `set -m`). Por POSIX (não medido — sessão de teste sem tty),
  um background process group que tente ler stdin do tty recebe SIGTTIN e para em estado `T` —
  o mesmo sintoma de travamento que esta REQ tenta resolver, mas agora causado pelo mecanismo
  de cura.
- Gate: o self-test de ML-1A deve verificar que um chunk lançado pelo driver com `set -m` não
  consome dados do stdin do driver (acrescentar à verificação de AC2).

**FN — kill deixa sobreviventes:**
- Sabotagem: usar apenas SIGTERM sem SIGKILL de fallback (processos com `trap "" TERM` sobrevivem).
- Sabotagem: não registrar PGID do chunk no momento do launch (antes de `set +m`); tentar obter
  PGID depois de o chunk já ter saído.
- Sabotagem: não instalar trap no driver para SIGTERM/SIGINT; chunk sobrevive à morte do driver.
- Gate: AC2 afirma "nenhum processo da árvore do chunk segue vivo". Verificação pós-execução
  com `ps axo pid,ppid,pgid,command | grep <marcador>`.

### 3.3 Superfície: guarda de conjunto após timeout

**FP — guarda acusa rótulos ausentes sem defeito real (COVERAGE_FAILED falso):**
- Sabotagem: guarda roda antes de o driver registrar que o chunk foi morto por timeout; pode
  confundir "killed intentionally" com "morreu inesperadamente".
- Mitigação atual: a guarda é unconditional e amplifica a falha, não a controla. O
  `COVERAGE_FAILED=1` + mensagem "nao chegou ao sentinela CHUNK_COMPLETE" já documenta a causa.
  Não é um FP — é comportamento correto: chunk morto = cobertura perdida.

**FN — guarda não dispara para chunk morto por timeout:**
- Sabotagem: `FAILED=1` + `exit 1` antes da guarda (ex: `exit` prematuro após timeout).
- Gate verificado: `grep -n 'COVERAGE_FAILED\|FAILED'` confirma que linhas 164-166 NÃO têm `exit`.
  A guarda está em linhas 168-207, DEPOIS. Nenhuma saída precoce existe hoje — a implementação de
  ML-1A NÃO deve adicionar `exit` ao bloco de timeout antes de linha 168.

### 3.4 Superfície: FAIL nomeado e árvore de processos

**FP — linha FAIL emitida quando o chunk terminou normalmente:**
- Sabotagem: condição do timeout inverte o teste; `elapsed_time < T` em vez de `>`.
- Gate: AC3 (suíte real sem override não pode ter linhas FAIL de timeout).

**FN — chunk travado não emite linha FAIL:**
- Sabotagem: `echo "FAIL ..."` ausente no bloco de timeout, ou vai apenas para o log e não para
  stderr (o chamador `make` não verá).
- Gate: AC2 exige "linha FAIL" e "rc≠0".

---

## 4. Resíduo declarado

**R1 — JOBS≤1 não tem timeout:** quando `JOBS=1`, o driver usa `exec bash "$SCRIPT"` (linha 100),
que é o script original sem qualquer limite. Esta REQ muda apenas o caminho paralelo. Declarado
sem mitigação.

**R2 — Gerador Python (linha 131) não tem timeout:** se `gen-falsify-chunks.py` travar, o driver
pendura antes de iniciar os chunks. Este ponto foi fora do escopo da REQ. Probabilidade de
ocorrência: baixa (o gerador analisa texto, sem I/O de rede ou lock). Declarado sem mitigação.

**R3 — Dois resíduos de kill não cobertos:**
- **`setsid()` em grandchild:** qualquer processo que chame `setsid()` cria nova sessão e PGID
  separado; escapa de `kill -- -PGID` (mas alcançado por `pgrep -P` enquanto o pai estiver vivo).
  Confirmado por grep: `check-gates-falsify.sh` não usa `setsid/nohup/disown/set -m`; binário Go
  (`internal/`, `cmd/`) não tem `Setsid|Setpgid|Foreground|SysProcAttr` (saída vazia).
  `setsid` requer chamada explícita ausente de ambos. Risco residual teórico.
- **`kill -9` do driver:** SIGKILL não pode ser interceptado por `trap`; chunks ficam orphanados
  se o driver receber SIGKILL. Com `set -m`, o chunk está no seu próprio grupo e não é alcançado
  pelo SIGKILL do driver nem pelo `kill` do `make`. Um chunk travado torna-se órfão permanente —
  não "termina sozinho". Aceito como resíduo irrecuperável; quem quer terminar tudo nesse cenário
  deve enviar `kill -9 -- -PGID` para cada PGID registrado pelo driver antes de sair.

**R4 — Windows/MSYS:** `kill -- -PGID`, `pgrep -P`, `ps -o pgid=` não funcionam corretamente em
Git Bash / MSYS. A implementação de ML-1A deve condicionar ou documentar. O CI de Windows usa
`run-gates-falsify-shard.sh`, não o paralelo — AC6 não é bloqueado por isso, mas o autodesenvolvimento
local em Windows ficará sem timeout. Declarado como lacuna de plataforma.

**R5 — Overhead do watchdog com `sleep $T`:** se o watchdog é um `sleep T & WATCHDOG_PID=$!` e
o chunk termina antes, o watchdog sleep precisa ser morto para não segurar recursos. Isso é
detalhe de implementação, não ameaça de segurança, mas se não for cuidado gera processos `sleep`
órfãos acumulados. Declarado para ML-1A tratar.

---

## Veredito: APROVA COM AJUSTES

O design da REQ é sólido. O limite de 1200 s tem margem de 3,3× sobre o pior caso real registrado (360 s, issue #504,
com agentes em paralelo) e 5,8× sobre o máximo medido diretamente (205 s, go test em loop).
A guarda de conjunto permanece fail-closed para chunks mortos por timeout. A lista de
sítios de espera está fechada para o caminho paralelo.

**Mecanismo de kill recomendado:** `set -m` + `kill -TERM -- -PGID; sleep 2; kill -9 -- -PGID`,
com `trap` no driver para SIGTERM/SIGINT/HUP (não EXIT) que mata todos os PGIDs registrados. Justificativa:
- `set -m` isola o chunk em seu próprio grupo → sem risco de matar outros chunks ou o driver
- SIGTERM primeiro, SIGKILL como fallback → cobre `trap "" TERM`
- `trap` INT/TERM/HUP no driver (não EXIT, que substituiria o rm -rf da linha 104) → cobre o cenário de kill externo; sem ele, chunks orphanados acumulam-se e contribuem para o pool de processos hipotético da issue 504

**Limite recomendado:** **1200 s** (valor da REQ confirmado).

**Ajustes obrigatórios para ML-1A:**

1. **`sleep infinity` não existe no macOS e `timeout` também não (sem coreutils):**
   ```
   $ sleep infinity
   usage: sleep number[unit] [...]
   $ timeout 3 true
   zsh: command not found: timeout
   ```
   O chunk sintético (AC2) deve usar `sleep 999999` ou `while :; do sleep 60; done`. Usar `sleep
   infinity` faz o BSD `sleep` falhar com erro de uso, não com timeout — o teste passaria pelo
   motivo errado. O watchdog NÃO pode usar `timeout` ou `gtimeout` sem verificar sua presença.

2. **Registrar PGID no launch via `$!`:** com `set -m` ativo, bash atribui ao job recém-lançado um
   PGID igual ao seu próprio PID — logo `$!` **é** o PGID. Capturar imediatamente após o `&`,
   antes de `set +m`:
   ```bash
   set -m
   ( TRACKFW_ROOT_DIR="$ROOT_DIR" bash "$chunk" ) >"$log" 2>&1 </dev/null &
   CHUNK_PGIDS+=("$!")   # $! == PGID sob set -m
   set +m
   ```
   Se o PGID for consultado depois de o processo pai sair (`ps -o pgid= -p <pid_morto>` retorna
   vazio), o órfão escapa — medido em exp3. A captura via `$!` elimina a janela de corrida.

3. **Trap para INT/TERM/HUP no driver (corrige defeito existente):** instalar ANTES do loop de
   launch. **NÃO usar EXIT** — isso substituiria o `trap 'rm -rf "$WORKDIR"' EXIT` da linha 104:
   ```bash
   trap 'for pgid in "${CHUNK_PGIDS[@]:-}"; do
           kill -TERM -- "-$pgid" 2>/dev/null || true
           kill -9 -- "-$pgid" 2>/dev/null || true
         done; exit 1' INT TERM HUP
   ```
   Sem este trap, Ctrl+C ou SIGTERM ao driver deixa todos os chunks vivos como órfãos —
   medido em kill_exp4.sh C: bash ignora SIGINT para comandos assíncronos sem job control.
   Após o `wait` de um chunk retornar (timeout ou conclusão normal), o ML-1A deve varrer o PGID
   correspondente uma vez (não-bloqueante) para limpar qualquer grandchild órfão antes de remover
   o PGID do registry.
   O `kill -9` direto ao driver não pode ser interceptado — resíduo R3.
   **O self-test (AC2) deve verificar ambas as direções: SIGTERM e SIGINT ao driver não deixam
   chunks vivos.**

4. **Nenhum `exit` prematuro antes da guarda de conjunto (linhas 168-207):** o bloco de timeout
   deve setar `FAILED=1` e continuar. A guarda é o que nomeia os rótulos perdidos. Medido: sem
   `exit` no bloco atual, a guarda sempre roda.

5. **Wiring do self-test somente em targets ubuntu (não Windows):** o job `parity-other-gates`
   (`make parity-rest`) roda em `ubuntu-latest`. O self-test deve ser ligado SOMENTE a esse
   target. Se for ligado a qualquer target que o `windows-full-suites` execute, `ps -o pgid=`,
   `kill -- -PGID` e `mapfile` falharão em Git Bash, quebrando AC6.

6. **Validar o override `TRACKFW_FALSIFY_CHUNK_TIMEOUT` contra `^[1-9][0-9]*$`:**
   `$(( abc ))` avalia a 0 em bash (string não-numérica); um timeout de 0 mata o chunk
   imediatamente. Um valor vazio passa pelo `:-` de bash mas não pelo aritmético — qualquer
   conversão deve ser explicitamente validada.

7. **AC5 — declarar que o driver requer bash 4+:** `mapfile` (linha 134) não existe em bash 3.2
   (`/bin/bash: mapfile: command not found`). O self-test deve usar o mesmo bash do PATH que
   `make` usa (bash 5.x no macOS com Homebrew), não `/bin/bash`. O AC5 deve declarar isso
   explicitamente.

8. **`</dev/null` explícito obrigatório na linha de launch sob `set -m`:** sem `set -m`, bash
   redireciona stdin de jobs assíncronos implicitamente para `/dev/null`. Com `set -m` ativo, o
   job herda o stdin do driver (medido: `bash -c 'set -m; ( wc -c ) & wait' < file` → 3 bytes;
   sem `set -m` → 0 bytes). Por POSIX (não medido — a sessão de teste não tem tty), um background
   process group que tente ler stdin do tty recebe SIGTTIN — para em estado `T`, indistinguível de
   um travamento. Chunks que leem de um pipe ou de um arquivo consumiriam os dados do `make`
   silenciosamente. A linha de launch
   **deve** incluir `</dev/null` explícito:
   ```bash
   ( TRACKFW_ROOT_DIR="$ROOT_DIR" bash "$chunk" ) >"$log" 2>&1 </dev/null &
   ```
   Esta é uma regressão introduzida pelo próprio mecanismo de kill, não uma melhoria preexistente.
   O self-test de ML-1A deve verificar que um chunk lançado com `set -m` não lê dados do stdin do
   driver (acrescentar à verificação de AC2).
