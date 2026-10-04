# Wave 0 — Threat Model: Recalibração dos pesos do falsify a partir do CI

> Date: 2026-10-03 | REQ: REQ-2026-10-03 | ADR: ADR-2026-10-03 | Issue #403

---

## 1. Completude de Superfícies

**Pergunta:** a lista de superfícies no roadmap está completa?

### Consumidores de `falsify-scenario-weights.json`

```bash
grep -rn 'gen-falsify-chunks\|TRACKFW_FALSIFY_WEIGHTS\|FALSIFY_TIMING_FILE' \
  scripts/ Makefile .github/ 2>/dev/null | grep -v '.pyc'
```

| Consumidor | Mecanismo | Observação |
|---|---|---|
| `scripts/gen-falsify-chunks.py` | lê via `os.environ.get("TRACKFW_FALSIFY_WEIGHTS", DEFAULT_WEIGHTS_PATH)` (L.493) | única implementação de leitura |
| `scripts/run-gates-falsify-shard.sh` | invoca gen-falsify-chunks.py; herda `TRACKFW_FALSIFY_WEIGHTS` do ambiente | não define o var diretamente |
| `scripts/run-gates-falsify-parallel.sh` | idem; driver local | não define o var diretamente |
| `scripts/check-falsify-shard-coverage.sh` | invoca gen-falsify-chunks.py via path hardcoded (`$ROOT_DIR/scripts/gen-falsify-chunks.py`); herda `TRACKFW_FALSIFY_WEIGHTS` do ambiente | sem override via `TRACKFW_FALSIFY_GEN` |

### Consumidores de `FALSIFY_TIMING_FILE`

| Consumidor | Mecanismo |
|---|---|
| chunks gerados por `gen-falsify-chunks.py` (L.631-636) | `__falsify_timing_mark()` escreve `>> "$FALSIFY_TIMING_FILE" 2>/dev/null \|\| true` |
| `scripts/gen-falsify-scenario-weights.py` | lê o arquivo produzido acima |

### Superfícies não listadas no roadmap

1. **`check-falsify-shard-coverage.sh` herda `TRACKFW_FALSIFY_WEIGHTS` via ambiente.** Em CI, os jobs são VMs separadas — o job de agregação não herda do job de shard. Localmente, se o desenvolvedor setar `TRACKFW_FALSIFY_WEIGHTS` para um arquivo corrompido, tanto os shards quanto a guarda usam a mesma distribuição corrompida. Igual ao vetor de JSON comprometido no repo (resíduo 5).

2. **Escrita de `scripts/falsify-scenario-weights.json` não é atômica** em `gen-falsify-scenario-weights.py` (L.159: `open(out_path, 'w')` diretamente). Crash após abertura e antes do flush corromperia o arquivo. O `falsify-recalibrate.sh` (novo em ML-1A) deve usar temp + mv.

**Veredito:** a lista do roadmap cobre as superfícies operacionais normais. As lacunas acima são resíduos declarados (ver seção 4).

---

## 2. Threat Model

**Adversário:** o implementador apressado e o arquiteto otimista — quem implementa D1–D4 sem ler as ressalvas desta análise.

### T1 — Instrumentação muda veredito do shard

**Alegação do implementador otimista:** "FALSIFY_TIMING_FILE é opt-in; se não estiver setada, retorna 0. Se estiver, só escreve em arquivo."

**Medição — shard real (chunk_0, 1217 linhas):**

```bash
# OFF (controle):
SHARD_INDEX=0 SHARD_COUNT=4 \
  OUTPUT_DIR=/private/tmp/.../scratchpad/shard-off-out \
  TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 \
  bash scripts/run-gates-falsify-shard.sh \
  > /private/tmp/.../shard-off-stdout.txt 2>/private/tmp/.../shard-off-stderr.txt

# ON (tratamento):
SHARD_INDEX=0 SHARD_COUNT=4 \
  OUTPUT_DIR=/private/tmp/.../scratchpad/shard-on-out \
  FALSIFY_TIMING_FILE=/private/tmp/.../scratchpad/shard-on-out/timing_0.log \
  TRACKFW_DISABLE_EXTERNAL_COMMANDS=1 \
  bash scripts/run-gates-falsify-shard.sh \
  > /private/tmp/.../shard-on-stdout.txt 2>/private/tmp/.../shard-on-stderr.txt
```

| Métrica | OFF | ON |
|---|---|---|
| `shard_0.rc` | `0` | `0` |
| `tail -1 shard_0.log` | `CHUNK_COMPLETE 0` | `CHUNK_COMPLETE 0` |
| `wc -l shard_0.log` | `207` | `207` |
| diff dos logs completos | — | 44 linhas; todas são diferenças de path de `mktemp` (`trackfw-falsify.XXXXXX`); zero linhas sem esse padrão |
| `diff <(sort shard_0.actual OFF) <(sort shard_0.actual ON)` | — | IDENTICAL |
| `grep -c FALSIFY_TIMING shard_0.log ON` | — | `0` |
| `grep -c 'No such file' shard_0.log ON` | — | `0` |
| linhas no timing file | — | `26` (13 start + 13 end) |
| block_ids únicos | — | `13` (= `fused_blocks=13` do chunk 0) |
| todos os block_ids começam com `0-` | — | sim |
| unmatched em `gen-falsify-scenario-weights.py` | — | `0` |

**Mecanismo de isolamento:** `__falsify_timing_mark` escreve com `>>` para `$FALSIFY_TIMING_FILE` (um arquivo), não para stdout. O coletor em `run-gates-falsify-shard.sh:113` usa `grep -oE '^(OK|FAIL|PROOF)'` — linhas `FALSIFY_TIMING` começam com `FALSIFY_TIMING`, não com `OK/FAIL/PROOF`. `CHUNK_COMPLETE` é adicionado por `gen-falsify-chunks.py:699` após o último timing mark end (L.654), portanto sempre é a última linha.

**Ausência de poluição aninhada:** `check-gates-falsify.sh` não contém nenhuma invocação live de `gen-falsify-chunks.py` ou `run-gates-falsify-shard.sh`:

```bash
grep -n 'gen-falsify-chunks\|run-gates-falsify-shard' scripts/check-gates-falsify.sh \
  | grep -v '^[0-9]*:.*#'
# (sem resultado)
```

Todos os block_ids do timing file real começam com `0-` — sem aninhamento.

**Veredito:** COBERTO. Nenhuma linha de timing aparece no log; labels e rc são idênticos; CHUNK_COMPLETE permanece a última linha.

### T2 — `FALSIFY_TIMING_FILE` não chega ao processo do chunk

**Medição:**

```bash
grep -n 'env -i\|unset\|FALSIFY_TIMING' scripts/run-gates-falsify-shard.sh
# (sem resultado)
```

`run-gates-falsify-shard.sh:99`:

```bash
( TRACKFW_ROOT_DIR="$ROOT_DIR" bash "$CHUNK" ) >"$LOG" 2>&1 || RC=$?
```

Subshell herda todo o ambiente do pai, incluindo `FALSIFY_TIMING_FILE`. Nenhum `env -i` ou `unset`.

**Ordem de criação do diretório:**

```
L.96: mkdir -p "$OUTPUT_DIR"      ← diretório criado
L.99: ( ... bash "$CHUNK" ) ...   ← chunk executa, encontra diretório já existente
```

Com D1 (`FALSIFY_TIMING_FILE = ${{ github.workspace }}/falsify-shard-out/timing_N.log` e `OUTPUT_DIR = falsify-shard-out`), a L.96 cria `falsify-shard-out/` antes do chunk. O timing file pode ser criado sem erro.

**Veredito:** COBERTO.

### T3 — Redirect order: erro de dir ausente vaza para o log

**Medição:**

```bash
bash -c '
FALSIFY_TIMING_FILE=/nonexistent/dir/timing.log
printf "FALSIFY_TIMING ...\n" >> "$FALSIFY_TIMING_FILE" 2>/dev/null || true
' > stdout.txt 2>stderr.txt
# stderr.txt: "bash: line 3: /nonexistent/dir/timing.log: No such file or directory"
# stdout.txt: (vazio)
# exit: 0 (|| true garante)
```

`2>/dev/null` redireciona o stderr do comando printf, mas o erro de abertura de arquivo (`>>`) é emitido pelo bash antes do comando existir — vaza para o stderr do processo pai, que no chunk é capturado por `>"$LOG" 2>&1`. O erro apareceria em CADA chamada de mark (start e end de cada bloco), espalhado ao longo do log — não apenas no início.

**Impacto:** a mensagem "bash: line N: ... No such file or directory" apareceria no log do shard repetidamente, mas NÃO afeta labels (não começa com OK/FAIL/PROOF) nem CHUNK_COMPLETE (que vem após a última mark end). O `|| true` preserva rc=0.

**Mitigação:** D1 aponta `FALSIFY_TIMING_FILE` para dentro de `OUTPUT_DIR`, que a L.96 cria com `mkdir -p` antes do chunk. O erro nunca ocorre na configuração correta. O timing file deve usar caminho absoluto (o `${{ github.workspace }}/...` do roadmap já é absoluto), pois qualquer `cd` dentro de um cenário deslocaria um path relativo.

**Ajuste para ML-1A (AJ-T3):** o autoteste deve verificar que, quando `OUTPUT_DIR` existe antes do chunk e `FALSIFY_TIMING_FILE` aponta para dentro dele com path absoluto, nenhuma linha "No such file" aparece no log.

**Veredito:** REQUER AJUSTE (AJ-T3) — no autoteste; o código de produto (D1) já aponta corretamente.

### T4 — Fork injeta pesos via `gh run download`

**Premissa:** `--repo kgsaran/trackfw` impede download de artefatos de fork?

**Prova da topologia:** em GitHub Actions, workflows com `on: pull_request` (caso de `quality.yml`) executam no contexto do repositório BASE quando o PR vem de um fork. O `run_id` fica registrado no repositório base.

**Medição direta** (usando cli/cli como repositório público de referência com fork PRs ativos):

```bash
gh api 'repos/cli/cli/actions/runs?event=pull_request&per_page=100' \
  --jq '.workflow_runs[]|select(.head_repository.full_name!="cli/cli")
        |[.id,.repository.full_name,.head_repository.full_name]|@tsv' | head -3
# 37123041664   cli/cli   bodapatisaikrishna/cli
# 37123041658   cli/cli   bodapatisaikrishna/cli
# 37123041685   cli/cli   bodapatisaikrishna/cli
```

Run IDs de fork PRs são listados em `cli/cli` com `head_repository.full_name` = o fork. `gh run download 37123041664 --repo cli/cli` funcionaria — o run está no repo base. Para kgsaran/trackfw: `gh run download <fork-pr-run-id> --repo kgsaran/trackfw` funcionaria da mesma forma.

**Medição — run inválido e run expirado:**

```bash
gh run download 0 --repo kgsaran/trackfw --dir scratch/bogus 2>&1
# error fetching artifacts: HTTP 404: Not Found
# exit: 1

gh run download 34277875332 --repo kgsaran/trackfw --pattern 'falsify-shard-*' --dir scratch/expired 2>&1
# no valid artifacts found to download
# exit: 1
```

**Conclusão:** `--repo` é necessário (previne apontar para outro repositório) mas não exclui fork PRs que rodaram no repositório upstream. A verificação de `head_repository.full_name` é o controle correto.

**Ajuste para ML-1A (AJ-T4):** o `falsify-recalibrate.sh` deve:
1. Validar `RUN` = `^[0-9]+$` antes de qualquer chamada `gh`.
2. Verificar `head_repository.full_name == "kgsaran/trackfw"` via `gh api repos/kgsaran/trackfw/actions/runs/$RUN_ID`.
3. `null` ou ausente em `head_repository` (fork deletado) deve ser tratado como rejeição pela própria comparação de string.
4. O autoteste ganha um terceiro braço: `gh` falso que retorna `head_repo = "fork/trackfw"` → script reprova e o JSON não é tocado.
5. O `gh` falso deve verificar que `--repo kgsaran/trackfw` aparece nos args de AMBOS os comandos (`run download` e `api`).

**Veredito:** REQUER AJUSTE (AJ-T4). `--repo` sozinho é insuficiente.

### T5 — Pesos adulterados: colapso de distribuição via timing forjado

**Cadeia realista de ataque** (D2 como escrito):

1. Um fork PR abre com código válido mas timing file forjado (todas as marcas com `ts=1.0`).
2. O maintainer executa `make falsify-recalibrate RUN=<id do fork PR>` com apenas `--repo`.
3. `parse_marks` aceita `duration == 0` (apenas `duration < 0` é rejeitado, ver L.93-95 do gerador de pesos).
4. Todos os 47 labels recebem peso 0.0. JSON escrito com sucesso (exit 0).
5. O maintainer commita `scripts/falsify-scenario-weights.json` sem revisar os 47 floats.
6. Shards e guarda de cobertura leem o MESMO arquivo do checkout — guarda auto-consistente.

**Medição da cadeia completa:**

```bash
# Passo 1: forjar o timing file (substituir todos os timestamps por 1.0)
sed -E 's/ts=[0-9.]+$/ts=1.0/' shard-on-out/timing_0.log > forged.log

# Passo 2: gerar pesos a partir do forged timing
python3 scripts/gen-falsify-scenario-weights.py forged.log w-forged.json > wf.out 2>&1
# exit=0; 47 labels; valores: {0.0}

# Passo 3: gerar chunks com pesos forjados
TRACKFW_FALSIFY_WEIGHTS=w-forged.json python3 scripts/gen-falsify-chunks.py \
  scripts/check-gates-falsify.sh c-forged/ 4 > cf.out 2>/dev/null
# exit=0
# chunk=0 ... fused_blocks=47 lines=6863 weight=0.0000 n_labels=61
# chunk=1 ... fused_blocks=0 lines=0 weight=0.0000 n_labels=0
# chunk=2 ... fused_blocks=0 lines=0 weight=0.0000 n_labels=0
# chunk=3 ... fused_blocks=0 lines=0 weight=0.0000 n_labels=0
```

**Cobertura:** todos os 47 blocos vão para chunk_0. Nenhum cenário é descartado. A guarda de completude (`gen-falsify-chunks.py:605-613`) conta linhas independentemente de peso — é estrutural, não pode ser enganada por valores de peso. Com os pesos forjados comprometidos no repo, AMBOS os shards E a guarda usam a mesma distribuição → a guarda é self-consistent e passa.

**Impacto real:** disponibilidade. Shard_0 executa 4× mais cenários que os outros três, tornando-se o caminho crítico (~4× mais lento). Localmente, pode acionar o timeout por chunk em `run-gates-falsify-parallel.sh`. Sem perda de cobertura.

**NaN:** `max(weights.values())` com um NaN no primeiro lugar do dict retorna NaN. O `pessimistic = NaN` contamina TODOS os 106 rótulos sem peso calibrado (o log mostra `peso pessimista nans`). Resultado: distribuição assimétrica (27/7/7/6 blocos), ainda com todos os 272 labels atribuídos.

**Peso negativo:** mesmo comportamento do zero — todos os blocos colapsam para chunk_0 (bins vazios têm peso 0.0, que é sempre > qualquer valor negativo no LPT, então cada novo bloco vai para o bin "menos cheio" que começa em 0.0 e só decresce).

**Ajuste para ML-1A (AJ-T5):** o `falsify-recalibrate.sh` deve, ANTES do `mv` atômico, verificar que todos os pesos no JSON são `finite and > 0`:

```python
for label, w in weights.items():
    if not (math.isfinite(w) and w > 0):
        raise SystemExit(f"peso inválido para '{label}': {w}")
```

Isso fecha o vetor de timing forjado (duração 0) E o de pesos negativos/NaN inseridos manualmente. O autoteste ganha um braço com timing file onde start_ts == end_ts → script reprova e JSON não é tocado.

**Nota:** `load_weights()` em `gen-falsify-chunks.py` também seria o ponto correto para rejeitar pesos inválidos lidos de qualquer fonte (TRACKFW_FALSIFY_WEIGHTS ou default). Esse sítio está fora da lista de arquivos do ML-1A; a decisão de escopo fica com o arquiteto.

**Veredito:** REQUER AJUSTE (AJ-T5). O impacto é disponibilidade, não perda de cobertura. A cobertura é garantida por construção (guarda de completude independe de peso; pack_lpt não filtra por valor).

### T6 — Timestamp malformado no arquivo de marcas

**Medição:**

```bash
echo "FALSIFY_TIMING phase=start block=0-1 labels=foo ts=1.2.3" > timing-bad.log
echo "FALSIFY_TIMING phase=end block=0-1 labels=foo ts=1791000000.0" >> timing-bad.log
python3 scripts/gen-falsify-scenario-weights.py timing-bad.log out.json > /dev/null 2>&1
echo "exit: $?"
# exit: 1
# ValueError: could not convert string to float: '1.2.3'
```

O regex `ts=([0-9.]+)` aceita `1.2.3` (não âncora a exatamente um ponto decimal), mas `float('1.2.3')` falha em `parse_marks:85`. O script sai com exit 1 antes de abrir o arquivo de saída.

**Mitigação necessária (AJ-T6):** o `falsify-recalibrate.sh` deve usar escrita atômica para o JSON final: escrever para arquivo temporário, então `mv`. Assim qualquer crash no pipeline `gen-falsify-scenario-weights.py` não corromperia o arquivo versionado.

**Veredito:** REQUER AJUSTE (AJ-T6): temp + mv no `falsify-recalibrate.sh`.

### T7 — Shard parcial, run com < 4 shards, ou run expirado

**Cenários:**

1. **Run expirado (>7 dias):** `gh run download` retorna "no valid artifacts found to download", exit 1. O script deve checar o exit code.
2. **Run com < 4 shards (job não rodou):** `gh run download --pattern falsify-shard-*` baixa apenas os artefatos existentes. Timing files ausentes não disparam erro do `gh`. O script deve verificar que todos os N `timing_<n>.log` estão presentes e não-vazios.
3. **Shard que falhou antes de gravar qualquer marca:** o step de upload usa `if: always()`, então o artefato é subido mesmo com shard rc != 0. O timing file pode estar presente mas vazio, ou ausente se o shard falhou antes de criar `OUTPUT_DIR`. Dados de tempo de um run com shard reprovado são não-confiáveis.

**Ajuste para ML-1A (AJ-T7):** o `falsify-recalibrate.sh` deve verificar:
- Exit code de `gh run download` != 0 → abortar.
- Para cada n=0..FALSIFY_SHARD_COUNT-1: `timing_<n>.log` existe e tem tamanho > 0.
- Para cada n: `shard_<n>.rc` contém `0` (shard que falhou produz timing de run incompleto).

**Veredito:** REQUER AJUSTE (AJ-T7).

### T8 — D4 linha de resumo: M=0 e canal errado

**D4** define: `gen-falsify-chunks.py` deve emitir `N de M rotulos sem peso calibrado (X%)` em stderr.

**Dois riscos de implementação:**

1. **M=0 (ZeroDivisionError):** só ocorreria se `check-gates-falsify.sh` não tivesse nenhum bloco de asserção. Na prática, a guarda `"nenhuma fronteira '# Cenário N'"` (L.202) já reprova antes. O implementador deve guardar com `if M == 0: imprimir "0 de 0 rótulos sem peso calibrado (n/a)"` para robustez.

2. **Canal errado (print() em vez de sys.stderr.write()):** se a linha for para stdout, vai para `manifest.txt` via pipe. Os leitores de manifest só consomem `chunk=N label=` e `chunk=N label_glob=` — a linha seria silenciosamente ignorada. Não é defeito de cobertura, mas viola D4.

**Ajuste para ML-1A (AJ-T8):** o autoteste deve capturar stdout e stderr separadamente e verificar:
- A linha de resumo está em stderr.
- Stdout NÃO contém a linha de resumo.
- Com pesos sintéticos onde 1 rótulo está ausente, N=1 e X=1/M*100 (não ZeroDivisionError).

**Veredito:** REQUER AJUSTE (AJ-T8) — no autoteste.

---

## 3. Alvos de Falsificação nas Duas Direções

### Superfície: instrumentação no CI (D1)

| Direção | Alvo de falsificação | Gate que captura |
|---|---|---|
| FN — timing não grava | Remover `FALSIFY_TIMING_FILE` do step | `falsify-recalibrate.sh` exit 1 (timing file ausente) |
| FN — timing grava no dir errado | Apontar `FALSIFY_TIMING_FILE` fora de `OUTPUT_DIR` | `falsify-recalibrate.sh` verificação de presença (AJ-T7) |
| FP — timing file presente mas vazio | Shard falhou antes de qualquer bloco | `falsify-recalibrate.sh` verificação de tamanho + rc (AJ-T7) |

### Superfície: `make falsify-recalibrate` / `falsify-recalibrate.sh` (D2)

| Direção | Alvo de falsificação | Gate que captura |
|---|---|---|
| FN — fork injeta pesos | Passar run_id de fork PR via `--repo` | Verificação de `head_repository.full_name` (AJ-T4) |
| FN — run expirado silencioso | Passar run_id > 7 dias | `gh run download` exit 1; verificar timing files (AJ-T7) |
| FN — shard parcial aceito | Shard com rc != 0 mas timing presente | Verificar `shard_<n>.rc == 0` (AJ-T7) |
| FN — timing forjado com duração 0 | `ts=1.0` em todos as marcas | Rejeitar peso ≤ 0 ou não-finito (AJ-T5) |
| FP — JSON corrompido após crash | Crash parcial na escrita | Temp + mv atômico (AJ-T6) |
| FN — `--repo` omitido | Quem remove `--repo` do script | Autoteste: `gh` falso valida `--repo` nos args (AJ-T4) |

### Superfície: pesos no JSON (qualquer atualização de `scripts/falsify-scenario-weights.json`)

| Direção | Alvo de falsificação | Gate que captura |
|---|---|---|
| FN — colapso zero/negativo | Timing forjado com duração 0 ou negativa | Rejeitar peso ≤ 0 (AJ-T5); sem isso: guarda self-consistent passa |
| FN — NaN envenena pessimístico | NaN em qualquer peso | Rejeitar não-finito (AJ-T5); sem isso: distribuição assimétrica, sem perda de cobertura |
| FN — string causa abort do shard | String em valor de peso | `float()` → ValueError → gen exit 1 → shard exit 1 → guarda de cobertura detecta rc != 0 |

### Superfície: linha de resumo (D4)

| Direção | Alvo de falsificação | Gate que captura |
|---|---|---|
| FP — linha vai para stdout | `print()` em vez de `sys.stderr.write()` | Autoteste captura stdout/stderr separadamente (AJ-T8) |
| FN — linha ausente com N > 0 | Omitir a linha no branch de rótulos ausentes | Autoteste com pesos sintéticos com 1 rótulo ausente (AJ-T8) |

---

## 4. Resíduo Declarado

1. **Fork PR com timing forjado não testado com run real.** A cadeia foi provada estruturalmente (fork PR runs registrados no base repo confirmados via `gh api` em cli/cli) e a geração de pesos zero foi medida de ponta a ponta. Não existe fork PR ativo em kgsaran/trackfw para testar o download real. O AJ-T5 (rejeitar peso ≤ 0) fecha o vetor independente da verificação de fork.

2. **`load_weights()` em `gen-falsify-chunks.py` não valida valores.** Um `TRACKFW_FALSIFY_WEIGHTS` apontando para arquivo com zeros/NaN não é rejeitado na leitura — aceito silenciosamente. O AJ-T5 fecha o sítio do recalibrador; `load_weights()` (fora do escopo do ML-1A) permanece aberto para quem usar `TRACKFW_FALSIFY_WEIGHTS` diretamente.

3. **Envelhecimento dos pesos entre recalibrações.** Os pesos são medidos na VM do CI (Linux, 4 vCPU). O balanceamento é por peso relativo; a transferência para máquinas com outro perfil não está medida. A linha de resumo (D4) torna o número de rótulos não-calibrados visível; não impede o envelhecimento.

4. **NaN injeta distribuição assimétrica sem reprova.** Com NaN no dicionário, `max()` retorna NaN (dependendo da posição na iteração), contaminando o peso pessimístico para todos os 106 rótulos sem calibração. Impacto: desequilíbrio de performance (chunk_0 com ~3,8× mais blocos). Nenhum cenário é perdido. O AJ-T5 fecha o vetor de inserção via recalibrador; inserção manual direta no JSON permanece residual (requer PR aprovado).

5. **`check-falsify-shard-coverage.sh` herda `TRACKFW_FALSIFY_WEIGHTS` do ambiente.** Em CI, os jobs são VMs separadas — sem herança entre shard e aggregation jobs. Localmente, se o desenvolvedor exportar `TRACKFW_FALSIFY_WEIGHTS` apontando para arquivo comprometido, a guarda usa a mesma distribuição e passa. Aceitável: `TRACKFW_FALSIFY_WEIGHTS` é um override de desenvolvimento; o valor em produção é `scripts/falsify-scenario-weights.json` do checkout.

---

## Veredito por Cenário — Sumário para ML-1A

| Cenário | Veredito | Ajuste para ML-1A |
|---|---|---|
| T1 — Instrumentação muda veredito do shard | COBERTO | — |
| T2 — `FALSIFY_TIMING_FILE` não chega ao chunk | COBERTO | — |
| T3 — Redirect order vaza erro com dir ausente | REQUER AJUSTE | AJ-T3: autoteste verifica ausência de "No such file" no log; path absoluto para `FALSIFY_TIMING_FILE` |
| T4 — Fork injeta pesos via `gh run download` | REQUER AJUSTE | AJ-T4: verificar `head_repository.full_name`; autoteste braço 3; validar `^[0-9]+$` no RUN |
| T5 — Timing forjado gera pesos zero (colapso) | REQUER AJUSTE | AJ-T5: rejeitar peso ≤ 0 ou não-finito antes do mv; autoteste com start==end |
| T6 — Timestamp malformado no timing file | REQUER AJUSTE | AJ-T6: temp + mv atômico no `falsify-recalibrate.sh` |
| T7 — Shard parcial / < 4 shards / expirado | REQUER AJUSTE | AJ-T7: verificar timing_<n>.log presença + tamanho + shard_<n>.rc == 0 |
| T8 — D4 linha de resumo (M=0, canal errado) | REQUER AJUSTE | AJ-T8: autoteste captura stdout/stderr; guarda M == 0 |
