---
status: Open
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: Título de roadmap com newline forja seção de wave inteira, e o `barrier` executa o gate forjado

> Date: 2026-08-30 | Status: Open

## Motivation

Achado em 2026-08-23 pelo `hades-tf`, registrado em
`vault/notes/roadmap-title-newline-forges-wave-section-barrier-executes-gate-2026-08-23.md` com o
status *"NÃO CORRIGIDO — requer REQ própria"*. **Sete dias e nenhuma REQ aberta** — esta.

O título passado a `trackfw roadmap new` é interpolado no corpo do documento. Um título contendo
**newline** injeta linhas arbitrárias — inclusive uma seção `## Wave N` completa, com bloco
`**Gates da wave:**` e comandos. O `barrier` depois **executa** esses comandos.

Mesma classe dos dois bypasses do `barrier` corrigidos no PR #217 (ML fantasma em cerca; fechamento
de cerca com sufixo), e da nota irmã
`rewrite-frontmatter-newline-injection-escape-hatch-2026-08-21`, que **já foi corrigida** com
`containsControlChar` — a defesa existe no projeto, só não foi aplicada aqui.

**Vetor:** quem controla o título controla o conteúdo. Um agente que receba o título de fonte não
confiável, ou um `roadmap new` chamado a partir de um campo de issue/PR, injeta o gate que quiser.

## Acceptance Criteria

- [ ] **AC1** — Título com newline (`\n`, `\r\n`, `\r`) é **rejeitado** no `roadmap new`, nos 3 CLIs,
      com mensagem nomeando o motivo. Reaproveitar `containsControlChar` e equivalentes, que já
      existem.
- [ ] **AC2** — Vale para os demais geradores que interpolam título: `req new`, `adr new`,
      `note new`. **Enumerar** — não corrigir só o `roadmap new`.
- [ ] **AC3** — Outros caracteres de controle também rejeitados; declarar o conjunto.
- [ ] **AC4** — Falsificação nas duas direções: título com newline → recusa; título legítimo com
      acento, hífen, parêntese e `/` → **aceito**. Restringir demais quebra uso real.
- [ ] **AC5** — **Defesa em profundidade no `barrier`:** mesmo que um roadmap forjado exista em
      disco, decidir e documentar se o `barrier` deve detectar seção de wave inconsistente. A
      validação na entrada não protege documento já gravado.
- [ ] **AC6** — Paridade nos 3; gate falsificável.
- [ ] **AC7** — `make quality` exit 0 e CI verde.

## Negative Scope

- **Não** alterar o formato do roadmap nem o parsing de wave do `barrier` além do AC5.
- **Não** sanitizar silenciosamente (trocar newline por espaço) — **recusar** e avisar. Sanitização
  silenciosa esconde a tentativa.

## Linked ADR
<!-- Provável: postura canônica para interpolação de entrada de usuário em artefato gerado. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
