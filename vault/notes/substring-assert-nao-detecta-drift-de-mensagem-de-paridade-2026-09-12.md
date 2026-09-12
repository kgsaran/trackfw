# substring assert não detecta drift de mensagem de paridade — 2026-09-12

## Contexto

ML-2E (REQ fix/by-agent-req-new-e-roadmap-new) alinhou a mensagem de ambiguidade de agente nos
3 runtimes. A mensagem existia desde a Wave 1 e não era byte-idêntica:

| runtime | mensagem anterior |
|---------|------------------|
| Go      | `by_agent project has multiple agent namespaces (alpha, beta): use --agent to specify one` |
| Node    | `--agent is required when multiple namespaces are configured: alpha, beta` |
| Python  | `multiple agent namespaces declared (alpha, beta): use --agent to specify one` |

## Como passou pela Wave 1

O arquiteto auditou com `grep -o "alpha, beta"` — que casa nos três. Concluiu que "os nomes aparecem"
e declarou paridade. As suítes de teste usavam `msg.includes('alpha')` e `"alpha" in msg`, que
também passam com mensagens completamente diferentes.

**Dois erros distintos, compondo:**

1. **Grep de substring não é diff de mensagem** — presença de nomes não implica texto idêntico.
2. **Assertions de substring nas suítes** — passam no verde mesmo com mensagens divergentes.

## Causa raiz

Nenhum teste em nenhum dos três runtimes afirmava a mensagem inteira como string exata. A barreira
que teria reprovado — `diff` entre saídas dos binários — não foi executada.

## Correção

- Mensagem alinhada ao Go (canônica) nos três runtimes.
- Testes existentes convertidos de substring para `strictEqual` / `==` (igualdade exata).
- Dois novos testes Go adicionados em `validator_namespacing_test.go` com igualdade exata.
- Prova binária: `diff` entre stdout/stderr dos três binários com duas fixtures → resultado vazio.
- Falsificação: mudar um caractere em um runtime reprova a asserção (colado no relatório do ML).

## Mensagens desta família com divergência residual (não corrigidas — escopo de outra REQ)

| família | Go | Node | Python |
|---------|-----|------|--------|
| `is not a regular file` | `"not a regular file"` (sem path) | `"${filePath} is not a regular file"` | `"{path} is not a regular file (mode ...)"` |
| `ler baseline` | `"erro ao ler baseline: %w"` (lowercase) | `"Erro ao ler baseline: ..."` | `"Erro ao ler baseline: ..."` |

## Regra geral

> **Substring check é necessário mas não suficiente para paridade.** O teste de paridade de mensagem
> deve afirmar a string completa — `strictEqual` / `==` / `t.Errorf` com full string. A prova de paridade
> cross-runtime deve ser um `diff` de saída de binários, nunca um grep da substring comum.
