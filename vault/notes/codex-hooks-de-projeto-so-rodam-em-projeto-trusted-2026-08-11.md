# Codex CLI: hooks de projeto só rodam se o projeto for "trusted"

> 2026-08-11 · descoberto durante o ML-3A do
> `ROADMAP-2026-08-11-resolucao-de-caminho-dos-hooks-de-agente-independente-do-cwd`

## Sintoma

Você gera `.codex/hooks.json` com `trackfw init`/`trackfw update`, o arquivo está correto, o script
existe e é executável — e **o hook simplesmente não dispara**. Nenhum erro óbvio. Fácil concluir
"o trackfw gerou errado" e sair caçando bug no gerador, que é onde se perde tempo.

## Causa raiz

O Codex CLI **só carrega hooks de projeto se aquele projeto estiver marcado como confiável** em
`~/.codex/config.toml`:

```toml
[projects."/caminho/absoluto/do/projeto"]
trust_level = "trusted"
```

Sem isso, os hooks do repositório são ignorados em silêncio. É uma decisão de segurança do Codex
(um `.codex/hooks.json` versionado é código executável vindo do repositório), não um defeito.

Para testes automatizados existe a flag documentada `--dangerously-bypass-hook-trust`.

## Como confirmar rapidamente

Rode o `codex` num repo de fixture, dispare o evento, e veja se o hook executou. Se não executou,
teste de novo com `--dangerously-bypass-hook-trust`: se aí funcionar, o problema é trust, não o
arquivo gerado.

## Por que isso importa para o trackfw

Não muda nada no que o trackfw **gera** — só a condição para o que ele gera **surtir efeito na
máquina do usuário**. Consequências práticas:

- Ao validar empiricamente qualquer mudança no wiring de hooks do Codex, **use um `$HOME` isolado**
  e a flag de bypass; sem isso o teste dá falso negativo e você conclui que a mudança não funciona.
- Nunca escreva no `~/.codex/config.toml` real do usuário para "fazer o teste passar". No ML-3A o
  sandbox bloqueou essa escrita e o comportamento correto foi **não** contornar.
- Vale uma linha em `docs/cli-parity.md`: o fix de caminho do Codex
  (`"$(git rev-parse --show-toplevel)/..."`) só produz efeito para o usuário final em projeto
  trusted.

## Relacionado

- `docs/adr/ADR-2026-08-11-resolucao-de-caminho-dos-hooks-de-projeto-por-cli-mecanismo-especifico-do-fornecedor-sem-caminho-absoluto.md`
- `docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md` — a doc primária do Codex **não**
  menciona esse pré-requisito na página de hooks; foi descoberto empiricamente.
- `vault/notes/check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08.md` — mesma família
  de armadilha: estado global do `$HOME` contaminando verificação de hooks.
