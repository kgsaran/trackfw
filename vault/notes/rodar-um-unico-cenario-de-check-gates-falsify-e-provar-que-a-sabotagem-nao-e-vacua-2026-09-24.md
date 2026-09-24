# Rodar **um único** cenário de `check-gates-falsify.sh`, e provar que a sabotagem dele não é vácua

> Data: 2026-09-24 · Autor: `artemis-tf` · ML-1E (G5) da
> `ROADMAP-2026-09-24-treze-rotulos-falham-no-censo-de-windows-e-cinco-sao-setup-que-aborta-o-cenario-inteiro.md`
> Sítios: `scripts/check-gates-falsify.sh`, `scripts/gen-falsify-chunks.py`

## O problema

`scripts/check-gates-falsify.sh` tem ~250 cenários e roda dentro de `make quality` — medido em outra
REQ como **610 s dos 780 s** do job de paridade. Quando um ML toca **um** cenário, rodar o arquivo
inteiro (ou `make quality`) é caro e, pior, atrasa o diagnóstico.

🔴 **Não existe variável de ambiente de filtro.** `TRACKFW_FALSIFY_ENUMERATE=1` **não é** seletor de
cenário — é o modo de *contagem* do censo (não aborta no primeiro FAIL). Procurar um `--only` perde
tempo: ele não existe.

## A forma que funciona: usar o chunker como seletor

`scripts/gen-falsify-chunks.py` já sabe cortar o arquivo em pedaços **executáveis standalone**
(preâmbulo estendido + blocos, com fechamento de dependência **por variável**). Isso é exatamente o
que se quer: o chunk que contém o cenário alvo traz junto, **por construção**, todo bloco anterior
de quem ele lê variável.

```bash
python3 scripts/gen-falsify-chunks.py scripts/check-gates-falsify.sh "$OUT/chunks" 60
grep -l "<marcador-único-do-cenário>" "$OUT/chunks"/*.sh      # ex.: s65-go-stdin-drain-removed
TRACKFW_ROOT_DIR="$PWD" bash "$OUT/chunks/chunk_11.sh"
```

Dois detalhes que custam tempo se faltarem:

- 🔴 **`TRACKFW_ROOT_DIR="$PWD` é obrigatório.** O chunk é materializado fora de `scripts/`, e sem o
  override `${BASH_SOURCE[0]}` resolve `ROOT_DIR` errado — falha em `cp: .../cmd/.: No such file or
  directory`, que parece defeito do cenário e não é.
- **`bash` explícito.** O shell interativo aqui é `zsh`, e o script depende de `mapfile`,
  `PIPESTATUS` e word-splitting do bash.

Medido: N=60 colocou os Cenários **64 e 65 juntos** no `chunk_11` — correto, porque o 65 consome
`$T64_BASE_OUT`, produzido pelo 64. O chunk inteiro custou poucos minutos contra a hora de
`make quality`.

## 🔴 O que um chunk verde **não** prova: a sabotagem pode ser vácua

Um cenário de falsificação tem um braço de **detecção** que corrompe o produto e afirma que o
defeito volta. Se o literal corrompido não for mais o sítio certo — ou se a substituição não
neutralizar nada —, o braço passa a medir o **braço base** duas vezes e fica verde sem provar nada.

O contra-braço que decide é barato: **substituir a sabotagem por uma identidade** (trocar o literal
por ele mesmo) e rodar o chunk de novo. O `corrupt_literal` continua achando exatamente 1 ocorrência,
o build sai íntegro, e o braço de detecção **tem de reprovar**.

Medido neste ML, sobre `git-branch-guard/stdin-drain-before-noop/detection-catches-epipe-regression`:

| sabotagem | `chunk_rc` | braço de detecção |
|---|---|---|
| real (`read …` → `true`) | **0** | `OK … escritor_erro=1` |
| **identidade** (`read …` → `read …`) | **1** | `FAIL … escritor terminou limpo, EPIPE esperado não ocorreu (cenário vácuo)` |

Ou seja: o `EPIPE` observado é **atribuível à sabotagem**, não a qualquer outra coisa do ambiente.
Neste caso o `assert_writer_no_epipe` **já tinha** guarda de vacuidade (`want_writer_ok=0 &&
writer_had_error=0` → FAIL); o contra-braço serve para provar que ela **re-falsifica**, em vez de
presumir que existe e funciona.

## Regra de bolso sobre o literal do `corrupt_literal`

O texto do literal **é o contrato do cenário**, e ele é frágil por natureza: qualquer ML que mexa na
linha corrompida quebra o gate com
`[<label>] expected exactly 1 occurrence of pattern, got 0` — não um FAIL de cenário, um **abort**.
Aconteceu duas vezes com o dreno de stdin do `git-branch-guard` (ML-3A de 2026-09-09 e ML-1B de
2026-09-24).

- Antes de aplicar um literal novo, **conte você mesmo** contra o fonte; `corrupt_literal` exige
  **exatamente 1**.
- O literal viaja por **string de bash entre aspas duplas**: `$?` tem de virar `\$?`. Aspas simples
  não servem porque o próprio literal contém `''` (o `-d ''` do `read`).
- Confirme que a substituição **neutraliza o mecanismo**, não só a linha. Aqui, trocar o `read` por
  `true` deixa `_TRACKFW_STDIN_RC=0`, o `[ … -le 128 ]` quebra o laço na primeira volta e o stdin
  nunca é consumido — exatamente o defeito que o cenário mede.

## Falsificação do outro artefato deste ML: `scripts/trackfw-git-branch-guard.sh`

Esse arquivo é a **cópia instalada** do script no próprio repositório e exige byte-identidade com
`GenerateGitBranchGuardScript`. 🔴 **Não editar à mão** — regenerar pelo gerador:

```bash
mkdir -p zz_dumpguard   # main.go chamando generators.GenerateGitBranchGuardScript(os.Args[1])
go run ./zz_dumpguard "$OUT"
diff -u scripts/trackfw-git-branch-guard.sh "$OUT/scripts/trackfw-git-branch-guard.sh"
cat "$OUT/scripts/trackfw-git-branch-guard.sh" > scripts/trackfw-git-branch-guard.sh   # preserva o modo
rm -rf zz_dumpguard
```

⚠️ **`TestGitBranchGuardScriptReference_MatchesGenerator` não prova isto.** Ele compara duas
**constantes Go** (gerador × referência do validator) e não olha o arquivo em disco. Quem prova o
arquivo é a regra `git_branch_guard_script_integrity` do `trackfw validate`. Falsificada nas duas
direções aqui: com o script antigo em disco → `⚠ scripts/trackfw-git-branch-guard.sh content diverges
from the template`; com o regenerado → a linha some.

Relacionadas: [[guard-aprova-quando-nao-conseguiu-ler-o-comando-orcamento-total-do-read-2026-09-24]],
[[trap-dentro-de-bloco-de-cenario-cobre-subconjunto-que-depende-da-particao-2026-09-24]],
[[emissao-de-sucesso-incondicional-reporta-ok-para-braco-que-reprovou-2026-09-24]].
