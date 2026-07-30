---
status: Done
date: 2026-07-30
author: "trackfw_architect"
adr: "docs/adr/ADR-001-trackfw-como-trilho-de-governanca-para-agentes-ia.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-30-reservar-v-para-verbose-e-remover-atalho-de-versao-no-go.md"
---

# REQ: reservar -v para verbose e remover atalho de versao no Go

> Date: 2026-07-30 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

A flag curta `-v` é aceita **apenas pelo CLI Go**, como atalho de `--version`. Medido na `v5.0.0`:

| Runtime | `trackfw -v` | Exit |
|---|---|---|
| Go | `trackfw 5.0.0` | 0 |
| Node.js | `error: unknown option '-v'` | 1 |
| Python | `usage: trackfw [-h] [--version] COMMAND ...` | 2 |

Não é divergência de formato — essa foi fechada na
`REQ-2026-07-30-padrao-unico-de-saida-de-versao-nos-tres-clis`. É divergência de **quais flags o CLI
aceita**: um script com `trackfw -v` funciona numa instalação e quebra noutra, conforme o runtime que o
usuário instalou.

A origem é o cobra: com o campo `Version` preenchido (`internal/commands/root.go:22`), o
`InitDefaultVersionFlag` registra `--version` **com shorthand `v`** quando esse atalho ainda está livre.
Ninguém decidiu expor `-v`; ele apareceu por default do framework.

## Decisão: remover, e reservar `-v` para verbose

`-v` **deixa de ser atalho de `--version`** no Go, e passa a ser **reservado** para futuro modo verboso
nos três runtimes. Nenhum runtime pode vinculá-lo a outra semântica.

Razão: em boa parte do ecossistema — `docker`, `kubectl`, `ansible`, `ssh`, `curl` — `-v` significa
**verbose**, não *version*. Verificado: **nenhum dos três CLIs tem `--verbose` hoje**. Manter `-v` como
atalho de versão queimaria o atalho permanentemente; no dia em que o trackfw precisar de saída verbosa
— e um CLI que executa gates, barriers e validações é forte candidato — liberá-lo seria outro breaking
change.

Paridade por **subtração**: o `--version` por extenso já cobre o caso de uso nos três, e o ganho
ergonômico do atalho é pequeno diante do custo de ocupar `-v` para sempre.

## Escopo desta REQ: reserva, não implementação

Esta REQ **não implementa** o modo verboso. Reserva o atalho e remove o vínculo indevido.

Descartado explicitamente: fazer os três **aceitarem** `-v` como no-op para "garantir a reserva na
prática". Um `-v` aceito sem efeito é pior que `unknown option` — o usuário passa a flag, espera saída
verbosa e recebe silêncio sem erro, sem conseguir distinguir "reservado" de "quebrado". A reserva é de
**contrato**, não de superfície.

A semântica de verbose exige decidir o que fica verboso por comando e qual o formato dessa saída.
Projetar isso sem caso de uso concreto é projetar no escuro — merece REQ própria, quando houver a
necessidade.

## Fronteira de paridade — verificada, e deliberadamente não forçada

Após a remoção, os três **rejeitam** `-v`, mas com mensagens e exit codes diferentes, porque são
gerados pelos frameworks. Baseline medido com uma flag desconhecida qualquer (`--zzz`):

| Runtime | Mensagem | Exit |
|---|---|---|
| Go (cobra) | `Error: unknown flag: --zzz` | 1 |
| Node.js (commander) | `error: unknown option '--zzz'` | 1 |
| Python (argparse) | `trackfw: error: unrecognized arguments: --zzz` | 2 |

Essa divergência é **pré-existente e vale para toda flag desconhecida**, não só para `-v`. O exit 2 do
argparse é convenção POSIX para erro de uso.

Portanto o contrato exige apenas que `-v` **não seja vinculado** e **saia com código não-zero** nos
três. **Não** exige mensagem byte-idêntica nem exit code idêntico: forçar isso significaria sobrescrever
o tratamento de erro de cobra, commander e argparse globalmente — mudança de escopo muito maior, que
afeta todo comando e toda flag, e que merece REQ própria se um dia for desejada.

Registrar esta fronteira importa: sem ela, um implementador tentaria alcançar identidade byte-a-byte,
falharia, e provavelmente recorreria a um hack no tratamento de erro de um dos frameworks.

## Acceptance Criteria

- [x] `trackfw -v` **não** imprime a versão em runtime nenhum.
- [x] `trackfw -v` sai com código **não-zero** nos três runtimes.
- [x] `trackfw --version` e `trackfw version` permanecem **inalterados** — `trackfw <semver>`,
      byte-idênticos nos três, conforme já pinado em `## Version output`.
- [x] `docs/cli-parity.md` registra `-v` / `--verbose` como **reservado**, com a proibição explícita de
      vinculá-lo a qualquer outra semântica e o motivo.
- [x] Gate de paridade cobre o comportamento de `-v` nos três, com prova de falsificação que reintroduza
      o atalho e demonstre reprovação.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

## Escopo negativo

- **Não** implementar modo verboso.
- **Não** fazer nenhum runtime aceitar `-v` como no-op.
- **Não** unificar mensagem ou exit code de flag desconhecida — divergência pré-existente de framework,
  aplicável a todas as flags.
- **Não** alterar `--version` nem o subcomando `version`.

## Impacto observável

**Breaking change:** `trackfw -v` deixa de funcionar no CLI Go. Scripts que o usem devem passar a usar
`trackfw --version` ou `trackfw version`, que funcionam nos três runtimes desde a `v5.0.0`.

Deve constar no CHANGELOG do próximo release.

## Linked ADR
ADR: `docs/adr/ADR-001-trackfw-como-trilho-de-governanca-para-agentes-ia.md`

Não altera decisão arquitetural; aplica a regra dura de paridade a uma superfície exposta por default de
framework, e registra uma reserva de nomenclatura para evitar decisão implícita no futuro.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-07-30-reservar-v-para-verbose-e-remover-atalho-de-versao-no-go.md`
