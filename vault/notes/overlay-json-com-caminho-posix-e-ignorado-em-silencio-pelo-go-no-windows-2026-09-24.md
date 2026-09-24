# `go test -overlay` com chave em caminho POSIX é ignorado **em silêncio** no Windows

> 2026-09-24 · Wave 2 da REQ-2026-09-24 (caminho POSIX interpolado no código Python) ·
> medido na VM Windows 11 ARM64, Git Bash (MSYS x86_64 emulado), `go1.27.0 windows/arm64`

## Por que a nota existe

A Wave 1 tirou o caminho POSIX de dentro do texto do programa Python em 6 sítios. Rodando o gate
de segurança **inteiro** no Windows — coisa que o ML-0A declarou como residual e o ML-1A não fez —
o `FileNotFoundError` sumiu e **apareceu outra falha, escondida atrás dele**:

```
FAIL AC6 Go falsificação: teste passou na versão vulnerável — deve reprovar
     ok  	github.com/kgsaran/trackfw/internal/serve	0.049s
```

Quem for depurar isso amanhã vai suspeitar do teste, do `symlinkOrSkip` ou do `t.Skip`. Não é nada
disso: **o overlay nunca foi aplicado**, e o `go` não reclama.

## A medição, nas duas direções, mesma máquina e mesmo minuto

`scripts/check-serve-api-file-security.sh:96` monta o overlay com `printf` de variáveis do shell,
que no Git Bash são **caminhos POSIX-MSYS**:

```
{"Replace": {"/c/Users/Lab/trackfw/internal/serve/api_file.go": "/tmp/ac6-repro.9YyqXD/api_file_vuln.go"}}
   → go test …  ->  ok  	github.com/kgsaran/trackfw/internal/serve	0.038s        ← overlay IGNORADO
```

Controle, a mesma chave passada por `cygpath -m`:

```
{"Replace": {"C:/Users/Lab/trackfw/internal/serve/api_file.go": "C:/Users/Lab/AppData/Local/Temp/ac6-repro.9YyqXD/api_file_vuln.go"}}
   → go test …  ->  --- FAIL: TestFileHandler_SymlinkEscape (0.01s)
                        api_file_test.go:169: esperado 403, obteve 200; body: HADES_SECRET_TOKEN_ABC123
                        api_file_test.go:173: corpo vazou segredo: HADES_SECRET_TOKEN_ABC123
```

O `python3` escreveu o arquivo vulnerável corretamente nas duas execuções
(`grep -c 'if false && !filePathAllowed'` → **1**). A única variável é a **grafia da chave**.

## 🔴 O que faz isto ser caro

`go test -overlay` com chave que **não casa nenhum arquivo do build** não é erro: o Go aplica o que
casa e segue. O teste roda contra o **fonte correto**, passa, e o braço de falsificação conclui
"passou na versão vulnerável". A falha é ruidosa (o gate reprova), mas o **diagnóstico aponta para
o lugar errado** — para o teste, não para o `printf`.

## A família, e por que ela não acaba no Python

É o **mesmo mecanismo da REQ** — caminho POSIX-MSYS que não passa por `argv` de um programa Windows
nativo — numa **superfície diferente**: em vez de texto de programa Python, um **payload JSON** lido
pelo `go.exe`. A correção por `argv` não serve aqui (não há argv): serve `cygpath -m`, ou escrever o
JSON pelo próprio Python, que no Windows já tem `os.getcwd()` em grafia nativa (é o que
`.github/workflows/quality.yml:1172` faz, e por isso aquele sítio **não** é afetado).

**Regra de bolso:** todo caminho que sai do bash do MSYS e entra num programa Windows nativo por
**arquivo de configuração** (overlay, manifesto, `.json`, `.yaml`) precisa de `cygpath -m`. Por
`argv` o MSYS converte; por conteúdo de arquivo, nunca.

**Alcance:** o único sítio de overlay montado por `printf` do shell é
`scripts/check-serve-api-file-security.sh:96`. Em `ubuntu-latest` — o único lugar onde o gate roda
em CI hoje — POSIX **é** a grafia nativa, a chave casa e o braço passa (run `36023336663`,
`parity-other-gates`: `ok  AC6 Go falsificação: sem filePathAllowed(realAbsPath) o teste
SymlinkEscape FALHA`). O defeito é **exclusivo do Windows**.

**O gate do ML-1B não vê este sítio** — ele varre corpos de programa Python, e aqui é um `printf`
para dentro de um JSON.

## Achado irmão, medido no mesmo dia: `chmod` não tira o bit de execução em NTFS

Na mesma VM, `/tmp` é `C:/Users/Lab/AppData/Local/Temp` montado
`ntfs (binary,noacl,posix=0,usertemp)`:

```
chmod 644 f.sh  →  ls -l: -rwxr-xr-x   stat %a: 755   [ -x f.sh ] → verdadeiro
```

Consequência direta: a fixture `pin7-noexec` de `scripts/check-validate-rule-pins.sh` (arquivo
presente **e não executável**) **não é construível** nesse Windows, e o gate reprova com
`[pin7-noexec] vacuity: … expected 'credential_guard_hook_resolvable' violation, none found (rc=0)`.
Isso é **permissão**, não tradução de caminho — não confundir com a família acima.

---

## Correção (ML-2A, 2026-09-24) — e a regra de bolso **medida**, não presumida

`:96` deixou de existir como `printf`. O `overlay.json` passou a ser escrito pelo **próprio Python**,
com `json.dumps` sobre os caminhos recebidos por `argv` e passados por `os.path.abspath` — a mesma
forma de `.github/workflows/quality.yml:1172`, que **roda em `windows-latest`** (job
`windows-symlink-unprivileged`, `quality.yml:1026`) e por isso é precedente medido, não inferido.

**Duas razões, e a segunda não é sobre Windows:**

1. **grafia** — `argv` o MSYS converte; conteúdo de arquivo, nunca;
2. 🔴 **escape** — `printf '{"Replace": {"%s": ...}}'` é interpolação de string crua dentro de
   formato estruturado, e isso **não é um problema de Windows**: qualquer `\` ou `"` no caminho
   quebra o JSON. `cygpath -m` corrigiria a grafia e escaparia do problema **por acidente**, por
   emitir `/`; `cygpath -w` emitiria `C:\Users\...` e produziria **JSON inválido**. `json.dumps`
   fecha os dois **por construção**, em todas as plataformas, sem ramo condicional.

**Segundo canal de vacuidade, fechado nas mesmas linhas:** o `src.replace(needle, ...)` era um
**no-op silencioso** se o needle mudasse — o "vulnerável" sairia byte a byte igual ao correto, o
overlay aplicaria, o teste passaria, e o gate imprimiria **a mesma linha enganosa** (`AC6 … passou na
versão vulnerável`) por outro mecanismo. Agora há `assert count >= 1`. Medido: com needle inexistente,
`AssertionError: … (count=0)`, rc=1.

### A regra de bolso, corrigida pela medição

A primeira versão desta nota dizia "por `argv` o MSYS converte". **Ele converte `argv` E o
ambiente.** Medido na mesma VM:

```
GOCACHE="/tmp/envprobe.0NJyt9/gc" go env GOCACHE
  → C:/Users/Lab/AppData/Local/Temp/envprobe.0NJyt9/gc      (go build com ele: rc=0, binário gerado)
```

Isso **isenta** toda a família `GOCACHE=`/`GOPATH=`/`GIT_CONFIG_GLOBAL=` dos gates — que de outra
forma seria a maior população suspeita do repositório.

> **Regra de bolso — o que foi medido:** o MSYS converte caminho que sai do bash por **`argv`**
> (Wave 1) e por **variável de ambiente** (probe acima). Ele **não** converte caminho que sai por
> **conteúdo de arquivo** (overlay, manifesto, `.json`, `.yaml`) — é o A/B desta nota. Caminho que
> atravessa por conteúdo para um binário Windows nativo precisa ser escrito pelo consumidor, ou
> convertido antes.
>
> **Inferido, sem sítio no repositório para medir:** **stdin** canalizado para binário nativo deve
> cair no mesmo caso do conteúdo de arquivo. A varredura do ML-2A não achou nenhum sítio dessa forma,
> então isto **não** foi medido — está aqui como hipótese rotulada, não como achado.

### Isenção por sítio — duas razões diferentes, não uma

| família | consumidor | razão | estado |
|---|---|---|---|
| `GOCACHE=` / `GOPATH=` | `go.exe`, **nativo puro** | o MSYS converte o ambiente antes de entregar | **medido** (probe acima) |
| `GIT_CONFIG_GLOBAL=` / `GIT_CONFIG_SYSTEM=` | `git.exe` do Git for Windows, programa **MSYS-linked** | aceita caminho POSIX nativamente (é por isso que `/dev/null` funciona nesses gates) | **inferido** — não medido pelo ML-2A |
| `TRACKFW_ROOT_DIR=` / `TRACKFW_INSTALL_DIR=` | `bash`/`install.sh` | consumidor é o próprio shell MSYS | por construção |
