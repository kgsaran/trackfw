# A extensão escolhida para esconder do gate esconde também do `.gitattributes` — e `-text` não é `text eol=lf`

> ML-9B · REQ-2026-08-31 · `apolo-tf` · 2026-09-25

## Contexto

O `ML-7C` versionou o corpus pré-fix do `#400` em
`internal/pathguard/testdata/corpus-pre-fix/`, 16 arquivos com extensão **`.go.txt`** e um
`MANIFEST.sha256`. A extensão foi escolhida de propósito, e por dois motivos bons: manter os
arquivos **fora do build** do Go e **fora do `find internal -name '*.go'`** de
`scripts/check-write-containment.sh` (que senão veria 157 sítios de escrita injustificados).

No `windows-full-suites` do PR #441 os três braços do corpus ficaram vermelhos com
`… has sha256 234fbb1c…, the manifest pins 5d7a85ef… — the frozen pre-fix evidence was modified`.

## Achado 1 🔴 — o truque que fez o corpus funcionar é a causa da falha

O `.gitattributes` deste repo declara `*.go text=auto eol=lf`. `agentfiles.go.txt` **não é `*.go`** —
é `*.txt`, e não há regra para `*.txt`. Medido antes da correção:

```
$ git ls-files --eol internal/pathguard/testdata/corpus-pre-fix/internal/generators/agentfiles.go.txt
i/lf    w/lf    attr/                      ← nenhum atributo
```

No runner Windows (`core.autocrlf=true`) o **checkout** converte, os bytes mudam, e o sha256 do
manifesto — que existe justamente para detectar modificação — acusa modificação. **Instrumento, não
produto.**

**Regra prática:** toda vez que se escolhe uma extensão para *esconder* um arquivo de um gate, ela o
esconde de **todas** as regras que casam por extensão — inclusive das que você queria. Ao versionar
evidência byte-exata, declare o `.gitattributes` **no mesmo commit** do artefato.

## Achado 2 — reproduzir conversão de Windows no macOS, com braço-contrário

Não é preciso runner Windows para medir isto:

```bash
git -c core.autocrlf=true checkout-index --prefix="$TMP/co/" -a
grep -c $'\r' "$TMP/co/internal/pathguard/testdata/corpus-pre-fix/internal/generators/agentfiles.go.txt"
```

Antes da regra: **2172**. Depois: **0**.

🔴 **Zero sozinho não prova nada** — é indistinguível de "a conversão nunca disparou". O
braço-contrário é obrigatório, **na mesma árvore**: `docs/cli-parity.md` (deliberadamente fora de
qualquer regra de `eol`) chega com **7451** linhas CRLF, e `internal/pathguard/pathguard.go` (coberto
por `*.go eol=lf`) chega com **0**. Um prova que o harness reproduz; o outro, que a regra segura.

## Achado 3 🔴 — `-text` e `text eol=lf` não são equivalentes para EVIDÊNCIA

Os dois evitam CRLF no checkout. A diferença está no **check-in**:

- `text eol=lf` autoriza o git a **normalizar CRLF→LF ao gravar**. Uma edição acidental com editor
  CRLF seria reescrita para LF, o sha256 continuaria batendo, e a adulteração ficaria **invisível
  exatamente para o instrumento feito para detectá-la**.
- `-text` grava e entrega os bytes **verbatim nas duas direções**: qualquer modificação aparece como
  divergência de hash.

Para artefato cuja **identidade em bytes é a afirmação**, a regra tem de tornar a adulteração
**barulhenta**, não normalizá-la. Não usar o macro `binary` (= `-text -diff`): queremos o diff
legível.

E a linha vai no **fim** do `.gitattributes`: o último padrão que casa vence, então nenhuma regra
futura de `*.txt`/`*.md` reabilita a conversão em silêncio.

## Achado 4 🔴 — pinar o golden **não** fecha o teste de golden, e o motivo refuta a explicação óbvia

A varredura de mesma classe achou o outro `testdata` de evidência byte-exata sem regra de `eol`:
`internal/integrations/testdata/*.golden.*`, cujo teste
(`TestRenderWithoutIdentityMatchesFrozenGoldens`) está em `.github/windows-known-failures.json` desde
2026-09-10. A explicação intuitiva era *"o renderer normaliza CRLF→LF (`NormalizeCRLF`,
`render.go:393`), então `got` é LF e só o golden virou CRLF"*.

**Medido, e é falso.** Na árvore convertida o `got` sai **com CRLF** — o caminho `subagent` do
`Render` não passa por `NormalizeCRLF`:

```
got.bin:    -  -  -  \r \n  n  a  m  e ...
golden.md:  -  -  -  \n     n  a  m  e ...
```

Antes do pin os **dois** lados vinham CRLF e a divergência se **cancelava em parte** (3 subtestes
vermelhos → 1 depois do pin). Pinar o golden continua certo — torna a comparação honesta e faz a
cegueira a CRLF **do produto** aparecer —, mas a causa remanescente é **de produto, não de
checkout**: mecanismo distinto do corpus e, por isso, fora desta REQ pelo teste literal da Regra Dura
("se eu corrigir esta causa, exatamente estas falhas fecham").

**Regra prática:** antes de atribuir a uma conversão de checkout um teste de comparação, meça **de
que lado está o `\r`**. Dois lados convertidos podem se cancelar, e o verde (ou o vermelho parcial)
mente nas duas direções.

## Achado 5 — a asserção de caminho quebrou porque o **emissor** mudou, não o teste

`TestUpdateNeverWritesThroughSymlinkAtDiscoverWorkflowPath` fazia
`strings.Contains(stderr, DiscoverGitHubActionsWorkflowPath)`, e a constante usa `/`. Até o `ML-7B` a
mensagem era `aviso: %s é um symlink` com caminho **relativo** (sempre `/`); com a gramática única do
emissor ela passou a imprimir o **caminho absoluto do SO** (`C:\…\.github\workflows\…` no Windows).

A correção é `filepath.FromSlash(...)` — **não** um `Contains("symlink")` genérico, que passaria com
o caminho **errado** na mensagem, que é justamente o que a asserção existe para reprovar. E não se
compara o caminho absoluto inteiro: no Windows o `t.TempDir()` pode cair sob nome 8.3
(`C:\Users\RUNNE~1\…`, ver
[nota de 2026-09-10](windows-8dot3-short-name-quebra-path-relative-em-fixture-git-2026-09-10.md)) e o
prefixo divergiria **legitimamente**. O que se exige é o **sufixo nativo**.

## Achado 6 — a regra nova é um comentário sem braço que a falsifique

`TestCorpusIsPinnedAgainstEOLConversion` (`internal/pathguard/containment_corpus_test.go`) tem dois
braços: **bytes** (nenhum CRLF na árvore de trabalho — reprova no Windows) e **declaração**
(`git check-attr text` tem de responder `unset` para os 17 caminhos — reprova no Linux/macOS, onde
não há conversão nenhuma para reprovar). Sem o segundo, apagar a linha do `.gitattributes` passaria
despercebido em todo CI que não é Windows. Falsificado removendo a linha: **rc=1**, com
`text: unspecified` nomeado arquivo a arquivo.
