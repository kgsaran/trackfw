---
status: Accepted
date: 2026-11-21
author: "zeus-tf (decisão do KG em 2026-11-21)"
---

# ADR: agents e skills install com scope global não escrevem no projeto

> Date: 2026-11-21 | Status: Accepted

## Context

`trackfw agents install` e `trackfw skills install` aceitam `--scope project|global`, e `global` é o default
(`internal/commands/integrations_flags.go:364-365`). Com qualquer escopo, depois de instalar o catálogo, o comando chama
sem olhar o escopo (`integrations_flags.go:277-296`):

- `generators.InjectRulesForTool(target, manager.ProjectRoot)`, que escreve `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`,
  `.github/copilot-instructions.md`, `.windsurfrules` e os demais arquivos de regras;
- `generators.InjectHooksForTools(opts.targets, manager.ProjectRoot)`, que escreve os hooks de guard do projeto;
- o aviso D8 da ADR-2026-11-18 (`ValidateAntigravityIDEGlobalGuardWarningsForDoctor`), que fala de "Antigravity
  instalado no projeto".

O bloco foi escrito para restaurar o comportamento dos aliases removidos (ML-5E do
ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador) e para dar hooks ao install (ML-1B da
REQ-2026-10-07). Nenhum dos dois discutiu o escopo global.

**Medido em 2026-11-17** (red team ML-FA da REQ-2026-11-15, reproduzido pelo arquiteto): com `HOME`, `USERPROFILE` e
`CODEX_HOME` isolados, `agents install --targets codex --scope global --items architect --force`, rodado na raiz do
repositório, reescreveu o `CLAUDE.md` e o `AGENTS.md` do trackfw. O usuário pediu escopo global; o projeto em que ele
estava parado foi alterado. Como `global` é o default, basta rodar o comando sem `--scope` dentro de qualquer projeto.

## Decision

**D1 — Escopo global não escreve no projeto.** Com `--scope global`, `agents install` e `skills install` não escrevem
nada sob `manager.ProjectRoot`: nem regras (`InjectRulesForTool`) nem hooks (`InjectHooksForTools`). Vale para o
`global` explícito e para o default.

**D2 — O comando diz o que não fez.** Com `--scope global`, depois de instalar, o comando imprime uma linha dizendo
que as regras e os hooks do projeto não foram alterados, e como alterá-los: `--scope project` ou `trackfw update`.
A mensagem não cita o caminho do cwd: ela não depende de onde o comando rodou.

**D3 — Escopo project não muda.** Com `--scope project`, regras, hooks, aviso D8 e registro no `trackfw.yaml`
(`integrations_flags.go:314`) continuam exatamente como hoje.

**D4 — O aviso D8 do Antigravity segue o escopo.** Ele afirma que o Antigravity acabou de ser instalado no projeto;
com `--scope global` isso é falso, então o aviso só sai com `--scope project`. O `validate` e o `doctor` continuam
emitindo o mesmo aviso, independentes do install.

**D5 — Mesma causa, mesmo contrato.** Qualquer outro comando com escopo global que escreva regras ou hooks no cwd
recebe D1–D2 nesta REQ. `trackfw init`, `trackfw update` e `trackfw discover --init` são comandos de projeto por
contrato e ficam fora (escopo negativo da REQ).

## Consequences

- **Positivo:** o escopo global cumpre o que o nome promete. Rodar o install em qualquer diretório deixa de alterar
  arquivos versionados de outro projeto.
- **Negativo:** quem usava `agents install` sem `--scope` para também atualizar as regras do projeto precisa passar
  `--scope project` ou rodar `trackfw update`. A mudança entra no changelog em *Changed*, com o comando de remédio.
  O trackfw ainda não tem consumidores conhecidos que dependam disso.
- **Testes:** testes que afirmam que o install global escreve no projeto fixam o contrato antigo e mudam junto.

## Alternatives Considered

- **Escrever e avisar qual projeto foi tocado:** mantém o efeito colateral num comando que o usuário roda sem saber
  em que diretório está. Rejeitada pelo KG.
- **Só hooks de guard no escopo global:** proteger mais parece bom, mas continua alterando arquivos versionados do
  projeto sem pedido, e o projeto pode não ser o que o usuário quis. Os guards do projeto já têm caminho próprio
  (`trackfw update`) e o `validate` avisa quando faltam. Rejeitada pelo KG.
