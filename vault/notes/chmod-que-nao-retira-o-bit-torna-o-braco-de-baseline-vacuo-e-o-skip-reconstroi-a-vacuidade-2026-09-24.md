# `chmod` que não retira o bit torna o braço de baseline **vácuo** — e o `SKIP` reconstrói a vacuidade um nível acima

> 2026-09-24 · ML-2A (G3) da REQ do cluster de Windows · medido em macOS 27 (APFS + imagem FAT32 via `hdiutil`) e no log do censo `36036473391` (`windows-latest`, x64)

## O que estava em aberto

O Cenário 181 de `scripts/check-gates-falsify.sh` (`scaffold-update-chmod-removed/direction-c-*`)
é o **único rótulo de categoria C** do cluster de Windows: *passagem vacuosa*. Os dois braços
rebaixam a fixture com `chmod 0644` e leem `test -x`; em NTFS montado `noacl` (padrão do Git for
Windows, #421) o `chmod` **não retira o bit**, então `test -x` é verdadeiro **nos dois braços**
independentemente de `os.Chmod` ter rodado. O `OK` do baseline não provava nada.

## O que é fácil perder 10 minutos amanhã

### 1. A vacuidade se reproduz em darwin — não é preciso Windows

Uma imagem FAT32 é um FS que **não representa** o bit de execução, igual ao NTFS `noacl`:

```bash
hdiutil create -size 6g -type SPARSE -fs MS-DOS -volname NOEXECBIT -ov /tmp/fat
hdiutil attach /tmp/fat.sparseimage -mountpoint /tmp/fatmnt -nobrowse
printf x > /tmp/fatmnt/p.sh; chmod 0755 /tmp/fatmnt/p.sh; chmod 0644 /tmp/fatmnt/p.sh
test -x /tmp/fatmnt/p.sh && echo "o FS nao retira o bit"   # imprime
```

O cenário respeita `TMPDIR` (`WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-falsify.XXXXXX")`), então
`TMPDIR=/tmp/fatmnt` põe a fixture inteira no FS que não representa o bit. ⚠️ **200 MB não bastam**:
o cenário compila o binário sabotado dentro do `$WORK` e a primeira medição morreu em
`no space left on device` — use imagem esparsa de ~6 GB.

### 2. 🔴 `SKIP` **não** era uma saída disponível aqui — e a razão não é estilo

`run-gates-falsify-parallel.sh` colhe rótulos com `grep -oE '^(OK|FAIL|PROOF)[[:space:]]+\[falsify/…\]'`
e exige, por chunk, todo rótulo literal que `gen-falsify-chunks.py` extraiu dos `echo "OK|PROOF …"`
do fonte. Consequências, as duas contraintuitivas:

- uma linha `SKIP [falsify/<rótulo>]` **não** é colhida → o rótulo vira *"esperado AUSENTE"*,
  diagnóstico errado (parece chunk morto); e o processo **sai 0**, satisfazendo a guarda por um
  caminho que ninguém lê — **a categoria C reconstruída dentro da guarda**;
- uma linha `FAIL [falsify/<rótulo>]: …` **é** colhida e satisfaz a exigência de rótulo, e ainda
  reprova pelo exit code. Verificado aplicando o pipeline exato do driver ao log medido.

Por isso a saída escolhida foi **falhar alto** (C → A), e por isso os **dois** rótulos são emitidos
no ramo de fixture inconstruível: emitir só um deixaria o irmão como "esperado AUSENTE".

### 3. O que autoriza falhar alto em vez de pular

`windows-census.yml` é `workflow_dispatch` + `continue-on-error: true` — **nunca bloqueia merge**.
Um `FAIL` nomeado custa honestidade e nada mais. Se algum dia o censo virar bloqueante, esta decisão
precisa ser reaberta: aí o `FAIL` seria vermelho permanente sem ação disponível, e a saída correta
passaria a exigir mudança no driver (que **não** é do escopo deste arquivo).

## A matriz medida (imagem FAT32, `TRACKFW_FALSIFY_ENUMERATE` em 0 e 1)

| variante | resultado no FS que não representa o bit |
|---|---|
| código de `HEAD`, **braço de detecção removido** | `OK [.../direction-c-baseline]`, **rc=0 — verde inteiro** 🔴 |
| código de `HEAD`, completo | `OK` baseline (vácuo) + `FAIL` detecção — o estado do censo |
| código novo, **braço de detecção removido** | 2× `FAIL` nomeados, rc=1 — **a vacuidade deixou de existir** |
| código novo, completo | 2× `FAIL` nomeados, rc=1 |

Em APFS os dois braços seguem `OK` e rc=0 — a garantia continua exercitada de verdade em POSIX.

## Testemunha de CI: suficiente para o **efeito**, insuficiente para o **mecanismo**

O log do run `36036473391` (linha 4187) traz, em `windows-latest` x64:

```
-rwxr-xr-x 1 runneradmin 197121 176 Sep 24 17:50 /tmp/trackfw-falsify.VrtbpP/s181-det/project/scripts/trackfw-validate.sh
```

— `rwxr-xr-x` **depois** de `chmod 0644`. Isso fecha o "não medido" da #421 quanto ao **efeito**.
🔴 Não fecha a **atribuição ao `noacl`**: o log **não** contém nenhuma linha de `mount`. Fechar exige
uma linha `mount` (ou `findmnt -T "$TMPDIR"`) no censo — uma linha, e a atribuição sai de presunção.

## Rejeição do `icacls`, declarada como presunção

Construir a fixture por ACL nativa é **inmensurável desta sessão** (darwin). Além disso enfrenta a
**mesma** pergunta não medida da fixture: se, sob `noacl`, o `test -x` do MSYS consulta ACL. Rejeitado
por custo e por não ser verificável aqui — não por mecanismo conhecido.

## Observação lateral, não corrigida

O braço de baseline roda `$ROOT_DIR/bin/trackfw` — o binário **committado**, que pode estar defasado
em relação a `internal/`. O veredito do baseline atesta o que estiver em `bin/`, não a árvore.
Reportado ao arquiteto; fora do escopo deste ML.

## Detalhes que fecham perguntas de auditoria

- A sonda se chama `s181-exec-bit-probe.**sh**` — **mesma extensão da fixture real**, deliberado:
  se o MSYS decidir executabilidade parcialmente por extensão, sonda sem `.sh` mediria outra coisa.
- Todas as variáveis `T181_*`/`s181` ficam **dentro** do bloco do cenário (setup acima da sonda,
  braços dentro do `else`) — nenhuma referência a jusante, então o ramo não-tomado **não** expõe o
  arquivo a morte por `set -u` no Windows. Conferido por `grep -n 'T181\|s181'`.
- Conjunto de rótulos **inalterado**: `gen-falsify-chunks.py` em N=4/8/24, `HEAD` vs árvore →
  **265 vs 265, 0 adicionados, 0 removidos** (só linhas `FAIL` foram acrescentadas e
  `ECHO_LABEL_PAT` colhe apenas `OK|PROOF`).
- **Acoplamento dos braços:** diretórios de projeto, `HOME`s e binários distintos — não há
  instância de fixture compartilhada. O acoplamento real era uma **propriedade do FS**, herdada
  pelos dois braços; a sonda a decide uma vez, antes de qualquer braço rodar.


Relacionada: [[overlay-json-com-caminho-posix-e-ignorado-em-silencio-pelo-go-no-windows-2026-09-24]]
(achado irmão: a fixture `pin7-noexec` de `check-validate-rule-pins.sh`, mesma causa),
[[rodar-um-unico-cenario-de-check-gates-falsify-e-provar-que-a-sabotagem-nao-e-vacua-2026-09-24]]
(como isolar um cenário com `gen-falsify-chunks.py`),
[[rotulo-de-falha-nao-pode-virar-exigencia-da-guarda-de-conjunto-2026-09-24]].
