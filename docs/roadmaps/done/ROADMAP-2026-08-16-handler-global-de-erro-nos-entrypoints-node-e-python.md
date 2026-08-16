---
status: done
date: 2026-08-16
req: "docs/req/REQ-2026-08-16-erro-nao-tratado-no-cli-node-vaza-stack-trace-caminhos-absolutos-e-versao-do-runtime.md"
squad: "apolo-tf, hades-tf"
---

# Roadmap: Handler global de erro nos entrypoints Node e Python

> Created: 2026-08-16 | Status: done | **Prioridade: urgente (KG)**

## Context

REQ: `docs/req/REQ-2026-08-16-erro-nao-tratado-no-cli-node-vaza-stack-trace-caminhos-absolutos-e-versao-do-runtime.md`

Divulgação de informação em caminho de erro **esperado**: o CLI Node despeja stack trace, caminhos
absolutos, linha de código-fonte e **versão do runtime**. Medido pelo arquiteto; severidade
declarada como **baixa a moderada** (não é execução de código nem escalação), com urgência
justificada pela facilidade de disparo e pela circulação em log de CI e screenshot.

### Estado medido (2026-08-16)

| CLI | vaza | handler global | entrypoint |
|---|---|---|---|
| **Node** | **sim** | **não** | `npm/bin/trackfw`, `npm/src/commands/index.js` |
| Python | não (nos caminhos testados) | **não** | `pypi/trackfw/cli.py` (`main()`, :46) |
| Go | não | cobra trata | `cmd/trackfw/main.go` → `commands.Execute()` |

Comandos que capturam internamente imprimem limpo (`roadmap move`); os que não capturam vazam.
É por-comando — daí a correção ser **no entrypoint**, e não mais um `try/catch` onde alguém lembrar.

## Acceptance Criteria
- [x] AC1 — Nenhum caminho de erro do Node imprime stack, caminho absoluto, linha de fonte ou versão do runtime.
- [x] AC2 — Handler global no Node **e** no Python; mensagem limpa em stderr, exit ≠ 0.
- [x] AC3 — `TRACKFW_DEBUG=1` restaura a stack completa.
- [x] AC4 — Caminhos de erro já corretos **não regridem** (byte a byte).
- [x] AC5 — Gate de paridade + cenário de falsificação (P4).
- [x] AC6 — `make quality` verde.

---

## Wave 1 — Correção

### ML-1A — Handler global (Node e Python) + gate
**Status:** ✅ Concluído (2026-08-16) — vazamento eliminado, auditado por reprodução real · **Agente:** `apolo-tf` (`subagent_type: apolo-tf`)
**Arquivos:** `npm/bin/trackfw`, `npm/src/commands/index.js`, `pypi/trackfw/cli.py`,
script de paridade novo ou existente, `scripts/check-gates-falsify.sh`, + testes dos 3 stacks.

**Ações:**
1. **Node:** capturar erro não tratado no entrypoint (incluindo rejeição de promessa, dado que o
   fluxo usa `parseAsync`). Imprimir **apenas** a mensagem, em stderr, exit ≠ 0.
2. **Python:** mesmo tratamento em `main()` — defesa em profundidade, já que hoje nenhum caminho
   global captura.
3. **`TRACKFW_DEBUG=1`** restaura a stack completa nos dois.
4. Gate de paridade: erro esperado **não** contém stack/caminho absoluto/versão de runtime, nos 3 CLIs.
5. **P4:** cenário em `check-gates-falsify.sh` com braço baseline e braço detecção.

**🔴 Onde este ML pode falhar em silêncio:**
- **Engolir o exit code.** Um handler que captura e sai com **0** transformaria erro em sucesso —
  pior que o vazamento. Exit ≠ 0 é critério de aceite, com teste.
- **Engolir a mensagem.** Capturar e imprimir algo genérico ("an error occurred") destruiria a
  diagnosticabilidade que o ML-2C da higiene acabou de melhorar. A **mensagem tem que sobreviver
  íntegra** — inclusive a linha `Adopt it with: ...`, que é multilinha.
- **Não cobrir rejeição assíncrona.** `parseAsync` significa que erro pode vir como promessa
  rejeitada; um `try/catch` síncrono não pega.

**Critérios de aceite:**
- [ ] `agents update --force` sobre artefato unmanaged, no Node: **sem** stack, **sem** caminho
      absoluto de instalação, **sem** `Node.js vX`; mensagem íntegra (as duas linhas) em stderr; exit ≠ 0.
- [ ] `TRACKFW_DEBUG=1` no mesmo comando: stack presente.
- [ ] Caminho já limpo (ex.: `roadmap move` inexistente) **byte-idêntico** ao de hoje nos 3 CLIs.
- [ ] Erro assíncrono também coberto — teste explícito.
- [ ] `make quality` verde.

**Comando de validação:** `make quality`

---

## Wave 2 — Barreira

### ML-2A — `hades-tf`: revisão do vazamento
**Status:** ✅ Concluído (2026-08-16) — LIBERA; achado grave em `serve` reportado à parte · **Agente:** `hades-tf` (`subagent_type: hades-tf`)
**Escreve:** `docs/seguranca/2026-08-16-vazamento-de-stack-no-cli-node.md`
**Ações:** confirmar que **nenhum** caminho de erro dos 3 CLIs vaza stack, caminho absoluto ou
versão de runtime — varrer, não confiar no caminho corrigido. Avaliar se `TRACKFW_DEBUG` cria
superfície nova. Avaliar se **outros** artefatos do produto (hooks gerados, `serve`, saída `--json`)
vazam a mesma classe de informação. **Veredito explícito; bloquear é saída legítima.**

---

## Notas
- **Fora de escopo, já registrado:** unificar o *prefixo* de erro entre CLIs (`Error:` vs
  `trackfw <cmd>:`) é o débito de wrapper da REQ de higiene — não misturar correção de segurança com
  harmonização cosmética.
- Commits e branch são exclusivos do `trackfw_architect`.
