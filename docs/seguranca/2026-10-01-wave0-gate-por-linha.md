# Wave 0 — Gate por linha: medição do acervo e decisão entre (a), (b) e (c)

> Hades (hades-tf) · 2026-10-01 · REQ #491 · Branch: `fix/barrier-executa-cada-linha-do-bloco-de-gates`

---

## 1. Enumeração das superfícies

Busca ampla:
```bash
grep -rn "Gates da wave\|replace this placeholder\|Wave 0 gate" . \
  --include="*.md" --include="*.go" --include="*.sh" \
  | grep -v "_test\.go\|docs/roadmaps\|testdata\|Binary"
```

| Superfície | Arquivo | Linhas | Conteúdo sobre per-line |
|---|---|---|---|
| Constante `wave0GateFence` | `internal/generators/roadmap.go` | 45–49 | Placeholder `exit 1`. Sem aviso sobre semântica por linha. |
| Constante `wave0Block` | `internal/generators/roadmap.go` | 87 | Emite `**Gates da wave:**` + `wave0GateFence`. |
| Template roadmap no skill do arquiteto | `internal/generators/scaffold.go` | 467–471 | Idêntico. |
| Asset `barrier.md` (skill de liberação de wave) | `internal/generators/scaffold.go` | 513–554 | Checklist operacional; não descreve como escrever um gate. |
| Regra 5 | `docs/cli-parity.md` | 2620 | Define "each non-empty, non-comment line... is one gate command". Sem a consequência (sem estado compartilhado, sem construção multilinha). |
| Seção POSIX shell contract | `docs/cli-parity.md` | pós-2620 | Documenta `sh` vs `cmd.exe`. Não menciona isolamento por linha. |
| ADR-2026-09-01 | `docs/adr/ADR-2026-09-01-...` | inteiro | Decide `sh -c` vs shell do SO. Não menciona isolamento por linha. |
| README.md | `README.md` | 480 | Descreve o marcador. Sem aviso sobre per-line. |
| CHANGELOG.md | `CHANGELOG.md` | 37 | "O gate da Wave 0 aceita prosa entre `**Gates da wave:**` e a cerca de código" — trata da gramática, não da semântica per-line. Sem aviso de consequências. |
| Vault note `gates-da-wave-sao-um-comando-por-linha-2026-08-29.md` | `vault/notes/` | inteiro | **Documenta o problema completo.** Inclui sugestão de `validate`/`barrier` aviso. NÃO vinculada ao template nem à regra 5. |
| Vault note `parsegates-per-line-isolation-fuse-same-line-2026-09-30.md` | `vault/notes/` | inteiro | Documenta que `export T_FUSE` em uma linha não sobrevive para a próxima. NÃO vinculada ao template. |
| Vault note `barrier-gate-auto-referencial-vira-fork-bomb-2026-09-30.md`, linhas 49–52 | `vault/notes/` | 49–52 | **Ensinando o padrão quebrado.** O "Molde que funciona" mostra `n=$(jq...)` e `test "$n" = "14"` em duas linhas separadas — no roadmap real (#364), o mesmo gate usa `;` para unir tudo em uma linha. Copiado como escrito, a linha 2 vê `$n` vazio e falha. |

**Conclusão de completeness:** lista fechada. A vault note de 2026-08-29 documenta o mecanismo com precisão. A vault note de 2026-09-30 inadvertidamente ensina o padrão quebrado em seu "Molde que funciona" (linhas 49–52). Ambas precisam ser endereçadas: a primeira deve ser vinculada à regra 5; a segunda deve ter o molde corrigido para uma linha.

---

## 2. Modelo de ameaça

**Adversário:** o implementador apressado e o arquiteto otimista.

### 2.1 Como o defeito entra

**Caminho A — Multi-linha com `\` (line continuation).**
Bloco escrito como script com `\` no final de linhas. Testa com `bash`. Passa. ParseGates extrai linha a linha. Linhas com `$(... \` produzem exit 2. Barrier reporta "gate reprovou" indistinguível de gate de conteúdo inválido.

**Caminho B — Heredoc opener.**
`cat <<EOF` em uma linha: bash 3.2 e dash passam `sh -n` (exit 0, FN do detector). Ao executar: `sh -c 'cat <<EOF'` lê o heredoc body do próprio argumento `-c` (que termina na mesma linha) — corpo vazio, sai 0. `echo LEAK | sh -c 'cat <<EOF'` também sai 0 sem imprimir LEAK: o body vem da string `-c`, não do stdin. As linhas do body executam como comandos independentes (cada `sh -c` separado). O delimitador `EOF` produz "command not found" (exit 127) — gate bloqueia pelo delimitador, não pelo conteúdo errado. Medido: `sh -c 'cat <<EOF'` exit=0; `sh -c 'echo BODY_RAN'` exit=0; `sh -c 'EOF'` exit=127.

**Caminho C — `if ... then` / `fi` / `else` isolados.**
Produzem exit 2 de `sh -n`. Indistinguíveis de "gate reprovou".

**Caminho D — `set -eu` no topo do bloco.**
Roda em próprio subprocess. Sem efeito nas linhas seguintes.

**Caminho E — `export VAR=val` seguido de uso.**
O export não persiste. `$VAR` nas linhas seguintes herda o env do barrier (normalmente vazio).

**Caminho F — Fusível T_FUSE em linha separada da chamada ao barrier.**
Descrito em `vault/notes/parsegates-per-line-isolation-fuse-same-line-2026-09-30.md`. Fusível nunca dispara — a checagem vê `T_FUSE` não definido no barrier pai e sempre passa.

**Caminho G — Padrão `A && B` como asserção negativa.**
`grep -q "padrão" arquivo && { echo "achou" >&2; exit 1; }`: quando grep não acha (caso esperado), a lista `A && B` retorna o exit de A = 1. Barrier bloqueia quando deveria passar. Descrito em `vault/notes/gates-da-wave-sao-um-comando-por-linha-2026-08-29.md`.

### 2.2 Qual gate captura cada caminho hoje

Nenhum. Caminhos A–C chegam ao `sh -c` e produzem exit 2 (indistinguível de "gate reprovou"). Caminhos D–G chegam ao `sh -c` e produzem false green ou false red silencioso.

---

## 3. Falsificação em ambas as direções

### 3.1 Medição do acervo

**Método:** reimplementação de `ParseGates` (roadmapdoc.go:664) em Python. Divergência em relação ao Go: usa `^## ` para delimitar waves (Go usa `WaveHeadingRe = /^## Wave (\S+) /`) e não aplica `FenceMask`. Impacto prático: zero — nenhuma seção não-Wave contém `**Gates da wave:**`, e nenhum roadmap tem `## Wave` dentro de cerca no corpus. Detector de (i): `sh -n -c '<linha>'` (bash 3.2, `/bin/sh` macOS) + `cmd.endswith('\\')` para trailing-`\` puro. Detector de (ii): regex `[A-Za-z_][A-Za-z0-9_]*` para variáveis. Detector de (iii): `len(cmds) >= 2`.

````python
#!/usr/bin/env python3
"""
Reimplementação fiel de ParseGates (roadmapdoc.go:664) para medir o acervo.
Divergência em relação ao Go: wave boundary usa `^## ` (Go: WaveHeadingRe).
Impacto: nulo — nenhuma seção não-Wave contém **Gates da wave:** no corpus.
"""
import re, os, subprocess

GATES_HEADER_RE = re.compile(r'^\*\*Gates da wave:\*\*')

def parse_gates(lines):
    wave_boundaries = []
    for idx, line in enumerate(lines):
        if line.startswith('## '):
            wave_boundaries.append(idx)
    results = []
    for wb_idx, wave_start in enumerate(wave_boundaries):
        wave_label = lines[wave_start].strip()
        wave_end = wave_boundaries[wb_idx + 1] if wb_idx + 1 < len(wave_boundaries) else len(lines)
        i = wave_start
        while i < wave_end:
            if not GATES_HEADER_RE.match(lines[i]):
                i += 1; continue
            j = i + 1
            fence_start = None
            while j < wave_end:
                t = lines[j].strip()
                if t == '```bash':
                    fence_start = j; break
                if lines[j].startswith('## ') or lines[j].startswith('### '):
                    j = wave_end; break
                j += 1
            if fence_start is None or j >= wave_end:
                break
            cmds = []
            k = fence_start + 1
            closed = False
            while k < wave_end:
                if lines[k].strip() == '```':
                    closed = True; break
                line = lines[k].strip()
                if line and not line.startswith('#'):
                    cmds.append(line)
                k += 1
            if closed:
                results.append((wave_label, cmds, wave_start))
            break
    return results

def sh_n_check(cmd):
    try:
        r = subprocess.run(['sh', '-n', '-c', cmd], capture_output=True, text=True, timeout=5)
        return r.returncode == 0, r.stderr.strip()
    except subprocess.TimeoutExpired:
        return False, 'timeout'
    except Exception as e:
        return False, str(e)

def is_incomplete(cmd):
    ok, stderr = sh_n_check(cmd)
    if not ok:
        return [f'sh-n-fail: {stderr[:60]}']
    trailing = len(cmd) - len(cmd.rstrip('\\'))
    if trailing % 2 == 1:
        return ['trailing-backslash-odd (continuation, missed by sh -n)']
    return []

def is_state_dependent(cmd, prev_cmds):
    vars_set = set()
    for prev in prev_cmds:
        for m in re.finditer(r'(?:^|;|\s)(?:export\s+)?([A-Za-z_][A-Za-z0-9_]*)=', prev):
            vars_set.add(m.group(1))
        for m in re.finditer(r'\b([a-z_][a-z0-9_]*)\s*=\s*\$\(', prev):
            vars_set.add(m.group(1))
    reasons = []
    for var in vars_set:
        if re.search(r'\$\{?' + re.escape(var) + r'(?:\}|[^a-zA-Z0-9_]|$)', cmd):
            reasons.append(f'uses ${var} set by prev line')
    return bool(reasons), reasons

universes = {
    'docs/roadmaps': 'docs/roadmaps',
    'scripts/testdata': 'scripts/testdata',
    'internal/roadmapdoc/testdata': 'internal/roadmapdoc/testdata',
}
for uname, upath in universes.items():
    counts = dict(files=0, gates_files=0, blocks=0, cmds=0,
                  shn_ok=0, shn_fail=0, bi=0, bii=0, biii=0)
    state_counts = {}
    bi_files, bii_files = [], []
    for root, _, files in os.walk(upath):
        for fname in files:
            if not fname.endswith('.md'): continue
            counts['files'] += 1
            parts = os.path.relpath(os.path.join(root, fname), upath).split(os.sep)
            state = parts[0] if len(parts) > 1 else 'root'
            if state not in state_counts:
                state_counts[state] = dict(blocks=0, cmds=0, shn_ok=0, shn_fail=0, bi=0, bii=0, biii=0)
            lines = open(os.path.join(root, fname), encoding='utf-8', errors='replace').read().splitlines()
            waves = parse_gates(lines)
            if any(cmds for (_, cmds, _) in waves): counts['gates_files'] += 1
            for _, cmds, _ in waves:
                if not cmds: continue
                counts['blocks'] += 1; counts['cmds'] += len(cmds)
                state_counts[state]['blocks'] += 1; state_counts[state]['cmds'] += len(cmds)
                bi = any(is_incomplete(c) for c in cmds)
                bii = any(is_state_dependent(c, cmds[:i])[0] for i, c in enumerate(cmds))
                for c in cmds:
                    ok, _ = sh_n_check(c)
                    if ok: counts['shn_ok'] += 1; state_counts[state]['shn_ok'] += 1
                    else: counts['shn_fail'] += 1; state_counts[state]['shn_fail'] += 1
                if bi: counts['bi'] += 1; state_counts[state]['bi'] += 1; bi_files.append(fname)
                if bii: counts['bii'] += 1; state_counts[state]['bii'] += 1; bii_files.append(fname)
                if len(cmds) >= 2: counts['biii'] += 1; state_counts[state]['biii'] += 1
    print(uname, counts)
    for s, sc in sorted(state_counts.items()):
        if sc['blocks'] > 0: print(' ', s, sc)
    if bi_files: print('  bi:', sorted(set(bi_files)))
    if bii_files: print('  bii:', sorted(set(bii_files)))
````

**Contagens por universo (sem dedup entre universos):**

| Universo | MD files | Files c/ gate | Blocos | Comandos | `sh -n` OK | `sh -n` FAIL | (i) blocos | (ii) blocos | (iii) blocos ≥2 cmds |
|---|---|---|---|---|---|---|---|---|---|
| `docs/roadmaps/` | 238 | 61 | 86 | 173 | 162 | 11 | 1 | 1 | 41 |
| `scripts/testdata/` | 144 | 10 | 15 | 55 | 44 | 11 | 1 (cópia) | 1 (cópia) | 13 |
| `internal/roadmapdoc/testdata/` | 195 | 29 | 42 | 105 | 94 | 11 | 1 (cópia) | 1 (cópia) | 26 |

**Nota sobre FenceMask (resultado medido):** `ParseGates` (roadmapdoc.go:664) recebe `[waveStart, waveEnd]` de `ParseWaves` (que aplica FenceMask para wave headers), mas não aplica FenceMask internamente para gate headers ou fences. O scanner Python tem o mesmo comportamento. Medição explícita com CommonMark close-rule (mesma classe, comprimento ≥ opener, nada após):

```bash
python3 -c 'import os,re; ...'  # sonda CommonMark acima
```

Resultado: 1 `**Gates da wave:**` dentro de fence em cada universo — mesmo arquivo (`ROADMAP-2026-08-22-...`, linha 226 em docs/roadmaps/, linha 226 em scripts/testdata/, linha 226 em internal/testdata/). Todas são cópias do mesmo arquivo. Go e Python extraem da mesma forma: **delta = 0 entre scanner e Go**. O comando extraído é `exit 1  # placeholder gate fails closed...` (Gate Wave 2 de arquivo de auditoria de 2026-08-22, escrito dentro de bloco de exemplo com 4 backticks). `trackfw barrier --wave "Wave 2"` nesse arquivo com trust executaria `exit 1` e bloquearia — comportamento não intencional (gate de documentação executável). **Findingo separado, causa distinta de #491: ParseGates não aplica FenceMask para o cabeçalho e cerca do gate.** Os contadores em §3.1 incluem corretamente esse bloco (ambos Go e Python o extraem).

**Breakout por pasta de estado (`docs/roadmaps/`):**

| Estado | Blocos | `sh -n` FAIL | (i) blocos | (ii) blocos | (iii) blocos ≥2 cmds |
|---|---|---|---|---|---|
| `done/` | 63 | 11 | 1 | 1 | 38 |
| `backlog/` | 16 | 0 | 0 | 0 | 1 |
| `wip/` | 1 | 0 | 0 | 0 | 1 |
| `abandoned/` | 6 | 0 | 0 | 0 | 1 |

**O único bloco com (i) e (ii):** `done/ROADMAP-2026-08-28-gate-de-ci-pinado-na-versao-geradora-e-install-sh-honrando-trackfw-version.md`, Wave 0. Escrito em 2026-08-28, antes do ADR-2026-09-01 que formalizou a semântica por linha. Gate escrito como script multi-linha com `$(... \` continuations, `esperado="...\n..."` multilinha, `set -eu`, `if ... then ... fi`. Dos 22 comandos extraídos: 11 falham `sh -n` (fragmentos reais), nenhum com trailing-`\` puro (as backslashes são sempre acompanhadas de `$(` unclosed, que `sh -n` pega antes). Os outros 11 que passam `sh -n` são `echo "$prod"`, `exit 1` etc. — sintaticamente completos mas dependentes de `$prod`/`$esperado`/`$medido` definidos em linhas anteriores.

### 3.2 `sh -n` — gaps medidos

Testados com bash 3.2 (`/bin/sh` macOS) e `/bin/dash`:

| Vetor | `sh -n` exit | Tipo |
|---|---|---|
| `cat <<EOF` (heredoc opener) | 0 | **FN** — bash 3.2 e dash (medido) |
| `echo word \` (trailing-`\` puro) | 0 | **FN** — bash 3.2 e dash (medido) |
| `grep -q '<<EOF' f` | 0 (correto) | OK — `<<` dentro de aspas simples como argumento |
| `x=$((a<<b))` | 0 (correto) | OK — `<<` é bitshift aritmético |
| `prod=$(git ls-files ...\` | 2 | **TP** — `$(` unclosed detectado |
| `| xargs ...` | 2 | **TP** — pipe inicial |
| `if [ -n "$x" ]; then` | 2 | **TP** — `if` sem `fi` |
| `fi` (sozinho) | 2 | **TP** — `fi` sem `if` |
| `esperado="scaffold.go` | 2 | **TP** — aspas abertas |
| `grep x f &&` (trailing `&&`) | 2 | **TP** — bash 3.2 e dash capturam operador pendente |

**Consequência per-line do heredoc (medida):** `cat <<EOF` como linha isolada (`sh -c 'cat <<EOF'`) lê o body do argumento `-c` (que termina na mesma linha) — corpo vazio, sai 0. `echo LEAK | sh -c 'cat <<EOF'` sai 0 sem imprimir LEAK: o body vem da string `-c`, não do stdin. Body lines executam como comandos independentes e `EOF` produz "command not found" (exit 127) — gate bloqueia pelo delimitador.

**Supplemento para trailing-`\` puro:** verificação de **número ímpar de `\` no final** (`trailing = len(cmd) - len(cmd.rstrip('\\'))` / `trailing % 2 == 1`). Um número PAR de trailing `\` (ex.: `echo foo\\`) é um backslash literal escapado — comando completo. `endswith('\\')` seria FP nesse caso. A regra ímpar é correta e tem zero FP no corpus.

**Supplemento para heredoc:** regex simples tem FP em `grep -q '<<EOF' f` (literal dentro de aspas) e `x=$((a<<b))` (bitshift). Custo maior que o ganho: zero heredocs no corpus.

**Conclusão:** `sh -n` + trailing-`\` ímpar cobre todas as classes de fragmento presentes no corpus com zero FP. Heredoc: sem instâncias no corpus; omitir do supplemento inicial, documentar como resíduo.

### 3.3 Opção (a) — Documentar e avisar

**Falso negativo:** todos os caminhos A–G continuam invisíveis para o barrier. "Gate reprovou" e "bloco malformado" são indistinguíveis.

**Falso positivo:** zero.

**Contra-braço:** 162 comandos OK no corpus (docs/roadmaps) continuam passando. Impacto zero.

**Veredito:** correto como pré-requisito de (b). Insuficiente como única medida.

### 3.4 Opção (b) — Detectar fragmento via `sh -n` + trailing-`\`, reportar "bloco malformado"

**Falso negativo de (b):**
- Caminhos D–G (state-dep, `set -eu`, padrão `&&` como asserção negativa): passam `sh -n`. Semântica silenciosamente errada.
- Heredoc opener sem supplement: passa `sh -n`. Consequência: body executa como comandos separados, delimitador causa exit 127. Sem instâncias no corpus.

**Falso positivo de (b):** zero no corpus (162 OK, 11 FAIL, todos os 11 são fragmentos reais). Zero para trailing-`\` ímpar no corpus (as backslashes do corpus vêm acompanhadas de `$(` unclosed, que `sh -n` já captura).

**Trust check:** `sh -n` deve rodar APÓS o trust check, não antes. O trust check é o único controle que mantém bytes não confiáveis fora do `sh`. Com (b) corretamente implementado, o `sh -n` é um novo spawn de `sh` apenas para roadmaps já confiáveis — mesma superfície de ataque de hoje. Adicionalmente: se `sh -n` falhar em spawnar (ENOENT), o resultado deve ser `not_evaluated`, nunca "malformed".

**Impacto no corpus test:** `check-roadmap-barrier-contract.sh` roda sem `--trust-local-gates`; gates são `not_evaluated`. O `sh -n` em (b) não é invocado para roadmaps não confiáveis. `PINNED_CORPUS_HASH` (que cobre apenas `mls_complete` e `acceptance_evidence`, não `gates`) permanece inalterado.

**Impacto no formato de evidência:** formato `<cmd>: exit N` por linha mantido. `barrier_test.go:639` e `cli-parity.md` continuam válidos. Nenhuma mudança de contrato.

**Contra-braço (AC5):** os 41 blocos com ≥2 comandos em `docs/roadmaps` (38 em `done/`, 3 em outros) continuam produzindo evidência individual por comando. O bloco 2026-08-28, quando executado com `--trust-local-gates`, passa de "blocked: 11× exit 2" para "malformed block" — mesmo resultado observável (blocked), mensagem mais precisa.

**Veredito:** menor corte que fecha o item 3 da motivação sem mudar contrato de evidência, sem afetar corpus hash, sem regredir AC5.

### 3.5 Opção (c) — Executar bloco inteiro como script único

**Mascaramento sem `set -e`:** se o barrier não prefixa o bloco com `set -e`, o exit code é o da última linha. Gate com 3 comandos onde o 2.º falha: hoje barrier retorna blocked (linha 2 registra falha); com (c) sem `set -e` e linha 3 passando, retorna passed. **Regressão de detecção.**

**Com `set -e` obrigatório:** `set -e` tem comportamento não óbvio com pipes (`grep -q ... | tee`) e listas OR (`cmd || true` é seguro, mas `cmd || exit 1` pode surpreender). Especificação nova que não existe hoje.

**Impacto no contrato de evidência:** formato `<cmd>: exit N` por linha está fixado em `barrier_test.go:639` e `docs/cli-parity.md:2839`. Com (c), a evidência seria `<script>: exit N` (um item). O teste de unidade e a seção de contrato precisariam mudar. O corpus hash NÃO muda (hash não cobre gates), mas o Go unit test falha.

**Compatibilidade com o acervo:** 40 dos 41 blocos em `docs/roadmaps` com ≥2 cmds têm comandos independentes (37 dos 38 em `done/`). Para resultado final (passed/blocked), (b) e (c)+set-e são equivalentes. Diferença: evidência menos granular.

**Trust check e (c) — `barrier.go:239,334–335`:** o trust check compara o **arquivo inteiro byte-a-byte** contra `origin/main`. O comentário explícito (linha 239: *"exact bytes that will be executed — F1 invariant: what is proved = what executes"*; linha 334–335: *"the buffer compared is the same one parsed for gate commands"*). Sob (c), o F1 invariant ainda vale para os bytes do arquivo. Mas o que "executa" muda: `ParseGates` hoje filtra linhas `#` (comentários) e linhas vazias antes de passar ao `sh -c`. Sob (c), o bloco inteiro (incluindo comentários) passa como script — os comentários viram no-ops em shell. A superfície de ataque não aumenta: qualquer conteúdo já está nos bytes confiados. O que muda: (a) comentários executam como no-ops; (b) `\` continuations e heredoc bodies tornam-se código vivo; (c) `#` no meio do bloco pode interagir com linha anterior via continuação — não acontece hoje porque cada linha é `sh -c` independente.

**Vault note 2026-08-29:** recomenda explicitamente "ponha num script em `scripts/`" para blocos grandes — que é (c) via script externo, sem mudar o contrato do barrier. Isso é o design correto.

**Veredito:** mais invasivo que (b), requer decisão de produto sobre `set -e`, muda contrato Go unit-tested. Não é o menor corte.

---

## 4. Resíduo declarado

### 4.1 Recomendação

**Esta análise recomenda a opção (b), com o componente documental de (a) como pré-requisito.**

1. **Corpus funcional sem defeito.** Zero blocos em wip/backlog dependem de cross-line state ou de fragmento multilinha.
2. **(b) fecha o item 3 da motivação** (distinguir "bloco malformado" de "gate reprovou") sem mudar contrato de evidência.
3. **Zero FP no corpus.** Os 11 comandos que falham `sh -n` são todos fragmentos reais.
4. **(c) requer mudança de contrato** (`barrier_test.go:639`) e especificação nova sobre `set -e`.
5. **Trust check ordering** crítico: `sh -n` deve ser pós-trust-check; falha de spawn de `sh -n` → `not_evaluated`.
6. **Corpus hash:** `PINNED_CORPUS_HASH` não muda sob (b) corretamente implementado.

### 4.2 O que (b) NÃO detecta (resíduo)

- Fragmentos semanticamente dependentes mas sintaticamente completos: `echo "$prod"`, `exit 1` isolado em bloco multilinha, `[ -n "$x" ]` com `$x` de linha anterior (false green silencioso).
- Padrão `A && B` como asserção negativa (Caminho G): passa `sh -n`, executa, bloqueia quando deveria passar.
- Heredoc opener puro: passa `sh -n`; nenhuma instância no corpus. Consequência medida: `cat <<EOF` como linha isolada recebe body vazio e sai 0; body lines executam como comandos separados; delimitador `EOF` causa "command not found" (exit 127) — gate bloqueia por razão errada, não "não detectado".
- Trailing-`\` puro sem outro fragmento: passa `sh -n`; zero instâncias no corpus com este padrão isolado.
- Se `sh -n` falhar em spawnar (ENOENT), deve reportar `not_evaluated`. Implementação deve tratar o caso.

### 4.3 Vault note a corrigir

`vault/notes/barrier-gate-auto-referencial-vira-fork-bomb-2026-09-30.md`, linhas 49–52: o "Molde que funciona" mostra `n=$(jq...)` e `test "$n"` em linhas separadas. Copiado como escrito, `$n` na linha 2 herda o env do barrier (vazio) e o gate reprova. O roadmap real usa `;` na mesma linha. A nota deve ser corrigida ou deve acrescentar aviso explícito.

### 4.4 Premissas corrigidas

| Premissa original | Status | Evidência |
|---|---|---|
| "(ii) = 0: nenhum gate depende de estado entre linhas" | **Caiu para o corpus histórico.** | O 2026-08-28 `done/` usa `$prod`, `$esperado`, `$medido` (lowercase) definidos em linhas anteriores. Causa: detector original usava `[A-Z_]` em vez de `[A-Za-z_]`. Para corpus funcional (wip/backlog): 0 blocos com cross-line state. |
| "(b) é o menor corte" | **Confirmada.** | (c) muda `barrier_test.go:639` e o contrato de evidência. (b) não. Para o resultado final no corpus, (b) e (c)+set-e são equivalentes; (b) preserva o contrato. |
| "sh -n não tem FP" | **Confirmada no corpus.** 162 OK / 11 FAIL, zero FP. Gap para heredoc e trailing-`\` puro confirmado; zero instâncias desses padrões no corpus. |
| "corpus hash não muda com (b)" | **Confirmada.** Corpus hash cobre apenas `mls_complete` e `acceptance_evidence` (não `gates`). |
| "(iii) = 0 blocos dependem de 'todas as linhas rodarem quando uma falha'" | **Reformulada.** Interpretação correta: quantos blocos têm ≥2 comandos independentes cuja evidência muda sob (c)? Resposta: 41 blocos em docs/roadmaps (38 em done/, 1 em backlog, 1 em wip, 1 em abandoned). Desses, 40 dos 41 em docs/roadmaps (37 dos 38 em done/) têm comandos genuinamente independentes. Nenhum REQUER semântica de script para funcionar corretamente. |
