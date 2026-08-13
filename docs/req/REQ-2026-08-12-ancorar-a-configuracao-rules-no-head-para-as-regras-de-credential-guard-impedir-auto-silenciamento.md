---
status: Done
date: 2026-08-12
author: "Zeus (Arquiteto)"
adr: "docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-head-e-disco.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard.md"
---

# REQ: Ancorar a configuracao rules no HEAD para as regras de credential-guard — impedir auto-silenciamento

> Date: 2026-08-12 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

Achado da revisão de segurança final do `ROADMAP-2026-08-12-deteccao-de-adulteracao-...` (ML-3B,
`docs/seguranca/2026-08-12-estado-final-deteccao-credential-guard.md`), **não previsto no despacho**:

**A regra `credential_guard_mode_downgrade` pode se auto-silenciar.**

`ruleSeverity()` (`internal/validator/validator.go:107`) resolve a severidade lendo `rules:` do
`trackfw.yaml` **em disco** — nunca do `HEAD`. Consequência: uma **única edição não commitada** que:

```yaml
credential_guard:
  mode: warn          # <- rebaixa o controle
rules:
  credential_guard_mode_downgrade: off   # <- e desliga quem avisaria
```

**derrota a detecção sem commitar nada.**

### Por que isto é pior que os limites já aceitos

O `ADR-2026-08-12` aceita, explicitamente, que um adversário que **commita** a alteração não seja
detectado — o que sobra ali é o **rastro auditável**: o diff existe, aparece no PR, é revisável.

Este caso é diferente: **não há commit**, logo **não há rastro** — e a própria regra que produziria o
sinal é desligada pelo mesmo arquivo que ela deveria vigiar. É o controle sendo desativado pelo
artefato que ele guarda.

Registrado em `docs/cli-parity.md` e no `README.md` como limite conhecido, mas **documentar não é
resolver**.

## Acceptance Criteria

- [ ] A severidade das regras de credential-guard **não** é rebaixável por edição **não commitada**
      do `rules:` — decidir e implementar o mecanismo (ex.: para estas regras, resolver `rules:` a
      partir do `HEAD` quando houver `HEAD`).
- [ ] Comportamento **sem `HEAD`** (repo sem commits, `trackfw.yaml` não versionado) explícito e
      testado — a convenção do projeto é não violar por ausência, mas aqui há tensão: sem `HEAD`, a
      resolução cai no disco e o buraco volta. **Decidir conscientemente e escrever.**
- [ ] Não altera a resolução de `rules:` das **demais** regras — mudança de escopo amplo em máquina
      compartilhada é risco desproporcional. Se for inevitável, justificar.
- [ ] Desligar a regra **de forma legítima e commitada** continua funcionando — o objetivo é impedir
      o rebaixamento **silencioso**, não remover a configurabilidade.
- [ ] **Paridade nos 3 CLIs** (Go, Node.js, Python) — regra dura.
- [ ] Cenário de falsificação: a edição combinada (`mode: warn` + `rule: off`, não commitada)
      **continua sendo reportada**. Com prova de não-vacuidade e braço autodiscriminante.
- [ ] `docs/cli-parity.md` e `README.md` atualizados removendo o limite, se resolvido.
- [ ] `make quality` verde; `trackfw validate` sem violações.

### Escopo negativo

- **Não** reabre a decisão de prevenção × detecção (`ADR-2026-08-12`) — isto é sobre a **detecção não
  ser desligável em silêncio**, não sobre bloquear o adversário.
- **Não** altera as âncoras decididas (template do binário para o script, `HEAD` para o `mode`).
- **Não** transforma detecção em bloqueio de chamada de ferramenta.

### Segundo limite, menor, do mesmo parecer

- [ ] Avaliar: **a cobertura de deleção é condicional ao wiring** — se o script **e** a entrada de
      hook forem removidos **juntos**, as três regras silenciam
      (`internal/validator/validator_credential_guard.go:106-108`). Decidir se vale detectar
      "projeto que já teve wiring de guard e não tem mais" (exigiria âncora no `HEAD` também), ou
      aceitar e documentar.

## Linked ADR
ADR: docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-head-e-disco.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard.md
