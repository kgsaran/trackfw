---
status: Accepted
date: 2026-09-04
author: "trackfw_architect (Zeus)"
---

# ADR: O parser de frontmatter tolera CRLF na fronteira de entrada

> Date: 2026-09-04 | Status: Accepted

## Contexto

O parser de frontmatter é **cego a CRLF**. Sob `core.autocrlf=true` — default do runner Windows — ele
**emitiu frontmatter duplicado** em `TestRenderOpenCodeAgent`: a regex não casa o delimitador com
`\r\n`, então o parser conclui que não há frontmatter e escreve um novo por cima do existente.

~14 das 217 falhas reais de Windows.

**Não é defeito só de teste.** Quem edita um agent-md no Windows com um editor que grava CRLF
produz exatamente essa entrada — e o produto corrompe o artefato dele em silêncio.

## A alternativa foi medida e recusada

Declarar `eol=lf` sobre os assets que o produto processa **parece** resolver e **não resolve**. O
ML-1C mediu:

```
goldens convertidos para LF, asset intacto  ->  teste SEGUE vermelho
```

O CRLF entra pelos **dois lados** da comparação, porque o `source` é o asset embedado. E o agente
que fez a medição **removeu o pin que já tinha aplicado**, com o argumento certo:

> Fixar só o lado esperado **cura zero** e **apaga o `\r` que é a evidência** do parser.

🔴 **A recusa é o ponto:** declarar `eol` ali reduziria a contagem **escondendo** o defeito. Ficaria
verde no CI e continuaria corrompendo o arquivo de quem edita no Windows.

## Decisão

### D1 — Normalizar na fronteira de **entrada**, não na de saída

O parser normaliza `\r\n` → `\n` **ao ler**, antes de qualquer casamento de delimitador. Um arquivo
com CRLF é **entrada válida**, não erro.

Razão: o produtor do arquivo é frequentemente **outra pessoa, em outro SO**, com um editor que
escolheu o fim de linha por ela. Recusar é transferir a um humano um problema que o parser resolve
em uma linha.

### D2 — A normalização é de **leitura**; a escrita continua LF

Nada aqui muda o que o trackfw **escreve**. Ele grava LF nos 3 CLIs, e isso já é contrato
(`check-python-writes-lf.sh`). Tolerar na entrada **não** autoriza emitir CRLF.

### D3 — Ponto único por runtime

Uma função de normalização por CLI, aplicada onde o conteúdo **entra** no parser. Não espalhar
`ReplaceAll` pelos chamadores — mesmo padrão do ponto único de resolução de REQ (`ADR-2026-09-03`) e
pela mesma razão: **duas noções de formato no mesmo runtime é a origem das três ocorrências de
gerador-verificador discordando do contrato.**

### D4 — 🔴 Os assets e goldens continuam **sem** `eol` declarado

O `.gitattributes` da raiz declara `eol=lf` para os **fontes** e deixa de fora, com enumeração
escrita, `*/integrations/assets/**`, `internal/integrations/testdata/*.golden.*`, o corpus da
barreira e `*.md` em geral.

**Isso permanece depois desta ADR.** A tolerância vive no parser; o `.gitattributes` não deve
mascarar a entrada, ou perdemos a capacidade de detectar a regressão.

## Consequências

**O teste que hoje falha vira o teste que protege.** `TestRenderOpenCodeAgent` deixa de emitir
frontmatter duplicado, e a fixture com CRLF passa a ser o caso de regressão.

**Regra dura de paridade:** os 3 CLIs, byte-idêntico. E aqui há um risco específico: o `open()` do
Python faz **universal newlines** por padrão, então **o Python pode já passar** onde Go e Node
falham. 🔴 **Medir num runtime só engana** — foi medido: os 3 divergiam no teste de contenção do
`.gitattributes` exatamente por isso.

**Não fecha CR sozinho (`\r` sem `\n`).** Mac clássico não é caso de uso; se aparecer, é REQ própria.
Declarar aqui em vez de tratar em silêncio.

## Verificação exigida de quem implementar

- Falsificação **nas duas direções**: fixture com CRLF → parseia igual à com LF; **removendo a
  normalização**, volta a duplicar o frontmatter.
- 🔴 **Controle de escrita:** o que o produto **grava** continua LF — `check-python-writes-lf.sh`
  verde, e comparação byte-a-byte da saída em POSIX antes/depois.
- 🔴 **Os 3 runtimes medidos separadamente**, porque o `open()` do Python pode mascarar o defeito.
