# Wave 0 — Threat Model: mktemp sem limpeza garantida

> REQ: REQ-2026-11-11-mktemp-sem-limpeza-garantida-no-mesmo-escopo-vaza-temporarios-em-self-tests.md
> ADR: ADR-2026-11-11-mktemp-em-script-tem-limpeza-garantida-por-variavel-no-mesmo-escopo.md
> Produzido por: hades-tf em 2026-11-11 (ML-0A)

---

## 1. Enumeration Completeness — ✅

### Superfície declarada no roadmap vs. realidade

O ADR D1 governa `var=$(mktemp …)` em arquivos `.sh` versionados. O roadmap cobre
`git ls-files 'scripts/*.sh'`; o gate futuro (ML-1B) cobrirá `git ls-files '*.sh'` (escopo
completo do ADR). A REQ declara fora de escopo: `mktemp` em `run:` de workflows do GitHub.

**Arquivos `.sh` fora de `scripts/`:** `git ls-files '*.sh' | grep -v '^scripts/'` retorna:
```
internal/commands/testdata/attention-legacy/cleanup.sh
internal/commands/testdata/attention-legacy/signal.sh
internal/generators/testdata/guard-sh-reference/credential-guard-global.sh
internal/generators/testdata/guard-sh-reference/credential-guard-project.sh
internal/generators/testdata/guard-sh-reference/git-branch-guard.sh
```

`grep -n mktemp` em `cleanup.sh`, `signal.sh` e `git-branch-guard.sh`: vazio (zero hits). Em
`credential-guard-global.sh` e `credential-guard-project.sh`: 3 hits cada:
- L27/L30: `# (mktemp-derived path or /dev/null)` — comentários; não reportados pelo instrumento
- L35/L38: `*mktemp*) return 0 ;;` — case pattern; reportados pelo instrumento como NOVAR
  (scope `is_ephemeral_target`)
- L40/L43: `pattern="*${varname}="'$(mktemp'"*"` — `'$(mktemp'` está dentro de sq-string;
  filtrados pelo instrumento via sq-state detection

Zero `var=$(mktemp…)` em execução em todos os 5 arquivos. Estes sítios são FP em potencial
para o gate futuro — ver §3 FP-4 e FN-11.

`grep -rn mktemp internal/generators/ | grep -v testdata` retorna:
- `internal/generators/credential_guard_test.go:508,514,521,526,532` — código de teste Go
- `internal/generators/scaffold.go:1269,1270` — comentários Go

Nenhuma dessas linhas é `var=$(mktemp…)` em script shell. Zero sítios adicionais.

**Workflows GitHub:** `git ls-files '.github/workflows/*.yml' | xargs grep -n mktemp`:
```
.github/workflows/check-annotations.yml:96:    ANNOTATIONS_FILE=$(mktemp)
.github/workflows/release.yml:329:            tmpdir=$(mktemp -d)
.github/workflows/windows-probe.yml:543:          T=$(mktemp -d)
.github/workflows/windows-probe.yml:607:          T14=$(mktemp -d)
```
Explicitamente fora de escopo pela REQ. Nenhum destes 4 hits afeta a contagem de `scripts/`.

**`demo.tape`:** `cd "$(mktemp -d)"` em instrução VHS tape — não é arquivo `.sh`. Fora do escopo D1.

### Instrumento e metodologia

Foi construído um parser Python (analyze_v4.py, em scratchpad) que atravessa os arquivos
rastreados por `git ls-files '*.sh'` (escopo completo do ADR, confirmado em L28 do script).
O instrumento classifica cada `var=$(mktemp …)` pelo discriminante D2 do ADR:

1. **DIRECT** — algum `trap … EXIT` no mesmo escopo cita `$V`, `${V}`, `${V:-}` ou `${V:-x}`
2. **ALIAS** — algum `trap … EXIT` cita alias `A` onde `printf -v A '%q' "$V"` ou `A=$V` precede
3. **CONTAINED** — `V` criado dentro de diretório já coberto (`mktemp -d "$W/…"`)
4. **FUNCTION** — o trap chama função cuja body ou traps citam `$V` ou alias global de `$V`

Bugs significativos corrigidos (v1→v4):

| Versão | Bug | Efeito |
|---|---|---|
| v1→v2 | brace-depth off-by-one; `is_dq` bloqueando ALIAS em corpos DQ | 2 FUNCTION e 3 ALIAS perdidos |
| v2→v3 | sq-toggle contava `'` dentro de `"…"` | sq-state corrompido em vários arquivos |
| v3→v4 | regex `([^)"]*)` parava em `"` em args como `"${TMPDIR:-/tmp}/…"` | 8 sítios perdidos |
| v3→v4 | `in_singlequote` atualizado ANTES de `find_mktemp_var` | 4 FP em check-no-trap-local-var.sh |
| v3→v4 | one-liner function flush timing na pré-varredura | 2 sítios FUNCTION perdidos |

### Resultados do inventário (instrumento v4, HEAD)

O instrumento reporta 5 NOVAR totais: 2 em `internal/generators/testdata/`
(credential-guard-{global,project}.sh:35,38 — case patterns) e 3 em `scripts/`
(check-install-checksum.sh:401,466,589 — utility list). Estes são excluídos da tabela de
cobertura (não são `var=$(mktemp…)` em execução).

Tabela de cobertura para sítios `var=$(mktemp…)` em `scripts/`:

| Classificação | Sítios em scripts/ |
|---|---|
| DIRECT | 44 |
| ALIAS | 5 |
| CONTAINED | 2 |
| FUNCTION | 6 |
| **UNCOVERED** | **11** |
| **Total atribuído** | **68** |

**Nota multi-assign:** o instrumento reporta apenas a última atribuição de cada var-name:
- `check-channels-content.sh`: 3× WORK (L592, L1369, L1482 — blocos mutuamente exclusivos);
  todos FUNCTION via `cleanup_selftest()`. Instrumento reporta L1482; L592 e L1369 são igualmente
  cobertos. Confirmado por `grep -n mktemp scripts/check-channels-content.sh | grep -v '#'`.
- `check-platform-matrix-parity.sh`: 2× WORK (L51, L72 — ramos `--falsify-goreleaser` e default);
  ambos FUNCTION via `trap cleanup EXIT` → `cleanup() { rm -rf "$WORK" }`. Instrumento reporta L72;
  L51 é igualmente coberto. Confirmado por `grep -n mktemp scripts/check-platform-matrix-parity.sh | grep -v '#'`.

**check-no-trap-local-var.sh:503:** `printf -v q '%q' "$(mktemp -d)"` está DENTRO do sq-string
argumento do `run_case "N2-double-quote-body"` (fixture do caso N2 — double-quoted trap body). O
`$(mktemp -d)` está coberto pelo mecanismo trap sendo testado no próprio fixture. Filtrado pelo
instrumento via sq-state detection. Não é sítio da classe.

### Segunda via de contagem (grep) — verificação de completude

Escopo: `git ls-files 'scripts/*.sh'` (75 arquivos em HEAD).

```bash
# Total
git ls-files 'scripts/*.sh' | xargs grep -n mktemp | wc -l            # → 113

# Comentários (regex ancorado por posição)
git ls-files 'scripts/*.sh' | xargs grep -n mktemp \
  | grep -E '^[^:]+:[0-9]+:[[:space:]]*#' | wc -l                     # → 14
# Nenhum dos 14 está em check-no-trap-local-var.sh (confirmado: grep retorna 0)

# Total de hits em check-no-trap-local-var.sh
grep -n mktemp scripts/check-no-trap-local-var.sh | wc -l             # → 26
# Dos 26: L306 é real (assignment-shaped, em instrument); 24 são fixtures assignment-shaped
#         (dentro de sq-string, filtrados); 1 é L503 (printf-v q, não assignment-shaped)
# Non-instrument total de check-no-trap-local-var.sh: 25 (24 fixtures + L503)

# Utility list em check-install-checksum.sh (palavra mktemp em lista de utilitários)
grep -n mktemp scripts/check-install-checksum.sh | grep -c 'for '     # → 3 (L401, L466, L589)
# Estes 3 são reportados pelo instrumento como NOVAR; contados em ambos os lados da equação

# Overwrites confirmados por grep direto
grep -n mktemp scripts/check-channels-content.sh | grep -v '#'        # → L592, L1369, L1482
grep -n mktemp scripts/check-platform-matrix-parity.sh | grep -v '#'  # → L51, L72
# Instrumento reporta última; reporter conta todas — +2 channels + +1 platform = +3 extra
```

Equação de reconciliação (scripts/ em HEAD, disjunta):

**113 = 68 (instrumento assigned) + 25 (non-instrument de check-no-trap-local-var.sh) + 14 (comentários) + 3 (utility list NOVAR) + 3 (overwrites extras)**

Verificação: 68 + 25 + 14 + 3 + 3 = 113 ✓.

As categorias são disjuntas:
- 68: sítios `var=$(mktemp…)` atribuídos, reportados pelo instrumento
- 25: 24 fixtures assignment-shaped em check-no-trap-local-var.sh (sq-string, filtrados) + 1 L503
- 14: comentários (anchored regex)
- 3: utility list (NOVAR no instrumento)
- 3: linhas de overwrite que o instrumento deduplica

### Reconciliação com o reporter (issue #676 comment)

O reporter escaneou `scripts/` em `main` ao commit `9fd2d84b` (74 scripts, 2026-11-08).
`check-marker-corpus-parity.sh` foi adicionado em 2026-11-10, DEPOIS do scan.

Âncora: 111 total grep hits em scripts/ ao scan time (113 − 2 de marker-corpus).
94 = 111 − 14 (comentários) − 3 (utility list) = hits sem comentários e sem utility list.
O reporter encontrou 93 assigned + 1 NOVAR (L503) = 94 hits classificados. ✓

| Dimensão | Reporter (@9fd2d84b, 74 scripts) | Instrumento v4 (@HEAD, 75 scripts) | Explicação |
|---|---|---|---|
| Scripts | 74 | 75 | check-marker-corpus-parity.sh adicionado depois do scan |
| Assigned | 93 | 68 | A equação 93 = 68 + 24 + 3 − 2 implica que o reporter contou: 24 fixtures assignment-shaped de check-no-trap-local-var.sh (que o instrumento filtra por sq-state), 3 overwrites extras, e não contou 2 sítios de marker-corpus (que não existiam no scan) |
| NOVAR | 1 (L503: printf-v) | 3 (check-install-checksum.sh:401,466,589, excluídos pelo reporter per 94 = 111 − 14 − 3); L503 filtrado por sq-state | L503 está dentro de sq-string; utility list NOVAR não são `var=$(mktemp…)` em execução |
| Covered | 76 | 57 | 76 = 57 − 2 (marker-corpus) + 24 (fixtures que reporter conta como cobertos) + 3 (overwrites extras) − 6 (FPs que v4 cobre; reporter classificou como não cobertos) → 76+17=93 e 57+11=68 ✓ |
| Não cobertos (discriminante v3 do reporter) | 17 | 17 | **Idêntico**: 11 UNCOVERED + 6 FP |

O único número que importa para a REQ AC1 — 11 + 6 — é **idêntico** entre reporter e instrumento.

### Parse warnings e resolução de R2

O instrumento emitiu parse warnings em 10 arquivos únicos. Para cada um, verifica-se se algum
grep hit mktemp do arquivo está ausente da saída do instrumento:

| Arquivo (warning) | mktemp hits (grep) | Classificados pelo instrumento | Resultado |
|---|---|---|---|
| check-adr-enumeration-single-point.sh | 3: L107,L121 comentários; L370 real | L370 DIRECT, trap em L371 (`'rm -rf "$TMPDIR_ST"'`), verificado por `sed -n '370,372p'` | ✓ |
| check-crlf-normalize-capture.sh | 0 | — | ✓ |
| check-emitting-capture-fallback.sh | 0 | — | ✓ |
| check-gates-falsify.sh | L33 (DIRECT), L1476 (UNCOVERED) | Ambos na saída do instrumento; L33 e L34 verificados por `sed` | ✓ |
| check-git-branch-guard-hook-schema.sh | 2: L67 comentário; L92 real | L92 DIRECT, trap em L93 (`'rm -rf "$WORK"'`), verificado por `sed -n '92,94p'` | ✓ |
| check-integration-assets.sh | 1: L6 real | L6 DIRECT, trap em L7 (`'rm -rf "$TMP_ROOT"' EXIT HUP INT TERM`), verificado por `sed -n '6,8p'` | ✓ |
| check-interpolated-path-in-python.sh | 0 | — | ✓ |
| check-release-tag-parity.sh | L69 (DIRECT), L330 (UNCOVERED) | Ambos na saída do instrumento; L69/L70 verificados por `sed` | ✓ |
| check-roadmap-slug-matching.sh | 1: L67 real | L67 DIRECT, trap em L68 (`'rm -rf "$WORK"'`), verificado por `sed -n '67,69p'` | ✓ |
| check-unguarded-capture-rc.sh | 0 | — | ✓ |

Verificação independente: nenhum dos 6 sítios cobertos em arquivos com warning aparece na lista
de 17 do reporter (11 UNCOVERED + 6 FP) — confirmado por comparação direta.

Nenhum grep hit nos arquivos com warning está fora da saída do instrumento. A lista de 11
UNCOVERED é provada final.

**check-gates-falsify.sh:1476 (bare_origin):** `s5x_commit_fixture_with_origin` não tem `trap EXIT`;
a única `trap EXIT` global é L34 cobrindo `$WORK`; `bare_origin` não está sob `$WORK`. UNCOVERED
confirmado por leitura direta de L1472–1490.

**check-release-tag-parity.sh:330 (_src_dir):** a função `_build_gh_stub_shim_once` não instala
`trap EXIT`; `_src_dir` não é containment sob `$WORK` (L69). UNCOVERED confirmado por leitura
direta de L326–340.

### Os 11 UNCOVERED confirmados e os 6 FP do reporter

Reporter: 11 UNCOVERED + 6 FP (issue #676 comment, lourivalgarciajunior, segunda passada).

Os 6 FP por file:line exato e classificação v4 (prefixo RFP para distinguir dos vetores do gate em §3):

| RFP do reporter | Arquivo:linha | Var | Classificação v4 | Mecanismo coberto |
|---|---|---|---|---|
| RFP-1 | check-orphan-gates.sh:142 | WORK | ALIAS | `printf -v q '%q' "$WORK"` → `trap "rm -rf -- $q" EXIT` (DQ body) |
| RFP-2 | check-req-roadmap-rule-is-status-aware.sh:231 | WORK | ALIAS | mesma forma canônica |
| RFP-3 | check-shim-diff-applies.sh:177 | tmp | ALIAS | `printf -v q '%q' "$tmp"` → `trap "rm -rf -- $q" EXIT` (DQ body) |
| RFP-4 | check-validate-rule-pins.sh:909 | FENCE_TMP | DIRECT | `${FENCE_TMP:-}` no body do trap em L35 |
| RFP-5 | check-update-parity.sh:162 | scratch_dir | CONTAINED | `mktemp -d "$WORK/agents-install-cwd.XXXXXX"` |
| RFP-6 | check-update-parity.sh:172 | scratch | CONTAINED | `mktemp -d "$WORK/install-global-cwd.XXXXXX"` |

(Reporter citou check-orphan-gates.sh:141; em `main` atual, `WORK=$(mktemp -d …)` está em L142
— L141 é `local WORK`. Sem divergência de classificação.)

**Os 11 sítios UNCOVERED:**

| Arquivo | Linha | Var | Escopo | Situação atual | Forma D3 |
|---|---|---|---|---|---|
| check-shim-diff-applies.sh | 158 | tmpfile | run_classify (invocado como subprocesso via `bash <script> --classify`) | o L179 existente instala `trap "rm -rf -- $q" EXIT` para `$tmp` DEPOIS de L158; standalone trap em L158 seria substituído em L179 | D3 combinada: adicionar `printf -v q_tf '%q' "$tmpfile"` após L158; modificar L179 para `trap "rm -rf -- $q; rm -f -- $q_tf" EXIT`; sq-body não funciona: `local tmpfile` sai de escopo antes do EXIT do processo |
| check-shim-diff-applies.sh | 490 | plumb_repo | run_self_test | L179 `"rm -rf -- $q"` limpa apenas `$q`=`$tmp` | containment: `mktemp -d "$tmp/plumb-repo.XXXXXX"` |
| check-shim-diff-applies.sh | 491 | script_copy_dir | run_self_test | idem L179 | containment: `mktemp -d "$tmp/script-copy-dir.XXXXXX"` |
| check-no-literal-nul-in-source.sh | 89 | EMPTY_EXC | global | L73 `'rm -rf "$WORK"'`; EMPTY_EXC em `${TMPDIR:-/tmp}`, fora de $WORK; `git -C "$WORK" add .` em L108 e L164 estagearia estes files se dentro de $WORK | trap combinado no mesmo L73: `'rm -rf "$WORK" "${EMPTY_EXC:-}" "${EXC_MISSING:-}" "${EXC_ZERO:-}" "${EXC_WRONG:-}"'` — files permanecem fora de $WORK (ver §2 T8); BSD `rm -rf` com operando vazio: removido, rc=0 (medido nesta sessão: `rm -rf "$d" ""` → removed) |
| check-no-literal-nul-in-source.sh | 133 | EXC_MISSING | global | idem | idem |
| check-no-literal-nul-in-source.sh | 148 | EXC_ZERO | global | idem | idem |
| check-no-literal-nul-in-source.sh | 165 | EXC_WRONG | global | idem | idem |
| check-parity-call-site-pins.sh | 222 | local_dryrun | global (corpo de `while … done <<<"$consumers"` — here-string, loop in-process) | L121 `'rm -f "$RECIPE_LINES_FILE"'`; `local_dryrun` explicitamente limpo via `rm -f` em L226, L235, L240 dentro de cada iteração | trap combinado em L121: `'rm -f "$RECIPE_LINES_FILE" "${local_dryrun:-}"'`; BSD `rm -f` com operando vazio: removido, rc=0 (medido nesta sessão); cobre a última iteração em caso de kill entre L222 e L226/L235/L240 |
| check-release-tag-parity.sh | 330 | _src_dir | _build_gh_stub_shim_once | L69 WORK, L70 `'rm -rf "$WORK"'`; `_src_dir` foi criado fora de `$WORK` | containment: `mktemp -d "$WORK/src-dir.XXXXXX"` |
| check-gates-falsify.sh | 1476 | bare_origin | s5x_commit_fixture_with_origin | L33 WORK, L34 `'rm -rf "$WORK"'`; `bare_origin` foi criado fora de `$WORK` | containment: `mktemp -d "$WORK/bare-origin.XXXXXX"` |
| check-usage-silencing.sh | 30 | d | novo_projeto (sempre chamada como `d=$(novo_projeto)` — subshell) | nenhuma trap global; `d` explicitamente limpo via `rm -rf "$d"` em L45 e L70 no caller (L43 e L68: assignment); `rodar_cenarios` chamada tanto em self-test (L115) quanto globalmente (L156, sem trap) | adicionar `WORK=$(mktemp -d); trap 'rm -rf "$WORK"' EXIT` próximo ao topo; mudar `novo_projeto` para `d=$(mktemp -d "$WORK/proj.XXXXXX")`; unificar trap de L95 ao trap global |

### Prova de alcançabilidade (componente de AC1)

Instrumento de prova: cópia do script em diretório temporário com `exit 73` injetado após o
mktemp alvo. Dependências providas via symlinks.

**Site 1: check-no-literal-nul-in-source.sh:89 (EMPTY_EXC)**
- Injeção: `exit 73` após linha 89; template com `$TMPDIR_ISO`
- Execução: `TMPDIR="$TMPDIR_ISO" bash ./check-no-literal-nul-in-source.sh --self-test`
- Resultado: `RC=73`, delta = +1 arquivo em `$TMPDIR_ISO` (`checknul-exc.*` persiste)
- Explicação: `trap 'rm -rf "$WORK"' EXIT` (L73) limpou o dir `$WORK`; não removeu `$EMPTY_EXC`
  (arquivo regular em `$TMPDIR_ISO`, fora de `$WORK`)

**Site 2: check-shim-diff-applies.sh:490,491 (plumb_repo + script_copy_dir)**
- Injeção: `exit 73` após linhas 490-491; paths capturados via stderr antes do exit
- Execução: `bash ./check-shim-diff-applies.sh --self-test`
- Resultado: `RC=73`, ambos os diretórios **still_exists=YES** após saída (verificado por `test -d`)
- Explicação: `trap "rm -rf -- $q" EXIT` (L179) limpou apenas `$q`=`$tmp`; `$plumb_repo` e
  `$script_copy_dir` não constam no corpo do trap

---

## 2. Threat Model — ✅

O adversário aqui é o implementador apressado e o arquiteto otimista.

**T1 — Gate superficial que testa presença de `trap` mas não cita a variável.**
Todo arquivo com sítio UNCOVERED já tem `trap EXIT` em algum escopo. Um gate baseado em
proximidade textual (`trap EXIT` nos N vizinhos do `mktemp`) passaria em todos os 11 sítios.
O gate deve verificar que o corpo do trap cita `$V` pelo nome (ou alias rastreável, containment
confirmado, ou função que cobre `$V`).

**T2 — Fix "combinado" usando dois traps separados.**
`trap 'rm -rf "$A"' EXIT; trap 'rm -rf "$B"' EXIT` — o segundo substitui o primeiro. Corpus
atual (4 arquivos com ≥2 `trap EXIT`):
- `check-no-trap-local-var.sh`: 51+ traps, todos em fixtures — sem efeito real fora do fixture
- `check-marker-corpus-parity.sh`: L37 (self-test) + L325 (main) + L348 (upgrade). Self-test
  termina em `exit $?` (L307) ANTES de L325. L348 upgrade intencional mantém `$_q_tmp` no
  corpo combinado. Sem colisão efetiva.
- `check-channels-content.sh`: 3 traps em 3 blocos de modo mutuamente exclusivos, cada um
  terminando com `exit` antes do próximo. Sem colisão efetiva.
- `check-gates-falsify.sh`: apenas 1 trap EXIT real (L34); L57 é comentário.

Conclusão: last-trap-wins não cria sítios UNCOVERED não declarados no corpus atual.

Corolário sobre as correções D3 desta REQ: a forma canônica D3 (trap standalone) é segura
apenas onde nenhuma trap EXIT já está em vigor no mesmo caminho de execução. check-shim-diff-applies.sh:158
(`tmpfile` em `run_classify`) é o único sítio onde um trap standalone seria substituído pelo
trap existente de L179. D3 neste sítio usa a forma combinada (ver UNCOVERED table, §1).

**T3 — Fix via containment com path traversal no argumento.**
`mktemp -d "$WORK/../sibling.XXXXXX"` — instrumento v4 verifica prefixo `$VAR/` como texto
literal. `git ls-files 'scripts/*.sh' | xargs grep -n mktemp | grep -F '/..'` → 1 resultado:
`scripts/check-agent-namespace-union.sh:822:#  Measured: $WORK=$(mktemp -d ...) → /var/...`
(comentário, não execução shell). Zero cases de path traversal em mktemp args em scripts
executados. Risco baixo no corpus; o gate de produção deve rejeitar args com `..`.

**T4 — Wave 0 marcado ✅ sem prova de alcançabilidade.**
Análise estática classifica sítios mas não prova impacto em tempo de execução. Mitigação: prova
de alcançabilidade está em §1 com evidência concreta.

**T5 — Fix de check-usage-silencing.sh:30 implementado dentro de novo_projeto via DQ trap.**
`novo_projeto()` é sempre chamada como `d=$(novo_projeto)` — subshell. Um `trap "rm -rf -- $q"
EXIT` (DQ) dentro de `novo_projeto`, com `printf -v q '%q' "$d"`, capturaria o path ANTES de o
caller receber o valor — mas o trap dispararia quando a SUBSHELL sai, antes do caller capturar
`$d`, limpando o diretório que o chamador vai usar. Este erro passa em ambos os gates: o gate
do ML-1B (`check-mktemp-cleanup.sh`) veria COVERED via DIRECT; `check-no-trap-local-var.sh` não
verifica trap dentro de função chamada como subshell. Só o teste de sucesso do ML-1A detectaria.

**T6 — `exec` substituindo o shell sem disparar EXIT trap.**
`exec CMD` em bash não dispara EXIT traps. Corpus: todas as ocorrências de `exec` nos scripts
com mktemp estão em check-gates-falsify.sh dentro de `bash -c "... exec '$BIN' validate"` —
subshells separados. O pai (com `$WORK` trapped em L34) não chama `exec` diretamente. Risco
baixo no corpus; FN documentado em §3 (FN-7).

**T7 — `trap - EXIT` / `trap '' EXIT` removendo cobertura instalada.**
`git ls-files 'scripts/*.sh' | xargs grep -l mktemp | xargs grep -nE "trap[[:space:]]+(-|''|\"\")[[:space:]]+EXIT"` →
zero resultados. Nenhum script com sítio mktemp usa `trap -` ou `trap ''`. FN documentado em §3
(FN-8) por construção (zero casos hoje, pode aparecer em futuros commits).

**T8 — Containment ou trap-combinado muda o que o teste exercita (git worktree, upward discovery).**
5 sítios têm fixes que envolvem novos containers (4 containment, 1 novo WORK global);
4 sítios (nul-source EXC_*) passam a usar trap combinado sem nenhum container novo;
2 sítios (shim-diff:158 e parity-pins:222) passam a usar trap combinado no trap existente do escopo.

| Sítio | Fix | Verificação T8 |
|---|---|---|
| nul-source:89,133,148,165 | Trap combinado; files em `${TMPDIR:-/tmp}`, fora de $WORK | `git -C "$WORK" add .` em L108 e L164 estagearia EXC_* se dentro de $WORK. Com trap combinado os files permanecem fora — sem risco |
| shim-diff:490,491 | Containment em `$tmp` | `$tmp` contém `$tmp/repo` (git repo de Parte 2); `plumb_repo` e `script_copy_dir` seriam siblings de `$tmp/repo`, não dentro. Nenhum `git init` em `$tmp` diretamente. `script_copy_dir` continua fora de `plumb_repo` — premissa "script fora do repositório" mantida |
| release-tag:330 | Containment em `$WORK` | `go.mod` escrito em `_src_dir` explicitamente (L403); `go build` roda `cd "$_src_dir"` (L408). Go encontra `_src_dir/go.mod` primeiro. `$WORK` não tem git init. Sem impacto |
| gates-falsify:1476 | Containment em `$WORK` | Nenhum `git -C "$WORK"` ou `git init` em $WORK nos scripts. `bare_origin` acessado via `file://$bare_origin` (URL absoluta). Sem impacto |
| usage-silencing:30 | Novo $WORK global; `novo_projeto` → `mktemp -d "$WORK/proj.XXXXXX"` | Cada `proj.XXXXXX` recebe `trackfw.yaml` via `trackfw init`. Upward discovery a partir de `proj.XXXXXX` encontra `proj.XXXXXX/trackfw.yaml` primeiro. `$WORK` em `${TMPDIR:-/tmp}`, fora do workspace. Sem impacto |

| shim-diff:158 | D3 combinada em L179 (adiciona `$q_tf` ao body) | `tmpfile` é local em `run_classify`; a função é invocada apenas via `bash <script> --classify` (subshell). `$q_tf` é global ao subshell, retém valor no EXIT. `run_classify` não chama `git` nem interage com o worktree. Sem impacto |
| parity-pins:222 | Trap combinado em L121 (adiciona `${local_dryrun:-}`) | `local_dryrun` é global no processo; BSD `rm -f` com operando vazio: removido, rc=0 (medido nesta sessão). L226, L235, L240 continuam a limpar explicitamente dentro de cada iteração — o trap cobre apenas a última criação. Sem impacto |

Nenhum dos 11 sítios com fix tem impacto no que o teste exercita.

---

## 3. Falsification Targets — ✅

### Falsos negativos do gate futuro

| Vetor | Como ocorre | Portão esperado | Verificado no corpus |
|---|---|---|---|
| FN-1: template com `"${TMPDIR:-/tmp}/…"` | regex `([^)"]*)` para na primeira `"` antes de capturar os args completos | regex deve ser `([^)]*)` | 8 sítios perdidos sem a correção (confirmado v3 vs v4) |
| FN-2: fixture `'` na primeira coluna | `in_singlequote` atualizado antes de `_in_sq_at` → `initial_sq=True` trata `'` inicial como fechamento | usar `line_start_sq` | 4 FP em check-no-trap-local-var.sh desaparecem com a correção |
| FN-3: cleanup via função one-liner | brace flush na pré-varredura: corpo mapeado na linha seguinte | flush imediato quando `brace_depth=0` no cabeçalho | check-wheel-*.sh:WORK seria UNCOVERED sem a correção |
| FN-4: padrão FUNCTION+ALIAS externo | FUNCTION check padrão verifica apenas se a função cita `$V` diretamente | adicionar 4c: se outer-scope alias de `$V` aparece no body da função → FUNCTION | check-channels-content.sh:592,1369,1482 seriam UNCOVERED sem 4c |
| FN-5: trap substituído depois | `trap "rm $A" EXIT` correto instalado, depois `trap "rm $B" EXIT` sobrescreve | gate deve encontrar trap em vigor no momento da saída | nenhum caso de colisão efetiva no corpus (ver T2), mas FN existe por construção |
| FN-6: containment sob diretório não coberto | `mktemp -d "$X/…"` onde `$X` é UNCOVERED; gate marca CONTAINED | check_coverage verifica recursivamente cobertura de `$X` | nenhum caso no corpus, mas FN existe por construção |
| FN-7: `exec CMD` após covered var | `exec` substituindo o shell sem disparar EXIT trap | gate deve detectar `exec CMD` após a criação de var com trap | nenhum caso no corpus (all exec em bash -c subshells de check-gates-falsify.sh) |
| FN-8: `trap - EXIT` removendo cobertura | `trap -` ou `trap '' EXIT` após o trap que cobre V | gate deve verificar que nenhum `trap -` ou `trap '' EXIT` aparece depois do trap | zero casos no corpus |
| FN-9: var em subshell, trap do pai | var criada em `(…)` ou `$(…)` não é acessível pelo EXIT trap do pai | gate deve reprovar se var é criada em subshell não coberto | caso novo_projeto → ver T5 e R1 |
| FN-10: var reassigned em loop, trap cita apenas último valor | `local_dryrun=$(mktemp …)` recriado em cada iteração do loop; `${local_dryrun:-}` no trap EXIT do processo só captura o valor da ÚLTIMA iteração | constraint obrigatória: cada iteração deve cleanup explícito antes da próxima; trap cobre apenas a criação viva no momento de EXIT | parity-pins:222: cada iteração faz `rm -f "$local_dryrun"` em L226, L235 e L240 ANTES do próximo ciclo — constraint satisfeita; trap cobre a última criação em caso de kill entre L222 e L226/L235/L240 |
| FN-11: exclusão por path de testdata/ | gate com `:!**/testdata/**` exclui golden copies dos hooks que o trackfw instala em repos consumidores — exatamente o lugar onde um mktemp não coberto seria mais perigoso | exclusão deve ser sintática (ver FP-4), não por path | 5 arquivos em `internal/generators/testdata/` e `internal/commands/testdata/`; nenhum tem `var=$(mktemp…)` em execução hoje, mas o critério de exclusão é sintaxe, não localização |
| FN-12: assignment na mesma linha de case pattern | implementação greedy de FP-4(a) poderia excluir `x*) d=$(mktemp -d) ;;` porque a linha contém `mktemp` entre `*` e `)` | critério (a) deve verificar ausência de `$(` **antes** de qualquer `mktemp` na linha, não presença de mktemp entre globs; `NAME=$(mktemp` continua candidata na mesma linha | nenhum caso no corpus, mas o critério ambíguo criaria FN por construção |

### Falsos positivos do gate futuro

| Vetor | Como ocorre | Portão esperado | Verificado no corpus |
|---|---|---|---|
| FP-1: `${V:-}` no corpo do trap | busca ingênua de `$V` ou `${V}` não inclui `${V:-…}` | `trap_cites_var` deve reconhecer `${V:` como cobertura; BSD `rm -rf "$T" "${V:-}"` com V vazio funciona: `removed, rc=0` (medido nesta sessão: `rm -rf "$d" ""` → removed) | check-validate-rule-pins.sh:909 (FENCE_TMP via `${FENCE_TMP:-}`) — 1 caso |
| FP-2: alias via `printf -v q '%q' "$V"` → `trap "rm -- $q"` (DQ) | filtro `is_dq` bloqueava corpos double-quoted | remover filtro `is_dq` | 3 casos no reporter — confirmados ALIAS em v4 |
| FP-3: containment em `mktemp -d "$CONTAINER/…"` onde `$CONTAINER` é coberto | gate acusa UNCOVERED sem reconhecer containment | check_coverage verifica cobertura de `$CONTAINER` | check-update-parity.sh:162,172 — 2 casos |
| FP-4: pattern-string em guard-sh-reference | `*mktemp*)` não contém `$(` (nenhuma candidata sem `NAME=$(mktemp`); `'$(mktemp'` está dentro de sq-string no argumento de `pattern=` (caso b); ambos já resolvidos pelo instrumento v4 por sintaxe | gate deve aplicar dois critérios sintáticos independentes: (a) uma linha só é candidata se contiver `NAME=$(mktemp` fora de comentários e sq-strings — `*mktemp*)` não tem `$(`, impossível de ser candidata; (b) `$(mktemp` dentro de sq-string → filtrado via sq-state | credential-guard-{global,project}.sh:35,38 (caso a — NOVAR via instrumento, pois não são candidatas) e L40,L43 (caso b — sq-filtrado); todos os 5 arquivos fora de `scripts/` têm zero execução de mktemp |

---

## 4. Declared Residual — ✅

**R1 — novo_projeto chamada em subshell `d=$(novo_projeto)` (ML-1A).**
O ADR D2 não cobre explicitamente funções chamadas via substituição de comando onde o trap
dentro da função limparia o valor antes de ele ser retornado ao chamador (FN-9). Este é o único
caso no corpus. Um gate automático não detecta sem análise de ponto de chamada interprocessual.
Aceito como residual; documentado em ML-1A (sítio entra na lista do ML-1A). Fix via WORK global
com containment em `novo_projeto`. Verificação pelo ML-1A: o diretório existe após
`d=$(novo_projeto)` retornar e está ausente após o script sair normalmente.

**R2 — Resolvido: a lista de 11 é provada final.**
Para todos os 10 arquivos com parse warnings, nenhum grep hit de mktemp está ausente da saída
do instrumento. Os 6 sítios cobertos em arquivos com warning foram verificados manualmente (trap
na linha imediatamente seguinte, citando `$V`). Nenhum dos 6 aparece na lista de 17 do reporter.
A lista de 11 UNCOVERED é provada final.

**R3 — NOVAR e utility list.**
O ADR D1 governa `var=$(mktemp …)` — padrões sem atribuição (utility list e pattern strings)
estão fora do escopo por design. Nenhuma dessas linhas cria arquivo temporário.

**R4 — Indireção de dois níveis no FUNCTION check.**
O instrumento e ADR D2 declaram "um nível de indireção". Cadeia `trap → wrapper() → cleanup()
→ $V` não seria detectada. Nenhum caso desse tipo existe no corpus. Aceito como residual de
baixo risco.

**R5 — Instrumento classifica apenas a última atribuição de mesmo nome.**
Verificado que todos os casos no corpus (channels-content e platform-matrix) são cobertos nas
atribuições sobrescritas via FUNCTION. Se uma sobrescrita fosse UNCOVERED, o instrumento a
perderia. O gate de produção (ML-1B) deve acumular todas as atribuições.

**R5b — Medição de `rm` com operando vazio é BSD/macOS only.**
O teste `rm -rf "$d" ""` → `removed, rc=0` foi medido nesta sessão no macOS. GNU `rm` e
`rm` do Git-for-Windows com operando vazio não foram medidos; o pior caso é uma linha em
stderr (não aborta). Os testes de sucesso dos sítios nul-source e parity-pins no CI são o
check definitivo nessas plataformas. Aceito como residual; não muda D3.

**R6 — Escopo do gate futuro (ML-1B): `git ls-files '*.sh'` completo.**
O gate `check-mktemp-cleanup.sh` (ML-1B) cobrirá `git ls-files '*.sh'` — escopo completo do
ADR D1, incluindo `internal/generators/testdata/`. O FP-4 resolve por critério sintático (ver
§3): sem exclusão por path. O ML-1B deve declarar o critério sintático explicitamente nos seus
ACs, e adicionar FN-11 como vetor a testar no self-test.
