# Censo de Windows — o chunk que morre em silêncio, e a apuração que morre no shard limpo

> ML-0A (Wave 0) do `ROADMAP-2026-09-23-a-apuracao-do-censo-morre-no-shard-limpo-e-os-19-rotulos-ausentes-vem-de-um-unico-chunk-que-morre-em-silencio.md`
> Autor: `hades-tf` (revisão de segurança) · Data: 2026-09-23
> Corpus: run `35872779844` (commit `11eb7ff09ee43b9a248eead314ac4c0ff0c9f616`), 8 artefatos + log do job `censo (1)`
> Plataforma de medição local: macOS/ARM64 (`bash` explícito, nunca `zsh`)
>
> 🔴 Este documento é **medição**. Nenhuma linha de implementação foi escrita neste ML.

---

## 0. Sumário dos vereditos

| | o que a REQ afirmava | veredito deste ML |
|---|---|---|
| **A** — captura `$(grep -ac … \|\| echo 0)` | 4 sítios (a) · 3 (b) · 2 (c) | **Confirmada quanto ao defeito** (4 sítios (a), exatos). **Refutada quanto à população e a uma classificação:** a família tem 12 sítios, não 9, e `gen-falsify-chunks.py:559` está classificado como "fixture/corpus" sendo código executável |
| **B** — chunk 1 morre em silêncio | mecanismo desconhecido | **Camada de silêncio: identificada e reproduzida** (`set -e` + `2>/dev/null` sobre `git`, `chunk_rc=128`). **Causa da falha do `git`: NÃO identificada** — lista de eliminações e sonda decisiva abaixo |
| **12 controles sem prova** | 12 | **15** controles de segurança entre os 19, não 12 — a aritmética está na §4.1; a REQ subconta em 3. Destes 15, **11 ficam sem prova alguma**, 1 parcial, 3 cobertos |
| achado novo (mesma causa, mesma REQ) | — | 🔴 **67 rótulos literais são invisíveis à guarda de cobertura**, 29 deles de controle de segurança. O chunk 1 é a instância visível de um buraco muito maior |

---

## 1. Causa B — mecanismo

### 1.1 O dado que não estava no artefato

O artefato `falsify-shard-1/shard_1.log` tem **3 linhas** e termina, sem `\n` extra nem qualquer
diagnóstico, em `OK   [falsify/barrier/blocked-not-detected]`. O log do **job** (não do artefato)
carrega o número que faltava:

```
censo (1) … run-gates-falsify-shard: shard 1/8 -- chunk_rc=128 guarda_local_falhou=1 -> final_rc=1
```

Comando que o produziu:

```bash
gh run view --job 107221243440 --log | grep -n 'chunk_rc\|GUARDA LOCAL'
```

🔴 **`chunk_rc=128`.** Esse número é o discriminante — e ele elimina quase tudo sozinho.

### 1.2 Reprodução exata do particionamento

Para garantir que o "chunk 1" medido aqui é o chunk 1 do CI, os três insumos foram extraídos do
commit do run, não do `HEAD` (o `check-gates-falsify.sh` mudou `+50/-2` desde então):

```bash
git show 11eb7ff0:scripts/check-gates-falsify.sh     > at_sha/check-gates-falsify.sh
git show 11eb7ff0:scripts/gen-falsify-chunks.py      > at_sha/gen-falsify-chunks.py
git show 11eb7ff0:scripts/falsify-scenario-weights.json > at_sha/falsify-scenario-weights.json
python3 at_sha/gen-falsify-chunks.py at_sha/check-gates-falsify.sh c8sha 8 > manifest_sha.txt
grep '^chunk=1 ' manifest_sha.txt
```

Resultado: **21 rótulos literais + 1 glob**, byte-a-byte o mesmo conjunto que o log do CI lista como
`AUSENTE` (19) mais os 2 emitidos. Reprodução do particionamento: **exata**.

### 1.3 O que roda depois do último rótulo vivo

Os marcadores de bloco do chunk mostram que `barrier/blocked-not-detected` **não é um bloco
sozinho**:

```
chunk_1.sh:1477  __falsify_timing_mark start "1-1460" "barrier/blocked-not-detected"
chunk_1.sh:1624  __falsify_timing_mark start "1-1636" "roadmap-acceptance-heading/go/,…"
```

Entre a linha 1477 e a 1623 do chunk vivem **dois** cenários do fonte: o Cenário 13
(`barrier/blocked-not-detected`, que emitiu seu `OK`) e, colado nele, o **Cenário 18 —
`no-repo-mutation`** (`check-gates-falsify.sh:1478-1603`). O chunk morreu **dentro do Cenário 18**:
ele nunca emitiu `OK   [falsify/no-repo-mutation]` (linha 1603) e nada do bloco seguinte apareceu.

### 1.4 O único candidato que sobrevive aos dois filtros

Filtro 1 — **`rc=128`**. Medido localmente (`bash` explícito):

```
$ git add -A                     # fora de repositório   -> rc=128
$ git -C /nao-existe-xyz add -A                          -> rc=128
$ git write-tree                 # fora de repositório   -> rc=128
$ git status --porcelain         # fora de repositório   -> rc=128
```

`128` é o código de **erro fatal do `git`** e de mais nada na região: `cp`/`rm`/`mkdir` falham com 1,
o `python3` de `remove_roadmap_acceptance_heading` sai com 1 (`SystemExit` com string), `go build`
está dentro de `set +e` com captura de status.

Filtro 2 — **silêncio absoluto**. O chunk roda com `>"$LOG" 2>&1`: **todo** stderr vai para o log.
Logo, o comando que abortou não pode ter escrito em stderr. Isso elimina todo `git` da região exceto
os que têm `2>/dev/null` colado.

Cruzando os dois filtros sobre `check-gates-falsify.sh:1478-1603`, comando a comando:

| linha | comando | rc possível 128? | stderr chega ao log? | sobrevive? |
|---|---|---|---|---|
| 1554 | `mkdir -p "$_MUTATION_COPY"` | não (1) | sim | não |
| 1555 | `cp -R "$ROOT_DIR/." …` | não (1) | sim | não |
| 1556 | `rm -rf …/.git` | não (1) | sim | não |
| 1557 | `git … init -q` | sim | **sim** (`-q` só cala stdout) | não |
| **1559** | **`git … add -A 2>/dev/null`** | **sim** | **não** | 🔴 **SIM** |
| 1560 | `git … commit -q -m …` | sim | **sim** | não |
| 1564 | `_initial_porcelain=$(git … status --porcelain)` | sim | **sim** | não |
| 1572 | `_baseline_oid=$(git … write-tree)` | sim | **sim** | não |
| 1579 | `( … bash "scripts/$_bname" ) >"$_log" 2>&1 \|\| _gate_exit=$?` | — | guardado por `\|\|` | não |
| **1583** | **`git … add -A 2>/dev/null`** (dentro do laço) | **sim** | **não** | 🔴 **SIM** |
| 1584 | `_after_oid=$(git … write-tree)` | sim | **sim** | não |

Sobram **exatamente dois sítios**, e são o mesmo comando escrito duas vezes.

### 1.5 A assinatura, reproduzida

```bash
$ cat /tmp/se128.sh
set -euo pipefail
echo "OK   [falsify/antes]"
git -C /nao-existe-xyz add -A 2>/dev/null
echo "NUNCA CHEGA"

$ bash /tmp/se128.sh > /tmp/se128.out 2>&1 ; echo "rc=$?"
rc=128
$ cat /tmp/se128.out
OK   [falsify/antes]
```

Log termina no último `OK`, zero diagnóstico, `rc=128`. **É a assinatura do shard 1, byte por byte.**

### 1.6 Entre os dois sítios, qual

**Sítio provável: 1559 (o `git add -A` de baseline).** Evidência: o 1583 é o mesmo comando, no mesmo
diretório, segundos depois — para ele falhar e o 1559 não, a falha teria de ser criada pelo primeiro
gate do laço; e nenhum `WARN [falsify/no-repo-mutation]` (linha 1597) aparece no log, que é o que um
gate do laço teria produzido ao sair `!= 0`. **Não discriminado** — a sonda da §1.8 decide.

### 1.7 Por que o shard não denuncia a própria morte

Essa é a parte que **não é acidente de plataforma** e que precisa ser corrigida independentemente da
§1.6. O `set -e` mata o processo do chunk **antes** do epílogo que o gerador anexa em
`gen-falsify-chunks.py:552-566`:

1. o bloco `if [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]]` que converte tally > 0 em `exit 1` com a
   linha `N cenário(s) reprovaram` — **nunca roda**;
2. `echo "CHUNK_COMPLETE 1"` — **nunca roda**.

Por isso o shard 1 é o único dos 8 sem **nenhum** dos dois sentinelas. A guarda local do
`run-gates-falsify-shard.sh` percebe (ela reporta `nao chegou ao sentinela` e lista os 19 ausentes),
mas o **censo** não lê nem o `.rc` nem o stderr do job: ele conta `^OK`/`^FAIL` no `.log`. Para o
censo, um chunk morto e um chunk limpo são indistinguíveis — **exceto** pelo fato de que a causa A
(§2) já tinha matado a apuração antes disso.

### 1.8 O que foi eliminado sobre *por que o `git` falhou* — e o que fica em aberto

🔴 Nenhuma destas é apresentada como causa. É a lista de eliminações, com a medição de cada uma.

| hipótese | medição | veredito |
|---|---|---|
| `TRACKFW_DISABLE_EXTERNAL_COMMANDS=1` (posto pelo job, `windows-census.yml:377`) atinge o `git` da região | `grep -rn TRACKFW_DISABLE_EXTERNAL_COMMANDS --include='*.go' internal/` → único leitor é `internal/discover/discover.go:37`, no binário Go. Nenhum script da região o lê; `check-barrier.sh:52` e `check-release-tag-parity.sh:134` fazem `unset` no próprio corpo | **eliminada** |
| `cp -R` falhou e o `git add` tropeçou na cópia parcial | `cp` escreve em stderr e sai 1; nada apareceu no log, e o rc é 128 | **eliminada como causa direta**; não elimina cópia parcial *silenciosa*, que não tem mecanismo conhecido |
| `git commit` sem identidade | `-c user.email` e `-c user.name` estão explícitos nas linhas 1558/1560 | **eliminada** |
| Falta de `python3`/helper indefinido (o modo de falha documentado no docstring do gerador) | o docstring descreve `exit 2`/`127`, não 128; e o bloco morto não chega a chamar helper de outro chunk | **eliminada pelo rc** |
| `MAX_PATH` (260) — `cp` do MSYS copia via API de caminho longo, `git-for-windows` com `core.longpaths=false` recusa com `Filename too long` (fatal → 128) | caminho relativo mais longo do repo medido: **198 chars** (`internal/roadmapdoc/testdata/corpus/.../ROADMAP-2026-09-17-jira-base-url-….md`); somado a um `$TMPDIR` de runner (~40-54) fica na faixa 240-255 — **abaixo** de 260, mas sem folga | 🟡 **em aberto, enfraquecida.** Não medida: o valor efetivo de `core.longpaths` na imagem do runner |
| "é Windows, genericamente" | `windows-census.yml:18` registra a linha de base ARM64 de 2026-09-08 como `1=25` FAILs — o shard 1 **produziu 25 FAILs** num Windows, e agora produz 2 OKs e morre. ⚠️ Ressalva obrigatória: o particionamento pode ter mudado desde 2026-09-08, então "shard 1" pode não ser o mesmo conjunto de cenários | **enfraquecida**: a morte é *nova* relativa a um Windows, o que também enfraquece MAX_PATH (os caminhos longos já existiam na VM) |
| `safe.directory` / "dubious ownership" (fatal → 128) | não medida | 🟡 **em aberto** |

**Sonda decisiva para a Wave 2** (nomeada, não "precisa de runner"): um job `windows-latest` que,
após `npm ci`, execute **verbatim** `check-gates-falsify.sh:1553-1572` com o `2>/dev/null`
**removido** dos dois `git add -A`, ecoando `rc` e stderr por comando. Essa sonda responde as duas
perguntas de uma vez: *qual dos dois sítios* e *por que o `git` falhou*.

---

## 2. Causa A — enumeração confirmada no defeito, refutada na população

### 2.1 Comando que produziu a lista da REQ (reproduzido)

```bash
grep -rn '|| echo "\?0"\?' .github/workflows/ scripts/ Makefile
```

→ **9 sítios**, os mesmos da REQ. Classificação pelo critério *"o comando já emite saída no caminho
de falha"*:

| # | sítio | comando | classe | razão |
|---|---|---|---|---|
| 1 | `.github/workflows/windows-census.yml:485` | `grep -ac '^FAIL'` | 🔴 **(a)** | `grep -c` imprime `0` **e** sai 1 → captura vira `$'0\n0'` |
| 2 | `windows-census.yml:486` | `grep -ac '^OK'` | 🔴 **(a)** | idem |
| 3 | `windows-census.yml:564` | `grep -ac '^FAIL'` | 🔴 **(a)** | idem |
| 4 | `windows-census.yml:565` | `grep -ac '^OK'` | 🔴 **(a)** | idem |
| 5 | `.github/workflows/check-annotations.yml:91` | `jq 'length' … 2>/dev/null` | (b) | `jq` não escreve em stdout no erro |
| 6 | `scripts/check-gates-falsify.sh:7236` | `wc -l < … 2>/dev/null` | (b) | redirect falho ⇒ comando não roda ⇒ stdout vazio |
| 7 | `scripts/check-gates-falsify.sh:7251` | `wc -l < … 2>/dev/null` | (b) | idem |
| 8 | `scripts/gen-falsify-chunks.py:559` | `wc -l < … 2>/dev/null` (literal Python que **gera** shell) | (b) | 🔴 a REQ classificou como **(c) "fixture/corpus, não é código"** — está **errado**: é o template do epílogo que roda em **todo** chunk de **todo** shard. Não é defeito (`wc -l` é seguro), mas a classificação é |
| 9 | `scripts/testdata/roadmap-barrier-corpus-snapshot/ROADMAP-2026-08-08-….md:52` | dentro de markdown de corpus | (c) | fixture de verdade |

**Veredito:** o número de defeitos **(a) = 4 está correto e os 4 sítios são exatamente os da REQ**.
A partição `4/3/2` só fecha porque o sítio 8 foi posto em (c). A partição correta é **4 (a) · 4 (b) ·
1 (c)**.

### 2.2 O que a forma da REQ não pegou

```bash
grep -rnE '\$\(([^()]*)\|\|[^()]*\)' .github/workflows/ scripts/ Makefile   # qualquer || dentro de captura
grep -rnE '\$\([^)]*grep +-[a-z]*c[a-z]* ' .github/workflows/ scripts/ Makefile   # grep -c/-ac em captura
```

Três sítios da mesma família que o `|| echo` **não enxerga** — todos já na **forma correta**, e por
isso valiosos: são a prova de que a correção proposta pelo ML-1A é a que o repositório já pratica.

| sítio | forma | classe |
|---|---|---|
| `scripts/check-git-branch-guard-hook-schema.sh:528` | `$(printf … \| grep -c . \|\| true)` | (b) — `\|\| true` não acrescenta linha |
| `scripts/run-gates-falsify-parallel.sh:225` | `$(cat … \| grep -c '^OK' \|\| true)` | (b) |
| `scripts/run-gates-falsify-parallel.sh:226` | `$(cat … \| grep -c '^FAIL' \|\| true)` | (b) |

População real da família: **12 sítios**, não 9.

Varreduras adicionais, **todas com resultado negativo** (nenhum sítio (a) novo):

- `|| printf`, `|| cat`, `|| wc`, `|| head`, `|| tail`, `|| jq`, `|| awk`, `|| sed`, `|| grep` →
  `grep -rnE '\|\| *(printf|cat|wc|head|tail|seq|yes|jq|awk|sed|grep)\b'` → 1 ocorrência, que é um
  `if grep … || grep …` (não é captura).
- captura com `2>&1` (onde o stderr **vira** stdout no caminho de falha) →
  `grep -rnE '\$\([^)]*2>&1[^)]*\|\|'` → 10 ocorrências, **todas `|| true`**, nenhuma injeta linha.
- `target_ids_json … 2>/dev/null || echo "PARSE_ERROR"` (`check-update-parity.sh:336`): a função é
  `python3 -c … | strip_cr`; com `set -o pipefail` (linha 19) o status do `python3` propaga, então o
  `||` dispara de verdade. **(b)**.

### 2.3 Fechamento pelo consumidor (a direção que a forma não alcança)

O critério da REQ é sobre a **emissão**; o dano é no **consumo**. Enumerados os consumidores:

```bash
grep -rnE '\$\(\(' .github/workflows/ scripts/ Makefile | grep -v testdata | wc -l   # 118
grep -rnE '\-(eq|ne|lt|gt|le|ge) ' .github/workflows/ scripts/ Makefile | grep -v testdata | wc -l # 387
```

Em `windows-census.yml`, os 6 consumidores aritméticos são `482, 503, 504, 506, 513, 566`, e
**5 deles** (`503, 504, 506, 513, 566`) consomem direta ou transitivamente as variáveis dos 4 sítios
(a). É esse acoplamento que transforma um contador errado em `arithmetic syntax error` e mata o laço
na primeira iteração de shard limpo.

### 2.4 Cegueiras declaradas desta varredura

🔴 Declaradas porque não medi-las é o que produz a próxima enumeração estreita:

1. `grep` é orientado a linha: um `||` em **linha de continuação** (`\` no fim) é invisível a todas
   as formas acima. Não varri por isso.
2. `[^()]*` **exclui captura aninhada** (`$( … $( … ) … || echo 0 )`). Não varri por isso.
3. **Crases** (`` ` … ` ``) como forma de captura: varridas em `.yml`/`.sh`/`Makefile`, só há
   ocorrências de markdown e de interpolação PowerShell (`` `n ``) — nenhuma captura POSIX.
4. Fora do escopo varrido: `internal/`, `npm/`, `pypi/` (não são shell de CI).

---

## 3. Achado novo — mesma causa, mesma REQ: 67 rótulos invisíveis à guarda

Este achado **não é o chunk 1**; é a razão de o chunk 1 ter podido morrer dentro de um controle de
segurança sem que guarda nenhuma nomeasse o controle.

### 3.1 O Cenário 18 não é ponto de corte, e por isso foi fundido

O gerador corta blocos por:

```python
# scripts/gen-falsify-chunks.py:81
HDR_PAT = re.compile(r'^# Cen[aá]rio[s]?\s+([0-9][0-9a-zA-Z/–\-]*)\s+(--|—)')
```

O cabeçalho do Cenário 18 é:

```
# Cenário 18 (AC1/AC2/AC3 — REQ #366) — não-mutação: nenhum gate que invoca
```

O parêntese entra **entre o número e o travessão**, a classe `[0-9a-zA-Z/–\-]*` não casa `(`, e o
padrão falha. Medido sobre os **75** cabeçalhos `# Cenário <n>` do arquivo: **17 não casam
`HDR_PAT`**, e **16 desses 17 são prosa** (linhas que *mencionam* cenários). O 17º —
`check-gates-falsify.sh:1478` — é um cabeçalho legítimo. Ou seja: **exatamente um cenário real do
arquivo não é ponto de corte**, e é justamente o `no-repo-mutation`.

⚠️ Isso contraria diretamente o docstring de `gen-falsify-chunks.py`, que declara que a hipótese "42
cabeçalhos usam grafia diferente" *"NÃO se confirmou"* e que afrouxar o padrão pioraria. Ambas as
afirmações continuam válidas em ordem de grandeza (1 ≠ 42; afrouxar casaria prosa) — mas **1 ≠ 0**, e
o um que falta é um controle de segurança.

### 3.2 A guarda de cobertura não enxerga rótulo emitido por `echo` literal

Mecanismo, verificado nos dois sítios do gerador (não inferido por nome):

- `gen-falsify-chunks.py:314-336` — `extract_expected_labels` itera **apenas** `ASSERT_CALL_PAT`
  (`for m in ASSERT_CALL_PAT.finditer(line)`, linha 326). É a única função que produz as entradas
  `label=` / `label_glob=` do manifesto.
- `gen-falsify-chunks.py:93` — `ECHO_SIGNAL_PAT = re.compile(r'echo\s+"(OK|FAIL)\s')` **existe**,
  mas seu único uso é a linha 134 (`if ASSERT_CALL_PAT.search(line) or ECHO_SIGNAL_PAT.search(line)`),
  que apenas classifica se o bloco **tem asserção própria** — insumo da decisão de içar para o
  preâmbulo. Ele **não** gera rótulo esperado.

Logo: cenário que emite o veredito com `echo "OK   [falsify/<rótulo>]"` direto **não entra no
manifesto** — e o manifesto é a única fonte das guardas local (`run-gates-falsify-shard.sh`) e de
conjunto (`check-falsify-shard-coverage.sh`).

Medição (descontando rótulos com `$` interpolado, os `setup*` — que são diagnóstico de preparo, não
veredito de controle — e os já cobertos por `label_glob`):

```
rótulos literais por echo (descontando `$` interpolado e `setup*`): 69
INVISÍVEIS a manifesto E a label_glob:                              67
destes, controle de segurança:                                      29
```

Os 29 de segurança, na íntegra: `no-repo-mutation`, `write-containment`, `vacuity-guard`,
`ac2-sanitization/direction-a-baseline`, `barrier/wave-zero-flag-guard-rejected-again-baseline`,
`credential-guard-baseline-carveout`, `credential-guard-git-env-bypass/*` (6),
`credential-guard-global-script-integrity/no-double-report`,
`credential-guard-hook-resolvable/baseline`, `credential-guard-mode-downgrade/baseline`,
`credential-guard-script-integrity/baseline`, `git-branch-guard-dedup/*` (5),
`git-branch-guard-global-hook-resolvable/kiro-dedicated-file/*` (4),
`git-branch-guard-global-script-integrity/{baseline,absent-is-not-a-violation,no-double-report}`,
`trust-check/direction-b-baseline`.

🔴 **A consequência é de segurança, nas duas direções:**

- **falso negativo por morte** — se o chunk morre antes de um desses, nenhuma guarda nomeia o
  controle ausente (foi o que aconteceu com `no-repo-mutation`: ele não está na lista dos 19, porque
  a guarda nem sabia esperá-lo);
- **falso negativo por sabotagem** — apagar a linha `echo "OK   [falsify/no-repo-mutation]"` e as
  asserções acima dela **não é detectado por guarda nenhuma**. O gate fica verde com o controle
  removido.

Por CLAUDE.md (*Regra Dura de Causa Raiz — mesma causa, mesma REQ*), isto é a **mesma causa** que o
chunk 1 expõe — cegueira da guarda de cobertura — e pertence a esta REQ, nomeado aqui. Não é REQ
nova.

---

## 4. Tabela garantia-sem-prova — os controles de segurança que o chunk 1 não exerceu

### 4.1 A aritmética dos "12"

Dos 19 rótulos ausentes:

| família | n | segurança? |
|---|---|---|
| `git-branch-guard/*` (62a + 62b) | 10 | sim |
| `git-branch-guard-global-script-integrity/*` | 2 | sim |
| `credential-guard-hook-resolvable/detected` | 1 | sim |
| `trust-check/direction-b-detected` | 1 | sim |
| `barrier/wave-zero-flag-guard-rejected-again-detected` | 1 | sim (guarda de governança que produz `exit 2`) |
| `integration-assets/*` | 2 | não |
| `roadmap-req-frontmatter-path/go/*` | 2 | não |

**15 de segurança + 4 não = 19.** A REQ diz 12: ela conta `git-branch-guard/*` (10) +
`credential-guard-hook-resolvable` + `trust-check`, e **deixa de fora os 2
`…-global-script-integrity/*` que a própria frase acrescenta com "além de"**, mais o `barrier`. A
tabela abaixo tem as **15** linhas; as **12** que a REQ pretendia estão marcadas com ★.

### 4.2 Legenda dos vereditos

- **Caminho equivalente no Windows** = `go test ./...` no job `windows-full-suites`
  (`quality.yml:160`, `runs-on: windows-latest`). Verificado: `.github/windows-known-failures.json`
  **não contém nenhum** teste `GitBranchGuard`/`CredentialGuard`/`generators` — logo a cobertura não
  é nominal por waiver. ⚠️ Ressalva: as suítes têm `continue-on-error: true` em nível de **step**; o
  juiz do job é o ratchet. É cobertura-como-medição com trava de regressão, **não** gate bloqueante
  direto.
- 🔴 **Nenhum teste Go pode cobrir um braço `detection-catches-bypass-*`.** Esse braço exige
  **corromper um literal do gerador e reconstruir** — é uma prova de que *aquela linha é
  carregadora*. Teste unitário prova o código atual; não prova que a linha é indispensável.

### 4.3 As 15 linhas

| ★ | rótulo | garantia que ele exercita | cobertura equivalente no Windows | veredito |
|---|---|---|---|---|
| ★ | `git-branch-guard/env-command-prefix/baseline-blocks-env` | guard limpo bloqueia `env git commit -m "x"` (exit 2) | `internal/generators/git_branch_guard_test.go:664` `EnvWithVarAssignment_Blocks` (`env FOO=bar git push`) e `:674` `…MultipleVarAssignments…` — **mesmo laço de stripping**, payload diferente (`env` nu não é testado) | 🟡 **parcial** — mecanismo coberto, payload não |
| ★ | `git-branch-guard/env-command-prefix/baseline-blocks-command` | guard limpo bloqueia `command git push` | `grep -rn 'command git' internal/ --include='*_test.go'` → **só markdown de corpus**, zero teste | 🔴 **SEM PROVA** |
| ★ | `git-branch-guard/env-command-prefix/detection-catches-bypass-env` | o `while [ "$base" = "env" ] …` de `scaffold.go` é **carregador**: sem ele, `env git commit` escapa (exit 0) | impossível por teste unitário (§4.2) | 🔴 **SEM PROVA** |
| ★ | `git-branch-guard/env-command-prefix/detection-catches-bypass-command` | idem para `command git push` | impossível por teste unitário | 🔴 **SEM PROVA** |
| ★ | `git-branch-guard/env-command-prefix/detection-does-not-break-plain-push` | auto-discriminação: a corrupção não calou o guard **inteiro** — `git push` puro segue exit 2 no build corrompido | nenhuma (só existe contra build corrompido) | 🔴 **SEM PROVA** |
| ★ | `git-branch-guard/checkout-flag-position/baseline-blocks-q-b` | guard limpo bloqueia `git checkout -q -b nova` | `git_branch_guard_test.go:273` tem nome sugestivo (`CheckoutDashB_WithFlagsBefore_Blocks`) mas o payload é `git -C . checkout -b feat/x` — flag antes do **subcomando**, não entre `checkout` e `-b`. **Classe diferente** | 🔴 **SEM PROVA** |
| ★ | `git-branch-guard/checkout-flag-position/baseline-blocks-no-track` | guard limpo bloqueia `git checkout --no-track -b nova` | `grep -rn -- '--no-track' internal/ --include='*_test.go'` → **só markdown de corpus** | 🔴 **SEM PROVA** |
| ★ | `git-branch-guard/checkout-flag-position/detection-catches-bypass-q-b` | a varredura de todos os tokens é carregadora | impossível por teste unitário | 🔴 **SEM PROVA** |
| ★ | `git-branch-guard/checkout-flag-position/detection-catches-bypass-no-track` | idem | impossível por teste unitário | 🔴 **SEM PROVA** |
| ★ | `git-branch-guard/checkout-flag-position/detection-does-not-break-plain-checkout-b` | auto-discriminação da corrupção 62b | nenhuma | 🔴 **SEM PROVA** |
| | `git-branch-guard-global-script-integrity/detected-without-wiring` | `validate` acusa script **global** adulterado **mesmo sem fiação** em config nenhuma | **corpo lido:** `internal/validator/validator_git_branch_guard_test.go:632` `TestGuardGlobalScriptIntegrity_DisparaSemNenhumaFiacao` — escreve o script global corrompido, **nenhum arquivo de config**, exige violation `diverges from the template`. ⚠️ Não é `…_integrity_external_test.go`, que só tem `MatchesGenerator`/`MatchesGlobalGenerator` (paridade cópia↔gerador, outra garantia) | 🟢 **coberto** (com a ressalva do ratchet) |
| | `git-branch-guard-global-script-integrity/non-vacuity` | prova de que a regra **discrimina** (não acusa sempre) — o rótulo é emitido por `assert_would_now_fail`, e por isso **está** no manifesto e na lista dos 19 | **corpos lidos:** `…_test.go:659` `TestGuardGlobalScriptIntegrity_AusenciaDoArtefato_Silencio` (script global nunca instalado → silêncio) cobre a mesma discriminação; `:599` `TestGitBranchGuardGlobal_SemWiringGlobalHoje_Silencio` é o par do outro lado (corpo **não** lido — citado por nome) | 🟢 **coberto** |
| ★ | `credential-guard-hook-resolvable/detected` | `validate` acusa hook de credential-guard de projeto cujo script referenciado não existe | **corpo lido:** `internal/validator/validator_credential_guard_test.go:35` `TestCredentialGuardHookResolvable_DisparaScriptAusente` — escreve `.claude/settings.json` apontando para script que não cria, exige violation `does not exist` | 🟢 **coberto** |
| ★ | `trust-check/direction-b-detected` | `check-barrier.sh` **não executa gate hostil** — mensagem `hostile gate EXECUTED` é a falha | detecção exige corromper o binário Go e rebuildar; `internal/commands/integrations_thirdparty_validate_test.go` cobre a regra de provenance, **não** a execução de gate pelo barrier | 🔴 **SEM PROVA** |
| | `barrier/wave-zero-flag-guard-rejected-again-detected` | `trackfw barrier --wave 0` nunca sai com 2 (flag rejeitada) — a guarda que impede a Wave 0 de ser esvaziada por erro de flag | detecção exige binário corrompido; nenhum teste Go afirma "nunca 2" pelo `check-barrier.sh` | 🔴 **SEM PROVA** |

**Resumo:** de 15 controles, **3 têm cobertura equivalente** (corpos de teste lidos, não nomes),
**1 é parcial**, e **11 ficam sem prova alguma no Windows** enquanto o chunk 1 morrer.

### 4.4 O risco real desta REQ, dito sem rodeio

Os cenários 62a/62b existem porque um veredito **BLOQUEIA** meu, em
`docs/seguranca/2026-08-16-revisao-do-git-branch-guard.md`, reproduziu duas evasões reais do
`trackfw-git-branch-guard.sh`: prefixo `env`/`command` antes de `git`, e flag do `checkout -b` fora
da primeira posição. Enquanto o chunk 1 morre:

> 🔴 **No Windows — a plataforma onde as escapadas de `git` bruto mais divergem — ninguém prova que
> as linhas que fecharam essas duas evasões continuam lá.** Reverter
> `while [ "$base" = "env" ] || [ "$base" = "command" ]; do` em `internal/generators/scaffold.go`
> reabre as duas evasões e **nenhuma medição de Windows reclama**. No Linux, o
> `check-gates-falsify.sh` completo pega; o censo de Windows, que é o instrumento oficial desta
> plataforma, não.

### 4.5 Residual declarado (não é achado novo, é dívida já escrita)

`internal/generators/git_branch_guard_test.go:684` — `TestGitBranchGuard_EnvWithFlag_StillEvades`
afirma, por medição, que `env -i git push` **continua evadindo o guard com exit 0**. Está declarado e
fora do escopo desta REQ; fica registrado aqui porque um leitor da §4.4 precisa saber que a superfície
`env` não está inteira nem quando tudo roda.

---

## 5. As duas frases de fechamento

### 5.1 Causa A

> **Corrijo a forma de captura nos 4 sítios `(a)` — `.github/workflows/windows-census.yml:485, 486,
> 564, 565` — e fecham exatamente estes efeitos: o `arithmetic syntax error` na primeira iteração de
> shard sem `FAIL`; o `SHARDS_FOUND` parando em 2; o `TOTAL INCOMPLETO — 2/8`; as 6 linhas
> `AUSENTE (artefato não baixado)` para artefatos que existem; e a `DISCREPÂNCIA FAIL shard 1:
> grep=0\n0 awk=0`, que é divergência inexistente. E nenhum outro — em particular, não fecha nenhum
> dos 19 rótulos ausentes, e não fecha nenhum dos 8 sítios `(b)`/`(c)`, que não são defeito.**

Falsificação nas duas direções (já executada pelo arquiteto contra os 8 artefatos reais e reproduzida
aqui pelo particionamento): forma atual → `SHARDS_FOUND=2`; forma corrigida → `SHARDS_FOUND=8`,
`OK=225`, `FAIL=9`.

### 5.2 Causa B — duas frases, porque são duas causas

🔴 Escrever uma frase só aqui seria falso: tirar o silêncio **não** faz os rótulos voltarem.

> **B1 — o silenciamento.** *Corrijo a supressão de diagnóstico em `check-gates-falsify.sh:1559` e
> `:1583` (o `2>/dev/null` sobre um `git` cuja falha é fatal sob `set -e`) e/ou faço o chunk
> denunciar a própria morte (trap de `EXIT`/`ERR` que emita sentinela de aborto antes do `set -e`
> matar o processo), e fecha exatamente isto: o shard passa a dizer **onde** e **com que rc** morreu,
> em vez de terminar sem `CHUNK_COMPLETE` e sem `N cenário(s) reprovaram`. E nenhum outro — **fecha
> zero dos 19 rótulos**.*

> **B2 — a falha do `git`.** *Causa **não identificada**. Nenhuma frase de fechamento é escrita para
> B2 neste ML. A sonda da §1.8 é o que autoriza escrevê-la; até lá, qualquer afirmação sobre os 19
> seria hipótese apresentada como causa.*

> **B3 (achado novo, mesma causa, §3).** *Corrijo `HDR_PAT` para aceitar parêntese entre o número e
> o travessão **e** faço `extract_expected_labels` colher também `echo "OK   [falsify/<literal>]"`,
> e fecha exatamente isto: o Cenário 18 volta a ser ponto de corte, e os **67** rótulos hoje
> invisíveis (29 de segurança) passam a ser exigidos pela guarda local e pela de conjunto — nas duas
> direções (morte do chunk e remoção da asserção). E nenhum outro — não fecha B2 nem A.*

---

## 6. Escopo e omissões declaradas

- Este ML não escreveu **nenhum arquivo do repositório além deste**. Nenhuma linha de implementação,
  nenhum commit, nenhum push, nenhuma branch criada.
- `make quality` **não** foi executado (proibição explícita do ML).
- `go test` **não** foi executado — a análise de cobertura foi feita por leitura de corpo de teste,
  não por nome, e pelo conteúdo de `.github/windows-known-failures.json`.
- A AC deste ML nomeia este arquivo como único entregável. Por isso **não** escrevi entrada em
  `docs/agents-working-context.md` nem nota de vault, embora as regras de papel as peçam: o achado da
  §3 (67 rótulos invisíveis) é exatamente do tipo que custa >10 min a outro agente amanhã e
  **precisa** de nota de vault — recomendo que a Wave 2 a escreva junto com a correção.
