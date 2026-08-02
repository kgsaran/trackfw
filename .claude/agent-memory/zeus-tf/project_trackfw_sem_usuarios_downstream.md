---
name: trackfw-sem-usuarios-downstream
description: O trackfw ainda não tem usuários externos — compatibilidade retroativa não é restrição de peso nas decisões
metadata:
  type: project
---

O trackfw **ainda não tem ninguém usando** além do próprio repositório. KG deixou isso explícito
em 2026-08-02, ao rejeitar meu enquadramento de "breaking change" para unificar o comando `status`.

**Why:** eu vinha tratando compatibilidade downstream como restrição dura e deixando defeitos de
pé por causa dela. Exemplos reais desta sessão em que argumentei "não fazer para não quebrar
projetos downstream":

- manter o nome `blocked_by_draft_adr`, historicamente impreciso, por ser "chave pública de
  configuração"
- não unificar os mecanismos de strip dos 3 extratores
- manter `Draft` e `Proposed` como estados separados

Nenhum desses argumentos tem o peso que eu dei.

**How to apply:** ao pesar trade-offs neste projeto, o custo de churn é **interno** — testes,
fixtures, os 3 CLIs, documentação. Não há migração de usuário a proteger. Isso favorece
**corrigir na origem** em vez de contornar, e favorece renomear/unificar quando o nome ou o
mecanismo está errado.

Não confundir com licença para mudança gratuita: o custo de manter os 3 CLIs em paridade e de
reescrever fixtures continua real. O que muda é que "quebraria quem já usa" deixa de ser
argumento — porque não há quem.

Vale revisitar as decisões acima se o assunto voltar. Relacionado:
[[trackfw-dashboard-light-only]]
