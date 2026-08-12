---
status: Open
date: 2026-08-11
author: "Zeus (Arquiteto)"
adr: ""
roadmap: ""
---

# REQ: Semantica de falha de hook (fail-open vs fail-closed) por CLI — verificacao empirica do credential-guard como controle de negacao

> Date: 2026-08-11 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

O `trackfw-credential-guard.sh` é um controle de **negação**: ele bloqueia a materialização de
credenciais reais por subagentes. Um controle de negação só vale se falhar **fechado**.

A revisão de segurança do ML-8B do `ROADMAP-2026-08-11` (parecer em
`docs/seguranca/2026-08-11-revisao-hooks-cwd.md`, pergunta Q3) registrou um achado explícito, e
**não inferido**: **nenhuma fonte consultada** naquele roadmap — a pesquisa em doc primária dos 6
CLIs (`docs/pesquisa/2026-08-11-hook-cwd-e-placeholders-por-cli.md`), o ADR, ou a documentação dos
próprios fornecedores — estabelece se a **falha na execução de um hook** é tratada como *fail-open*
(a ferramenta prossegue) ou *fail-closed* (a ferramenta bloqueia), por CLI.

Isso importa porque hoje existem **três caminhos documentados** que terminam em "o guard não roda,
em silêncio" (`docs/cli-parity.md`, seção "Pré-condições do fix do Codex"):

1. execução fora de um repositório git;
2. execução dentro de submódulo/worktree, onde `git rev-parse --show-toplevel` aponta para outra raiz;
3. `GIT_DIR`/`GIT_WORK_TREE` definidas no ambiente, redirecionando a resolução da raiz.

E, além desses, o Codex ignora hooks de projeto **em silêncio** quando o projeto não está marcado
como *trusted* (ADR-2026-08-11, Emenda 1).

Se a semântica for *fail-open* em algum CLI, o guard se torna **contornável** por quem consiga
influenciar o cwd ou o ambiente do processo. Se for *fail-closed*, os três caminhos acima são
degradação de disponibilidade, não de segurança — resultado muito diferente, e hoje **não sabemos
qual é**.

> Nota de escopo histórico: o `ROADMAP-2026-08-11` **não alterou** essa semântica para nenhum CLI —
> ela já era desconhecida antes dele. Por isso o parecer classificou como follow-up e não como
> bloqueio. Esta REQ existe para que o achado não evapore junto com o fechamento daquele roadmap.

## Acceptance Criteria

- [ ] Para cada um dos 6 CLIs (Claude, Codex, Gemini, Kiro, Copilot, Cursor), determinar
      empiricamente — com o CLI real, não por leitura de doc — o comportamento quando o comando do
      hook **não existe** e quando **sai com código != 0**: a ferramenta prossegue ou bloqueia?
- [ ] Distinguir os dois casos, que podem divergir: **script ausente/caminho inválido** × **script
      presente que sai com erro**. O contrato de bloqueio conhecido (exit 2 + stderr) cobre só o
      segundo.
- [ ] Resultado registrado em `docs/cli-parity.md` com uma tabela por CLI e a evidência de cada
      célula (comando executado + saída observada).
- [ ] Para cada CLI cujo comportamento seja *fail-open*, avaliar e propor mitigação — ou registrar
      explicitamente como risco aceito, com justificativa.
- [ ] Se a verificação empírica for impraticável para algum CLI (indisponível na máquina, exige
      autenticação interativa), registrar `INDETERMINADO` com o que foi tentado — **nunca inferir
      por analogia com outro CLI**.

### Escopo negativo

- **Não** altera o conteúdo de `scripts/trackfw-credential-guard.sh`.
- **Não** altera o wiring de caminho dos hooks (assunto encerrado no `ROADMAP-2026-08-11`).
- **Não** corrige os três caminhos de "guard não roda" — eles são consequência, não causa; só faz
  sentido corrigi-los depois de saber se são problema de segurança ou de disponibilidade.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
