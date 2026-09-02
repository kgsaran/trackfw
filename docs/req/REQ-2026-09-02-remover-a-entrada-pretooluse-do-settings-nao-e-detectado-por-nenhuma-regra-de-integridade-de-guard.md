---
status: Open
date: 2026-09-02
author: "zeus-tf"
adr: ""
roadmap: ""
---

# REQ: Remover a entrada `PreToolUse` do settings não é detectado por nenhuma regra de integridade de guard

> Date: 2026-09-02 | Status: Open

## Motivation

Achado do `hefesto-tf` ao documentar a camada de anti-adulteração, **medido, não inferido** — e
**demonstrado ao vivo por um acidente na mesma sessão**.

As cinco regras de integridade de guard cobrem o **script** e o **modo**, mas não a **fiação**:

| ação do usuário | detectado? |
|---|---|
| apagar o script do guard, com a referência ainda no config | ✅ **violação**, em modo estrito |
| alterar o conteúdo do script | ✅ aviso (`*_script_integrity`) |
| rebaixar o modo de `block` para `warn` | ✅ violação (`credential_guard_mode_downgrade`) |
| **remover a entrada `hooks.PreToolUse` inteira do config** | ❌ **nenhuma das 5 regras, em nenhum modo** |

**O caminho mais fácil de burlar o guard é o único não coberto.** Apagar o script deixa rastro;
remover a fiação deixa um arquivo JSON válido, menor, e silencioso.

## Não é hipótese — aconteceu nesta sessão, por acidente

Durante a redação da própria seção que documenta esta camada, um subagente **esvaziou o
`PreToolUse` do `.claude/settings.json` deste repositório** enquanto manipulava cópias para uma
fixture. Os dois matchers — `AskUserQuestion` e `Bash` (o git-branch-guard) — desapareceram.

**Nenhuma regra acusou.** O dano só foi notado porque o agente percebeu, tentou reparar e **parou
para reportar** em vez de contornar.

Dois detalhes do incidente que informam o desenho da correção:

1. **O guard de escopo global continuou vivo** e bloqueou a tentativa de reparo por
   `git checkout --`. **As duas camadas se comportam de forma independente**, e a de projeto é a que
   some sem aviso.
2. **A responsabilidade foi do arquiteto**, não do agente: o handoff mandou "mexer numa cópia em
   `/tmp`" sem **restringir a escrita ao diretório de scratch**. Fica registrado porque o remédio de
   processo é diferente do remédio de produto.

## Por que importa além deste repositório

O `trackfw` **instala** esses guards em quem o adota. Se a fiação removida não é detectada, então
**todo projeto que adota o produto tem o mesmo ponto cego** — e ele é justamente o de menor esforço
para quem quer se livrar do controle.

E compõe com duas coisas já medidas: o `credential_guard` **nasce em `warn`**
(`scripts/trackfw-credential-guard.sh:119`), e neste repositório o `core.hooksPath` está em
`/dev/null` com o `.git/hooks/` vazio. **As regras de integridade existem, e a camada que elas
verificam segue desligada.**

## Acceptance Criteria

- [ ] **AC1** — Remover a entrada `hooks.PreToolUse` do config é **detectado**, com severidade
      coerente com a das regras irmãs.
- [ ] **AC2** — 🔴 **Detectar a ausência exige saber o que deveria existir.** A regra precisa de uma
      referência de "fiação esperada" — e essa referência **não pode ser o próprio config**, senão
      ela some junto. Decidir e justificar: âncora no HEAD do git (como as 3 regras de
      `credential_guard` já fazem), manifesto de instalação, ou outra. **É a decisão que separa
      controle de teatro.**
- [ ] **AC3** — 🔴 **Falsificação nas duas direções.** (a) config com a fiação removida → detectado;
      (b) **controle:** projeto que **nunca instalou** o guard **não** é acusado. Sem (b),
      transformaríamos "não instalado" em "adulterado" e o aviso viraria ruído em todo repositório
      novo.
- [ ] **AC4** — Distinguir **"nunca instalado"** de **"instalado e removido"**. São fatos
      diferentes com remédios diferentes — um é `trackfw update harness`, o outro é investigação.
- [ ] **AC5** — 🔴 **A assimetria do 6.3 endereçada ou declarada.** O `hefesto-tf` achou, lendo
      `internal/validator/validator_credential_guard_integrity.go:196-200`, que a âncora no HEAD e a
      isenção por baseline valem **só** para as 3 regras de `credential_guard`; as 2 de
      `git_branch_guard` **não são ancoradas** e **podem ser toleradas por baseline**. **Nada no
      repositório documenta isso como deliberado.** Se for, registrar; se não for, corrigir.
- [ ] **AC6** — Paridade nos 3 CLIs.
- [ ] **AC7** — `make quality` e **CI** verdes.

## Negative Scope

- **Não** tratar o `core.hooksPath = /dev/null` — o `doctor --remote` já o acusa
  (`hooks-path-neutralized`), e ligar hooks de git para humanos é a Wave pendente da
  `REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw`.
- **Não** mudar o padrão `warn` do `credential_guard` — merece decisão própria, com ADR.
- **Não** prometer proteção contra adversário com permissão de commit. O `ADR-2026-08-12` já declara
  esse limite; esta REQ trata **remoção silenciosa**, não adversário determinado.

## Linked ADR

ADR: <!-- avaliar na análise: se a AC2 concluir que a referência de fiação esperada precisa de
mecanismo novo, isso é decisão arquitetural. -->

## Linked Roadmap

Roadmap:
