# Triagem do cluster de Windows — ML-0A (Wave 0)

> Autor: `hades-tf` · Data: 2026-09-25
> REQ: `docs/req/REQ-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-o-cenario-inteiro.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-o-cenario-inteiro.md`
> Fonte de todo número: censo `36036473391` (`main`, `8a9dba76`, 2026-09-24, `windows-latest` x64, 8/8 shards).
> 🔴 Nenhuma linha de implementação. Nenhum arquivo além deste e da entrada em `docs/agents-working-context.md`.

---

## 0. 🔴 Achado que precede a triagem: a população não é 13, é 11

A REQ e o roadmap enumeram **13 rótulos distintos em `FAIL`**. O censo mede **`FAIL=11`**, e os
rótulos distintos em `FAIL` são **11**. Os dois números não se reconciliam porque **dois dos treze
não são falhas**.

### Medição

Contagem de linhas `FAIL [falsify/…]` por shard, extraída de `gh run view 36036473391 --log`:

| shard | 0 | 1 | 2 | 3 | 4 | 5 | 6 | 7 | total |
|---|---|---|---|---|---|---|---|---|---|
| `FAIL [falsify/…]` | 1 | 2 | 1 | 3 | 4 | 0 | 0 | 0 | **11** |

Esta contagem **bate linha a linha** com a tabela que o job `apuracao` computou dentro do CI a partir
dos artefatos (`grep -ac '^FAIL'` por shard), log linhas 8617-8624:

```
shard 0: OK=51 FAIL=1   shard 1: OK=45 FAIL=2   shard 2: OK=63 FAIL=1   shard 3: OK=33 FAIL=3
shard 4: OK=49 FAIL=4   shard 5: OK=21 FAIL=0   shard 6: OK=20 FAIL=0   shard 7: OK=65 FAIL=0
```

A concordância entre duas apurações independentes (a minha, sobre o log do run; a do CI, sobre os
artefatos dos shards) fecha a leitura alternativa *"o log que você baixou veio truncado"*.

### Os dois fantasmas, nomeados

| rótulo listado na REQ | o que o censo realmente mediu |
|---|---|
| `credential-guard-script-integrity/detected` | **`OK`** — log linha 5134, shard 4 |
| `git-branch-guard-global-script-integrity/detected` | 🔴 **não existe**. O rótulo real do braço é `git-branch-guard-global-script-integrity/detected-without-wiring`, e ele está **`OK`** — log linha 4163, shard 3 |

Ambos entraram na lista porque aparecem como **texto citado dentro de uma mensagem `PROOF` de
não-vacuidade** — não como veredito. Linhas 4164 e 5135, literalmente:

```
PROOF [falsify/credential-guard-script-integrity/non-vacuity]: com a regra desligada, o braço de
detecção FALHARIA — assert_fails_with ecoaria "FAIL [falsify/credential-guard-script-integrity/
detected]: saiu com 0, esperava != 0" (mensagem 'content diverges …' ausente, exit=0).
```

Um filtro que casa o **token** `FAIL [falsify/` em vez da **forma** (linha ancorada em `^FAIL`)
colhe as duas. O censo faz a coisa certa — usa `grep -ac '^FAIL'`. A enumeração que produziu a REQ,
não.

### Por que este achado é a primeira seção e não um rodapé

1. O aviso operacional desta campanha é *"três enumerações consecutivas foram **limite inferior**
   porque o filtro caçava um token em vez da forma"*. Este é **o mesmo defeito produzindo um limite
   superior**. A regra precisa ser lida nas duas direções: *enumere pela forma*, não *enumere mais*.
2. 🔴 **A inflação caiu exatamente sobre a superfície de segurança.** Os dois fantasmas são os dois
   controles de integridade de script — `credential-guard` e `git-branch-guard` global. Lidos como
   `FAIL`, dizem que a integridade dos guards não é provada no Windows. Medidos, dizem o contrário:
   **os dois estão verdes**. Uma wave inteira teria sido desenhada para um buraco que não existe,
   enquanto os buracos reais (seção 5) ficavam sem dono.

**Consequência para o escopo:** a população desta REQ é **14 rótulos** — 11 em `FAIL` + 3 ausentes.
A tabela da seção 1 entrega as 16 linhas pedidas: 14 reais e 2 marcadas como fantasma, com a
evidência. Não padded, não silenciosamente reduzida.

---

## 1. Tabela: rótulo → mecanismo → grupo

| # | rótulo | estado medido | mecanismo | grupo |
|---|---|---|---|---|
| 1 | `setup-s75` | FAIL (sh 3) | `check-release-tag-parity.sh` reprova com o binário REAL: cenário `no-forge-cli/go`, filho nativo morre no `NO_FORGE_PATH` curado | **G1** |
| 2 | `setup-s76` | FAIL (sh 1) | idem | **G1** |
| 3 | `setup-s87-baseline` | FAIL (sh 0) | idem | **G1** |
| 4 | `setup-s158-baseline` | FAIL (sh 2) | idem | **G1** |
| 5 | `release-tag-parity/success/baseline-clean` | **AUSENTE** | o `echo OK` está no `else` do braço do `setup-s75`; setup reprovou ⇒ rótulo nunca impresso | **G1** |
| 6 | `release-tag-parity/forge-commit-diverges-update-ref/baseline-clean` | **AUSENTE** | idem, `else` do `setup-s76` | **G1** |
| 7 | `setup-s175-baseline` | FAIL (sh 4) | `check-update-parity.sh` reprova com o binário REAL: morre no `ln -sf` do Cenário 9 sob `set -euo pipefail` | **G2** |
| 8 | `sandbox-gap-e/direction-a-detected` | FAIL (sh 4) | `assert_fails_with` recebe rc=1 sem o diagnóstico `sandbox/gap-e/dry-vs-real` — o gate morreu antes do cenário, pela mesma causa | **G2** |
| 9 | `sandbox-walkdir-reintroduced/direction-b-detected` | FAIL (sh 3) | idem, sem o diagnóstico `sandbox/dangling-outside-set/exit-zero` | **G2** |
| 10 | `sandbox-gap-e/direction-a-baseline` | **AUSENTE** | `else` do braço do `setup-s175-baseline` | **G2** |
| 11 | `scaffold-update-chmod-removed/direction-c-detected` | FAIL (sh 3) | `chmod 0644` não retira o bit de execução; `test -x` verdadeiro nos dois braços | **G3** |
| 12 | `git-branch-guard-dedup/baseline-skips-project-entry` | FAIL (sh 4) | `HOME` em grafia MSYS chega ao Go; o caminho reconstruído não casa com o gravado no `settings.json` | **G4** |
| 13 | `git-branch-guard-dedup/double-slash-tolerance` | FAIL (sh 4) | idem — a divergência é a **montante** do tratamento de `//` | **G4** |
| 14 | `git-branch-guard/stdin-drain-before-noop/baseline-writer-clean-large-payload` | FAIL (sh 1) | dreno de stdin não consome 200 KB antes do guard sair; escritor recebe `BrokenPipeError` | **G5** |
| — | ~~`credential-guard-script-integrity/detected`~~ | **`OK`** (sh 4, l. 5134) | 🔴 fantasma: texto citado dentro de `PROOF … /non-vacuity` (l. 5135) | **fora** |
| — | ~~`git-branch-guard-global-script-integrity/detected`~~ | **rótulo inexistente**; o real é `/detected-without-wiring`, **`OK`** (sh 3, l. 4163) | 🔴 fantasma: texto citado dentro de `PROOF … /non-vacuity` (l. 4164) | **fora** |

---

## 2. 🔴 A hipótese do `setup`: **CAI**, e cai por dois lados

> *"Um `setup` que falha aborta o cenário inteiro."*

### Medição A — o código (`scripts/check-gates-falsify.sh:327`)

```bash
falsify_fail_point() {
  if [[ "$TRACKFW_FALSIFY_ENUMERATE" == "1" ]]; then
    printf 'x\n' >> "$FALSIFY_ENUM_TALLY"
    return 0            # ← não aborta nada
  fi
  exit 1                # ← aborta o SCRIPT INTEIRO, não "o cenário"
}
```

O censo roda com `TRACKFW_FALSIFY_ENUMERATE=1`. **No censo, um `setup` que falha não aborta coisa
alguma** — tallya e segue.

### Medição B — o log, que corrobora

Shard 3, em sequência (log 4168 → 4186): `FAIL [setup-s75]` é seguido por **nove rótulos adicionais**
do mesmo shard, incluindo `OK [release-tag-parity/success-lightweight-tag-false-negative]` (o braço
de detecção do próprio Cenário 75) e depois os Cenários 176 e 181 inteiros. O cenário **não abortou**;
nem o seguinte.

### Veredito

| a hipótese diz | a medição diz |
|---|---|
| o `setup` aborta **o cenário** | em modo enumerate, **não aborta nada**; em modo normal, aborta **o script inteiro** — granularidade errada nas duas direções |
| os 8 não-`setup` fecham quando o `setup` passa | **nenhum** fecha *porque o setup passou* |

**O que o `setup` que falha realmente causa:** o `echo OK` do braço limpo mora no `else` do `if`, logo
o rótulo **não é impresso**. Isso explica, com precisão, **3 rótulos — e são exatamente os 3
ausentes** (#5, #6, #10 da tabela). Zero dos 11 `FAIL`.

### 🔴 O que sobrevive da intuição, reformulado — e importa

4 dos 11 `FAIL` (#1-#4) e 3 outros (#7-#9) **compartilham causa** com os setups — mas por **causa
comum**, não por encadeamento. `setup-s175-baseline` e `sandbox-gap-e/direction-a-detected` falham
porque **o mesmo gate está quebrado no Windows**, não porque um dependa do outro.

A distinção não é retórica, é operacional: **corrigir o braço de `setup` não fecharia os outros.**
Fechar o **gate** fecha os dois. Uma wave desenhada sobre a hipótese original consertaria os braços de
baseline e recontaria um delta menor que o previsto — e, pelo critério do roadmap, isso viraria
"achado" quando na verdade seria consequência de uma premissa errada.

---

## 3. 🔴 Segundo achado não previsto: dois `setup` reprovam e o cenário imprime `OK` mesmo assim

Existem **duas formas** para o mesmo braço de baseline no `check-gates-falsify.sh`.

**Forma A — correta** (Cenários 75, 76, 175):

```bash
if ! GO_BIN="$FALSIFY_GO_BIN" bash "…/check-release-tag-parity.sh" >/dev/null 2>&1; then
  echo "FAIL [falsify/setup-s75]: …" >&2
  falsify_fail_point
else
  falsify_count_success
  echo "OK   [falsify/release-tag-parity/success/baseline-clean]"
fi
```

**Forma B — 🔴 falso verde** (Cenários 87 `:6066`, 158 `:6123`):

```bash
if ! GO_BIN="$FALSIFY_GO_BIN" bash "…/check-release-tag-parity.sh" >/dev/null 2>&1; then
  echo 'FAIL [falsify/setup-s87-baseline]: …' >&2
  falsify_fail_point
fi
echo 'OK   [falsify/release-tag-parity/content-from-commit-baseline]'   # ← FORA do if
```

### Medido no censo, linhas adjacentes

```
1457  FAIL [falsify/setup-s87-baseline]: check-release-tag-parity.sh ja reprova com binario real
1458  OK   [falsify/release-tag-parity/content-from-commit-baseline]
1459  OK   [falsify/release-tag-parity/content-from-commit-false-negative]

2343  FAIL [falsify/setup-s158-baseline]: check-release-tag-parity.sh ja reprova com binario real
2344  OK   [falsify/release-tag-parity/refs-replace-bypass-baseline]
2345  OK   [falsify/release-tag-parity/refs-replace-bypass-false-negative]
```

O rótulo que **afirma** "o gate passa limpo com o binário real" é impresso `OK` na linha imediatamente
seguinte àquela que mede o contrário.

### O que exatamente se perde — com precisão

Os braços de detecção (1459, 2345) **não são vazios**: `assert_fails_with` casou a mensagem esperada
(`provenance: tag message must contain 'forge-only'`, etc.), então a asserção **disparou de verdade**.
🔴 O que se perde é o **discriminante de delta único**: o desenho prova "o binário sabotado faz o gate
reprovar" contra "o binário real faz o gate passar". Com a baseline reprovando, o segundo termo é
falso, e o `OK` do braço de detecção deixa de distinguir *"a sabotagem foi detectada"* de *"o gate já
reprovava por outro motivo e a mensagem coincidiu"*.

### Um detector mecânico para a Wave 1

A Forma B imprime `echo OK` **sem** chamar `falsify_count_success`. Logo **a contagem de linhas `^OK`
do log diverge do arquivo de tally** exatamente nos sítios afetados. Isso é uma forma verificável por
construção — não exige adivinhar a intenção de cada cenário. É o formato natural do gate que a Wave 1
vai precisar, e é o único achado desta triagem em que **o instrumento fica verde sobre algo que não
provou** (os demais reprovam alto).

**Inflação de `OK=347` que consigo atribuir:** 2 linhas da Forma B (1458, 2344) + 1 baseline vacuosa
do G3 (seção 5.3) + 2 braços não-discriminantes do G4 (seção 5.4). Não afirmo nada além disso: não
auditei os 347.

---

## 4. Destino do #307 e do #308

### #307 — **ABSORVIDO**, com as diferenças escritas

O #307 descreve **dois** bloqueios. Os **dois** aparecem no censo, em **gates diferentes**, e juntos
explicam **10 dos 14 rótulos reais**.

**Bloqueio 1 — `ln -s` degrada para cópia no Git for Windows → G2 (4 rótulos).**
`scripts/check-update-parity.sh:439-444`, sob `set -euo pipefail` (`:19`):

```bash
S9_PROJ="$WORK/s9-proj"
mkdir -p "$S9_PROJ/.venv/bin"
ln -sf /nonexistent-python3.99 "$S9_PROJ/.venv/bin/python"
```

Medido no censo (shards 3 e 4):

```
ln: failed to create symbolic link '/tmp/trackfw-update-parity.CuOkys/s9-proj/.venv/bin/python':
    No such file or directory
```

🔴 **É a última linha que o gate imprime.** Não há `OK [sandbox/dangling-outside-set]`, não há
Cenários 10-14, não há `check-update-parity: behavioral pin failures detected`, não há
`All check-update-parity.sh scenarios passed`. O gate **morre no `ln`** e sai 1.

O mecanismo é o do #307 §2: sem `winsymlinks`, `ln -s` copia em vez de linkar — e **o alvo é
deliberadamente inexistente**, então a cópia falha com `ENOENT`. O `-f` não ajuda: ele remove o
destino, não cria a origem.

**Bloqueio 2 — filho nativo não roda no `PATH` curado, e a guarda acusa o sujeito errado → G1
(6 rótulos).** Único cenário reprovando dentro do `check-release-tag-parity.sh` (log 5080):

```
FAIL [release-tag-parity/no-forge-cli/go]: vacuity guard: stderr missing the no-forge-CLI refusal;
  stderr: Error: trackfw release tag refuses to run: could not fetch origin
          (git fetch origin --prune exited with 3221225477).
```

Os outros 19 cenários do gate saem `OK` (log 5073-5091). `3221225477` = `0xC0000005`. A guarda
conclui *"falta a recusa de no-forge-CLI"* a partir de um observável que significa *"o filho nativo
morreu"* — a mesma forma que o #307 §4 nomeia: **a guarda mede uma coisa e declara outra**.

**As três diferenças medidas, que o #307 não previa e que a Wave 1 precisa saber:**

| | #307 (Win 11 x64, GfW 2.45.1, sem `winsymlinks`) | censo (`windows-latest` x64) |
|---|---|---|
| guarda pré-cenário de `python3` no `NO_FORGE_PATH` (`check-release-tag-parity.sh:185-188`) | **reprova** — `python3` copiado não inicia | **passa** — o gate chega a rodar os 20 cenários |
| `ln -s` do `python3` para `RUNTIME_BIN` | **morre** (`Permission denied`) | **não morre** — o que morre é o `ln` de alvo **inexistente** do `check-update-parity.sh` |
| sujeito que falha no `PATH` curado | `python3` | **`git`**, `0xC0000005` |

🔴 **Por que absorver mesmo assim.** A Regra Dura de Causa Raiz manda agrupar por **causa**, e a causa
é uma: *o `PATH`/FS curado do Windows não sustenta um processo nativo, e o diagnóstico publicado nomeia
o sujeito errado*. O #307 já escreveu essa causa, com medição de terceiro. Redescobri-la seria abrir
trabalho paralelo sobre a mesma causa — precisamente o que a regra proíbe.

⚠️ **Não afirmo por que o `git` sai `0xC0000005`.** O `NO_FORGE_PATH` = `$RUNTIME_BIN:$GIT_BIN_DIR`
exclui `/usr/bin` de propósito (`check-release-tag-parity.sh:153-163`), e a hipótese óbvia é
carregamento de DLL — mas o próprio autor do #307 registrou que **a primeira medição dele estava
contaminada pelo `PATH` do ambiente** nessa exata classe de erro. O diretório faltante é **projeto da
correção da Wave 1**, não conclusão desta triagem. O observável medido basta para o grupo e para a
frase de fechamento.

### #308 — **DESCARTADO** como membro do G4, com a diferença escrita

O #308 mede: *MSYS **expande chaves** (`{a}` → `a`, `{a,b}` → dois argumentos) ao reconstruir `argv`
quando o pai é um processo **nativo***; falsificou as duas hipóteses anteriores (citação do `os.Args`
do Go; conversão de caminho do MSYS — `MSYS_NO_PATHCONV=1` e `MSYS2_ARG_CONV_EXCL='*'` não mudam nada).

O G4 não tem `argv` no caminho, não tem chaves, e nenhum processo nativo reconstrói a string. A
divergência ocorre **inteiramente dentro do Go**, entre um caminho lido de um arquivo JSON e um
caminho construído por `filepath.Join`. Nada do que o #308 mediu se aplica; nada do remédio dele
tocaria o G4.

**Mecanismo do G4, por construção** (`internal/generators/agentfiles.go`):

- o fixture grava no `settings.json` global a grafia MSYS do `HOME`
  (`/tmp/trackfw-falsify.*/s67-fake-home-installed/.trackfw/scripts/trackfw-git-branch-guard.sh`) —
  `check-gates-falsify.sh:4724-4726`;
- o Go reconstrói o caminho com `filepath.Join(home, ".trackfw", "scripts", …)`
  (`globalGitBranchGuardScriptPath`, `:2098`), que no Windows emite `\` como separador;
- a comparação passa por `normalizeGuardPath`, e ela **só troca `\` por `/` se
  `hasWindowsDriveLetterPrefix` for verdadeiro** — isto é, `X:/` ou `X:\` nos 3 primeiros bytes. Um
  `HOME` de grafia MSYS **não tem letra de unidade**, então `\tmp\…` fica com `\` e nunca casa com
  `/tmp/…`.

🔴 **Isto explica por que os DOIS braços (#12 e #13) reprovam.** Se a causa fosse tolerância a `//`,
o braço `baseline-skips-project-entry` — que usa um `HOME` deliberadamente *sem* `//`
(`WORK67_CLEAN`, `:4717`) — teria passado. Os dois falham porque a divergência é de **separador**,
a montante de qualquer normalização de barra dupla.

⚠️ **Elo não determinado, e o discriminante para a Wave 1.** Há duas travessias possíveis do mesmo
`HOME` malformado: (a) `readGlobalHookJSON` (`:2102`) falha ao abrir `\tmp\…` (que no Windows
significa a raiz da unidade corrente) e o **fail-open** devolve `false`; ou (b) o arquivo é lido e a
**comparação** não casa. O sintoma é idêntico e o censo não distingue. Discriminante barato:
instrumentar/observar se `readGlobalHookJSON` retorna `ok=false`. **A causa raiz é a mesma nos dois
ramos** — grafia MSYS de `HOME` cruzando a fronteira para o Go — e o grupo não muda.

---

## 5. Grupos e frases de fechamento

### G1 — `check-release-tag-parity.sh` reprova com o binário real no `PATH` curado

**Rótulos:** `setup-s75`, `setup-s76`, `setup-s87-baseline`, `setup-s158-baseline`,
`release-tag-parity/success/baseline-clean` (ausente),
`release-tag-parity/forge-commit-diverges-update-ref/baseline-clean` (ausente). **6.**

> *Corrijo o cenário `no-forge-cli` — o filho nativo passa a rodar no `NO_FORGE_PATH`, ou a guarda
> passa a distinguir "recusa ausente" de "filho nativo não iniciou" — e **exatamente estes 6 rótulos
> fecham, e nenhum outro**.*

⚠️ **Ressalva declarada — o delta previsto é um par, não um número.** Os 6 fecham porque o gate volta
a passar com o binário real, mas eles **não se movem todos no mesmo eixo**:

| rótulos | hoje | depois da correção | contribui para |
|---|---|---|---|
| `setup-s75`, `setup-s76` | `FAIL` | somem (o `else` passa a rodar) | `FAIL −2` |
| `setup-s87-baseline`, `setup-s158-baseline` | `FAIL` | somem | `FAIL −2` |
| `release-tag-parity/success/baseline-clean`, `…/forge-commit-diverges-update-ref/baseline-clean` | **ausentes** | passam a imprimir `OK` | `OK +2` |
| `…/content-from-commit-baseline`, `…/refs-replace-bypass-baseline` | já imprimem `OK` (Forma B) | continuam `OK` | **0** |

**Previsão a comparar na recontagem: `FAIL −4` e `OK +2`** — não `−6`. Os dois `OK` da Forma B já
estavam sendo contados no log e **não** no tally (seção 3), então a divergência `contagem de ^OK ≠
tally` **encolhe em 2** pela mesma correção. Se a recontagem mostrar `−6`, ou `OK +4`, isso é achado.

### G2 — `check-update-parity.sh` morre no `ln -sf` do Cenário 9

**Rótulos:** `setup-s175-baseline`, `sandbox-gap-e/direction-a-detected`,
`sandbox-walkdir-reintroduced/direction-b-detected`, `sandbox-gap-e/direction-a-baseline` (ausente).
**4.**

> *Corrijo a construção da fixture de symlink pendurado do Cenário 9 — para que ela seja construível
> onde `ln -s` não é symlink, ou declare skip nomeando a garantia não exercitada — e **exatamente
> estes 4 rótulos fecham, e nenhum outro**.*

🔴 **Correção ao escopo negativo da REQ, com medição.** A REQ exclui `.venv/bin/python` como *"outra
causa, com medição escrita — venv no Windows usa `Scripts/`, não symlink em `bin/`"*. Essa
classificação **não se sustenta**: o Cenário 9 **não usa um venv**. Ele fabrica um symlink pendurado
sintético para um alvo inexistente, e o `bin/` é escolha da fixture, não do Python. A causa medida é a
degradação do `ln -s` — **a mesma do #307 §2** — e, pela Regra Dura, *"está fora do escopo declarado"*
não justifica separar: o escopo estava estreito demais. **Estes 4 rótulos entram nesta REQ.**

### G3 — `chmod` não retira o bit de execução (mesma causa da #421)

**Rótulos:** `scaffold-update-chmod-removed/direction-c-detected`. **1.**

> *Corrijo a construção da fixture "presente e não executável" — por ACL nativa, ou por skip que nomeie
> a garantia não exercitada — e **exatamente este rótulo fecha, e nenhum outro**.*

**Evidência de que é a causa da #421, e não outra.** O braço de detecção (`:6614-6634`) faz
`chmod 0644` e depois `test -x`; o censo imprime o `ls -la` de diagnóstico (log 4214):

```
-rwxr-xr-x 1 runneradmin 197121 176 Sep 24 17:50 …/s181-det/project/scripts/trackfw-validate.sh
```

`rwxr-xr-x` **depois de `chmod 0644`** — exatamente a medição da #421 (`chmod 644 → ls -l: -rwxr-xr-x`
em NTFS `noacl`). No **mesmo shard 4**, o `check-validate-rule-pins.sh` reprova em
`[pin7-noexec] vacuity: … expected 'credential_guard_hook_resolvable' violation, none found` (log
5127) — o sintoma nominal da #421.

🔴 **A #421 declara explicitamente não ter medido o runner do GitHub Actions:** *"Não medido: se o
runner (`windows-latest`, x64) monta o `TMPDIR` com as mesmas opções — provável, mas não verificado."*
**Este censo verifica.** A #421 vale também em x64 CI, e não só na VM ARM64.

**Decisão que não é minha:** a REQ coloca a #421 no escopo negativo. Pela Regra Dura, mesma causa =
mesma REQ, e este rótulo é a mesma causa. Ou a #421 é absorvida aqui, ou o rótulo viaja com a #421 —
mas ele **não pode ficar sem dono**, que é o estado em que a REQ o deixa hoje. Encaminho ao arquiteto.

### G4 — grafia MSYS de `HOME` cruza para o Go sem normalização de separador

**Rótulos:** `git-branch-guard-dedup/baseline-skips-project-entry`,
`git-branch-guard-dedup/double-slash-tolerance`. **2.**

> *Corrijo a fronteira de grafia de caminho entre o `HOME` do fixture e a comparação do Go — seja
> normalizando a grafia MSYS na entrada, seja alargando `normalizeGuardPath` para caminho nativo sem
> letra de unidade — e **exatamente estes 2 rótulos fecham, e nenhum outro**.*

🔴 **Não é só teste.** `normalizeGuardPath` e `hasWindowsDriveLetterPrefix` são **código de produto**
(`internal/generators/agentfiles.go`). A seção 6.4 trata o que isso significa para o controle.

### G5 — dreno de stdin não sustenta payload grande no pipe do Windows

**Rótulos:** `git-branch-guard/stdin-drain-before-noop/baseline-writer-clean-large-payload`. **1.**

> *Corrijo o dreno de stdin do `trackfw-git-branch-guard.sh` para consumir o payload inteiro antes de
> sair no caminho de no-op — sem depender do orçamento `-t 2` — e **exatamente este rótulo fecha, e
> nenhum outro**.*

**Medido (log 7156-7161):** o braço de payload pequeno é `OK` (`escritor_erro=0`); o de 200 KB reprova
com `BrokenPipeError: [Errno 32] Broken pipe` no escritor. O dreno é
`IFS= read -r -t 2 -d '' _TRACKFW_STDIN || true` (`:4649`). **Também é código de produto** — o script
é gerado por `internal/generators/scaffold.go`.

---

## 6. Modelo de ameaça — por rótulo de controle de segurança

⚠️ **A distinção que organiza esta seção.** *"Falha ruidosa"* ≠ *"garantia sem prova"*. Uma REQ
anterior mediu que um risco parecido era **menor** do que parecia, porque o gate **reprovava alto** em
vez de silenciar. Aqui a mesma disciplina produz **quatro** categorias, não duas — e são categorias de
**risco diferente**, não graus da mesma coisa:

| categoria | o que acontece | risco |
|---|---|---|
| **A — recusa ruidosa** | o gate reprova, alto, e o CI fica vermelho | **baixo**: a garantia não é exercitada, mas ninguém lê verde onde não há |
| **B — falso verde** | o rótulo imprime `OK` sobre algo que não provou | 🔴 **alto**: o instrumento afirma o que não mediu |
| **C — passagem vacuosa** | o rótulo imprime `OK` porque o predicado não discrimina | 🔴 **alto**: verde estrutural, indistinguível de verde real |
| **D — controle ausente no Windows** | o comportamento de produto medido no Windows **não é o contratado** | 🔴 **alto**: não é artefato de teste, é o controle |

### 6.0 Os dois fantasmas — os controles de integridade de script **estão provados** no Windows

| rótulo | garantia | estado |
|---|---|---|
| `credential-guard-script-integrity/baseline` + `/detected` | `validate` aceita o script de credential-guard íntegro e **reprova** o divergente do template | ✅ os dois `OK` (log 5133-5134) |
| `git-branch-guard-global-script-integrity/{baseline, absent-is-not-a-violation, detected-without-wiring, no-double-report}` | integridade do guard **global**, incluindo ausência-não-é-violação e não-duplicação | ✅ os quatro `OK` (log 4161-4166) |

**Nenhum risco.** É o inverso: o modelo de ameaça que a REQ encomendou pressupunha buraco onde há
cobertura. Registrar isso é o entregável, não a ausência dele.

### 6.1 G1 — `release-tag-parity` · **categoria A**

| garantia sem prova no Windows | cobertura equivalente |
|---|---|
| tag anotada não degrada para *lightweight* (s75) | ❌ nenhuma no Windows. O braço de detecção **não rodou** com baseline válida |
| divergência commit↔forge é recusada (s76, Emenda 1 da ADR) | ❌ idem |
| conteúdo da tag vem do commit do forge, não do `HEAD` (s87) | ⚠️ braço de detecção `OK`, mas **sem delta único** (seção 3) |
| `--no-replace-objects` bloqueia substituição via `refs/replace/` (s158) | ⚠️ idem |

**Risco real: baixo.** O gate **recusa alto** — `check-release-tag-parity.sh: one or more scenarios
FAILED`, o shard sai 1, o job fica vermelho. É exatamente a nota de mérito do #307: *"a guarda faz o
trabalho dela; o ponto cego é real, mas nunca vira verde falso"*. Os 19 outros cenários (incluindo
`forge-symref-repoint-neutralized`, `forge-commit-diverges-*`, `object-absent`,
`content-from-commit-provenance`, `refs-replace-bypass`) **passam** no Windows. Cobertura equivalente
existe: o mesmo gate roda inteiro em `ubuntu-latest` via `make parity-rest`.

**O que fica genuinamente descoberto:** o cenário `no-forge-cli` — *o comando recusa rodar quando não
há CLI de forge* — não é exercitado em nenhum Windows. 🔴 **E aqui há um agravante que não é ruído:**
o diagnóstico publicado manda procurar o lugar errado. Um mantenedor que leia *"stderr missing the
no-forge-CLI refusal"* investiga o `release tag`; o que quebrou foi o ambiente. Custo de diagnóstico,
não de segurança — mas o #307 já pagou esse custo uma vez, e este censo o pagaria de novo.

### 6.2 G2 — `check-update-parity.sh` / sandbox · **categoria A**

| garantia sem prova no Windows | cobertura equivalente |
|---|---|
| symlink pendurado **fora** do conjunto declarado não aborta `--dry-run` (Cen. 9) | ❌ o gate morre **ao construir a fixture** |
| symlink pendurado **dentro** do conjunto (Cen. 10) e Cenários 11-14 | ❌ nunca alcançados |
| `trackfw.yaml` no sandbox: `dry-run` ≡ run real (`sandbox/gap-e`) | ❌ |
| `copyProjectTree` por inclusão, não `WalkDir` — o incidente do CMDB | ❌ |

🔴 **Este é o grupo cuja garantia mais importa.** `copyProjectTree` por inclusão existe porque uma
travessia de árvore inteira **abortou num symlink pendurado** num repositório real. Windows é
justamente onde symlink tem semântica diferente. **A superfície de maior risco é a menos exercitada.**

**Risco real: baixo-a-médio.** Baixo porque a falha é ruidosa (gate sai 1, shard vermelho) e porque
os 8 cenários anteriores do gate **passam** no Windows — a regressão não é silenciosa. Médio porque
o corte é **por construção da fixture**, não por diferença de comportamento: nenhuma quantidade de
execução no Windows exercita esses 6 cenários enquanto o `ln -sf` morrer, e é fácil ler os 8 `OK`
anteriores como "o sandbox está coberto no Windows". Não está.

**Cobertura equivalente:** `ubuntu-latest`/macOS rodam os 14 cenários. A garantia **existe**; o que
não existe é a prova **no sistema operacional onde o mecanismo difere**.

### 6.3 G3 — bit de execução · **categoria C** 🔴

Este é o único rótulo do cluster em que o **braço de baseline passa vacuosamente**.

```
baseline  : chmod 0644 → update (binário REAL) → test -x  → verdadeiro → OK   ✅
detecção  : chmod 0644 → update (binário SABOTADO, sem os.Chmod) → test -x → verdadeiro → FAIL
```

`test -x` é verdadeiro **nos dois braços, independentemente de `os.Chmod` ter rodado**. O predicado
não discrimina. Logo:

- o `OK [.../direction-c-baseline]` **não prova** que o binário real restaura o bit — prova que o FS
  não representa o bit;
- 🔴 se o braço de detecção fosse removido, o cenário ficaria **inteiramente verde no Windows** sem
  exercitar nada. É esse o formato do falso verde, e ele só não se materializou porque o braço de
  detecção reprova.

| garantia sem prova no Windows | cobertura equivalente |
|---|---|
| `update` **restaura** o bit de execução de `trackfw-validate.sh` rebaixado | ❌ inconstruível nesse FS |
| `credential_guard_hook_resolvable` dispara com hook presente-mas-não-executável (`pin7-noexec`) | ❌ mesma causa, mesmo shard |

**Risco real: médio, e com um risco de segunda ordem.** O controle — *o hook de credential-guard tem
de ser executável, e o `validate` acusa quando não é* — é **real e consequente**: um hook não
executável é um guard que não roda. No Windows ele **não é exercitado nem exercitável** por `chmod`.
O risco de segunda ordem é o caminho de "correção" que a #421 já nomeia e proíbe: **relaxar a
asserção até passar**. Subscrevo. Se a saída for `skip`, ele tem de **nomear a garantia não
exercitada** — o roadmap já exige isso ("supressão exige nomear a garantia não exercitada"), e este é
o rótulo onde a exigência vale mais.

### 6.4 G4 — dedup do git-branch-guard · **categoria D** 🔴 — o risco real deste cluster

Os outros grupos são gates que não rodam. **Este é o produto se comportando diferente no Windows.**

`normalizeGuardPath`, `hasWindowsDriveLetterPrefix`, `globalGitBranchGuardScriptPath` e
`readGlobalHookJSON` são **`internal/generators/agentfiles.go`** — produto, não harness. O censo
mediu, com o binário real, que num `HOME` de grafia MSYS o `discover --init` **não reconhece a fiação
global** e grava a entrada de projeto assim mesmo.

| garantia | estado medido no Windows |
|---|---|
| com o guard global instalado, `discover --init` **não** grava a entrada de projeto | 🔴 **falsa** — a entrada foi gravada (log 5149) |
| a comparação normaliza `//` antes de comparar | 🔴 **falsa** — e falha também **sem** `//` (log 5237) |
| `readGlobalHookJSON` falha-aberto em erro de leitura | ⚠️ por desenho `fail-open: any read/parse error → false` (`:2096`). **Não determinado** se este ramo chega a disparar aqui — é o elo (a) da seção 4, e o censo não o distingue do elo (b) |

⚠️ **O que eu NÃO afirmo:** que o `fail-open` seja "o caminho comum no Windows". Isso exigiria saber
qual dos dois elos dispara, e a seção 4 registra que não sei. 🔴 **O argumento sobrevive nos dois
ramos** — se for (a), o `fail-open` devolve `false`; se for (b), a comparação devolve `false`. Nos
dois casos a resposta é *"não instalado"*, que é a **permissiva**.

**Impacto.** O sintoma imediato é benigno — **hook duplicado**, mensagem em dobro, não um guard
ausente. O guard ainda roda; roda duas vezes. **Não classifico isto como bypass de controle**, e não
tenho medição que sustente um bypass.

🔴 **O que me preocupa é a forma, e ela é reincidente neste repositório.** O padrão é o mesmo que já
medi duas vezes aqui: *o escritor grava numa gramática, o leitor reconstrói noutra, e a comparação
falha em silêncio*. Já apareceu em `provenanceKey`/`filepath.Rel` contra chaves JSON sempre-`/`, e na
REQ-2026-09-24 dos caminhos interpolados. **`normalizeGuardPath` tem uma guarda de gramática que
cobre só metade do espaço**: converte `\`→`/` **apenas** com letra de unidade. Um caminho nativo sem
letra de unidade — `\tmp\…` — é precisamente o caso que ela não cobre, e o `fail-open` transforma o
não-casamento em *"não instalado"*, que é a resposta **permissiva**.

**Cobertura equivalente: ⚠️ presumivelmente nenhuma.** Os testes Go de `agentfiles` rodam com caminhos
da plataforma hospedeira; num runner POSIX, `filepath.Join` emite `/` e a divergência **não pode**
aparecer. Não enumerei os testes Go desse pacote — **declaro como não medido**, e é o primeiro item
que a Wave que tratar o G4 deve verificar. Se confirmado, a única prova dessa fronteira no Windows é
justamente o rótulo que está reprovando.

**Formulação da ameaça (o adversário é o implementador apressado, não o atacante externo):** um ML da
Wave 1 que "corrija" o G4 **relaxando o predicado** — por exemplo, comparando só o *basename* do
script — faria os dois rótulos passarem e **aumentaria** a superfície: dois scripts homônimos em
diretórios diferentes passariam a ser considerados o mesmo, e o dedup **suprimiria** a entrada de
projeto legítima. 🔴 **Nessa direção o defeito deixa de ser hook duplicado e vira guard ausente.** A
falsificação do G4 tem de exercitar **as duas direções**: caminho igual em grafias diferentes → casa;
caminho **diferente** com mesmo basename → **não** casa.

### 6.5 G5 — dreno de stdin antes do no-op · **categoria D** 🔴

Também produto: o script vem de `internal/generators/scaffold.go`.

| garantia | estado medido no Windows |
|---|---|
| quem escreve o payload no pipe do hook **não** recebe `EPIPE` (payload pequeno) | ✅ `OK` — `escritor_erro=0` |
| idem sob pressão de buffer (>64 KB; medido com 200 KB) | 🔴 **falsa** — `BrokenPipeError: [Errno 32] Broken pipe` |

**Risco real: médio.** Esta garantia existe por um motivo registrado: a auditoria do arquiteto
reprovou o ML-1A porque o no-op saía antes de ler o stdin e **o harness que escreve o payload recebia
`EPIPE` em 100% das chamadas fora de projeto trackfw**. É defeito de **integração com o cliente de
agente** — o cliente vê um hook que estoura o pipe dele. No Windows, com payload grande, **a
regressão está presente**.

🔴 **E é aqui que a distinção ruidoso/silencioso inverte a favor do risco.** O guard sai **0**; o
erro aparece **no escritor**, não no guard. Em produção o escritor é o cliente de agente, que pode
tratar `EPIPE` em silêncio. **Não há recusa alta.** O rótulo só é visível porque o harness de
falsificação instrumenta o escritor de propósito — remova o rótulo e o defeito fica invisível.

**Cobertura equivalente: nenhuma conhecida no Windows.** O braço de payload pequeno passa, e passar
nele **não implica** passar no grande — é o próprio desenho de três braços do Cenário 65 que afirma
isso.

⚠️ **Advertência de causa para a Wave 1:** o dreno é `read -r -t 2 -d ''` — orçamento de **tempo**. O
braço pequeno passa e o grande não. Isso admite pelo menos duas causas (o orçamento estoura antes de
200 KB atravessarem o pipe MSYS; ou `read -d ''` não drena até EOF nesse pipe) e o censo **não
discrimina**. Medir antes de corrigir. 🔴 Se a causa for o orçamento, **aumentar o `-t`** é
maquiagem: transfere o defeito para o payload seguinte.

---

## 7. Síntese para o arquiteto

| grupo | rótulos | mecanismo | categoria | onde vive a correção |
|---|---|---|---|---|
| **G1** | 6 | filho nativo morre no `NO_FORGE_PATH`; guarda acusa o sujeito errado (#307 §3-4) | A — ruidosa | `scripts/check-release-tag-parity.sh` |
| **G2** | 4 | `ln -s` degrada para cópia; `set -e` mata o gate no Cenário 9 (#307 §2) | A — ruidosa | `scripts/check-update-parity.sh` |
| **G3** | 1 | `chmod` não retira o bit em NTFS `noacl` (#421, agora medido em x64 CI) | C — vacuosa | fixture; decisão de escopo pendente |
| **G4** | 2 | grafia MSYS de `HOME` cruza para o Go; `normalizeGuardPath` cobre meio espaço | **D — produto** | 🔴 `internal/generators/agentfiles.go` |
| **G5** | 1 | dreno de stdin não sustenta 200 KB no pipe do Windows | **D — produto** | 🔴 `internal/generators/scaffold.go` |
| — | +2 fora | fantasmas de enumeração por token | — | nada a corrigir; corrigir a **REQ** |

**Ordem que eu proporia, e o motivo:** G4 e G5 primeiro. São os dois onde o **produto** se comporta
diferente no Windows — os outros três são gates que não rodam, e falham alto. Inverter a ordem
significa gastar as primeiras waves deixando o CI verde enquanto os dois defeitos de produto seguem em
produção.

### Itens que exigem decisão do arquiteto, não minha

1. **Escopo negativo da REQ, duas correções.** G2 (`.venv/bin/python`) está classificado como "outra
   causa" e **não é** — é o `ln -s` do #307. G3 é a mesma causa da #421, que a REQ exclui. Pela Regra
   Dura, mesma causa = mesma REQ.
2. **A população da REQ é 14, não 13+3.** Os dois rótulos fantasma precisam sair, **com a razão
   escrita** — senão a recontagem da Wave 1 procurará um delta que nunca existiu.
3. 🔴 **A Forma B (seção 3) não está em nenhuma REQ.** É o único falso verde do cluster e não é um dos
   14 rótulos. Precisa de ML próprio nesta REQ (mesma causa: *o instrumento afirma o que não mediu*),
   com o gate mecânico `contagem de ^OK no log ≠ tally`.

### 🔴 Nota de vault DEVIDA e deliberadamente não escrita

Três causas raiz desta triagem passam do critério de >10 min para o próximo agente — a **Forma B**, a
**meia-gramática do `normalizeGuardPath`** e a **degradação do `ln -s` matando o
`check-update-parity.sh` no `set -e`**. A regra do vault manda escrever a nota.

**Não escrevi**: o handoff deste ML restringe os arquivos a este parecer, e essa instrução é a mais
específica. Registro aqui para que a nota não fique **devida e invisível** — que é exatamente a falha
que a regra do vault existe para evitar. O arquiteto cria a nota, ou me autoriza a criá-la.

### Residual declarado — o que esta triagem NÃO cobre

- **Não medi na VM.** Todo mecanismo acima se resolveu por leitura de fonte + log do censo, e o
  handoff manda que todo número que vire afirmação saia do `windows-census.yml`. A VM é **ARM64**
  contra um censo **x64**: um resultado divergente lá seria investigação, não contradição. Clone da
  VM **não tocado** — nenhuma sessão `ssh` foi aberta nesta ML.
- **Não auditei os 347 `OK`.** Atribuí a inflação que consigo provar (5 linhas; seção 3) e parei.
- **Não determinei por que o `git` sai `0xC0000005`** no `NO_FORGE_PATH` (seção 4).
- **Não determinei qual dos dois elos do G4** (`readGlobalHookJSON` vs. comparação) dispara primeiro.
- **Não enumerei os testes Go** de `internal/generators/agentfiles.go` para confirmar a ausência de
  cobertura equivalente do G4 (seção 6.4). Declarado como presunção, não medição.
- **Não discriminei a causa do G5** entre orçamento de tempo e semântica de `read -d ''`.
- **Não rodei** `make quality` nem `go test ./...`.
