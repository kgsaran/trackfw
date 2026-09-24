# Fixture que cruza a fronteira MSYS controla **as duas pontas a partir de uma string só**

> 2026-09-24 · ML-1C/ML-1D (Wave 1, G4) da REQ-2026-09-24 do cluster de Windows ·
> medido na VM Windows 11 ARM64, Git Bash (MINGW64_NT-10.0-26200-ARM64, MSYS x86_64 emulado),
> `go1.27.0`, com o **binário real** compilado de `0175634d`

## O que esta nota acrescenta

A assimetria ("o MSYS converte env var e `argv`, mas nunca conteúdo de arquivo") já está em
[[msys-converte-env-e-argv-mas-nunca-conteudo-de-arquivo-2026-09-24]]. **O que é novo aqui é o
remédio** — e as duas armadilhas que o remédio óbvio tem.

## Armadilha 1 — converter só o arquivo deixa o MSYS na fronteira

A correção intuitiva é converter **apenas** o `command` gravado no JSON (`cygpath -m`) e deixar o
MSYS converter o `HOME`. Isso **depende de as duas conversões serem byte-iguais**, e elas não são
necessariamente:

| onde | grafia observada |
|---|---|
| VM ARM64, MSYS convertendo `HOME` | `C:\Users\Lab\AppData\Local\Temp\…` (nome longo) |
| censo x64 (`36036473391`), `argv` | `C:/Users/RUNNER~1/…` (**8.3**) |

Se `cygpath -m` emitir o nome longo onde o MSYS emite 8.3, o cenário **passa na VM e segue vermelho
no CI** — e o critério de aceite ("medido na VM") teria sido satisfeito por um fixture ainda
quebrado.

🔴 **Remédio:** o fixture converte a grafia **uma vez** e usa **a mesma string** nos dois lados —
grava no JSON *e* entrega como `HOME` ao binário. O MSYS sai inteiramente da fronteira; não há duas
conversões para divergir. `filepath.Join` faz `Clean`, então a ponta do Go normaliza sozinha
qualquer forma que o `HOME` tenha.

Em POSIX o helper devolve a entrada **inalterada**, então as variáveis ficam byte a byte idênticas
ao que eram — o determinismo que o braço 1 declara no comentário sobrevive **por construção**, não
por inspeção.

## Armadilha 2 — `cygpath` apaga a malformação que o cenário mede

O braço 4 do Cenário 67 existe para exercitar **barra dupla** (`//`) no `command` gravado.
`cygpath -m` **colapsa** `//` embutido. Converter o caminho já malformado **apaga o caso**, e o
rótulo `double-slash-tolerance` passa sem provar nada — passagem vacuosa exata.

🔴 **Remédio, em duas partes:** converter a **base** e injetar o `//` **depois**, sobre a grafia
nativa; e **assertar** que o `//` sobreviveu até o arquivo gravado, com âncora no **segmento**
(`//s67-fake-home-installed-slash`), nunca em `//` solto — um prefixo nativo `C:/` satisfaria uma
âncora ingênua por acidente. A guarda emite
`PROOF [falsify/git-branch-guard-dedup/double-slash-tolerance/non-vacuity]`.

## O achado colateral que custa 10 min amanhã — braço 3 passava **vacuamente** no Windows

`git-branch-guard-dedup/detection-catches-regression` aparece **`OK` antes e depois** da correção.
Isso não é sinal de que ele estava bom: ele usa **o mesmo `$HOME` sintético do braço 1**, e afirma
"com o dedup neutralizado, a entrada de projeto REAPARECE". Antes do ML-1C a entrada reaparecia de
qualquer jeito — `MATCH=false` por espaço de nomes —, **independente de o binário estar corrompido**.
O braço não conseguia distinguir *dedup neutralizado* de *grafia divergente*: verde pelo motivo
errado. Depois do ML-1C ele passa pelo motivo certo.

🔴 **Regra transferível:** num cenário de detecção cujo braço positivo (baseline) está **vermelho**,
o braço de detecção verde **não prova nada** — os dois dependem do mesmo fixture. Ler o par
baseline/detecção junto é o que revela a vacuidade; ler só o `OK` do detector não revela.

## Medição (VM, mesmo chunk, antes e depois)

Chunk que contém o Cenário 67 = `chunk_4` de
`gen-falsify-chunks.py scripts/check-gates-falsify.sh <dir> 24`, rodado com
`TRACKFW_FALSIFY_ENUMERATE=1` (sem enumerate o `falsify_fail_point` do braço 1 aborta antes de o
braço 4 sequer gravar o fixture — e não haveria evidência nenhuma sobre `double-slash-tolerance`).

```
ANTES (0175634d, árvore intocada)                       CHUNK_RC=1
  FAIL [falsify/git-branch-guard-dedup/baseline-skips-project-entry]
  OK   [falsify/git-branch-guard-dedup/baseline-credential-guard-unaffected]
  OK   [falsify/git-branch-guard-dedup/reverse-vacuity]
  OK   [falsify/git-branch-guard-dedup/detection-catches-regression]   ← vacuosa
  FAIL [falsify/git-branch-guard-dedup/double-slash-tolerance]

DEPOIS (ML-1C)                                          CHUNK_RC=0
  OK   ×4  +  PROOF …/double-slash-tolerance/non-vacuity  +  OK double-slash-tolerance
```

⚠️ O `dir` de saída dos chunks **tem de ficar dentro do repositório**: cada chunk resolve
`ROOT_DIR` como `dirname($BASH_SOURCE)/..`. Gerar em `/tmp/ch` faz o chunk procurar
`/tmp/scripts/lib-crlf-normalize.sh` e morrer na linha 32, com **log de uma linha só** — fácil de
confundir com "o cenário não rodou".

## ML-1D — o veredito, e por que nenhum caso daquela tabela podia ter pego o G4

`{"relative path with backslash, no drive letter, untouched", "scripts\\guard.sh", …}` pina
**comportamento desejado**, não acidente: sem âncora de letra de unidade na posição 0, `\` é **byte
de nome de arquivo** em POSIX, e traduzi-lo faria dois arquivos genuinamente diferentes compararem
iguais — falso "já instalado" que desarma o dedup em silêncio. O resíduo em Windows está declarado
no doc comment de `normalizeGuardPath`, com direção sempre **APERTA**. Caso **mantido**, com o nome
reescrito para declarar a garantia em vez da implementação.

🔴 **E a moldura do handoff precisa de um reparo:** o G4 não ficou invisível *por causa* desse caso.
Nenhum caso daquela tabela poderia tê-lo pego — a tabela é de **sintaxe**, e o G4 diverge por
**montagem**. O próprio ML-1A já escreve isso entre parênteses ("o 'defeito' que ele fixa nem é o
mecanismo do G4").

A trava executável contra a correção errada de amanhã é
`TestSamePathCommand_MSYSAndNativeSpellingsMustNotMatch`: afirma que grafia POSIX e grafia nativa do
**mesmo arquivo** **não** comparam iguais. Falsificado nas duas direções — com `samePathCommand`
afrouxado para comparar por *basename* (o afrouxamento que a §6.4 do parecer proíbe), via
`go test -overlay`, ele **reprova**; na árvore íntegra, passa.
