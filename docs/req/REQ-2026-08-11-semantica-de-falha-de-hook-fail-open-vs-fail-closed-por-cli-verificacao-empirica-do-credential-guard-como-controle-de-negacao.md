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

> **Escopo decidido por KG em 2026-08-12** (a redação original pedia verificação empírica dos 6
> CLIs; o roadmap nasceria falhando o próprio critério). O foco vai para onde o risco concreto está:
> os **três** caminhos documentados que terminam em "guard não roda em silêncio" são **todos
> específicos do Codex** (contextos de resolução do `git rev-parse`). Claude e Gemini degradam para
> `/scripts/…` — **fail-to-run**, não fail-to-wrong-script, e ninguém sem privilégio planta arquivo
> na raiz do sistema. Os demais CLIs entram como varredura documental, não empírica.

### Núcleo — Codex CLI (empírico, bloqueante)

- [ ] Determinar empiricamente, com `codex-cli` real, o comportamento quando o comando de um hook
      `PreToolUse` **não existe / caminho inválido**: o Codex prossegue com a ferramenta
      (**fail-open**) ou bloqueia (**fail-closed**)?
- [ ] Determinar o mesmo para **script presente que sai com código != 0**. Os dois casos podem
      divergir, e o contrato de bloqueio conhecido (exit 2 + stderr) cobre **apenas o segundo** — o
      primeiro é o que os três caminhos documentados produzem.
- [ ] Reaproveitar o método já validado no ML-3A do `ROADMAP-2026-08-11`: repo de fixture, `$HOME`
      isolado, `--dangerously-bypass-hook-trust`. **Nunca** escrever no `~/.codex/config.toml` real.
- [ ] Se o resultado for **fail-open**, avaliar e propor mitigação — ou registrar como risco aceito
      com justificativa explícita. Se for **fail-closed**, os três caminhos documentados passam a
      ser degradação de **disponibilidade**, não de segurança, e isso deve ficar escrito.

### Varredura — demais CLIs (documental)

- [ ] Claude, Gemini, Cursor, Copilot e Kiro: registrar o que a **documentação primária** diz sobre
      falha de hook, com URL + citação. Onde não disser, `INDETERMINADO` com o que foi procurado —
      **nunca inferir por analogia com outro CLI**.
- [ ] Kiro é `INDETERMINADO` por construção nesta REQ: não está instalado nesta máquina e a doc de
      hooks já se mostrou omissa quanto a semântica de execução (`ROADMAP-2026-08-11`, ML-0A).
- [ ] Registrar, para Claude e Gemini, o raciocínio de por que a verificação empírica **não** foi
      considerada necessária (degradação fail-to-run) — para que a decisão seja reavaliável, e não
      pareça omissão.

### Comum

- [ ] Resultado em `docs/cli-parity.md`, tabela por CLI, com a evidência de cada célula (comando
      executado + saída observada, no caso empírico; URL + citação, no documental).
- [ ] Se a verificação empírica do Codex se mostrar impraticável após tentativa real, registrar
      `INDETERMINADO` com o que foi tentado — resultado legítimo, não falha.

### Escopo negativo

- **Não** altera o conteúdo de `scripts/trackfw-credential-guard.sh`.
- **Não** altera o wiring de caminho dos hooks (encerrado no `ROADMAP-2026-08-11`).
- **Não** corrige os três caminhos de "guard não roda" — são consequência, não causa; só faz sentido
  corrigi-los depois de saber se são problema de segurança ou de disponibilidade.
- **Não** faz verificação empírica em Claude, Gemini, Cursor, Copilot ou Kiro.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
