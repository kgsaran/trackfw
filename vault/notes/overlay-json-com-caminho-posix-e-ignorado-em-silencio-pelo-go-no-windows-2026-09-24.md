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
