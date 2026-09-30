---
date: 2026-09-30
roadmap: "ROADMAP-2026-09-30-gate-de-wave-que-reentra-no-barrier-recursa-sem-limite-e-um-roadmap-vira-fork-bomb.md"
ml: ML-2A
agente: hades-tf
---

# Revisão Independente — Reentrada no `barrier` (ML-2A, Wave 2)

> ML-2A · Wave 2 · 2026-09-30

**Veredito: BLOQUEIA**

Dois achados com fix proposto (F1, F2). Um achado de documentação (F3). Uma observação de
contenção de teste (F4).

---

## Método

Leitura de `internal/commands/barrier.go` (commit 231b2f4c) e `internal/roadmapdoc/roadmapdoc.go`
antes de abrir `barrier_reentry_test.go`. Binary construído em
`scratchpad/tf`. Cada vetor executado por injeção de `TRACKFW_BARRIER_STACK` (seguro) ou, onde
necessário, por cadeia viva com fusível; em ambos os casos com `perl -e 'alarm ...'` ou timeout de
processo. Processos identificados pelos nomes reais (`trackfw_bin`, `tf`) — não pelo nome `trackfw`
que `pkill -9 -x trackfw` teria alvejado.

### Incidente de contenção do teste 6b

O gate de teste do vetor de override da variável (`TRACKFW_BARRIER_STACK=[] ./trackfw_bin barrier`)
foi escrito em múltiplas linhas numa fixture de roadmap. `ParseGates` executa cada linha como
`sh -c` separado — o `export T_FUSE=...` na linha 2 não sobrevive à linha 4 onde a chamada
recursiva fica. O fusível era ineficaz. O comando durou ~120 s até o timeout e foi movido para
background. O `pkill -9 -x trackfw` não alvejou `trackfw_bin` nem `tf` — nomes incorretos.
Verificação pós-fato: `ps -eo pid,comm | grep -E '/(trackfw_bin|tf)$'` retornou 0 processos.
Não há confirmação de que nenhum orphan sobreviveu; o check foi com o nome correto após o kill.

**Lição operacional:** o fusível T_FUSE DEVE estar na mesma linha que a chamada ao barrier.
O comentário no topo de `barrier_reentry_test.go` (linha 6-8) já documenta isso. A fixture de
roadmap neste review violou essa regra por descuido.

---

## Vetores tentados

| # | Vetor | Método | Resultado |
|---|---|---|---|
| T1 | `./` no path do argumento | Cadeia viva (fusível single-line) | PASS — EvalSymlinks(Abs) normaliza |
| T1b | `..` no path do argumento | Cadeia viva (fusível single-line) | PASS — `resolveBarrierRoadmap` usa só basename; ambos os lados resolvem o mesmo path |
| T2 | Hardlink do roadmap | Injeção de stack + cadeia viva | BYPASS confirmado (F2) |
| T3 | Symlink no diretório (não no arquivo) | Injeção direta de stack | PASS — `EvalSymlinks` resolve componente de diretório |
| T4 | Wave label case (`1b` vs `1B`) | Injeção de stack | BYPASS confirmado (F1) |
| T5 | Zero-padding (`01` vs `1`) | Injeção de stack | PASS — `SplitWaveLabel` normaliza |
| T6 | Variável duplicada no env | Construção de cmd.Env com 2 entradas | PASS — exec deduplica (última vence); `buildChildEnv` sempre produz uma entrada |
| T6b | `TRACKFW_BARRIER_STACK=[] ./tf barrier` | Cadeia viva (fuse QUEBRADO — vide F4) | BYPASS — 120s timeout; fork bomb confirmado |
| T7a | JSON com chaves extras | Injeção de stack | PASS — json.Unmarshal ignora campos extras |
| T7b | JSON com elemento `null` | Injeção de stack | PASS silencioso — unmarshal retorna slice nil; operação continua normalmente |
| T7c | Array vazio (`[]`) | Injeção de stack | PASS — clean slate, operação normal |
| T7d | Objeto JSON em vez de array | Injeção de stack | PASS — exit 2 "is malformed" |
| T8 | `TRACKFW_BARRIER_STACK=""` (string vazia) | Injeção de env | BYPASS silencioso (F3-a) — linha 566 verifica `raw != ""`; vazio → pula unmarshal |
| T9 | `TRACKFW_BARRIER_STACK=null` | Injeção de env | BYPASS silencioso (F3-b) — `json.Unmarshal("null", &slice)` → nil; sem erro, pilha vazia |
| T10 | `wave not found` vs reentrada | Injeção de stack com wave inexistente | PASS — "wave not found" tem precedência; exit 2 pela causa correta |
| T11 | Sem `--trust-local-gates` | Injeção de stack | PASS — `childEnv` propagado em ambas as branches (linhas 662 e 675) |
| T12 | `done/` vs `wip/` mesmo basename | Injeção de stack | PASS — `resolveBarrierRoadmap` retorna o mesmo path para ambos os lados; keys batem |
| T13 | APFS basename case (`ROADMAP-test.md` vs `ROADMAP-TEST.MD`) | Injeção de stack | BYPASS confirmado (F2) |

### Comandos e saídas representativas

**T4 — Wave case bypass confirmado por injeção:**
```
OUTER_KEY='[{"roadmap":"<abs>/ROADMAP-wavecase.md","wave":"1b"}]'
TRACKFW_BARRIER_STACK="$OUTER_KEY" ./tf barrier ROADMAP-wavecase --wave 1B --trust-local-gates
→ EXIT=1  (rodar; SEM "reentrant call")

TRACKFW_BARRIER_STACK="$OUTER_KEY" ./tf barrier ROADMAP-wavecase --wave 1b --trust-local-gates
→ EXIT=2, stderr: "reentrant call — ROADMAP-wavecase.md wave 1b is already being evaluated"
```

**T2 — Hardlink bypass confirmado:**
```
ln -f ROADMAP-test.md ROADMAP-test-alias.md
# Outer: chave (test.md, 1) injetada no stack
# Gate: ./tf barrier ROADMAP-test-alias --wave 1
→ EXIT=1  (rodar; EvalSymlinks("test-alias.md") ≠ EvalSymlinks("test.md"))
```

**T13 — APFS case bypass:**
```
# Go EvalSymlinks preserva o case fornecido (medido):
EvalSymlinks("ROADMAP-test.md")  -> "ROADMAP-test.md"
EvalSymlinks("ROADMAP-TEST.MD")  -> "ROADMAP-TEST.MD"
os.SameFile(lower, upper) = true

# Injeção de stack com uppercase path, run com lowercase → EXIT=1 (bypass)
```

**T8 — String vazia:**
```
TRACKFW_BARRIER_STACK="" ./tf barrier ROADMAP-test --wave 1 --trust-local-gates
→ EXIT=1  (linha 566: raw != "" → false → pula unmarshal → pilha vazia)
```

---

## Achados

### F1 — Desvio de spec: `waveLabel` (CLI arg) usado em vez de `target.Label` (header parsed) — BLOQUEIA

**Arquivo:linha:** `internal/commands/barrier.go:573`

**Spec (roadmap ML-1A, linha 133):**
```
wave = fmt.Sprintf("%d%s", SplitWaveLabel(target.Label)), assim 1b e 1-b viram a mesma chave
```

**Implementação:**
```go
currentKey := barrierReentryKey(roadmapPath, waveLabel)   // waveLabel = CLI arg
```

`waveLabel` é o argumento da linha de comando, não o label normalizado do section header.
`CompareWaveLabels` (usado para ENCONTRAR a wave) normaliza case (`1b == 1B`), mas
`SplitWaveLabel` (usado na chave) preserva case no sufixo (`1b != 1B`). Resultado:

- Outer `--wave 1b` → chave wave = `"1b"`.
- Gate `--wave 1B` → mesma wave encontrada (CompareWaveLabels) → chave wave = `"1B"`.
- Comparação `"1b" == "1B"` → false → bypass.
- L3 (gate de L2, que usa `1B`) detecta `"1B"` na pilha → reentrada detectada em L3.
- **Impacto:** 1 avaliação extra antes da detecção. Não infinita.

**Fix proposto:** usar `target.Label` em vez de `waveLabel`:
```go
// barrier.go:573 — era:
currentKey := barrierReentryKey(roadmapPath, waveLabel)
// deve ser:
currentKey := barrierReentryKey(roadmapPath, target.Label)
```

`target.Label` é o label tal como está no section header da wave, que é o mesmo label que
`CompareWaveLabels` usará ao receber `--wave 1B` para encontrar `## Wave 1b`. A normalização
de `SplitWaveLabel` sobre `target.Label` produz a mesma chave para qualquer forma que resolva
à mesma wave.

**Teste ausente:** T7b (`1b/1-b`) PASSA (correto: `SplitWaveLabel` strips hyphen → mesmo key),
mas NÃO há teste para `1b/1B` (case). Com a correção `target.Label`, ambos passam pelo mesmo
mecanismo.

---

### F2 — Identidade de roadmap por string path, não por `os.SameFile` — BLOQUEIA

**Arquivo:linha:** `internal/commands/barrier.go:574-578` (loop de comparação)

**Problema:** a comparação `entry.Roadmap == currentKey.Roadmap` compara STRINGS. Em dois casos,
`EvalSymlinks` retorna strings diferentes para o mesmo arquivo físico:

1. **Hardlink:** dois nomes, mesmo inode. `EvalSymlinks` retorna o path como dado (não resolve
   hardlinks). `os.SameFile` retornaria `true`.
   ```
   ls -li ROADMAP-test.md ROADMAP-test-alias.md
   → mesmo inode (confirmado)
   EvalSymlinks("ROADMAP-test.md")      → "ROADMAP-test.md"
   EvalSymlinks("ROADMAP-test-alias.md") → "ROADMAP-test-alias.md"
   os.SameFile → true
   ```

2. **APFS basename case (macOS):** o filesystem é case-insensitive mas preserva case.
   `EvalSymlinks` retorna o case fornecido, não o canônico do disco.
   ```
   EvalSymlinks("ROADMAP-test.md")   → "ROADMAP-test.md"
   EvalSymlinks("ROADMAP-TEST.MD")   → "ROADMAP-TEST.MD"
   os.SameFile(lower, upper)         → true  (medido)
   ```
   Em APFS, um gate que chame `trackfw barrier ROADMAP-TEST --wave 1` quando o outer foi
   chamado com `ROADMAP-test --wave 1` não é detectado (EXIT=1, confirmado).

**Fix proposto:** substituir comparação de string por `os.SameFile`:

```go
// nova função helper
func sameRoadmapFile(a, b string) bool {
    if a == b {
        return true // fast path: paths idênticos
    }
    fa, err := os.Stat(a)
    if err != nil {
        return false
    }
    fb, err := os.Stat(b)
    if err != nil {
        return false
    }
    return os.SameFile(fa, fb)
}

// No loop (barrier.go:574-578), era:
if entry.Roadmap == currentKey.Roadmap && entry.Wave == currentKey.Wave {
// deve ser:
if sameRoadmapFile(entry.Roadmap, currentKey.Roadmap) && entry.Wave == currentKey.Wave {
```

`os.SameFile` usa inode + device no Unix, volume + file index no Windows (stdlib, zero deps extras).
Custo: no máximo 4 `os.Stat` por chamada (pilha máxima = 4 entradas). Fecha hardlink, APFS case
e qualquer outro aliasing de mesmo inode.

**Nota de Windows:** `filepath.EvalSymlinks` no Windows chama `NormBase`/`toNorm` para normalizar
o case do path — ver `$(go env GOROOT)/src/path/filepath/symlink_windows.go`. Se o EvalSymlinks
conseguir resolver (sem fallback para Abs), o case do path é normalizado. O caso residual
(EvalSymlinks falha, usa Abs) ainda tem o desvio, mas `os.SameFile` fecha essa brecha também.

---

### F3 — Formas adicionais de override de variável não documentadas no residual — DOC

**Arquivo:linha:** `docs/cli-parity.md:2286-2297`

O residual documentado lista: `env -i`, `sudo -i`, `docker run` sem `-e`, `ssh`.
Testei (e confirmei) formas mais acessíveis que não estão listadas:

| Forma | Efeito | Medido |
|---|---|---|
| `TRACKFW_BARRIER_STACK=""` | linha 566 skipa; pilha vazia | EXIT=1 (bypass) |
| `TRACKFW_BARRIER_STACK=null` | `json.Unmarshal("null")` → nil; pilha vazia | EXIT=1 (bypass) |
| `TRACKFW_BARRIER_STACK=[] ./tf barrier` | shell substitui a variável; bypass + fork bomb | timeout 120s |

As três estão na mesma classe do residual declarado (autor precisa saber o nome da variável
e escrever o override intencionalmente). A diferença é que `TRACKFW_BARRIER_STACK=[] cmd`
é mais acessível que `env -i` (não apaga o resto do env) e mais facilmente deduzível de
`--help` ou da documentação. **Nenhuma requer mudança de código.** O residual deve ser
expandido para cobrir essas formas.

**Fix proposto (documentação):** em `docs/cli-parity.md`, seção `#### Residual — env-clearing`,
acrescentar ao parágrafo:
> Direct variable override achieves the same effect: `TRACKFW_BARRIER_STACK=`  (empty),
> `TRACKFW_BARRIER_STACK=null`, or `TRACKFW_BARRIER_STACK=[] cmd` all produce an empty or
> nil stack and bypass both checks. The variable name is visible in `--help` output and
> `cli-parity.md`; an author who sets it is bypassing the protection intentionally.

---

### F4 (meta) — T_FUSE ineficaz em gate blocks multi-linha — OBSERVAÇÃO

`ParseGates` (roadmapdoc.go:668) executa CADA LINHA de um gate block como `sh -c` isolado.
`export T_FUSE=...` na linha N não persiste para a linha N+1. O fusível T_FUSE DEVE estar
na mesma linha que a chamada ao barrier — que é exatamente o que `barrier_reentry_test.go:6-8`
documenta no comentário de abertura.

Neste review, escrevi um gate multi-linha com o fuse numa linha separada. Resultado: teste 6b
sem contenção. Não é bug de produto — é armadilha de autoria de teste. A documentação do
roadmap (ML-1A) mostra o fusível como "começa com `[...]`", o que pode dar a impressão de
multi-linha. **Recomendação:** adicionar ao handoff do ML corretivo uma nota explícita:
"fusível + chamada na mesma linha de gate, como em T1/T2 dos testes existentes".

---

## Checklist AC do ML-2A

- [x] Cada contorno tentado com o comando e a saída
- [x] Veredito: BLOQUEIA com ML corretivo proposto abaixo

---

## ML corretivo proposto: ML-2C

**Owner:** `apolo-tf`
**Arquivos afetados:**
- `internal/commands/barrier.go`
- `internal/commands/barrier_reentry_test.go`
- `docs/cli-parity.md`

**Ações:**

1. `barrier.go:573` — mudar `barrierReentryKey(roadmapPath, waveLabel)` para
   `barrierReentryKey(roadmapPath, target.Label)`.

2. `barrier.go` — adicionar `sameRoadmapFile(a, b string) bool` (ver F2 acima).

3. `barrier.go:574-578` — mudar `entry.Roadmap == currentKey.Roadmap` para
   `sameRoadmapFile(entry.Roadmap, currentKey.Roadmap)`.

4. `barrier_reentry_test.go` — adicionar (como subtestes de T7 ou T8/T9 independentes,
   com fuse NA MESMA LINHA):
   - T7c: `--wave 1b` (outer) vs `--wave 1B` (gate) → deve detectar reentrada
   - T7d: hardlink do roadmap → deve detectar reentrada
   - T7e: APFS basename case (`ROADMAP-test` vs `ROADMAP-TEST`) → skip se FS case-sensitive;
     deve detectar reentrada

5. `docs/cli-parity.md` § residual — adicionar formas de override direto (F3).

**Critérios de aceite:**
- T7c/T7d/T7e passam contra o binário correto
- Contra o binário ANTES de (1): T7c reprova (bypass medido)
- `go test ./internal/commands/ -run Reentry` verde
- `make quality` verde

**Reconciliação (Regra de Reconciliação do CLAUDE.md):**
- T7c afirma: `barrierReentryKey` gera a mesma chave para `1b` e `1B` quando vindos do mesmo
  section header
- T7d afirma: dois nomes hardlinked ao mesmo inode são tratados como o mesmo roadmap
- T7e afirma: em filesystem case-insensitive, paths que diferem só em case são o mesmo roadmap
