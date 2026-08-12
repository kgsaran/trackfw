---
status: Open
date: 2026-08-12
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-12-defesa-do-credential-guard-vive-no-escopo-global-controle-que-mora-onde-o-agente-escreve-nao-e-controle.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-12-guard-global-por-padrao-wave-0-medicao-bloqueante-da-premissa-de-sandbox.md"
---

# REQ: Credential-guard de escopo global como caminho padrao — consentimento explicito, verificacao da premissa de sandbox e a via do credential_guard.mode

> Date: 2026-08-12 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

O `ADR-2026-08-12` decidiu que **a defesa real do credential-guard é o escopo global**: o guard
instalado em `~/.trackfw/` vive **fora do repositório** em que o agente trabalha, enquanto o de
escopo de projeto pode ser apagado, sobrescrito ou desligado pelo próprio agente induzido.

**Mas hoje o escopo global é estritamente opt-in**, e isso torna a decisão uma intenção, não uma
proteção:

- só é instalado por `trackfw update harness --targets <tool>-credential-guard --install-missing`;
- **`--install-missing` é obrigatório** — sem ele, alvo ausente é reportado como *missing* e deixado
  em paz;
- `trackfw update harness` **nunca** toca no repositório, e `trackfw init`/`update` **nunca** tocam
  em `~/`. Ou seja: **nenhum fluxo padrão instala o guard global.**

Enquanto isso não mudar, o ADR desprotegeu o escopo de projeto (revertendo `failClosed`, descartando
wrapper e integridade) **sem** ter tornado o global efetivo. Esta REQ fecha essa lacuna.

### As três lacunas que esta REQ precisa fechar

**1. Consentimento.** Instalar o guard global significa **escrever fora do repositório**, no `$HOME`
do usuário — em `~/.claude/settings.json`, `~/.codex/hooks.json`, `~/.gemini/settings.json`,
`~/.cursor/hooks.json`, `~/.copilot/settings.json`, `~/.kiro/hooks/`. Fazer isso **em silêncio**,
como efeito colateral de um `trackfw init`, seria invasivo e quebraria a expectativa hoje explícita
na ajuda do comando (*"it never touches anything inside the current repository — that is the job of
`trackfw update`"*, e o simétrico). **"Por padrão" não pode significar "sem perguntar."**

**2. A premissa de sandbox NÃO foi medida.** O ADR registra como risco aceito: *"não foi medido se o
agente alcança `~/.trackfw/` nos ambientes reais — a premissa 'agente restrito ao workspace' vale
para sandboxes que restringem escrita fora do projeto, **não universalmente**. Um agente sem sandbox
alcança `$HOME`."* Se a premissa for falsa nos ambientes que os usuários realmente usam, **o escopo
global não é defesa** e o ADR precisa ser reaberto. Isto é pré-requisito, não detalhe.

**3. `credential_guard.mode` continua rebaixável.** Lido em **runtime** de `trackfw.yaml`
(`internal/generators/scaffold.go:1005`), **dentro do repositório**. Uma linha de YAML desliga o
controle **independentemente de onde o script viva** — escopo global não fecha esta via. Foi
registrado no ADR como não fechado, e precisa de resposta aqui.

## Acceptance Criteria

### Bloqueante — antes de qualquer implementação

- [ ] **Medir a premissa de sandbox — PRIMEIRO trabalho da REQ, sem paralelizar com os demais eixos**
      (ML-4A avalia a probabilidade de a premissa ser falsa como **alta**: nenhum dos 6 CLIs roda
      sandboxed por padrão).
- [ ] Detalhe do método: Determinar empiricamente, para os CLIs instalados nesta
      máquina, se um agente consegue **escrever** em `~/.trackfw/` e nos arquivos de settings globais
      durante uma sessão normal. Distinguir configuração **com** e **sem** sandbox restritivo.
      Método: mesmo padrão dos ML-1A/1C do `ROADMAP-2026-08-12-semantica-de-falha-de-hook` —
      `$HOME`/`CODEX_HOME` isolado, **nunca** escrever no `$HOME` real do usuário.
- [ ] Se a premissa se mostrar **falsa** no ambiente padrão: **parar**, registrar, e reabrir o
      `ADR-2026-08-12` — sem premissa, a decisão de reverter as defesas de escopo de projeto perde a
      base. **Resultado legítimo**, não falha.

### Condicional ao resultado da medição (achado do ML-4A)

- [ ] **Se a medição mostrar que o agente alcança `$HOME`:** decidir e registrar o que fazer com a
      **rotação/sobrescrita do guard global** — ele passa a ser tão alcançável quanto o de projeto, e
      as mesmas vias (apagar, sobrescrever com `exit 0`) se aplicam. Não basta mover o arquivo de
      lugar.
- [ ] **Se a medição confirmar a premissa falsa:** o `ADR-2026-08-12` precisa ser **reaberto**, não
      emendado de novo — sua decisão de rejeitar as três mitigações de escopo de projeto foi tomada
      sob outra leitura de risco.

### Documentação voltada ao usuário final (achado do ML-4A)

- [ ] Registrar **fora** do `docs/cli-parity.md` — README e/ou saída de `--help` — o que a regra
      `credential_guard_hook_resolvable` **não** cobre. `cli-parity.md` é documento interno de
      paridade; o usuário que instala o trackfw não o lê, e o risco real é ele concluir que o guard
      está protegido porque existe uma regra com esse nome.

### Caminho padrão com consentimento

- [ ] O fluxo padrão (`trackfw init`, e/ou `trackfw update`) **detecta** que o guard global não está
      instalado e **oferece** a instalação — com o que será escrito, **onde**, e como desfazer.
- [ ] **Nunca** escreve em `$HOME` sem consentimento explícito do usuário nesta execução.
- [ ] Existe forma **não interativa** de consentir (flag e/ou config), para CI e automação — e o
      comportamento sem consentimento em ambiente não interativo é **não instalar**, nunca instalar
      silenciosamente.
- [ ] Recusar a instalação **não** bloqueia nem degrada o resto do fluxo; no máximo, informa.
- [ ] Paridade nos **3 CLIs** do trackfw (Go, Node.js, Python) — regra dura.

### A via do `credential_guard.mode`

- [ ] Decidir e implementar **uma** resposta, documentada: (a) o modo passa a ser lido de fonte fora
      do repositório quando o guard global está instalado; (b) o guard global **ignora** o
      `credential_guard.mode` do projeto; (c) rebaixamento passa a ser detectado e reportado; ou
      (d) risco aceito, **com justificativa escrita**. Não deixar implícito.

### Comum

- [ ] `docs/cli-parity.md` atualizado: qual é o caminho padrão, o que o escopo de projeto passa a
      significar, e **o que continua descoberto**.
- [ ] `make quality` verde; `trackfw validate` sem violações.

### Escopo negativo

- **Não** reintroduz `failClosed`, wrapper ou verificação de integridade no escopo de **projeto** —
  o `ADR-2026-08-12` os rejeitou. Reabrir exige emendar o ADR, não contorná-lo aqui.
- **Não** altera `scripts/trackfw-credential-guard.sh`.
- **Não** instala nada em `$HOME` durante testes automatizados — sempre `$HOME` isolado.
- **Não** transforma o `trackfw update harness` em algo que toca o repositório, nem o
  `trackfw update` em algo que toca `$HOME` sem consentimento.

## Linked ADR
ADR: docs/adr/ADR-2026-08-12-defesa-do-credential-guard-vive-no-escopo-global-controle-que-mora-onde-o-agente-escreve-nao-e-controle.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-12-guard-global-por-padrao-wave-0-medicao-bloqueante-da-premissa-de-sandbox.md
