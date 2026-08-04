---
status: Accepted
date: 2026-08-04
author: "Zeus"
---

# ADR: make quality — força locale fixo no gate de falsificação em vez de pin em inglês

> Date: 2026-08-04 | Status: Accepted

## Context

`REQ-2026-08-04-make-quality-falha-sob-locale-pt-br-teste-fixa-literal-em-ingles-no-violations-found.md`
documenta que o Cenário 29 de `scripts/check-gates-falsify.sh` pina byte-a-byte a mensagem de sucesso do
`validate` contra o literal em inglês `"✓ No violations found.\n"` (`S29_EXPECTED`, linha ~1947), mas os
3 CLIs imprimem essa mensagem via `i18n_t("validate.ok")`, que resolve pelo locale ativo do processo.
Sob `pt_BR.UTF-8` (locale comum no time), a saída real diverge do literal pinado e o gate falha sem
regressão real ter ocorrido.

O pin byte-a-byte é intencional e necessário — existe justamente para provar que os 3 CLIs continuam
imprimindo a mesma mensagem entre si e que nenhum volta a usar um literal hardcoded divergente (histórico:
o Python ficou meses imprimindo `"✓ Governance OK"` hardcoded sem que nenhum gate detectasse). A correção
não pode remover o pin — só torná-lo independente do locale do ambiente onde o script roda.

## Decision

O script `scripts/check-gates-falsify.sh` passa a fixar `LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8` (mesmo
padrão já usado manualmente para rodar `make quality` com sucesso durante esta investigação) no ambiente
dos 3 subprocessos comparados no Cenário 29 — e em qualquer outro cenário do mesmo script que compare
saída textual dependente de i18n. Isso é feito no início do script (ou por cenário, se subprocessos
individuais precisarem de controle fino), não delegado ao ambiente externo (Makefile/CI/shell do
desenvolvedor).

Rejeitada a alternativa de ler a mensagem esperada dinamicamente do arquivo de locale ativo — ver
Alternativas Consideradas.

## Consequences

**Positivas:**
- `make quality` passa a ser determinístico independente do locale do desenvolvedor/CI, eliminando o
  falso-negativo relatado.
- Mudança mínima e localizada — não requer alterar nenhum CLI de produção, só o script de gate.
- Preserva integralmente o propósito do Cenário 29 (prova de paridade + prova de detecção de regressão).

**Negativas:**
- O gate passa a testar explicitamente o comportamento sob `en_US.UTF-8`, não sob o locale real de cada
  execução — uma regressão de i18n que só se manifestasse em outro locale (ex.: `pt_BR` corrompido)
  não seria pega por este cenário especificamente. Aceito: não é o propósito do Cenário 29 validar
  correção de tradução, e sim paridade estrutural entre os 3 CLIs.

## Alternatives Considered

- **Ler a mensagem esperada dinamicamente do arquivo de locale ativo** (ex. `internal/i18n/locales/
  <locale-ativo>.json`, chave `validate.ok`) em vez de hardcode. Rejeitada: enfraquece o próprio
  propósito do cenário — se o Go também tivesse (hipoteticamente) um bug lendo `validate.ok`, a
  expectativa "dinâmica" leria o mesmo valor incorreto e o gate passaria mesmo com o bug presente. O pin
  fixo e independente do código sob teste é o que torna o Cenário 29 uma prova de regressão válida.
- **Deixar o desenvolvedor responsável por rodar com `LANG=en_US.UTF-8`.** Rejeitada: já é o estado
  atual, e é exatamente a causa do problema relatado — depender de configuração externa ao script é
  frágil e não documentado em lugar nenhum antes desta REQ.
