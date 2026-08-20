---
name: retomar-execucao-parcial
description: Quando um handoff diz "trabalho parcial já em disco, retomar" — confiar no que o usuário já verificou e focar só no delta listado, não reauditar tudo
metadata:
  type: feedback
---

Quando KG entrega um handoff de retomada de sessão interrompida ("já feito e verificado por mim —
não mexer" + "o que falta — exatamente N coisas"), o protocolo certo é: verificar rapidamente que o
já-feito ainda está no estado descrito (grep/leitura pontual), e então gastar o esforço real só nos
itens explicitamente listados como pendentes.

**Why:** KG já fez a auditoria do trabalho anterior antes de escrever o handoff (ex.: já confirmou
"zero ocorrências de stash", já rodou o gate isolado e viu exit 0). Reauditar tudo do zero
desperdiça o orçamento da sessão e é redundante com o que ele já fez. O valor que o executor
seguinte agrega está no delta, não em repetir verificação já feita.

**How to apply:** Ler o handoff com atenção a duas listas separadas — "não mexer" e "exatamente N
coisas". Rodar uma verificação leve (grep, leitura de 1 arquivo) para confirmar que o "não mexer"
ainda bate, e então implementar/verificar só a lista de pendências. Se um item pendente inclui "isto
nunca foi executado" (ex.: cenário de gate escrito mas nunca rodado), tratar como risco real — pode
falhar de primeira e exigir conserto — mas não expandir escopo para revisar cenários vizinhos que já
passaram antes.

Confirmado nesta sessão (ROADMAP-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md,
ML-2B): handoff listava 3 pendências exatas (contagem de cenários 137→136, rodar Cenário 75 nunca
executado, seção do cli-parity.md nunca tocada); as três foram fechadas sem reabrir o que já estava
correto, e o Cenário 75 passou de primeira — confirmando que a auditoria prévia do usuário era
confiável.
