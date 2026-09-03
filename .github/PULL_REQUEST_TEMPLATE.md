<!--
  🔴 NÃO TRADUZA A LINHA `Closes #` ABAIXO. LEIA O PORQUÊ ANTES DE MEXER.

  O corpo do PR é escrito em PORTUGUÊS — isso está certo e não muda.
  A linha de fechamento é a ÚNICA exceção, porque ela não é prosa: é
  SINTAXE DO GITHUB, interpretada por uma máquina que só entende inglês.

  O GitHub fecha uma issue no merge apenas quando o corpo contém uma destas
  palavras-chave, seguida da referência da issue:

      close · closes · closed
      fix   · fixes  · fixed
      resolve · resolves · resolved

  "Fecha #123", "Corrige o #123", "Encerra #123" NÃO fecham nada. O merge tem
  sucesso, o texto AFIRMA que fechou, e a issue continua aberta — falha
  silenciosa e invertida.

  MEDIDO NESTE REPOSITÓRIO (2026-09-02): dos 241 PRs mergeados, apenas 4
  fecharam issue automaticamente. O PR #247 abriu com "Fecha #246." e a issue
  #246 ficou aberta até alguém reparar e fechar à mão.

  Se você "consertar" esta linha para português, o defeito volta idêntico.
  O gate `scripts/check-pr-closing-keyword.sh` existe para pegar isso.
-->

## O que muda


## Por quê


## Como verificar


<!-- Sintaxe do GitHub (inglês obrigatório) — troque o número, não a palavra. -->
Closes #

<!-- Sem issue associada? Apague a linha `Closes #` inteira. -->
