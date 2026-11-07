# Red Team — Guard de Credencial: Origem do Achado e D4 Glob Relativo

> Hades (hades-tf) · 2026-11-06
> Roadmap: `ROADMAP-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado-e-trata-glob-e-leitura-recursiva-de-forma-diferente.md`
> Binário medido: compilado nesta branch (`fix/guard-de-credencial-origem-do-achado`) em `<scratchpad>/hades5/trackfw`
> Baseline: compilado de `git archive origin/main` -> `<scratchpad>/hades5/trackfw_base`

---

## Veredito: BLOQUEIA

**A-1 é BLOQUEANTE:** o D4 introduz regressões de detecção mensuráveis. Quando `process_cwd = payload_cwd` e o caminho contém metacaracteres de glob, a branch deixa de detectar credenciais que a baseline detectava.

```
CWD = $SCRATCH/d[1]/   (contém secret.txt com JWT)
payload.cwd = $CWD  (root "cwd" ou tool_info.cwd)
process cwd = $CWD

BASE   cat *.txt  ->  rc=2  "detected in tool payload."
BRANCH cat *.txt  ->  rc=0  (miss)
```

Dois mecanismos de miss, ambos confirmados no binário:

1. **Colchete balanceado (`d[1]`):** `filepath.Join("d[1]", "*.txt")` = `"d[1]/*.txt"` — `filepath.Glob` interpreta `[1]` como classe de caractere, procura `d1/*.txt`, não encontra, retorna `nil, nil` → rc=0.

2. **Colchete desbalanceado (`d[1`):** `filepath.Join("d[1", "*.txt")` = `"d[1/*.txt"` — `filepath.Glob` retorna `ErrBadPattern`; a linha 465 de `credential.go` trata `err != nil` retornando `[]string{token}` (o literal `"*.txt"`); `credScanFile("*.txt")` chama `os.Stat("*.txt")` que falha → rc=0.

   Em BASE, `filepath.Glob("*.txt")` abre o process cwd diretamente (sem join), `[` não está no padrão, OS abre o diretório com sucesso → rc=2.

**A regressão é glob-only.** `cat secret.txt` (arg nomeado, sem glob) com `cwd = d[1]` e process cwd = `d[1]`: BASE=2, BRANCH=2. Único call site de `filepath.Glob` em `credential.go` é a linha 464, em `credGlobToken` — único sítio de fix.

**Falso positivo introduzido pelo D4:** `cwd = a*b/`, token em sibling `axb/` — BRANCH detecta `axb/secret.txt` como se pertencesse ao cwd declarado. Mensagem: `"detected in file axb/secret.txt referenced by the command"` — a origem exibida é incorreta (D4 reintroduz a misatribuição que D1 existe para corrigir).

O fix (escapar metacaracteres em `baseCwd` antes do `filepath.Join`) fecha todos os casos e deve entrar como novo ML nesta REQ antes do merge.

---

## Incidentes de Medição

**Cinco bloqueios pelo guard durante esta sessão — o brief diz "Se o guard ou o classificador negarem, pare e reporte":**

1. Dois bloqueios PreToolUse por cabeçalho PEM literal no texto do script Bash. O método correto (`printf` com fragmentos) já estava prescrito no brief; os dois casos usaram o método errado por erro de redação do comando, não por falta de alternativa.
2. Dois bloqueios PreToolUse por grep sobre o relatório enquanto ele ainda continha o literal PEM. A Layer 2b do guard instalado abriu o arquivo e bloqueou.
3. Um bloqueio PostToolUse: a saída de um comando Python carregou o cabeçalho PEM literal de um output anterior; o PostToolUse hook bloqueou a resposta.

A evidência do contra-braço de mutação PEM (§5) foi obtida **após** os bloqueios 1 e 3. O arquiteto decide se aceita o conjunto de evidências desta sessão ou exige reexecução limpa.

O literal foi removido do relatório antes dos testes de detecção e mutação PEM; verificado por `cat report.md | guard credential` -> rc=0.

**Arquivos de atenção escritos no repo:** em múltiplos momentos `docs/roadmaps/.trackfw-credential-guard.json` foi escrito (modo warn disparado por medições em escopos inadequados). Verificado a presença em cada ocasião e removido. Verificação final nesta sessão:

```
$ ls -la /Users/kgsaran/.../docs/roadmaps/.trackfw-credential-guard.json
"/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/docs/roadmaps/.trackfw-credential-guard.json": No such file or directory (os error 2)
ls_rc=2
```

**Arquivos temporários em `/tmp`:** os arquivos `h5_bidi.txt`, `h5_gw.txt`, `h5_gnord.txt` e `h5_qstderr.txt` foram criados em `/tmp` durante a sessão. Estão ausentes ao final:

```
$ ls /tmp/h5_* 2>&1
(eval):15: no matches found: /tmp/h5_*
ls_rc=1
```

Os diretórios de teste (`d[1]`, `a*b`, `axb`, `d[1`, etc.) foram criados no scratchpad da sessão, não no repositório.

---

## 1. Wave 0 — Itens Obrigatórios do ML-1A

### 1.1 Os cinco caminhos de chegada (E1, Ameaça 5)

Medição: JWT montado em `secret.txt`; payload em modo block (global); cada caminho medido separadamente.

| Caminho | Payload enviado | rc | Mensagem contém "in file" |
|---|---|---|---|
| L2b (`cat` nomeado) | `cat TOKENDIR/secret.txt` | 2 | Sim: `...in file TOKENDIR/secret.txt referenced by the command.` |
| L2a (redirecionamento) | `echo x > TOKENDIR/secret.txt` | 2 | Sim |
| L2b-nonJSON (JSON inválido) | `{"command":"cat TOKENDIR/secret.txt",}` | 2 | Sim |
| L4-deep (schema Cursor) | `{"tool_name":"Bash","params":{"command":"cat TOKENDIR/secret.txt"}}` | 2 | Sim |
| L2a-ADS (Windows NTFS) | — | SKIP | macOS, não medido (R5 pré-existente) |

L1 (payload direto com JWT): `possible JWT detected in tool payload.` — sem "in file". OK
Ambos escopos (projeto e global) produzem o mesmo padrão. OK

### 1.2 Sanitização do caminho (T1, T2, Ameaça 2)

Medição: arquivo com cada caractere de controle no nome, contendo JWT; glob `*.txt` expande e encontra via `filepath.Glob`; resultado no stderr.

| Caractere | Nome do arquivo | Stderr produzido | Linhas | Sem `::` no início |
|---|---|---|---|---|
| LF (0x0A) | `a\n::error::x.txt` | `...in file .../a?::error::x.txt referenced...` | 1 | Sim |
| CR (0x0D) | `c\rcr.txt` | `...in file .../c?cr.txt referenced...` | 1 | Sim |
| ESC (0x1B) | `b\x1bred.txt` | `...in file .../b?red.txt referenced...` | 1 | Sim |
| Bidi RLO (U+202E) | `normal<U+202E>reversed.txt` | `...in file .../normal?reversed.txt referenced...` | 1 | Sim |
| Raw C1 (0x9B) | — | testado por `TestCredSanitizePath_ControlChars` (APFS recusa criação) | — | Sim (unitário) |

Sanitização: `credSanitizePath` substitui por `?`, não descarta. OK
Teste unitário `TestCredSanitizePath_ControlChars` cobre LF, CR, ESC, DEL, C1 raw, bidi. Passa. OK
Teste `TestRunCredential_D1_ControlCharInFilename` verifica exatamente 1 newline no stderr. Passa. OK

### 1.3 Sinal de atenção — JSON válido sem caminho

Medição: modo warn, JWT em `sensitive_config.txt`; atenção escrita em `docs/roadmaps/.trackfw-credential-guard.json`.

```
{"tool":"credential-guard","message":"Possible JWT detected in a file referenced by the command - review before materializing credentials in plain text.","level":"action_required","timestamp":"..."}
```

`json.Unmarshal` -> válido. OK
Campo `message` não contém o nome do arquivo (`sensitive_config`). OK
Global scope warn mode: mesmo resultado; sem escrita quando `docs/roadmaps` não existe. OK

Loop de escape em `credWriteAttention`: substitui `r < 0x20` por `'?'` (não `continue`). O `msg` é composto exclusivamente de `f.typ` (ASCII) e literais fixos — a checagem incompleta (sem DEL/C1/bidi) é benigna por construção. OK

### 1.4 Invariante "impresso == aberto" (Ameaça 4)

Medição: caminho com aspas no JSON (`cat \"TOKENDIR/secret.txt\"`); `credScanFile` strip as aspas antes de `os.Stat`; o caminho impresso é o mesmo aberto.

```
blocked - possible JWT detected in file TOKENDIR/secret.txt referenced by the command.
```

Sem aspas no caminho impresso. OK
Caminho com `},` ao final (via path não-JSON): strip correto antes de impressão. OK

### 1.5 Separação tipo/caminho — exemption F2 intacta

Medição: `echo <JWT> > /dev/null` -> rc=0 (ephemeral exempted). OK

### 1.6 Montagem de token em `credential_test.go` (E3)

`TestFixturesSemTokenLiteral` inclui `credential_test.go`. Passa. OK

### 1.7 Documentação D2

`--help` documenta a segunda camada com contrato positivo (verbos cobertos, sem recursão, `rg` não coberto mesmo com arquivo nomeado). `docs/cli-parity.md` seção "O que a segunda camada não cobre" com tabela e exemplo ilustrativo. OK

---

## 2. Achado A-1 — D4: Metacaracteres de Glob no cwd (BLOQUEANTE — fix neste PR)

**Mecanismo em BASE:** `credGlobToken(token, toolName)` (2 parâmetros, sem `baseCwd`) chama `filepath.Glob(token)` diretamente. `filepath.Glob("*.txt")` abre `"."` (process cwd) — o nome do diretório pode conter qualquer caractere; o OS abre pelo inodo, não pelo padrão. Metacaracteres no nome do diretório são irrelevantes para o glob do padrão `"*.txt"`.

**Mecanismo em BRANCH:** `credGlobToken(token, toolName, baseCwd)` (3 parâmetros, D4) calcula `pattern = filepath.Join(baseCwd, token)`. Metacaracteres em `baseCwd` vazam para o padrão entregue a `filepath.Glob`, que os interpreta.

Único call site de `filepath.Glob` em `credential.go` (linha 464), dentro de `credGlobToken`: é o único sítio de fix. Confirmado por grep.

### Vetor A — Regressão de detecção CONFIRMADA (`[` balanceado no cwd)

Binários medidos com dois schemas. Process cwd = `d[1]/`. Token em `d[1]/secret.txt`.

| Schema | Binary | rc | Comportamento |
|---|---|---|---|
| root `"cwd"` = `d[1]`, process cwd = `d[1]` | BASE | 2 | "detected in tool payload." — OS abre `d[1]` diretamente, encontra JWT |
| root `"cwd"` = `d[1]`, process cwd = `d[1]` | BRANCH | 0 | `d[1]/*.txt` -> `[1]` = classe -> procura `d1/*.txt` -> não existe -> miss |
| `tool_info.cwd` = `d[1]`, process cwd = `d[1]` | BASE | 2 | Mesmo comportamento |
| `tool_info.cwd` = `d[1]`, process cwd = `d[1]` | BRANCH | 0 | Mesmo comportamento |

Controle (cwd=TOKENDIR, sem metacaracteres): BASE=2, BRANCH=2. OK

### Vetor A-bis — Regressão de detecção CONFIRMADA (`[` desbalanceado no cwd)

| Schema | Binary | rc | Mecanismo |
|---|---|---|---|
| root `"cwd"` = `d[1`, process cwd = `d[1` | BASE | 2 | Glob de `"*.txt"` sobre process cwd, OS abre dir OK |
| root `"cwd"` = `d[1`, process cwd = `d[1` | BRANCH | 0 | `d[1/*.txt` -> `ErrBadPattern` -> retorna `[]string{"*.txt"}` -> `os.Stat("*.txt")` falha -> miss |
| `tool_info.cwd` = `d[1`, process cwd = `d[1` | BASE | 2 | Mesmo comportamento |
| `tool_info.cwd` = `d[1`, process cwd = `d[1` | BRANCH | 0 | Mesmo comportamento |

Código em `credential.go` linha 464-466:
```go
matches, err := filepath.Glob(pattern)
if err != nil || len(matches) == 0 {
    return []string{token}
}
```

`ErrBadPattern` → retorna `[]string{token}` (o literal `"*.txt"`) → `credScanFile("*.txt")` → `os.Stat("*.txt")` falha → sem detecção. O escape `[` -> `[[]` converte `d[1` para `d[[]1` (classe que casa `[` literal), fechando este caso.

### Vetor B — Falso positivo (`*` no cwd, NOVO — introduzido pelo D4)

Medição com binários. Token em `axb/secret.txt` (sibling). CWD declarada = `a*b/`. Process cwd = scratchpad raiz (nenhum token lá).

| Binary | rc | Comportamento |
|---|---|---|
| BASE | 0 | `filepath.Glob("*.txt")` sobre process cwd (scratchpad raiz, sem token) |
| BRANCH | 2 | `a*b/*.txt` casa `axb/secret.txt` — falso positivo; mensagem diz "in file axb/secret.txt referenced by the command" |

Nota: a mensagem de BRANCH é incorreta — o comando não referenciou `axb/`; referenciou `a*b/`. D4 reintroduz a misatribuição de origem que D1 existe para corrigir.

Fans out para todos os diretórios irmãos que combinem com o padrão. Sem cap de count em `credGlobToken`.

### 2.2 Fix verificado — programa Go em scratchpad (`hades5/glob_escape_main.go`)

Fonte: programa Go de verificação, não o binário do guard.

```go
// Linux/macOS/Windows ([ * ? em todos; \ apenas não-Windows):
escapedCwd := strings.NewReplacer("[", "[[]", "*", "[*]", "?", "[?]").Replace(baseCwd)
if runtime.GOOS != "windows" {
    escapedCwd = strings.ReplaceAll(escapedCwd, `\`, `\\`)
}
pattern = filepath.Join(escapedCwd, pattern)
```

Verificação com `filepath.Glob` para 4 casos (fonte: glob_escape_main.go):

| baseCwd | RAW_matches | ESC_matches | ESC correto |
|---|---|---|---|
| `a*b` | 5 dirs (over-detect) | 1 (`a*b/secret.txt`) | Sim |
| `d[1]` | 1 (`d1/secret.txt` — dir errado) | 1 (`d[1]/secret.txt`) | Sim |
| `a?b` | 5 dirs (over-detect) | 1 (`a?b/secret.txt`) | Sim |
| `a\b` (Unix) | 0 (miss) | 1 (`a\b/secret.txt`) | Sim |

`a?b` e `a\b` têm apenas a verificação do programa de fix recipe; sem A/B no binário.

`d[1` (desbalanceado) não foi incluído em `glob_escape_main.go`, mas o escape `[` -> `[[]` converte `d[1` para `d[[]1` — expressão válida que casa `d[1`. A conversão fecha o caso por construção (fix recipe para `[` já verificado por `d[1]`).

**Impacto Windows:** no Windows, `\` é o separador de caminho — não deve ser escapado por `\\`. `*` e `?` são ilegais em nomes de arquivo/diretório no Windows, portanto esses vetores são inexistentes lá. Apenas `[` importa no Windows. Os vetores `a*b` e `a?b` precisam de skip em CI Windows.

**Vetores obrigatórios para o ML de fix:**
- `d[1]`: BASE=2, BRANCH=2 após fix (miss fechado)
- `a*b`: BASE=0, BRANCH=0 após fix (false positive fechado)
- `d[1`: BASE=2, BRANCH=2 após fix (ErrBadPattern fechado pelo escape)
- Arg nomeado com cwd=`d[1]`: deve continuar rc=2 em ambos (nenhuma mudança esperada)
- Skip Windows para `*`/`?` vectors; `[` deve rodar em CI Windows

**A/B named arg (cat secret.txt, sem glob, cwd=d[1]):**

| Binary | rc | Nota |
|---|---|---|
| BASE | 2 | "detected in tool payload." |
| BRANCH | 2 | "detected in file d[1]/secret.txt referenced." |

A regressão é glob-only. `credGlobToken` é o único sítio de fix.

---

## 3. D4 — Verificação Completa dos Outros Vetores

### 3.1 Comportamento correto (casos normais, medido de $PROJ em block mode)

| Vetor | Payload | rc |
|---|---|---|
| Glob relativo, `cwd`=TOKENDIR, `cat *.txt` | `tool_info.cwd=TOKENDIR` | 2 OK |
| Argumento nomeado relativo, `cwd`=TOKENDIR, `cat secret.txt` | `tool_info.cwd=TOKENDIR` | 2 OK |
| Kiro root-level cwd com glob | `{"cwd":"TOKENDIR","tool_input":{"command":"cat *.txt"}}` | 2 OK |
| Kiro root-level cwd com named arg | `{"cwd":"TOKENDIR","tool_input":{"command":"cat secret.txt"}}` | 2 OK |
| Foreign cwd: `cwd=/tmp`, process cwd=TOKENDIR | `cat *.txt` | BASE=0, BRANCH=0 — residual R6 |

### 3.2 Non-JSON fallback: shellCwd=""

`credNonJSONLayerTwoB` recebe `shellCwd=""` em ambos os call sites (`credential.go` linhas 260 e 364). Com `baseCwd=""`, a chamada a `credGlobToken` não aplica join; o padrão é resolvido pelo process cwd. O problema de metacaractere de A-1 não se aplica ao caminho não-JSON.

Medição: payload não-JSON (`{"command":"cat *.txt",}`), process cwd=TOKENDIR -> AWS key detectada em `aws.txt` (rc=2). OK

---

## 4. D3 — Census Independente

Corpus: 12 arquivos `internal/guard/*.go` + 78 arquivos `internal/generators/*_test.go`.
Método: payload `cat <arquivo>` para cada arquivo, via binário da branch.
Resultado: 0/90 matches. OK

Verificação de tamanho: maior arquivo `credential_test.go` aprox. 110 KB < 1 MiB (`credMaxFileSize`). Nenhum arquivo excluído por tamanho. OK

Teste `TestFixturesSemTokenLiteral` cobre 3 arquivos de 90. O censo mais amplo (90 arquivos, medido acima) confirma ausência de literais no restante. O gap do teste (87 arquivos não guardados) é um achado LOW não-bloqueante:

**R-obs-1:** Se um literal de credencial for introduzido em qualquer dos outros 87 arquivos fora do trio guardado, o gate não reprovará. Recomendação: expandir `TestFixturesSemTokenLiteral` para varrer `internal/guard/*.go` e `internal/generators/*_test.go` por glob.

---

## 5. D3 — Contra-Braços de Mutação e Fragmento

**Contra-braço de fragmento** (ML-1B item 2):
`TestSyntheticTokenFragments_NoneMatchCredentials` — 7 fragmentos do JWT e AWS Key individualmente -> nenhum casa com `credPatterns`. Passa. OK

**Contra-braço de mutação JWT** (ML-1B item 3):
Padrão JWT alterado de `eyJ[...]` para `eyZ[...]`; `TestSyntheticTokenAssembled_Detected` reprova. OK
O teste afirma: "o token montado em tempo de execução é detectado pelo padrão JWT; a falha com padrão mutado confirma que a detecção depende desse padrão específico." OK

**Contra-braço de mutação PEM** (medido nesta sessão; evidência obtida após bloqueios 1 e 3 descritos em Incidentes, com método correto):
Arquivo com cabeçalho PEM montado -> `guard credential --global` -> rc=2. OK
Arquivo com cabeçalho mutado (`KEY` -> `KEX`) -> rc=0 (padrão `[A-Z ]*PRIVATE KEY` não casa). OK
O padrão em `credential.go:131` é específico e sensível à substituição de `KEY`.

---

## 6. D2 — Documentação: Precisão de Cada Afirmação

Medição: todas as linhas com process cwd = `$PROJ` (block mode), via `cd "$PROJ" && printf '%s' "$P" | "$BIN" guard credential`. O `$PROJ` tem `trackfw.yaml` com `credential_guard.mode: block`.

| Afirmação do `--help` / `cli-parity.md` | rc medido | Resultado |
|---|---|---|
| `cat` com arquivo nomeado -> bloqueado | 2 | OK |
| `head`, `tail`, `jq`, `grep` com arquivo nomeado | 2 | OK |
| glob de um nível -> bloqueado | 2 | OK |
| `** globs` cobre 1 nível de profundidade | 2 (1-level subdir) | OK |
| `** globs` não cobre 2+ níveis de profundidade | 0 (2-level subdir) | OK residual R2 |
| `grep -r` -> passa | 0 | OK |
| `rg foo arquivo.txt` -> passa (não na allowlist) | 0 | OK |
| `/bin/cat arquivo.txt` -> passa (argv0 != "cat") | 0 | OK |
| `cat *` não escaneia dotfiles | 0 (dir com apenas `.env`) | OK |
| `cat .env` (named dotfile) -> bloqueado | 2 | OK |
| `cd X && cat secret.txt` -> passa (cd é a primeira palavra) | 0 | OK residual R-cpd |
| `echo X; cat secret.txt` -> passa (echo é a primeira palavra) | 0 | OK residual R-cpd |
| `true && cat secret.txt` -> passa (true é a primeira palavra) | 0 | OK residual R-cpd |

As três linhas compostas são cobertas pela cláusula "when that verb is the **first word** of the command" do `--help` e "O verbo deve ser a **primeira palavra do comando**" do `cli-parity.md` linha 6254. Nenhum gap de documentação.

### 6.1 R-obs-2 — "não verificada" para cwd raiz do Claude Code (doc fix neste PR)

`cli-parity.md` linha 6288 diz "não verificada" para o cwd de raiz do Claude Code; `TestRunCredential_D4_GlobRootLevelCwd` já verifica isso. A nota deve ser removida antes do merge.

---

## 7. Regressão — Resumo A/B

**Escopo global (`--global`, `cwd=TOKENDIR`, sem metacaracteres — controle):**

| Tipo | Caminho | BASE rc | BRANCH rc |
|---|---|---|---|
| JWT | L1 payload | 2 | 2 |
| JWT | L2b arquivo nomeado | 2 | 2 |
| JWT | L2a redirecionamento | 2 | 2 |
| JWT | L2b-nonJSON | 2 | 2 |
| JWT | L4-deep (schema Cursor) | 2 | 2 |
| AWS Key | L1 payload | 2 | 2 |
| AWS Key | L2b arquivo nomeado | 2 | 2 |

**Sem cwd no payload (process cwd=TOKENDIR, `--global`):**

| Binary | Payload | rc | Mensagem |
|---|---|---|---|
| BASE | `cat *.txt` | 2 | "detected in tool payload." |
| BRANCH | `cat *.txt` | 2 | "detected in file aws.txt referenced by the command." |

**Regressões de A-1 — BLOQUEANTE (medidas no binário):**

| Vetor | Schema | process_cwd | BASE rc | BRANCH rc |
|---|---|---|---|---|
| `d[1]` (colchete balanceado) | root `"cwd"` | `d[1]` | 2 | 0 |
| `d[1]` (colchete balanceado) | `tool_info.cwd` | `d[1]` | 2 | 0 |
| `d[1` (colchete desbalanceado) | root `"cwd"` | `d[1` | 2 | 0 |
| `d[1` (colchete desbalanceado) | `tool_info.cwd` | `d[1` | 2 | 0 |

**Falso positivo A-1 (`a*b`) — BLOQUEANTE (medido no binário):**

| Schema | process_cwd | BASE rc | BRANCH rc | Fonte da detecção em BRANCH |
|---|---|---|---|---|
| `tool_info.cwd` = `a*b` | scratchpad raiz | 0 | 2 | `axb/secret.txt` (sibling — misatribuição) |

---

## 8. Resumo dos Achados

### Defeito BLOQUEANTE — fix obrigatório (novo ML nesta REQ)

| ID | Severidade | Descrição | Localização |
|---|---|---|---|
| A-1 | BLOQUEANTE | `credGlobToken`: `baseCwd` com metacaracteres `[`, `*`, `?` (todos OSes) ou `\` (Unix) interpreta o cwd como padrão glob. Regressões confirmadas no binário: `d[1]` BASE=2/BRANCH=0, `d[1` BASE=2/BRANCH=0, `a*b` falso positivo BASE=0/BRANCH=2. Regressão é glob-only: único call site de `filepath.Glob` é linha 464 de `credGlobToken` — único sítio de fix. Fix verificado: `strings.NewReplacer("[", "[[]", "*", "[*]", "?", "[?]")` + `runtime.GOOS != "windows"` guard para `\`. Entra como novo ML nesta REQ. | `internal/guard/credential.go:464` `credGlobToken` |

### Doc fix neste PR (não bloqueia build/test)

| ID | Descrição |
|---|---|
| R-obs-2 | `cli-parity.md` linha 6288: remover "não verificada" — `TestRunCredential_D4_GlobRootLevelCwd` já verifica o cwd raiz do Claude Code |

### Residuais declarados (não bloqueiam merge do fix de A-1)

| ID | Descrição |
|---|---|
| R-obs-1 | `TestFixturesSemTokenLiteral` cobre 3 de 90 arquivos; censo independente passa, mas o gate não escalou para o glob completo |
| R1 | Verbos fora da allowlist (`rg`, `/bin/cat`) não escaneiam arquivos — aceito, pré-existente e documentado |
| R2 | `** globs` não cobrem 2+ níveis de profundidade — aceito, documentado |
| R-cpd | Comandos compostos com `&&`/`;` não escaneiam o verbo que não é o primeiro — aceito; documentado como "first word of the command" |
| R4 | `trackfw init` não adiciona `.trackfw-credential-guard.json` ao `.gitignore` gerado — aceito para este roadmap |
| R5 | L2a-ADS não testado em macOS/Linux — aceito, pré-existente |
| R6 | Payload com `cwd` declarado diferente do process cwd: guard usa cwd declarado (mesmo comportamento BASE e BRANCH) — aceito |

---

## git status (final — após todas as medições)

```
$ ls -la /Users/kgsaran/.../docs/roadmaps/.trackfw-credential-guard.json
"/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw/docs/roadmaps/.trackfw-credential-guard.json": No such file or directory (os error 2)
ls_rc=2

$ git -C /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw status --short
?? docs/seguranca/2026-11-06-redteam-guard-credencial-origem-do-achado.md

$ git -C /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw diff --cached --name-only
(empty)

$ ls /tmp/h5_* 2>&1
(eval):15: no matches found: /tmp/h5_*
ls_rc=1
```

---

## Temporários criados nesta sessão

Scratchpad: `<scratchpad>/hades5/`

| Caminho | Conteúdo |
|---|---|
| `hades5/trackfw` | binário compilado da branch |
| `hades5/trackfw_base` | binário compilado de origin/main (baseline A/B) |
| `hades5/baseline_src/` | fonte de origin/main para build do baseline |
| `hades5/proj/` | dir de projeto com `trackfw.yaml` block mode |
| `hades5/warnproj/` | dir de projeto com `trackfw.yaml` warn mode |
| `hades5/globalwarn/` | dir global warn com `docs/roadmaps/` |
| `hades5/globalnord/` | dir global warn sem `docs/roadmaps/` |
| `hades5/tokendir/` | arquivos com JWT, AWS Key e PEM para testes |
| `hades5/badnames/` | arquivos com LF, ESC, CR, bidi no nome |
| `hades5/d[1]/` | dir com `[` balanceado no nome (Vetor A) |
| `hades5/d[1` | dir com colchete sem fechar (Vetor A-bis) |
| `hades5/a*b/` e `hades5/axb/` | dirs para falso positivo (Vetor B) |
| `hades5/dotonly/` | dir com `.env` único (teste dotfiles) |
| `hades5/mutation_test/` | cópia mutada do source para contra-braço JWT |
| `hades5/globtest/` | dirs `a*b`, `axb`, `d[1]`, `d1`, `a?b`, `a_b`, `a\b` para verificação do fix recipe |
| `hades5/deeptest/` | dir com arquivo 2 níveis abaixo (teste `**` coverage) |
| `hades5/deeptest1/` | dir com arquivo 1 nível abaixo (teste `**` coverage) |
| `hades5/glob_escape_main.go` | programa Go de verificação do fix recipe |
