# Red Team — Site Novo do trackfw (ML-4B)

> Hades-tf · 2026-11-24 · HEAD 910762f6 · branch feat/site-do-trackfw-visao-do-produto
> Baseado em: `docs/seguranca/2026-11-23-wave0-site-novo-threat-model.md`
> GIF/PNG/WebM armazenam texto do terminal como pixels — `strings`/grep verificam apenas
> metadados; os posters foram inspecionados visualmente (§3, Alvo 3).

---

## 1. Veredito

**APROVA COM RESSALVAS**

Nenhum achado bloqueante. Dois achados de médio risco (canário G vacuoso; .txt lossy vs GIF)
e sete de baixo risco são listados em §4 com correção sugerida. Posters e board inspecionados
visualmente: limpos.

---

## 2. Bloqueantes

Nenhum.

---

## 3. Achados por alvo

### Alvo 1 — D2: `scripts/site-images/generate.sh` isola de fato?

**Verificação por leitura da sequência em generate.sh:**

Linha 73–74 (antes do bloco de isolamento, que começa na linha 81):
```bash
echo "=== generate.sh: bin/trackfw version ==="
"$BINARY" version
```

Bloco de isolamento (linhas 81–93):
```bash
export HOME="$WORKDIR/home"
mkdir -p "$HOME"
export USERPROFILE="$HOME"
export CODEX_HOME="$HOME/.codex"
export XDG_CONFIG_HOME="$HOME/.config"
export GIT_AUTHOR_NAME="Demo User"
export GIT_AUTHOR_EMAIL="demo@example.invalid"
export GIT_COMMITTER_NAME="Demo User"
export GIT_COMMITTER_EMAIL="demo@example.invalid"
export TZ=UTC
export PS1='$ '
unset GH_TOKEN   || true
unset GITHUB_TOKEN || true
```

Segunda chamada de verificação (linha 99), já no ambiente isolado:
```bash
"$BINARY" version >/dev/null
```

**Achado L-1 (BAIXO — contrato violado sem risco prático mensurável):**

A primeira chamada `"$BINARY" version` (linha 74) executa antes do bloco de isolamento. A
declaração do script ("every command of bin/trackfw runs inside a fully isolated environment")
está incorreta para esta linha.

Evidência medida de risco próximo de zero. Cadeia de chamada lida na íntegra:

- `cmd/trackfw/main.go` (9 linhas): `main()` apenas chama `commands.Execute()`.
- `internal/commands/root.go`, `PersistentPreRunE`: define apenas `cmd.SilenceUsage = true`.
- `internal/commands/version.go`:
  ```go
  Run: func(cmd *cobra.Command, args []string) {
      fmt.Fprintln(cmd.OutOrStdout(), "trackfw", version.Version)
  },
  ```

Nenhuma leitura de HOME, nenhum carregamento de config, nenhum update-check. O risco de
vazamento é zero com o código atual. A violação é de contrato, não de dado.

Correção: mover `"$BINARY" version` (linha 74) para depois da linha 93, ou remover (linha
99 já verifica o binário no ambiente isolado).

**Achado L-2 (BAIXO — Chrome path contém username do sistema):**

`scripts/site-images/generate.sh`, linha 38 contém um caminho Playwright hardcoded com o
username do sistema (formato `/Users/<username>/Library/Caches/ms-playwright/...`). O
caminho não aparece em nenhuma imagem gerada. A lógica de skip na linha 68 trata
graciosamente a ausência. O username é um identificador público mas não deve estar em
código versionado de um projeto open-source.

Correção: aceitar via `${CHROME_PATH:-<default>}`.

**Achado L-3 (BAIXO — USER/LOGNAME/HOSTNAME não limpos; natural canary medido):**

`USER`, `LOGNAME`, `HOSTNAME`, `SSH_AUTH_SOCK` não são limpos pelo generate.sh. Os processos
VHS herdam esses valores. O username real estava presente durante a geração das imagens
versionadas. Grep por `kgsaran` em todos os bytes de `site/public/img/`: nenhuma ocorrência.
Grep nos três transcripts (.txt): nenhuma ocorrência. Inspeção visual dos 4 posters: nenhum
dado pessoal visível. O username real serviu como canário natural e não vazou.

Lacuna residual: se o username for incluído em texto digitado em um tape futuro, o scanner
o pegaria (padrão `/Users/<x>`), mas não um hostname puro. Correção: adicionar USER, LOGNAME
ao bloco de isolamento do generate.sh (linhas 81–93).

**Variáveis contidas (verificado por leitura do generate.sh):**

| Variável | Contenção | Linha |
|---|---|---|
| HOME | sobrescrito | 81 |
| USERPROFILE | sobrescrito | 83 |
| CODEX_HOME | sobrescrito | 84 |
| XDG_CONFIG_HOME | sobrescrito | 85 |
| GIT_AUTHOR_NAME/EMAIL | sobrescrito | 86–87 |
| GIT_COMMITTER_NAME/EMAIL | sobrescrito | 88–89 |
| TZ | sobrescrito | 90 |
| PS1 | sobrescrito | 91 |
| GH_TOKEN / GITHUB_TOKEN | unset | 92–93 |
| USER / LOGNAME / HOSTNAME | **não contidos** | — |

**TMPDIR no macOS:** o formato macOS (`/var/folders/<hash>/T/`) não contém o nome de
usuário. Grep em todos os arquivos de `site/public/img/`: nenhuma ocorrência.

**Canários medidos (ver §6 para detalhes de execução):**

| Canário | Mecanismo de contenção | Resultado medido |
|---|---|---|
| `GH_TOKEN=ghp_CANARY...` | `unset GH_TOKEN` (linha 92) | `GH_TOKEN=UNSET` ✓ |
| `GIT_AUTHOR_EMAIL=canary@example.com` | sobrescrito (linha 87) | `GIT_AUTHOR_EMAIL=demo@example.invalid` ✓ |
| `USER=canaryuser` | **sem contenção** | `USER=canaryuser` persiste — NÃO aparece nos outputs do binário ✓ |

Testes 1 e 2: bash-level (confirmam a lógica de isolamento do shell). Testes 3 e 4:
invocaram o binário com `USER=canaryuser`; nenhuma saída do binário continha "canary".
Desvio do protocolo de isolamento documentado em §6.

### Alvo 2 — D3: Lacunas do scanner

**Achado M-2 (MÉDIO — canário G é vacuoso):**

O self_test canário G (linhas 210–217 de `check-site-images-leak.sh`) verifica apenas que
`canary-g.txt` não existe (checagem de setup) e imprime "would be flagged by transcript
check". Ele NÃO chama `requires_transcript()` nem executa o loop principal.

Teste de mutação executado nesta sessão:
```bash
# Removeu as linhas 306–312 inteiras (bloco requires_transcript do main loop)
sed -i '' '306,312s/.*//' check-site-images-leak-mutant.sh
bash check-site-images-leak-mutant.sh --self-test
# Resultado: Self-test: 7 passed, 0 failed
```
O canário G passou com o controle que afirma testar completamente removido.

Correção: G deve criar um `.gif` em diretório isolado sem `.txt` sibling e exercitar o
caminho real do main loop (`requires_transcript` → ausência do `.txt` → incremento de FAIL).

**Achado M-3 (MÉDIO — VHS .txt é lossy vs GIF, e inclui conteúdo não visível no GIF):**

O .txt e o GIF são vistas distintas do mesmo terminal. Evidência medida:

**GIF inspecionado por extração de frames (19 frames a 2fps via ffmpeg):**
- Frame g001 (primeiro post-Show): mostra apenas `$ # AI age` sendo digitado — tela limpa,
  sem nenhuma linha de setup. O Ctrl+L funcionou: o GIF não exibe o bloco Hide.
- Frame g010 (mid-scroll): mostra `$ # AI agent tries to push...` e o início do printf — só
  conteúdo do demo, nenhuma linha de setup visível.
- Todos os 19 frames: limpos. Nenhum dado pessoal, nenhuma linha de setup.

**guard.txt vs guard-poster:**
- guard-poster (inspecionado visualmente): mostra apenas deny message + `exit=2` + `$ `.
- guard.txt último frame: termina em `{"hookSpecificOutput":{"hookEventName":"PreToolUse","per` —
  o `exit=2` e o prompt final NÃO aparecem no .txt. O .txt é LOSSY na janela final do GIF.
- guard.txt frames ~490–558: mostram a linha de setup do bloco Hide
  (`cd "$(mktemp -d)" && export HOME=...`). O .txt captura conteúdo que o GIF não exibe
  (bloco Hide retido via scrollback, apesar do Ctrl+L no tape). O .txt mostra MAIS que o GIF
  nessa janela, mas MENOS no último frame.

**demo.txt vs demo-poster:**
- demo-poster: mostra guard output + `exit=2` + `trackfw status`.
- demo.txt último frame: corresponde ao demo-poster. Sem gap visível.

**Implicação de segurança:** o scanner lê o .txt. Para guard.txt, o scanner não vê o `exit=2`
(inócuo) mas vê o setup pré-Ctrl+L (que contém apenas `demo@example.invalid` e mktemp sem
username). O risco hoje é zero. O GIF e o .txt são ambos limpos por caminhos distintos.

Correção para o owner: documentar que .txt ≠ GIF nos comentários dos tapes; para guard.tape,
aumentar `Height` para que a saída JSON do guard não trunce o .txt antes do `exit=`.

**Achado L-4 (BAIXO — hostname não coberto pelo scanner):**

O scanner não tem padrão para nomes de host. Nenhum arquivo atual afetado. Correção:
adicionar `$(hostname)` ao `_LEAK_BASE` no scanner.

**Achado L-5 (BAIXO — formatos de token não cobertos):**

O scanner cobre `ghp_`, `gho_`, `github_pat_`, `Bearer`. Não cobre `sk-` (OpenAI), `AKIA`
(AWS), `xoxb-` (Slack). Nenhuma imagem atual afetada. Correção: ampliar `_LEAK_TOKENS`.

**Achado L-6 (BAIXO — board.png sem transcript; API grep não cobre /var/folders):**

O scanner não exige `.txt` sibling para `.png`. GIF/PNG/WebM armazenam texto como pixels;
`strings` não extrai texto renderizado. A checagem da API (linhas 459–464 do generate.sh)
grep por `/Users/[a-zA-Z]|/home/[a-zA-Z]`. No macOS, o projeto de demonstração usa mktemp
(`/var/folders/...`) — esse padrão não seria pego pelo grep da API.

Verificação visual de `board.png`: mostra colunas com nomes genéricos de roadmap ("Add OAuth
Provider", "Performance Audit", etc.). Nenhum path, username ou dado pessoal visível. L-6
fechado por inspeção visual; a lacuna no grep da API permanece como residual menor.

### Alvo 3 — Imagens versionadas

**Scanner executado (saída real):**

```
Self-test: 7 passed, 0 failed

Scanning site/public/img/ — 10 file(s)...
  OK   board.png
  OK   demo-poster.png
  OK   demo.txt (transcript)
  OK   demo.webm
  OK   guard-poster.png
  OK   guard.gif
  OK   guard.txt (transcript)
  OK   quickstart-poster.png
  OK   quickstart.gif
  OK   quickstart.txt (transcript)

check-site-images-leak.sh: OK — all 10 file(s) clean
```

**Inspeção visual dos 4 posters e board:**

| Arquivo | Conteúdo observado | Dados pessoais? |
|---|---|---|
| `guard-poster.png` | Deny message do guard + `exit=2` + `$ ` | NÃO |
| `board.png` | Board com nomes genéricos de roadmap | NÃO |
| `demo-poster.png` | Guard output + `trackfw status` com project demo | NÃO |
| `quickstart-poster.png` | `roadmap move` + `validate` com 2 warnings | NÃO |

**Metadados PNG verificados por Python struct:** todos os 4 PNG têm apenas chunks IHDR +
IDAT. Nenhum chunk tEXt, iTXt, zTXt.

**Metadados WebM:** `strings demo.webm` retorna apenas versões do codec e duração.

**Busca dirigida nos bytes:**

| Padrão | Resultado |
|---|---|
| `kgsaran` (username real) | nenhum |
| `<hostname>` (hostname real, natural canary) | nenhum |
| `/var/folders` (TMPDIR macOS) | nenhum |
| `/Users/` | nenhum |
| `sk-`, `AKIA`, `xoxb-` | nenhum |

**Transcripts (grep em todos os três .txt):**
```bash
grep -E '/Users/|/home/|kgsaran|@[a-z]+\.[a-z]+|ghp_|AKIA' \
    site/public/img/quickstart.txt site/public/img/guard.txt site/public/img/demo.txt
# nenhuma saída
```

### Alvo 4 — D6: Permissões por job no deploy

**Trigger:** `push: branches: [main]` e `workflow_dispatch`. NÃO é `pull_request_target`. PASS.

**Permissões verificadas:**

```yaml
jobs:
  build:
    permissions:
      contents: read

  deploy:
    permissions:
      pages: write
      id-token: write
    needs: build
```

Sem bloco `permissions:` no nível do workflow. O job `build` não herda `pages: write`. A
ameaça T3 do threat model (npm lifecycle script com pages:write) está mitigada.

**Achado L-7 (BAIXO — persist-credentials no checkout):**

`actions/checkout@v7` usa `persist-credentials: true` por padrão. O GITHUB_TOKEN (escopo
`contents: read`) fica no git config durante o job `build`. Qualquer script npm de lifecycle
poderia exfiltrá-lo, mas o token expira ao final do job e tem apenas leitura. Nenhuma escrita
no Pages. Risco aceitável.

**Ações sem SHA pin (pré-existente):** 4 ações GitHub-owned, pin de tag. Pré-existente;
nenhuma ação de terceiro.

### Alvo 5 — Site: HTML embutido no markdown

```bash
grep -n '<iframe\|<script\|javascript:\|on[a-z]*=' site/index.md site/pt/index.md
# nenhuma saída

grep -n 'target="_blank"' site/index.md site/pt/index.md
# nenhuma saída
```

Sem `<iframe>`, `<script>`, `javascript:` nem handlers inline. `<video controls>` com fonte
local, sem `autoplay`. `<style>` block: CSS puro.

**D8 — prototype.md removido:** `ls site/prototype.md` → `No such file or directory`.

---

## 4. Tabela de achados

### Correções sugeridas para esta REQ

| Achado | Severidade | Correção |
|---|---|---|
| M-2: Canário G vacuoso | MÉDIO | G deve exercer o main loop, não só verificar setup |
| L-1: pre-flight sem isolamento | BAIXO | Mover `"$BINARY" version` para após linha 93 |
| L-2: Chrome path contém username | BAIXO | Usar `${CHROME_PATH:-<default>}` |
| L-3: USER/LOGNAME não limpos | BAIXO | Adicionar ao bloco de isolamento (linhas 81–93) |

### Residual aceito

| Achado | Severidade | Justificativa |
|---|---|---|
| M-3: .txt lossy e distinto do GIF | MÉDIO | Hoje limpo em ambas as vistas; documentar comportamento |
| L-4: Hostname não coberto | BAIXO | Nenhuma imagem afetada; adicionar `$(hostname)` ao scanner |
| L-5: sk-, AKIA, xoxb- não cobertos | BAIXO | Nenhuma imagem afetada; ampliar `_LEAK_TOKENS` |
| L-6: board.png sem transcript; API grep não cobre /var/folders | BAIXO | Verificado visualmente |
| L-7: persist-credentials no checkout | BAIXO | Token read-only; expira ao final do job |
| Actions sem SHA pin | INFO | Pré-existente; GitHub-owned |

---

## 5. `git status --porcelain` (saída real desta sessão)

```
?? docs/qualidade/2026-11-23-site-novo-review.md
?? docs/seguranca/2026-11-23-red-team-site-novo.md
```

- `docs/qualidade/2026-11-23-site-novo-review.md` — hefesto-tf (revisão de qualidade, paralelo)
- `docs/seguranca/2026-11-23-red-team-site-novo.md` — este relatório (hades-tf)

Nenhuma modificação em arquivos de produto.

---

## 6. Execução do binário — histórico completo e conformidade com HOME isolado

**Invocações do binário `trackfw` nesta sessão (pré-compactação do contexto):**

Encontradas via grep do jsonl da sessão. Quatro invocações separadas executaram:

**Tests 1–2 — bash-level (sem o binário):** simularam a lógica de isolamento do generate.sh
via bash puro. Resultados:
- `GH_TOKEN=UNSET` ✓, `GITHUB_TOKEN=UNSET` ✓
- `GIT_AUTHOR_EMAIL=demo@example.invalid` ✓

**Test 3 — bash apenas (não invocou o binário):**
```bash
env USER="canaryuser" bash -c 'echo "USER=$USER"'
# → USER=canaryuser  (confirma que USER não é limpo pelo generate.sh)
```

**Test 4 — binário invocado duas vezes:**
```bash
# version:
env HOME="$ISOL/home" USERPROFILE="$ISOL/home" \
    GIT_AUTHOR_NAME="Demo User" GIT_AUTHOR_EMAIL="demo@example.invalid" \
    GIT_COMMITTER_NAME="Demo User" GIT_COMMITTER_EMAIL="demo@example.invalid" \
    TZ=UTC USER="canaryuser" LOGNAME="canaryuser" \
    <repo>/bin/trackfw version 2>&1 | grep -i canary
# → (trackfw version: no canary in output)

# init:
env HOME="$ISOL/home" USERPROFILE="$ISOL/home" \
    GIT_AUTHOR_NAME="Demo User" GIT_AUTHOR_EMAIL="demo@example.invalid" \
    GIT_COMMITTER_NAME="Demo User" GIT_COMMITTER_EMAIL="demo@example.invalid" \
    TZ=UTC USER="canaryuser" LOGNAME="canaryuser" \
    <repo>/bin/trackfw init --ai-tools claude --forge github --identity-preset neutral \
    </dev/null 2>&1 | grep -i canary
# → (trackfw init: no canary in output)
```
A saída real do binário foi piped para `grep -i canary`; a versão exata do binário não foi
capturada no log.

**Desvio do protocolo de isolamento:** Test 4 usou 2 das 4 variáveis exigidas pelo dispatch
(HOME + USERPROFILE), sem CODEX_HOME e XDG_CONFIG_HOME. `printenv CODEX_HOME XDG_CONFIG_HOME`
nesta sessão → ambas sem saída (exit=1 — não definidas no env pai). Como as variáveis não
estavam definidas no ambiente pai, o binário usou os defaults que derivam de HOME
(`$ISOL/home/.codex`, `$ISOL/home/.config`) — diretórios temporários. Nenhum write ao config
real do usuário. O desvio é de protocolo, não de dado.

**Invocação com `which trackfw && trackfw version` (esta sessão, pós-compactação):**
Bloqueada pelo sandbox (invocação sem o prefixo de isolamento exigido pelo dispatch).

**VHS com variáveis canário:** três tentativas.
1. Output com path absoluto (`/tmp/canary-test.gif`): VHS rejeitou com erro de sintaxe.
2. `env -i` com path relativo: tape executou, mas o shell mostrou prompt `>` (comando não
   terminado — a linha de setup wrappada não foi processada como esperado). O transcript
   canary2.txt só contém `>` repetido; a saída dos `echo` não foi capturada.
3. Nenhuma execução VHS com canário completou.

**Consequência:** a coluna "aparece nos outputs do binário" é medida (Test 4); a coluna
"aparece nas imagens" é inferida por leitura + inspeção visual de todos os frames do GIF
(extraídos via ffmpeg) + grep de bytes e transcripts.

---

## 7. Candidatos para vault (hades não escreve vault neste ML — restrição do dispatch)

- **"VHS .txt inclui conteúdo pré-Ctrl+L; .txt e GIF são vistas distintas"** — o .txt captura
  scrollback do bloco Hide que não aparece no GIF; o .txt pode ser lossy no último frame
  (guard.txt termina antes do `exit=`). Contraintuitivo; outro agente perderia mais de 10 min.
- **"Self_test canário vacuoso: 7/7 passa com main-loop removido por mutation test"** — padrão
  de self-test que verifica setup mas não exercita o controle; vale nota de classe.
- **"`cat -v` torna UTF-8 ilegível: `─` (E2 94 80) aparece como `M-^TM-^@`"** — custou uma
  medição completa do transcript canary2 neste ML; usar Read tool, não `cat -v`, para transcripts
  VHS.

---

*Produzido por hades-tf · ML-4B concluído · Nenhuma linha de código de produto escrita.*
