# A cópia versionada de `scripts/trackfw-attention-signal.sh` está obsoleta e nenhum teste ou gate a compara com o literal

> 2026-09-02 · ML-2A do ROADMAP-2026-09-02-saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate · artemis-tf

## O achado

O `attentionSignalScript` existe em **duas naturezas** neste repositório, e é fácil confundi-las:

1. **O literal-fonte**, embutido byte-idêntico nos 3 CLIs
   (`internal/generators/scaffold.go`, `npm/src/generators/hooks.js`,
   `pypi/trackfw/generators/init_gen.py`). É o **contrato** — é o que `trackfw init` /
   `discover --init` escrevem na máquina de quem adota.
2. **A cópia versionada** em `scripts/trackfw-attention-signal.sh`, que é o hook do **próprio
   projeto trackfw**, gerado alguma vez no passado e commitado.

O ML-1A (`5b5391e`) corrigiu **só a natureza 1** — o prefixo `PYTHONIOENCODING=utf-8` foi para os 3
literais. A cópia versionada **não foi regenerada** e ficou divergente:

```
internal/generators/scaffold.go:773      ... | PYTHONIOENCODING=utf-8 python3 -c ...
npm/src/generators/hooks.js:147          ... | PYTHONIOENCODING=utf-8 python3 -c ...
pypi/trackfw/generators/init_gen.py:969  ... | PYTHONIOENCODING=utf-8 python3 -c ...
scripts/trackfw-attention-signal.sh:14   ... | python3 -c ...          <- sem o prefixo
```

## Por que ninguém percebeu — e por que ninguém vai perceber sozinho

**Nada compara a cópia versionada com o literal.** Medido:

- `scripts/check-attention-scripts-parity.sh` roda `discover --init` nos 3 runtimes dentro de um
  fixture `mktemp -d` e faz diff **entre os três**. Nunca lê `scripts/trackfw-attention-signal.sh`
  da raiz.
- `internal/generators/scaffold_parity_test.go` parece ler o arquivo
  (`os.ReadFile(filepath.Join("scripts", "trackfw-attention-signal.sh"))`), mas ele faz
  `os.Chdir(t.TempDir())` antes — o caminho relativo aponta para o fixture temporário, não para a
  raiz. `internal/discover/discover_test.go` e `internal/commands/init_test.go` fazem o mesmo.

Ou seja: a cópia versionada é **um artefato órfão**. Ela pode divergir arbitrariamente do literal
sem nenhum sinal vermelho, e é ela que roda quando um agente trabalha *neste* repositório.

## A armadilha de medição que isto arma

Quem for auditar "o prefixo está aplicado?" e abrir `scripts/trackfw-attention-signal.sh` — o
arquivo com o nome mais óbvio — vai concluir que o ML-1A **não** foi aplicado. E quem regenerar a
cópia e vir o diff pode concluir que "o gerador mudou sozinho". É a mesma classe de erro registrada
no ML-0A desta REQ ("medir algo próximo do que se quer"): o alvo do contrato é o **literal-fonte**,
não a cópia.

O `scripts/check-output-encoding-declared.sh` (ML-2A) ancora **deliberadamente** nos 3 arquivos-fonte
por isso, e **não** na cópia versionada — se ancorasse na cópia, reprovaria a árvore correta hoje.

## O que fazer

- Regenerar a cópia (`trackfw discover --init` neste repositório) é a correção, mas é mudança de
  artefato de produto versionado — **decisão do arquiteto**, não do ML que descobriu.
- Se a cópia passar a ser tratada como contrato, ela precisa de um gate próprio comparando-a com a
  saída do gerador. Hoje esse gate não existe, e a lacuna é maior que o prefixo de codificação:
  vale para **qualquer** mudança futura no literal.

## Referências

- `scripts/check-output-encoding-declared.sh` (alvo 2)
- `scripts/check-attention-scripts-parity.sh` (compara os 3 entre si — o cego que motivou o alvo 2)
- `vault/notes/barrier-so-casa-cabecalho-de-aceite-em-portugues-2026-08-28.md` (mesma classe: gate
  de paridade não é gate de correção)
- `vault/notes/cp1252-roundtrip-mascara-o-defeito-o-discriminante-e-decode-de-stdin-2026-09-02.md`
