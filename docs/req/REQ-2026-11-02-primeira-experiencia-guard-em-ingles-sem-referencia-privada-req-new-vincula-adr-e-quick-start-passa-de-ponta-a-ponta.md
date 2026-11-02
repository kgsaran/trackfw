---
status: Open
date: 2026-11-02
author: "Zeus (trackfw_architect)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-02-primeira-experiencia-guard-em-ingles-sem-referencia-privada-req-new-vincula-adr-e-quick-start-passa-de-ponta-a-ponta.md"
---

# REQ: Primeira experiência: guard em inglês sem referência privada, req new vincula ADR e Quick start passa de ponta a ponta

> Date: 2026-11-02 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Esta REQ pertence à direção de adoção decidida em 2026-10-28: o gargalo é ser descoberto, não a
técnica. Medido em 2026-11-02: 7 visitantes únicos na página do repositório em 14 dias, 6 stars e um
`demo.gif` de junho que mostra o `validate` despejando o `Usage:` do cobra.

O arquiteto seguiu o README como um usuário novo, num repositório limpo com o binário 11.2.5
compilado da `main`, e achou dois defeitos que derrubam a primeira experiência.

### Defeito 1 — a mensagem de bloqueio do guard sai em português e cita uma seção que não existe

`echo '{"tool_name":"Bash","tool_input":{"command":"git push origin main"}}' | trackfw guard git-branch`
sai com código 2 e emite:

```
trackfw: git push bruto bloqueado. Use `trackfw push` (...) Nada antes deste comando foi executado
(comando composto é bloqueado por inteiro). Ver CLAUDE.md §1.
```

- O resto do CLI é em inglês.
- O `CLAUDE.md` gerado pelo `trackfw init` não tem uma §1. A referência é da convenção pessoal do
  mantenedor e vazou para o produto.
- São cerca de 18 constantes `reason*` em `internal/guard/gitbranch.go`.
- Esse bloqueio é o momento de maior impacto do produto: é ali que o agente é barrado.

### Defeito 2 — o Quick start do README falha no passo 5

Seguindo os comandos à risca (`init --ai-tools claude --forge github`, `adr new`, `req new` e
`roadmap move <nome> wip`), o `trackfw validate` responde:

```
✗ req "REQ-…-user-authentication.md" has no linked ADR (...)
Error: 1 violation(s) found
```

O `req new` não vincula o ADR recém-criado e não tem um flag para isso
(`trackfw req new --help`: só `--agent` e `--no-roadmap`).

## Acceptance Criteria

- [x] **AC1 — Guard em inglês.** Todas as mensagens de bloqueio emitidas pelos guards (`git-branch`,
  `credential` e `write-scope`), no stderr e no `permissionDecisionReason`, saem em inglês.
  - Elas mantêm a mesma orientação: qual comando do trackfw usar e que nada antes do comando foi
    executado.
  - A lista completa de constantes e de sítios é entregável da Wave 0, por grep de mecanismo.
- [x] **AC2 — Nenhuma referência privada.** Nenhuma mensagem emitida pelo binário cita `CLAUDE.md §1`
  ou outra seção que o `trackfw init` não gera. Um teste varre as constantes e reprova se reaparecer
  `§` ou texto em português.
- [x] **AC3 — `req new --adr <ADR>`.**
  - O flag aceita o nome do arquivo do ADR, com ou sem `.md`, e o resolve pelo ponto único de
    enumeração (`ResolveADRFiles`).
  - Ele grava o vínculo nos dois lugares que a regra `req_has_adr` lê: o `adr:` do frontmatter e a
    linha `ADR:`.
  - ADR inexistente ou ambíguo dá erro, com saída diferente de zero e sem arquivo criado.
  - Sem o flag, o comportamento atual não muda.
- [x] **AC3b — `req new` não reescreve uma REQ existente** (T6 da Wave 0, medido em 2026-11-02: hoje a
  segunda execução com o mesmo título reescreve a REQ do zero, apaga as edições e sai 0).
  - Com a REQ já existente, o arquivo fica intacto byte a byte. O comando avisa que a REQ foi mantida e
    **continua** o reparo idempotente do vínculo com o roadmap (contrato do ML-4A,
    `req_chain_ml4a_test.go`), saindo 0.
  - Se vier `--adr` e a REQ existente não tiver esse ADR vinculado, o comando recusa, sai diferente de
    zero e não altera nada: vincular exigiria editar o arquivo.
  - Decisão do arquiteto em 2026-11-02, na auditoria do ML-1B. As alternativas eram exigir a remoção
    da REQ antes de rodar de novo, o que ensina a perder dado, ou recusar só no CLI, o que deixa a API
    com o defeito.
- [x] **AC4 — Quick start de ponta a ponta.** Um teste Go num `t.TempDir()` roda a sequência do Quick
  start do README, já com `--adr`, e o `trackfw validate` termina com **0 violações**. O Quick start do
  README e esse teste trazem a mesma sequência de comandos.
- [x] **AC4b — O padrão do assistente de identidade é o anunciado.** Achado pelo arquiteto ao gravar o GIF,
  em 2026-11-02, num terminal real. O `trackfw init` interativo abre o seletor de identidade com o cursor em
  "Panteão grego", embora a opção rotulada "(padrão)" seja "Nomes neutros". Isso acontece porque
  `identitySelect` começa vazio em `internal/commands/identity_wizard.go` e o `huh` cai na primeira opção.
  Um Enter aceita o preset grego e abre a pergunta de apelido. O cursor deve abrir em `neutral`, com teste.
- [x] **AC5 — Primeira tela do README.** Três frases do que o produto faz, o Quick start que passa
  pelo AC4 e o GIF novo logo abaixo. A ressalva do Windows vira uma linha com link para a seção
  completa.
- [x] **AC6 — GIF novo.**
  - `demo.tape` é reescrito para o fluxo do AC4 e mostra o guard barrando um `git push` na `main`, com
    a mensagem do AC1.
  - A fonte é instalada e nomeada corretamente (`JetBrainsMono Nerd Font`), sem o espaçamento quebrado
    do GIF atual.
  - Não aparece o `Usage:` do cobra depois de erro, nem caminho absoluto da máquina do mantenedor.
  - A verificação é visual, feita pelo arquiteto a partir de quadros extraídos.

## Negative scope

- Não muda a semântica de nenhum guard: o que é bloqueado ou permitido e os códigos de saída
  continuam iguais. Só o texto muda.
- Não internacionaliza o CLI. Não há seleção de idioma; o produto é em inglês.
- Não muda a regra `req_has_adr` nem a severidade dela.
- Não altera o assistente interativo do `req new` além de aceitar o flag.
- Não reescreve o README além da primeira tela e do Quick start.
- Não inclui listas *awesome*, posts nem o repositório de exemplo. São itens 2 a 4 do plano de
  visibilidade e ficam para depois.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-02-primeira-experiencia-guard-em-ingles-sem-referencia-privada-req-new-vincula-adr-e-quick-start-passa-de-ponta-a-ponta.md
