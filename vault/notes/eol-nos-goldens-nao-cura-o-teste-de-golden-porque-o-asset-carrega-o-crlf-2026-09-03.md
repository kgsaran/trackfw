---
title: fixar `eol=lf` nos goldens NÃO cura o teste de golden — o CRLF entra pelo asset, e a raiz do `.gitattributes` é proibida de carregar a regra
tags: [crlf, windows, gitattributes, goldens, paridade, gotcha]
date: 2026-09-03
related: [[barrier-crlf-divergencia-node-regex-2026-08-29]]
---

## Sintoma

`core.autocrlf=true` (default do runner Windows) em clone deste repositório: **18 falhas** só no
`go test ./internal/...`. A leitura natural — "os goldens versionados chegam com `\r\n`, o teste
compara bytes e diverge" — sugere que declarar `eol=lf` para os goldens resolve o grupo.

**Resolve zero desses testes.** Medido.

## Causa raiz — o CRLF entra pelos DOIS lados da comparação

`TestRenderWithoutIdentityMatchesFrozenGoldens` compara:

```
Render(item, ..., source=catalog.ReadAsset(item), ...)   vs   testdata/*.golden.*
```

O `source` é o asset **embedado** (`internal/integrations/assets/agents/*.md`), lido do disco pelo
`go:embed` **em tempo de build**. Sob `autocrlf=true` o asset também chega com CRLF. Então:

| lado | encoding sob autocrlf | efeito de `eol=lf` só no golden |
|---|---|---|
| esperado (golden) | CRLF → LF | vira LF |
| real (`Render`) | CRLF, herdado do asset | **continua CRLF** |

Fixar só o lado esperado **piora** a legibilidade da igualdade e não a restaura. Falsificado
diretamente: goldens convertidos para LF com `perl -pi`, asset intacto → o teste segue vermelho, e o
diff agora mostra o parser de frontmatter emitindo frontmatter duplicado, que é o defeito real.

**O grupo de render é metade de PRODUTO, não metade de fixture** — apesar de "golden" no nome. O
discriminante correto não é a extensão nem a pasta `testdata/`, é: *o arquivo alcança um parser do
produto, ou só a asserção do teste?*

## A metade que É fixture (e cura de verdade)

Medido 18 → 14 falhas. Os 4 curados não são os goldens de render; são comparações
**artefato-versionado × literal compilado**, onde nenhum código de produto lê o arquivo:

| teste | arquivo lido como texto |
|---|---|
| `TestSlashRoadmapCommandRequiresCanonicalFrontmatter` | `.claude/commands/trackfw/roadmap.md` |
| `TestCredentialGuardScript_ParityAcrossStacks` | `npm/src/generators/hooks.js`, `pypi/trackfw/generators/init_gen.py` |
| `TestGlobalCredentialGuardScript_ParityAcrossStacks` | idem |
| `TestScriptsParity_GoldenCanonicalBlocks` | idem + `npm/src/generators/init.js` |

Os três últimos leem **código-fonte de outro runtime como texto**, extraem template literals por
regex e comparam byte-a-byte com constantes Go. O `\r` entra no literal extraído e a paridade
"falha" por encoding de checkout, sem nenhuma divergência real de conteúdo.

## Armadilha: a raiz do `.gitattributes` é proibida de carregar a regra

`TestGitAttributesBlock_IgualAoArquivoVersionadoNaRaiz` (Go) + os equivalentes
`test_bloco_igual_ao_gitattributes_versionado_na_raiz` (Python) e `bloco gerado é igual ao
.gitattributes versionado na raiz do repositório` (Node) exigem:

```
conteúdo de ./.gitattributes  ==  GITATTRIBUTES_BLOCK
```

e `GITATTRIBUTES_BLOCK` é **constante de produto** nos 3 CLIs (o bloco que `trackfw init` escreve no
projeto do usuário). Consequências, ambas não óbvias:

1. **Qualquer linha acrescentada à raiz quebra 3 testes** — e o conserto exigiria mudar código de
   produto, além de vazar caminhos deste repositório (`internal/`, `npm/src/`) para o
   `.gitattributes` de todo usuário do `trackfw`.
2. **A própria raiz do `.gitattributes` precisa de `eol=lf`** — ela é comparada byte-a-byte com um
   literal Go de `\n` e falha sob CRLF. Mas a regra que a protegeria teria de estar *dentro dela*,
   o que a igualdade proíbe. Um `.gitattributes` de subdiretório não alcança a raiz. **Sem saída
   dentro do repositório**: `TestGitAttributesBlock_...` fica vermelho no Windows até o
   `GITATTRIBUTES_BLOCK` do produto mudar.

O contorno usado foi `.gitattributes` **por diretório** — `.claude/commands/trackfw/`, `npm/` (só
`src/generators/*.js`) e `pypi/` (só `trackfw/generators/*.py`). Git honra os três, a raiz fica
intacta e o `merge=union` do `.trackfw-log` continua valendo.

`internal/integrations/testdata/` foi **deliberadamente deixado sem pin**: pelo critério acima ele
está do lado de **produto**, e fixar só o lado esperado de uma comparação cujo lado real continua
CRLF não cura nada — só apaga o `\r` que é a evidência do defeito do parser. O pin passa a ser
correto no dia em que o parser normalizar CRLF na fronteira; antes disso, não.

Assimetria a não esquecer: o par Python desse teste (`test_bloco_igual_ao_gitattributes_versionado_
na_raiz`) **passa** sob CRLF, porque `open(path, encoding="utf-8")` faz universal newlines. Go
(`os.ReadFile`) e Node (`readFileSync(..., 'utf8')`) não fazem, e falham. O mesmo contrato reprova
em 2 dos 3 runtimes — medir num só dá conclusão errada.

## Como medir isso sem uma máquina Windows

```bash
W=$(mktemp -d)
git clone --local -c core.autocrlf=true <repo> "$W/crlf"   # o filtro aplica no checkout do clone
```

Aplicar `core.autocrlf` **no clone** (`-c`), não depois: `git reset --hard` está bloqueado pelo guard
de git deste repositório, e `git checkout -- <path>` também. Para re-materializar caminhos após
editar um `.gitattributes`, o caminho liberado é
`git ls-files -z -- <glob> | xargs -0 rm -f` seguido de `git checkout-index -f -- <paths>`.

Um `.gitattributes` **não rastreado** já é honrado pelo `check-attr` e pelo `checkout-index` — dá
para medir o efeito antes de commitar.

## Fora do alcance local

O default real de `core.autocrlf` no runner do GitHub Windows e qualquer interação com limite de
path do NTFS não são reproduzíveis em APFS. Só o run de Windows fecha.
