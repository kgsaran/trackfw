# Parecer de segurança — caminho POSIX interpolado dentro do código Python (MSYS)

> ML-0A (Wave 0) · `hades-tf` · 2026-09-24
> REQ: `docs/req/REQ-2026-09-24-caminho-posix-interpolado-dentro-do-codigo-python-nao-e-convertido-pelo-msys-e-o-open-morre-no-windows.md`
> Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-24-caminho-posix-interpolado-dentro-do-codigo-python-nao-e-convertido-pelo-msys-e-o-open-morre-no-windows.md`
> 🔴 Este documento é **medição**. Nenhuma linha de implementação foi escrita.

---

## 0. Critério — aplicável por terceiro, sem julgamento do revisor

Um sítio é **(a) defeito** se, e somente se, as **duas** condições valem:

1. **Interpolação do shell acontece dentro do texto do programa Python.** Operacionalmente: existe
   uma expansão (`$VAR`, `${VAR}`, `$(...)`) numa posição que o bash expande **antes** de o texto
   virar `argv[?]` do `python3` — ou seja, dentro de `-c "…"` (aspas **duplas**) ou dentro de
   heredoc com delimitador **não** citado (`<<PY`). Ficam **fora** por construção: `-c '…'` (aspas
   simples) e `<<'PY'`/`<<"PY"` (delimitador citado), onde o `$` chega literal ao Python.
2. **O valor interpolado é usado pelo Python para abrir/ler/escrever um arquivo** — `open(`,
   `pathlib.Path(...)` seguido de I/O, `os.chdir`, `shutil.*`, `listdir`, `makedirs`, `read_text`,
   `write_text`, `zipfile`/`tarfile`.

Classificação:

| | |
|---|---|
| **(a)** | as duas condições valem → **defeito** |
| **(b)** | o caminho chega por `argv` (`sys.argv[n]`), por variável de ambiente, ou já convertido (`cygpath`) → **correto** |
| **(c)** | o `$` interpolado **não é caminho**, ou é caminho que o Python nunca abre → **fora do escopo** |

O critério é decidível por leitura: condição 1 depende só da **forma de citação**, condição 2 só da
**chamada Python**. Nenhuma das duas exige saber o que o script faz.

---

## 1. Enumeração real

**Unidade de contagem declarada:** contamos **interpolações de caminho**, não arquivos e não blocos.
Total: **6 interpolações (a), em 5 blocos `python3 -c`, em 3 arquivos.**

### 1.1 Tabela dos sítios (a) — defeito

| # | `arquivo:linha` | o que interpola | consequência **medida** | onde foi medida |
|---|---|---|---|---|
| a1 | `scripts/check-gates-falsify.sh:6745` | `with open('$ROOT_DIR/npm/package.json') as f:` | 🔴 **aborta o `chunk_0`** — `CHUNK_ABORT rc=1 line=3609`, sem `CHUNK_COMPLETE`. Perda **medida por mim: 1 rótulo de asserção** (`integration-assets/direction-b-shim-absent`), não 4 — ver §1.1-bis | run `36017761462`, shard 0, **observado** + contagem no chunk regenerado |
| a2 | `scripts/check-serve-api-file-security.sh:87` | `src = open('$GO_API_FILE').read()` | **nenhuma hoje no CI** — o gate roda só em `ubuntu-latest`. No Windows cairia em `fail "AC6 … resultado inesperado"` (alto e ruidoso, sem falso-negativo) | análise de caminho + `set -euo pipefail:20` |
| a3 | `scripts/check-serve-api-file-security.sh:92` | `open('$VULN_GO', 'w').write(vuln)` | idem a2 — **inalcançável**: o `open()` de a2 mata o bloco antes | idem |
| a4 | `scripts/check-update-parity.sh:354` | `json.load(open('$manifest'))` (Cenário 6, `s6-home-go`) | **3 dos 11 rótulos FAIL do run**: `setup-s175-baseline`, `sandbox-gap-e/direction-a-detected` (chunk_4) e `sandbox-walkdir-reintroduced/direction-b-detected` (chunk_3) — **não** derruba chunk | run `36017761462`, shards 3 e 4, **observado** |
| a5 | `scripts/check-update-parity.sh:379` | `json.load(open('$manifest'))` (Cenário 7) | **latente** — `set -euo pipefail` (linha 19) mata o script em a4; nunca é alcançado. Mesma causa, fecha junto | derivada de a4 |
| a6 | `scripts/check-update-parity.sh:408` | `json.load(open('$manifest'))` (Cenário 8) | **latente**, idem a5 | derivada de a4 |

**Evidência bruta do run `36017761462` (`main`, `windows-census.yml`, 2026-09-24):**

```
censo (0)  FileNotFoundError: [Errno 2] No such file or directory: '/d/a/trackfw/trackfw/npm/package.json'
censo (0)  CHUNK_ABORT rc=1 line=3609 src=/tmp/trackfw-falsify-shard.6awTmO/chunk_0.sh cmd=python3 -c "
censo (3)  FileNotFoundError: [Errno 2] … '/tmp/trackfw-update-parity.vXvq48/s6-home-go/.trackfw/integrations-manifest.json'
censo (3)  FAIL [falsify/sandbox-walkdir-reintroduced/direction-b-detected]: saiu com 1 mas falta diagnóstico 'sandbox/dan…'
censo (4)  FAIL [falsify/setup-s175-baseline]: check-update-parity.sh ja reprova com o binario real -- prova P4 invalida
censo (4)  FAIL [falsify/sandbox-gap-e/direction-a-detected]: saiu com 1 mas falta diagnóstico 'sandbox/gap-e/dry-vs-real'
```

🔴 **A correspondência `chunk_0.sh:3609` ↔ `check-gates-falsify.sh:6745` foi verificada byte a byte**,
regerando os chunks localmente e lendo a linha:

```bash
python3 scripts/gen-falsify-chunks.py scripts/check-gates-falsify.sh "$S/chunks" 8
sed -n '3605,3615p' "$S/chunks/chunk_0.sh"   # → o bloco de :6743-6749, idêntico
```

O caminho no erro do shard 0 é `/d/a/trackfw/trackfw/npm/package.json` — a forma **POSIX-MSYS** de
`D:\a\trackfw\trackfw\...`. É a assinatura exata do mecanismo: o bash resolveu `$ROOT_DIR` para a
grafia POSIX, a interpolação entrou no texto do programa, e o Python nativo não a reconverteu.

### 1.1-bis 🔴 Correção de um número do handoff: são **1** rótulo, não 4

O handoff e o roadmap dizem que a1 *"leva 4 rótulos junto"*. **Medi, e não confere.** Contando os
emissores de veredito no chunk regenerado, depois do ponto de aborto:

```bash
awk 'NR>3609' "$S/chunks/chunk_0.sh" | grep -cE 'assert_fails_with'   # → 1
awk 'NR>3609' "$S/chunks/chunk_0.sh" | grep -cE 'falsify_count_success' # → 0
# (totais do chunk_0 inteiro: 37 e 35)
```

O único rótulo perdido é **`falsify/integration-assets/direction-b-shim-absent`**, a asserção do
próprio Cenário 185-B cuja fixture o `open()` estava montando.

De onde veio o 4: da apuração do run, que reporta **rótulos ausentes 19 → 4** — um **total do run
inteiro** contra uma lista-base, não a perda do `chunk_0`. São duas grandezas diferentes que
coincidiram na prosa. A tabela de apuração do `36017761462` fecha com os meus números:

```
shard 0: OK=50 FAIL=1   shard 1: OK=45 FAIL=2   shard 2: OK=63 FAIL=1   shard 3: OK=30 FAIL=3
shard 4: OK=46 FAIL=4   shard 5: OK=21 FAIL=0   shard 6: OK=20 FAIL=0   shard 7: OK=46 FAIL=0
                                                                        → TOTAL FAIL=11
```

🔴 **E o shard 0 NÃO caiu em `LOG SEM VEREDITO`** — emitiu 51 linhas de veredito antes de morrer,
então entrou no total. O dano de a1 é **menor** do que o handoff afirma.

**Atribuição declarada, para que a atribuição seja crível:** dos 11 FAIL, atribuo **3** a esta causa
(§1.1, a4). Os outros 8 **não** atribuo — entre eles `falsify/setup-s75`
(`check-release-tag-parity.sh`), `falsify/scaffold-update-chmod-removed/direction-c-detected` e
`falsify/git-branch-guard-dedup/*`, que têm outro mecanismo. A extração do log não foi truncada:
o filtro retornou 13 linhas, abaixo do `head -30`.

### 1.2 Ordem de prioridade para a Wave 1 (derivada da consequência, não do veredito)

1. **a1** — único que **aborta um chunk**. Custo medido: **1 rótulo que deixa de existir**. Vem
   primeiro não pelo volume (a4 produz o triplo), mas pela **natureza**: é o único cujo dano é
   *remover medição* em vez de produzir vermelho. Um rótulo ausente não reprova ninguém — ele some.
   🔴 **Se a ordenação for por volume, a4 vem primeiro.** A escolha aqui é por silêncio, e está dita.
2. **a4** (e com ele a5/a6, mesma causa, mesmo arquivo) — **3 dos 11 rótulos FAIL do run**. Volume
   maior, mas **honesto**: o censo diz que falhou e diz onde.
3. **a2/a3** — **zero** consequência no CI hoje. Entram porque são a mesma causa (Regra Dura), não
   por dano observado.

### 1.3 Sítios (b) — corretos, **não tocar**

| `arquivo:linha` | por quê é correto |
|---|---|
| `scripts/check-thirdparty-parity.sh:167` | `json.load(open(sys.argv[1]))` + caminho como argumento |
| `scripts/check-thirdparty-parity.sh:176` | `python3 - "$project/…" "$project" <<'PY'` — heredoc **citado** + `argv` |
| `scripts/check-thirdparty-parity.sh:194` | `open(sys.argv[1], encoding='utf-8')` + argumento |
| `scripts/check-manifest-version-gate.sh:79` | `json.load(open(sys.argv[1]))` + `"$pkg_json"` |
| `scripts/check-manifest-version-gate.sh:136` | `json.load(open(sys.argv[1]))` + `"$NPM_PKG"` |
| `scripts/check-roadmap-barrier-contract.sh:1095` | `open(sys.argv[1], 'wb')` + `"$FALSIFY_DIR/…"` |
| `scripts/check-platform-matrix-parity.sh:54` | `python3 - "$GORELEASER_YAML" "$WORK/…" <<'PYEOF'`, lê por `sys.argv[1]`/`[2]` |
| `scripts/check-platform-matrix-parity.sh:95` | idem, `sys.argv[1]` |
| `scripts/check-wheel-filename.sh:165` | heredoc **não** citado, mas os caminhos vão por `sys.argv[1]`/`[2]` |
| `scripts/check-validate-rule-pins.sh:369,383,397,426,440` | os 5 corrigidos pelo **#417** (`dc95ff34`) — heredoc citado + `argv` |

### 1.4 Sítios (c) — o `$` não é caminho, ou o Python nunca o abre

🔴 **Esta lista é entregável para o ML-1B**: são os casos que o gate anti-reintrodução **não pode**
reprovar. Dois deles são armadilhas reais.

| `arquivo:linha` | o que é | por que não pode ser reprovado |
|---|---|---|
| 🔴 `scripts/check-validate-rule-pins.sh:371` | `'command': '$PWD/scripts/trackfw-credential-guard.sh'` | **parece o defeito e é o oposto**: está dentro de `<<'PY'` **citado**, o `$PWD` é **literal** e a fixture (`pin10-pwd`) existe justamente para testar um `$PWD` **não expandido** num hook. Um gate ingênuo reprovaria a linha que o #417 acabou de consertar |
| 🔴 `scripts/check-serve-browser-security.sh:97` | `url = '$ZONE_VECTOR_URL'` → `subprocess.list2cmdline` | é **URL**, não caminho; operação puramente de string, sem tocar FS. E **depende de não haver conversão** para preservar o vetor `fe80::1%eth0&calc.exe&echo` intacto — mover para `argv` pode deixar o MSYS mexer nele |
| `scripts/check-channels-content.sh:460,462` | `Version('$VERSION')` | versão, não caminho |
| `scripts/check-wheel-filename.sh:60,95` | `parse_wheel_filename("$RAW_NAME")` | basename literal (`trackfw-8.0.0-rc1-…whl`), parsing de string, sem FS |
| `scripts/check-barrier.sh:*`, `scripts/check-update-parity.sh:204-708` (≈20) | `json.loads(sys.argv[1])` | o `$` é **JSON de stdout**, não caminho; e já vai por `argv` |
| `scripts/trackfw-attention-signal.sh:18,19` | `json.load(sys.stdin)` | sem caminho nenhum |
| `scripts/check-install-version-pin.sh:477-482` | `'case "$RAW_ARCH" in\n'` | texto de **shell** sendo gerado; o `$` é literal Python |
| `scripts/check-gates-falsify.sh:4114,7267` | fragmentos de shell/Makefile gerados | o `$` é literal |
| `.github/workflows/windows-probe.yml:529,535,644` | `os.environ.get('T_WIN')` etc. | **env var**, que o MSYS converte por ser valor inteiro; e é workflow de **sonda**, não gate |

---

## 2. Reconciliação — 15/10 (#417) vs 7 (arquiteto) vs **6** (este parecer)

**Nenhum dos dois estava errado. Contavam objetos diferentes.**

### 2.1 O #417 foi **mais largo no predicado** e **mais estreito na unidade**

O reportante escreveu, no próprio corpo do PR:

> *"41 blocos `python3 -c "` em `scripts/`, dos quais **15 interpolam caminho em chamada de
> arquivo**: 5 aqui e 10 em outros 9 arquivos … **Não toquei nos 10.** … a triagem acima é uma
> peneira, não um veredito — alguns podem receber caminho já convertido."*

A peneira dele casava **arquivo que contém `open(` e arquivo que contém `$`** — não exigia que
fossem a **mesma string**, nem que a citação permitisse expansão. Resultado, medido item a item na
lista de 10 que ele publicou:

| arquivo citado pelo #417 | veredito real | direção do desvio |
|---|---|---|
| `check-gates-falsify` | **(a)** ✅ | acerto |
| `check-serve-api-file-security` | **(a)** ✅ (são **2** interpolações, contadas como 1) | **estreito** |
| `check-update-parity` | **(a)** ✅ (são **3** interpolações, contadas como 1) | **estreito** |
| `check-manifest-version-gate` | **(b)** — `sys.argv[1]` | **largo** |
| `check-platform-matrix-parity` | **(b)** — heredoc citado + `argv` | **largo** |
| `check-roadmap-barrier-contract` ×2 | **(b)** — `sys.argv[1]`; e só existe **1** bloco, não 2 | **largo** |
| `check-thirdparty-parity` | **(b)** — `sys.argv[1]` | **largo** |
| `check-barrier` | **(c)** — o `$` é JSON, não caminho | **largo** |
| `trackfw-attention-signal` | **(c)** — `json.load(sys.stdin)` | **largo** |

Soma: dos 10 declarados, **3 arquivos** são (a) — e esses 3 contêm **6** interpolações, não 3. O
número 10 é **largo por 7 e estreito por 3 ao mesmo tempo**.

### 2.2 Uma subtração que nenhum dos dois previu: um sítio fechou por **deleção**

A issue **#363** nomeia **dois** arquivos. O segundo, `scripts/check-doctor-parity.sh:594`
(`p = pathlib.Path('$l_project/scripts/trackfw-validate.sh')`), **não existe mais**:

```
$ git ls-files | grep -i doctor-parity     →  (vazio)
$ git log --oneline -1 -- scripts/check-doctor-parity.sh
2eae0a44 2026-09-16 v8.0.0: uma implementação em Go, três canais … (#365)
```

Foi removido pela v8.0.0, **8 dias antes** do #417. O sítio fechou, mas **por deleção, não por
correção** — e isso tem duas consequências práticas:

- 🔴 **A REQ e o ML-1A mandam imitar um arquivo que não existe.** Os dois citam
  `check-doctor-parity.sh` / `_normalize_version_in_file` como o precedente a copiar. **Metade do
  precedente foi deletada.** O precedente vivo é `check-thirdparty-parity.sh:167` (confirmado
  presente e correto). **Isto precisa ser corrigido no handoff antes do ML-1A**, senão despacha um
  agente atrás de um arquivo ausente.
- O parágrafo *"efeito medido"* da REQ descreve uma falha do `check-doctor-parity` que **não é mais
  reproduzível nesta árvore**. Não é erro de medição do reportante — é árvore que andou.

### 2.3 Minha varredura (7) foi quase certa, e por sorte

O grep do arquiteto (`grep -rn '\$' scripts/*.sh | grep -E "(open|Path|…)\("`) exigia `$` **e** a
chamada de I/O **na mesma linha**. Achou 7, dos quais `check-thirdparty-parity:167` é (b) → 6
defeitos, que é o número certo. Mas ele acerta **por uma coincidência da árvore**: nenhum defeito
atual tem a interpolação e o `open()` em linhas separadas. Como floor, era frágil.

**Veredito: 6.** Nem o maior por precaução, nem o menor por conveniência — é o que sobra quando se
aplica o critério da §0 à unidade declarada em §1.

---

## 3. 🔴 Busca ativa pelo que **nenhum** dos dois varredores pega

O aviso do handoff é o padrão desta campanha: três enumerações consecutivas foram **limite inferior**
porque o filtro caçava **um token** em vez da **forma**. Rodei sete varreduras, cada uma mirando uma
forma distinta, e uma final **sem filtro de forma nenhum**.

| # | forma caçada | comando | resultado |
|---|---|---|---|
| **A** | heredoc **não** citado (`python3 - <<PY` com `$VAR` dentro) | parser de blocos (`scan.py`), 1727 arquivos rastreados | **4 sítios**, todos `check-wheel-filename.sh` → (b)/(c). **Nada novo** |
| **B** | `-c` **multi-linha** (o corpo continua em linhas que o `grep` de 1 linha nunca vê) | `awk` de delimitador, independente do parser | **a1, a2, a3**. Corrobora, nada novo |
| **C** | **forma-agnóstica**: `$VAR` dentro de aspas **simples** Python + chamada de I/O | `grep -rIn -E "'[^']*\$\{?[A-Za-z_][^']*'"` \| filtro de I/O | **exatamente os 6**. Terceira convergência |
| **D** | mesma, com aspas **duplas** (o que C, por construção, não vê) | `grep -E '(open\|Path\|chdir\|…)\s*\(\s*"[^"]*\$'` | **vazio** |
| **E** | `os.path.join(…$…)` / `pathlib.Path(…$…)` | `grep -E "os\.path\.join\([^)]*\\\$\|Path\([^)]*\\\$"` | **vazio** |
| **F** | **forma dividida**: caminho atribuído a variável Python numa linha, aberto noutra | varredura por corpo (`ASSIGN` + `IO` no mesmo bloco) | **0 sítios** |
| **G** | código Python guardado em **variável shell**, depois `python3 -c "$CODE"` | `grep -n -A12 "^STRIP_TS[A-Z_]*="` + busca por `$(cat`/`eval` | `STRIP_TS`, `STRIP_TS_TRUST`, `STRIP_TS_GATES` — **aspas simples, sem caminho** → (c). `python3 -c "$(…)"`: **vazio** |
| **H** | **produto que EMITE script** com o padrão (chegaria ao consumidor) | `grep -rn "python3\? -c" internal/ --include=*.go --include=*.tmpl` | 4 sítios em `claudemd.go:258`, `scaffold.go:901,902,2102`. Todos com `pathlib.Path('.')` literal ou `json.load(sys.stdin)` — **sem caminho interpolado**. 🔴 **O defeito não é distribuído aos consumidores** |
| **I** | caminho chegando por **env var exportada** | `grep -rIn "os\.environ"` | só `windows-probe.yml` (sonda) e `check-wheel-filename.sh:180` (`CI`). Env var é convertida pelo MSYS por ser valor inteiro → **(b) por construção** |
| **J** | `.ps1` (o outro caminho de execução no Windows) | `grep -rIn "python" scripts/windows-repro/*.ps1` | `run.ps1:586,889` — `python -c` com literais, e o corpo Python vai em **arquivo**. PowerShell não passa por MSYS. **Nada** |

### 3.1 O fechamento que não depende de forma nenhuma

Todas as varreduras acima ainda caçam **uma forma**. Para fechar por **enumeração**, extraí do
`sites.json` **toda linha de corpo Python, em arquivo não-`.md`, que contenha um `$`** — sem filtro
de I/O, sem exigência de mesma linha:

```
TOTAL linhas com $ no CORPO python (não-.md): 39
```

**Li as 39, uma a uma.** Estão todas classificadas na §1.3 ou §1.4. Nenhuma nova. Somadas às formas
de uma linha (que não têm "corpo" e já saíram na varredura C), o universo está **enumerado, não
amostrado**.

### 3.2 Resíduo declarado da busca ativa

O que esta enumeração **não** fecha, dito sem eufemismo:

1. **Caminho construído por expressão dentro do Python** — ex.: `p = os.path.join('$DIR', 'x')`. A
   varredura E mirou essa forma e voltou vazia, mas ela é reconhecível só por leitura; se alguém
   introduzir uma variante que E não case, nenhuma das 10 varreduras a pega.
2. **Arquivos `.md`** foram excluídos das §1/§3.1. A varredura H cobriu templates de produto e voltou
   limpa, mas um `.md` que venha a ser materializado em script no futuro não está sob o critério.
3. **Fora de `scripts/` e `.github/workflows/`** — o `Makefile` e os `.py` foram varridos e não têm o
   padrão, mas não há gate hoje que impeça o padrão de nascer num diretório novo. **Isso é trabalho
   do ML-1B**, não desta enumeração.

---

## 4. Threat model — sítios (a) em gate de **segurança**

**Há exatamente um: `scripts/check-serve-api-file-security.sh:87,92` (a2/a3).** Os outros quatro
(a1, a4–a6) são gates de paridade e de falsificação, não de segurança.

### 4.1 Qual é a garantia, nominalmente

O bloco a2/a3 monta o **braço vulnerável** do AC6: copia `internal/serve/api_file.go`, neutraliza a
chamada `filePathAllowed(realAbsPath, physicalAllowedDirs)` (trocando por `if false && …`), grava a
cópia sabotada, e roda `go test -overlay` contra `TestFileHandler_SymlinkEscape`
(`internal/serve/api_file_test.go:135`).

A garantia que esse braço dá **não é** "o serve está protegido contra escape por symlink". É a
garantia de **não-vacuidade** dela:

> **`TestFileHandler_SymlinkEscape` é carregado — ele realmente detecta a remoção de
> `filePathAllowed(realAbsPath, physicalAllowedDirs)`, e não passaria também num serve vulnerável.**

### 4.2 O que acontece no Windows, e por que **não** é falso-negativo

Traçado pelo código, com `set -euo pipefail` confirmado em `check-serve-api-file-security.sh:20`:

1. `open('$GO_API_FILE')` morre com `FileNotFoundError`.
2. `$VULN_GO` **nunca é escrito**.
3. `overlay.json` aponta para um arquivo inexistente; `go test -overlay` falha ao carregar.
4. `VULN_GO_OUT` não casa `^(FAIL|--- FAIL)` **nem** `^ok` → cai no `else`:
   `fail "AC6 Go falsificação: resultado inesperado"`.

🔴 **O gate reprova alto.** Não existe caminho em que o braço vulnerável seja pulado e o gate
declare sucesso. **Nenhum falso-negativo de segurança.**

### 4.3 O tamanho real do risco — menor do que parece, e digo por quê

| pergunta | resposta medida |
|---|---|
| O gate roda em algum job de Windows? | **Não.** `Makefile:49`, alcançado por `make parity-rest`, que `parity-other-gates` executa em **`ubuntu-latest`**. Nenhum job `windows-*` o invoca |
| Então a prova de não-vacuidade existe? | **Sim, e passa em todo PR.** Medido no run `36023336663`: `parity-other-gates \| Run make parity-rest \| ok  AC6 Go falsificação: sem filePathAllowed(realAbsPath) o teste SymlinkEscape FALHA` |
| E o **controle em si** (não a prova dele)? | 🔴 **Exercitado e verde no Windows.** Mesmo run: `windows-full-suites \| Go — suíte completa \| --- PASS: TestFileHandler_SymlinkEscape (0.01s)`. Verifiquei também o risco de `t.Skip`: `symlinkOrSkip` (`internal/serve/symlink_helper_test.go:31`) pula por **condição** (erro de privilégio / `WinError 1314`), não por `GOOS` — e no runner deste run **não pulou** |

**Residual exato, sem inflar** (as três linhas acima são medidas em log, não inferidas): no Windows,
o **teste** do controle roda **e passa**, mas a **prova de que esse teste não é vacuoso** não roda — e, se alguém decidir rodar `make quality` num Git Bash, ela morre
ruidosamente em vez de rodar. É um buraco de **meta-prova em uma plataforma**, não um controle de
segurança sem cobertura.

🔴 **Afirmar "a garantia de segurança fica sem prova no Windows" seria exagero medido.** A afirmação
correta é: *a prova de não-vacuidade do AC6 não é executável no Windows enquanto a2/a3 existirem, e
não há hoje job de Windows que a peça.*

### 4.4 O adversário desta Wave 0 — quem esvazia isto sem quebrar regra escrita

1. **O implementador apressado do ML-1A** corrige a1 (o que derrubou o chunk, o que dói) e deixa
   a2/a3 por "não têm consequência no CI". Está certo sobre o fato e errado sobre a regra: a **Regra
   Dura de Causa Raiz** manda fechar todos os sítios da mesma causa no mesmo PR. *Contramedida:* a
   tabela §1.1 lista consequência **zero** explicitamente, para que "zero" não vire "fora do escopo".
2. **O otimista que fecha pelo censo.** Se a Wave 2 declarar vitória porque o `chunk_0` voltou a
   completar, a2/a3 continuam vivos — eles **nunca** apareceram no censo. *Contramedida:* o critério
   de fechamento é a lista de 6, não a cor do censo.
3. **O gate do ML-1B que reprova por token.** As duas armadilhas da §1.4 (`check-validate-rule-pins.sh:371`
   e `check-serve-browser-security.sh:97`) reprovariam num gate que case `'…$VAR…'` sem olhar a
   **citação** do heredoc. A primeira é especialmente perversa: reprovaria a linha que o #417 acabou
   de **consertar**. *Contramedida:* §1.4 é entrada obrigatória do ML-1B como caso de **não-flag**.
4. **A tradução literal do handoff.** O ML-1A manda imitar `check-doctor-parity.sh`, que **não
   existe** (§2.2). *Contramedida:* corrigir o handoff antes do despacho.

### 4.5 Falsificação nas duas direções, por sítio (entrega para o ML-1B)

| sítio | onde a sabotagem entra | qual gate deve pegar | falso-**negativo** a provar | falso-**positivo** a provar |
|---|---|---|---|---|
| a1 | reintroduzir `open('$VAR')` em `check-gates-falsify.sh` | gate do ML-1B | gate passa com o padrão presente | gate reprova o `<<'PY'` + `argv` do #417 |
| a2/a3 | reintroduzir no gate de segurança | ML-1B **+** execução em Windows | gate não distingue `-c "` de `-c '` | gate reprova `check-validate-rule-pins.sh:371` (`$PWD` literal em heredoc citado) |
| a4–a6 | reintroduzir em `check-update-parity.sh` | ML-1B | conta 1 por arquivo em vez de 1 por interpolação (**foi o erro do #417**) | gate reprova `json.loads(sys.argv[1])`, que é JSON e não caminho |

### 4.6 Residual declarado desta Wave 0

- **Não medi em VM Windows.** O mecanismo já está medido três vezes — #363, #417 e o run
  `36017761462`. A VM verificaria mecanismo, e o mecanismo não está em dúvida; número que vira
  afirmação sai do CI, como manda o roadmap.
- **Não verifiquei a1 em POSIX** (`rc=0`) por execução própria: rodar o `check-gates-falsify.sh` é
  proibido nesta Wave e caro. O roadmap declara `rc=0` local medido pelo arquiteto; aceito como dado
  de terceiro, **identificado como tal**.
- **`.md` fora do critério** (§3.2).
- **O `t.Skip` do `symlinkOrSkip` é condicional, não estrutural.** Num runner Windows *sem*
  Developer Mode, `TestFileHandler_SymlinkEscape` pularia, e aí o controle ficaria sem cobertura na
  plataforma — e o §4.3 ficaria subestimado. No run medido não pulou, mas isso é **propriedade do
  runner**, não garantia do repositório. Declaro como residual porque não está sob controle desta REQ.
- **Não digo nada sobre a #363.** Fechá-la depende também do bit de execução em NTFS — escopo
  negativo explícito da REQ.

---

## 5. Frase de fechamento

> **Corrijo esta causa — caminho interpolado dentro do texto do programa Python, onde o bash expande
> antes de o texto virar `argv` — e exatamente estes 6 sítios fecham:
> `check-gates-falsify.sh:6745`, `check-serve-api-file-security.sh:87` e `:92`,
> `check-update-parity.sh:354`, `:379` e `:408`. E nenhum outro.**

O que **não** fecha com ela, dito para que ninguém cobre: a #363 (depende também do bit de execução
em NTFS), o `\r` de stdout (REQ do CRLF, #414, Done), a apuração do censo (REQ-2026-09-23), e o
sítio de `check-doctor-parity.sh` — que fechou por **deleção** na v8.0.0, não por correção.

---

## 6. Veredito

**APROVA a abertura da Wave 1**, com **três condições de handoff**:

1. 🔴 **Corrigir o ML-1A**: remover a referência a `check-doctor-parity.sh` /
   `_normalize_version_in_file` (arquivo deletado em `2eae0a44`). O precedente vivo é
   `check-thirdparty-parity.sh:167`.
2. 🔴 **Entregar a §1.4 ao `artemis-tf`** como conjunto obrigatório de **não-flag** do ML-1B, com
   `check-validate-rule-pins.sh:371` e `check-serve-browser-security.sh:97` nomeados.
3. **Ordem da §1.2**: a1 primeiro (derruba chunk), a4–a6 depois (3 FAIL), a2/a3 por último (zero
   consequência hoje) — mas **os três grupos no mesmo PR**, pela Regra Dura de Causa Raiz.

**Nenhuma linha de implementação foi escrita. Nenhum commit, nenhum push, nenhuma branch criada.
`make quality` não foi executado.**
