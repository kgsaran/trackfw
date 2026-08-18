---
status: Open
date: 2026-08-17
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md"
---

# REQ: o `git-branch-guard` global é instalado sem fiação, e a integridade do escopo global nunca é verificada

> Date: 2026-08-17 | Status: Open (backlog, sem roadmap)
| Linear Issue:
| Jira Issue:

## Motivation

Medido na máquina de KG em 2026-08-17, ao fechar os PRs #183/#184.

O script `~/.trackfw/scripts/trackfw-git-branch-guard.sh` estava com **123 linhas**; o template
atual gera **369**. Ou seja, o script global carregava a versão **anterior ao ML-1A** — sem a
correção do falso-positivo de prosa, sem o fechamento de `switch -c`, sem nada do ML-4B/4C. Três
entregas de correção não chegaram nele, e **nenhum sinal foi emitido em momento algum**.

`trackfw update harness` (com o binário novo) regenerou o script e ele passou a bater com o
template. A correção existe; o que não existe é a detecção.

### Achado principal: o guard global está instalado e **não está ligado em nada**

Medido nos 6 arquivos de config global que o validador conhece (`globalGuardConfigFiles`):

```
~/.claude/settings.json      credential-guard=2   git-branch-guard=0
~/.codex/hooks.json          credential-guard=2   git-branch-guard=0
~/.gemini/settings.json      credential-guard=2   git-branch-guard=0
~/.copilot/settings.json     credential-guard=2   git-branch-guard=0
~/.cursor/hooks.json         ausente
~/.kiro/hooks/...json        ausente
```

O `credential-guard` está cabeado em todos os 4 que existem. O `git-branch-guard` **em nenhum**.

E, mesmo assim, `trackfw update harness` **escreve** o script global:
`internal/generators/update.go:493` chama `GenerateGlobalGitBranchGuardScript(home)` incondicionalmente
(fora de dry-run). Resultado: um script de controle de segurança é gravado no `$HOME` do usuário e
**nada jamais o invoca**.

### Por que a integridade nunca foi verificada — o mecanismo, provado

`validateGuardGlobalScriptIntegrity` existe e faz exatamente a verificação certa. Mas a condição de
disparo é *"para cada um dos 6 arquivos de config global que **referencia** o `scriptMarker`"*.

Como nenhum config referencia o `git-branch-guard`, o laço **nunca entra**, a regra nunca avalia, e o
script pode apodrecer indefinidamente com `validate` verde. Foi o que aconteceu: 3 versões de atraso,
zero avisos.

**O padrão a nomear: a verificação de integridade está condicionada à fiação, e não à existência do
artefato.** Artefato instalado e não cabeado cai num ponto cego — está no disco, é executável, e é
invisível para o validador.

### Por que isso importa mais do que "script órfão"

O `ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido...` decidiu que o escopo global é onde a
defesa precisa morar — *"controle que mora onde o agente escreve não é controle"*. O
`git-branch-guard` é justamente um controle contra agente: impedir que subagente crie branch ou rode
`git` bruto. Ele **não tem proteção de escopo global nenhuma** hoje, apesar de o produto gravar o
script como se tivesse.

Duas leituras possíveis, e a REQ precisa escolher:

1. **A fiação global do `git-branch-guard` está faltando** → cabear nos mesmos 4+2 CLIs do
   `credential-guard`, e a integridade passa a ser verificada de graça pela regra existente.
2. **A ausência de fiação é deliberada** (o guard é de projeto por natureza — decide sobre branches
   de um repositório) → então **o script global não deveria ser escrito**, e `update.go:493` é que
   está errado.

**Não decidir é o pior dos mundos**, que é o estado atual: escrever o artefato, não ligá-lo, e não
verificá-lo.

### Correção de registro

Numa análise anterior eu atribuí ao guard **global** o falso-positivo que bloqueou meu
`gh pr create`. **Estava errado.** O bloqueio veio do guard de **projeto**: eu estava numa branch
criada a partir da `main` anterior ao #183, cujo `scripts/trackfw-git-branch-guard.sh` tinha as
mesmas 123 linhas pré-ML-1A. O guard global não estava envolvido — ele não é invocado por ninguém,
que é precisamente o assunto desta REQ.

## Escopo

1. **Decidir entre (1) e (2) acima, e registrar em ADR** — é decisão de arquitetura sobre onde o
   controle mora, com precedente direto no `ADR-2026-08-12`.
2. **Fechar o ponto cego do validador**: integridade de artefato global não pode depender de haver
   fiação apontando para ele. Se o trackfw escreveu o script, o trackfw verifica o script.
3. **Paridade nos 3 CLIs**, com gate — o mesmo padrão dos demais.

### Escopo negativo — declarado

- **Não é reescrever o guard** nem mexer no que ele bloqueia — isso foi feito nos ML-1A/4B/4C.
- **Não é criar prevenção contra agente induzido.** O `ADR-2026-08-12` já decidiu que não é
  alcançável; isto é integridade e fiação do que já existe.
- **Não é o escopo de projeto**, coberto pela
  `REQ-2026-08-17-validate-nao-detecta-hook-de-guard-na-forma-relativa-antiga-que-falha-fora-da-raiz`.

## Acceptance Criteria

- [ ] AC1 — ADR registra a decisão entre cabear o guard global ou parar de escrevê-lo, **com o
      motivo**. Decidir "não cabear" é resultado legítimo, desde que `update` pare de gravar.
- [ ] AC2 — Script global escrito pelo trackfw tem sua integridade verificada **independentemente**
      de existir config apontando para ele.
- [ ] AC3 — Repro fiel: `~/.trackfw/scripts/trackfw-git-branch-guard.sh` adulterado ou defasado é
      **acusado**. Hoje o `validate` passa limpo — foi assim que 3 versões de atraso passaram.
- [ ] AC4 — Não-regressão: a verificação do `credential-guard` global, que **hoje funciona** porque
      está cabeado, continua funcionando e não passa a acusar em dobro.
- [ ] AC5 — `$HOME` do teste é **controlado pelo fixture**, nunca o real. Há precedente registrado
      de vazamento de ambiente nesse tipo de cenário (Cenário 46, ML-1B de 2026-08-12).
- [ ] AC6 — Cenário de falsificação (P4) com baseline e detecção, provando não-vacuidade.
- [ ] AC7 — Paridade nos 3 CLIs com gate comparando saídas reais, não leitura de fonte.
- [ ] AC8 — `make quality` verde; `trackfw validate` sem novas violações.

## Riscos para quem executar

- **Escrever no `$HOME` do usuário** é a operação mais sensível do produto. Qualquer mudança em
  `update harness` precisa de dry-run honesto — e hoje ele **não** é: a geração dos scripts globais
  está dentro de `if !opts.DryRun`, então o dry-run reporta `updated=0` e a execução real reescreve
  os dois scripts. Isso reforça a
  `REQ-2026-08-17-update-dry-run-aborta-em-symlink-quebrado-ao-copiar-a-arvore-inteira-do-projeto`.
- **Falso-positivo em escopo global é pior que em projeto:** afeta todos os repositórios do usuário
  de uma vez.
- **Cuidado com o binário do `PATH`** — pode estar velho, e `--version` não distingue o build.
  Neste projeto o CLI vem do Homebrew (`/opt/homebrew/Cellar`), e `make install` grava em
  `/usr/local/bin`: instalar por ali cria uma segunda cópia sombreada. Compilar e usar `./bin/trackfw`.

## Linked ADR
ADR: `docs/adr/ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-trackfw.md` (decide a bifurcação desta REQ)
ADR de contexto: `docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md` (decide que a defesa mora no escopo global; esta REQ mostra que para o `git-branch-guard` isso não se concretizou)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-17-guard-global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md`
