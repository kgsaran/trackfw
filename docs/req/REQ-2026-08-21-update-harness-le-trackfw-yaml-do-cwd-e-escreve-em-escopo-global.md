---
status: Open
date: 2026-08-21
author: ""
adr: ""
roadmap: ""
---

# REQ: `update harness` lê `trackfw.yaml` do cwd e escreve em escopo global

> Date: 2026-08-21 | Status: Open (backlog, sem roadmap)

## Motivação

Segundo achado da barreira do ML-4A
(`docs/seguranca/2026-08-21-revisao-da-configuracao-de-modelo.md`), **deferido para REQ própria** pelo
executor do ML-5A, com motivo escrito e que eu endosso.

`config.Load()` lê `trackfw.yaml` **relativo ao cwd**; `UpdateHarness` grava em
`~/.claude/agents/`. Um `trackfw.yaml` num diretório qualquer influencia o que é escrito em
**escopo global** — válido para todos os projetos da máquina.

Medido nos 3 CLIs: `internal/generators/update.go:1723`, `npm/src/commands/update-harness.js:761`,
`pypi/trackfw/commands/update_harness.py:996`.

## Por que não foi corrigido junto

Registrado pelo executor e verificado por mim:

1. **A correção do caractere de controle (ML-5A) elimina a classe de dano mais grave.** Depois dela,
   a pior saída de um `trackfw.yaml` hostil é um ID de modelo arbitrário **de uma linha só** — não
   mais injeção de instrução no corpo do arquivo de agente. É uma ordem de magnitude menos severo.
2. **Restringir o que `update harness` aceita do cwd é mudança de comportamento com raio amplo** —
   afeta todo usuário que roda o comando fora de um projeto canônico, e merece ciclo próprio.
3. Fazer no ML-5A seria expansão de escopo não sancionada, no meio de uma correção de bloqueio.

## Residual documentado, e é o que esta REQ ataca

- Valor de uma linha ainda pode ser **arbitrário** — o usuário passa a rodar com um modelo que não
  escolheu, em **todos** os projetos.
- Valor com `"` ou `:` pode produzir frontmatter **inválido** — negação de serviço no agente, não
  injeção de instrução.

## Escopo

Decidir e implementar a fronteira entre **config de projeto** e **escrita em escopo global**.
Candidatos a avaliar no ADR:

1. `update harness` **ignora** `agent_models` do cwd e usa só config global.
2. `update harness` **exige** confirmação quando o cwd tem config que afeta escopo global.
3. `update harness` **recusa** fora de projeto trackfw — análogo ao no-op do guard
   (`ADR-2026-08-17`), que resolveu um dilema parecido.

## 🔴 Risco dominante

**Quebrar o uso legítimo.** Rodar `update harness` de dentro de um projeto com `agent_models`
configurado é caso normal, e a config **deve** valer ali. A fronteira precisa separar *"config do
meu projeto"* de *"diretório qualquer"*, e essa distinção não é óbvia — o cwd não diz qual é qual.

## Acceptance Criteria

- [ ] AC1 — Decisão registrada em **ADR**, com os candidatos descartados e o motivo.
- [ ] AC2 — Uso legítimo preservado: `update harness` de dentro de projeto com `agent_models`
      continua honrando a config — provado por cenário.
- [ ] AC3 — Diretório sem relação **não** influencia escopo global — provado.
- [ ] AC4 — Paridade nos 3 CLIs, com gate comparando saídas reais.
- [ ] AC5 — Cenário P4 com baseline e detecção.
- [ ] AC6 — `make quality` verde **e CI verde**.

## Riscos para quem executar

- **O caso legítimo e o hostil têm a mesma forma** — `trackfw.yaml` no cwd. A distinção precisa de
  critério, não de heurística frouxa.
- **Cuidado com o binário do `PATH`** — desatualizado, e `--version` não distingue o build.
- **Fixture com `HOME` redirecionado**, sempre: o comando escreve em escopo global.

## Linked ADR
ADR: <!-- a criar: fronteira entre config de projeto e escrita global -->

## Linked Roadmap
Roadmap: <!-- sem roadmap; backlog -->
