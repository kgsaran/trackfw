---
status: done
date: 2026-08-16
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-16-handler-global-de-erro-nos-entrypoints-node-e-python.md"
---

# REQ: erro nao tratado no CLI Node vaza stack trace caminhos absolutos e versao do runtime

> Date: 2026-08-16 | Status: done
| Linear Issue: 
| Jira Issue: 


## Motivation

**Divulgação de informação em caminho de erro esperado.** Encontrado pelo arquiteto ao auditar o
ML-2C da REQ de higiene, e priorizado por KG como correção urgente.

### Vazamento medido

`trackfw agents update --force` sobre um artefato *unmanaged* — erro **esperado**, previsto e
tratado pelo produto — produz no **Node**:

```
/Users/<usuario>/<caminho>/npm/src/integrations/manager.js:189
      if (!owned && status.state === 'modified') throw new Error(...)
                                                       ^
Error: unmanaged artifact "..." does not match a trackfw template — ...
    at IntegrationManager.preflight (/Users/<usuario>/<caminho>/manager.js:189:56)
    at IntegrationManager.mutate   (/Users/<usuario>/<caminho>/manager.js:150:25)
    ... 4 frames
Node.js v26.7.0
```

**O que sai junto da mensagem:** nome de usuário, layout do diretório home, caminho de instalação,
estrutura interna de módulos, linha de código-fonte e **versão do runtime**.

### Severidade — declarada com precisão, não inflada

É **divulgação de informação**, não execução de código nem escalação de privilégio. Classificação:
**baixa a moderada**. O item de maior valor para um atacante é a **versão do runtime**, que permite
mirar CVE conhecida sem sondagem.

O que justifica a urgência **não é a classificação**, e sim: (a) está num caminho de erro
**esperado e voltado ao usuário**, que qualquer pessoa dispara sem esforço; (b) o conteúdo vaza para
**log de CI, terminal compartilhado e screenshot de suporte**, que circulam mais que o terminal do
autor; (c) o conserto é barato.

### Escopo medido nos 3 CLIs

| CLI | vaza neste caminho | handler global | conclusão |
|---|---|---|---|
| **Node** | **sim** | **não existe** | correção |
| Python | não | **não existe** | defesa em profundidade (risco latente) |
| Go | não | coberto pelo cobra | sem ação |

O padrão real: comandos que capturam o erro **internamente** imprimem limpo (ex.: `roadmap move`);
os que **não** capturam despejam a stack. É por-comando — e é exatamente por isso que a correção
certa é **um handler no entrypoint**, não um `try/catch` a mais em cada lugar onde alguém lembrar.

## Acceptance Criteria

- [ ] **AC1** — Nenhum caminho de erro do CLI Node imprime stack trace, caminho absoluto de
      instalação, linha de código-fonte ou versão do runtime. Saída de erro é a mensagem, e só.
- [ ] **AC2** — Handler global no entrypoint **Node** e no **Python** (defesa em profundidade),
      capturando erro não tratado e emitindo mensagem limpa em **stderr** com exit code ≠ 0.
- [ ] **AC3** — **Diagnóstico preservado atrás de opção explícita**: variável de ambiente (ex.
      `TRACKFW_DEBUG=1`) restaura a stack completa, para não cegar quem precisa depurar.
- [ ] **AC4** — Comportamento de erro **já correto não regride**: os caminhos que hoje imprimem
      mensagem limpa continuam idênticos, byte a byte.
- [ ] **AC5** — Gate de paridade cobrindo "erro esperado não vaza stack" nos 3 CLIs, com **cenário
      de falsificação** conforme P4 do `ADR-2026-07-26-principios-de-design-de-gates-verificaveis`.
- [ ] **AC6** — `make quality` verde.

## Escopo negativo

- **Não** unifica o *prefixo* das mensagens de erro entre os CLIs (Go usa `Error:`, Python usa
  `trackfw <cmd>:`). Isso é o débito de wrapper já registrado na REQ de higiene — **item separado**,
  para não misturar correção de segurança com harmonização cosmética.
- **Não** altera nenhuma mensagem de erro existente.
- **Não** mexe em tratamento de `panic` no Go, que é crash e não caminho de erro esperado.

## Linked ADR

ADR: (não requerido — não há decisão arquitetural nova; AC3 é escolha de implementação)

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-16-handler-global-de-erro-nos-entrypoints-node-e-python.md`

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: 
