# Palavra-chave de fechamento do GitHub é só em inglês — e `resolve` é falso cognato que **funciona**

> 2026-09-02 · ML-1A do `ROADMAP-2026-09-02-gate-e-template-de-pr-exigem-palavra-chave-de-fechamento-em-ingles`
> Arquivos: `.github/PULL_REQUEST_TEMPLATE.md`, `scripts/check-pr-closing-keyword.sh`

## O mecanismo, em uma frase

O GitHub fecha uma issue no merge **apenas** com `close|closes|closed`, `fix|fixes|fixed`,
`resolve|resolves|resolved`. Um repositório cujos corpos de PR são escritos em português escreve
`Fecha #246.`, o merge tem sucesso, o texto **afirma** que fechou — e a issue continua aberta.
Falha silenciosa e **invertida**: o artefato se reporta saudável estando inerte.

## A medição, porque a intuição erra aqui

`gh pr list --state merged --limit 300 --json number,body,closingIssuesReferences` sobre este
repositório, 2026-09-02:

| | |
|---|---|
| PRs mergeados | 241 |
| PRs que fecharam issue automaticamente | **4** (#233, #238, #240, #245) |
| forma usada pelos 4 | `Fixes #N`, sempre |
| linhas com palavra-chave PT + `#N` na mesma linha | **43** |
| linhas com palavra-chave PT **adjacente** a `#N` | **3** |

`closingIssuesReferences` é o oráculo — é o que o GitHub realmente vinculou, não o que o texto
parece dizer. Sem ele, a leitura do texto dá o veredito errado: `Corrige o #237.` na primeira
linha dos PRs #238/#240 **parece** o defeito, mas os dois têm `Fixes #237`/`Fixes #239` no rodapé e
fecharam de verdade (delta de 1 segundo entre `mergedAt` e `closedAt`). O único defeito real no
corpus é o PR #247 (`Fecha #246.`, sem forma inglesa nenhuma — `closingIssuesReferences: []`).

## As três armadilhas ao construir o gate

**1. `Resolve #N` é falso cognato que FUNCIONA.** A grafia é idêntica em português e inglês, e o
inglês é válido no GitHub. Um gate que a recuse reprova um corpo que funciona — o pior falso
positivo possível: ele ensina a "consertar" o que estava certo. `Resolvido`/`Resolvida`/
`Resolvem`/`Resolver` são inequivocamente portugueses e não fecham nada; só a forma nua é ambígua.
O enunciado da REQ listava `Resolve #` como forma a proibir. Está errado, e foi medido.

**2. A isenção tem de ser POR NÚMERO DE ISSUE.** "Existe alguma palavra inglesa no corpo" parece
equivalente e não é: corpos deste repositório citam `Fixes #N` de **outras** issues o tempo todo.
`Fecha #246` + `Fixes #999` tem de **reprovar**; `Fecha #246` + `Fixes #246` tem de **passar**. Um
gate com a isenção global fica verde sobre o defeito real e ninguém percebe, porque o `make
quality` continua verde — a classe exata dos 4 gates vácuos medidos na auditoria do mesmo dia.
Falsificado por sabotagem no Cenário 182 de `check-gates-falsify.sh`.

**3. Adjacência é o que separa gate útil de gate desligado.** A diferença entre 43 e 3 linhas
reprovadas no mesmo corpus é só a exigência de que a palavra-chave esteja **colada** ao `#N` (com
no máximo artigo definido e/ou a palavra "issue" no meio). Prosa como `o mesmo sítio do #238`,
`portado do #223`, `Fecha o **item 4** da issue #216`, `Fecha a governança do PR #145` é normal
aqui e não é declaração de fechamento. Um gate que reprove 43 linhas é desligado na primeira
semana — e aí não guarda nada. O preço declarado é não cobrir paráfrase
(`este PR fecha, por fim, a #246`); é o preço certo.

Corolário do mesmo eixo: trechos em cerca de código e code span são removidos antes de casar,
senão **o próprio PR que documenta o defeito** reprova ao citá-lo.

## Por que é caro descobrir isso amanhã

Ninguém procura por uma issue que continuou aberta — procura-se por um erro, e aqui não há erro. O
único sinal é alguém reparar, dias depois, que a issue está aberta com o PR mergeado. O reflexo
seguinte é pior: sem a explicação escrita, o próximo revisor "conserta" a linha `Closes #` do
template para português, achando que corrige uma inconsistência de idioma, e o defeito volta
idêntico. Por isso o comentário do template é maior que o template.

## Referências

- `.github/PULL_REQUEST_TEMPLATE.md` (o porquê, escrito onde o tradutor vai olhar)
- `scripts/check-pr-closing-keyword.sh` (formas aceitas/recusadas declaradas no cabeçalho)
- `scripts/check-gates-falsify.sh`, Cenário 182 (isenção por número, falsificada por sabotagem)
- `docs/cli-parity.md`, seção "Palavra-chave de fechamento de issue no corpo do PR" (`partial=`)
- `vault/notes/gate-literal-regex-syntax-equivalent-bypass-2026-09-01.md` (por que `gate=` puro
  seria alegação forte demais para um casador de texto)
