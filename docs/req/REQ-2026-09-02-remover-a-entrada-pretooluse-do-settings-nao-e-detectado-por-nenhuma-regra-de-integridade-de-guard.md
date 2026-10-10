---
status: Open
date: 2026-09-02
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-10-a-fiacao-do-guard-e-ancorada-no-origin-main-por-tupla-e-a-remocao-e-violacao-fora-do-lenient-e-do-baseline.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-22-remover-a-entrada-pretooluse-do-settings-nao-e-detectado-por-nenhuma-regra-de-integridade-de-guard.md"
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

**Remedido em 2026-10-10** (worktree de `origin/main` `39ad84de`, com o harness global desta máquina
e com `HOME` vazio — a tabela acima é de 2026-09-02, quando o guard ainda era script):

| `.claude/settings.json` | `trackfw validate` |
|---|---|
| intacto | 169 warnings |
| `hooks.PreToolUse` apagado | 169 warnings — saída **idêntica byte a byte** |
| `trackfw guard git-branch`/`credential` trocados por `true` | 169 warnings — **idêntica** |

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

- [x] **AC0** — 🔴 **Wave 0 (`hades-tf`):** threat model do ADR ligado, com parecer em
      `docs/seguranca/`. Inclui completude da população de arquivos de hook (D3), contornos que não
      tocam o arquivo rastreado (`settings.local.json`, chaves de desligamento de hooks de cada CLI,
      verificadas na documentação) e o casamento do comando do guard.
      ✅ Evidência: `docs/seguranca/2026-10-10-wave0-fiacao-do-guard-ancorada.md` — aprovado com ajustes A1–A4; incorporados ao ADR (A1 divergido: D7)
- [ ] **AC1** — Remover a fiação de um guard é **detectado** — chave apagada, matcher apagado, matcher
      estreitado, comando neutralizado e chave de desligamento de hooks (D7) — como **violação mesmo em lenient e com baseline**
      (D2, D5 do ADR).
- [ ] **AC2** — 🔴 **Detectar a ausência exige saber o que deveria existir.** A regra precisa de uma
      referência de "fiação esperada" — e essa referência **não pode ser o próprio config**, senão
      ela some junto. Decidida no ADR ligado (D1): cópia do arquivo de hook em
      `origin/main`, pelo discriminante de 4 estados já existente — o HEAD é derrotado por um
      `git commit`. **É a decisão que separa controle de teatro.**
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
- [ ] **AC6** — Contrato da regra em `docs/cli-parity.md` com a anotação `trackfw-contract` (v8: o
      Go é a implementação única; não há paridade entre runtimes a provar).
- [ ] **AC7** — `make quality` EXIT=0 (máquina ociosa) e **CI** verde, inclusive `windows-full-suites`.
- [ ] **AC8** — Cada teste novo declara a conclusão que afirma e reprova sem a correção.
- [ ] **AC9** — Medição de volta: os três braços da tabela de 2026-10-10 repetidos com o binário da
      branch — os dois braços adulterados acusam; a base não.

## Negative Scope

- **Não** tratar o `core.hooksPath = /dev/null` — o `doctor --remote` já o acusa
  (`hooks-path-neutralized`), e ligar hooks de git para humanos é a Wave pendente da
  `REQ-2026-09-01-o-repositorio-do-trackfw-nao-esta-sob-os-cuidados-do-trackfw`.
- **Não** mudar o padrão `warn` do `credential_guard` — merece decisão própria, com ADR.
- **Não** cobrir o escopo global (`~/.claude/settings.json` etc.) — fora do git, sem âncora;
  resíduo declarado no ADR.
- **Não** cobrir o hook `AskUserQuestion` (attention-signal) — não é guard.
- **Não** prometer proteção contra adversário com permissão de commit. O `ADR-2026-08-12` já declara
  esse limite; esta REQ trata **remoção silenciosa**, não adversário determinado.

## Linked ADR

ADR: docs/adr/ADR-2026-10-10-a-fiacao-do-guard-e-ancorada-no-origin-main-por-tupla-e-a-remocao-e-violacao-fora-do-lenient-e-do-baseline.md

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-09-22-remover-a-entrada-pretooluse-do-settings-nao-e-detectado-por-nenhuma-regra-de-integridade-de-guard.md`
