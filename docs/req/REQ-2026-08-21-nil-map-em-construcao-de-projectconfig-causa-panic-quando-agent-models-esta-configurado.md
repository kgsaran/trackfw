---
status: Done
date: 2026-08-21
author: ""
adr: ""
roadmap: ""
---

# REQ: nil map em construção de `ProjectConfig` causa panic quando `agent_models` está configurado

> Date: 2026-08-21 | Status: Done

## Motivação

Defeito de **panic** introduzido pelo ML-1B da REQ do modelo por tier, encontrado por acidente no
ML-2B e **classificado como nota de rodapé** ("bug adjacente"). Panic não é nota de rodapé.

**Reproduzido por mim**, com o caminho certo — exige `CLAUDE.md` presente, para que a geração de
regras seja alcançada em vez de pulada:

```
sem o fix:  trackfw update  ->  panic, stack ate root.go:94
com o fix:  trackfw update  ->  normal
```

O ML-1B acrescentou o campo `AgentModels` ao `ProjectConfig`, e `parse()` escreve nele. Toda
construção do struct que **não inicializa o mapa** e depois chama `parse()` estoura com
`assignment to entry in nil map` se o `trackfw.yaml` tiver a chave.

### O fix do ML-2B corrigiu **uma** das duas ocorrências

Confirmado por mim, com teste dedicado:

```
internal/config/config.go:181  ReadAgentConventions     -> CORRIGIDO no ML-2B
internal/config/config.go:165  ParseRulesFromContent    -> AINDA PANICA
```

```
TestParseRulesFromContentNilMapPanic
  PANIC com agent_models: assignment to entry in nil map
```

É o padrão **"condição estreita demais"** de novo — a nona vez nesta série. Consertou-se a
ocorrência, não a classe.

## 🔴 Duas falhas de processo que este defeito expõe

**1. A minha auditoria do ML-1B deixou passar.** Exercitei `agents install` e `agents models`; nos
dois, o caminho da geração de regras é **pulado**. Testei o que a feature nova usa, não o que a
**mudança de struct** atinge. Um campo novo num struct compartilhado tem raio de alcance maior que
a feature que o motivou.

**2. `make quality` ficou verde com o panic presente.** Nenhum teste cobria `parse()` com
`agent_models` a partir das construções afetadas. Um panic num caminho comum de escrita — `trackfw
update` num projeto com `CLAUDE.md` — atravessou a suíte inteira.

## Escopo

1. Corrigir `ParseRulesFromContent` e **qualquer outra construção** de `ProjectConfig` seguida de
   `parse()`. Varrer, não corrigir a apontada.
2. **Fechar a classe, não a instância.** Avaliar: construtor único que inicializa todos os mapas, ou
   `parse()` inicializando defensivamente antes de escrever. A decisão vale mais que o remendo,
   porque o próximo campo de mapa terá o mesmo problema.
3. Verificar os espelhos Node e Python — a linguagem difere, mas o padrão de "campo novo, construção
   antiga" não.
4. Cenário P4 e teste que **falhe** sem o fix.

## O que **não** é escopo

- Mudar a semântica de `agent_models` ou a resolução. Ela está entregue e auditada.

## Acceptance Criteria

- [ ] AC1 — Varredura de **todas** as construções de `ProjectConfig` seguidas de `parse()`; nenhuma
      com mapa nil.
- [ ] AC2 — Decisão registrada sobre **fechar a classe** (construtor único ou init defensivo no
      `parse`), com o motivo.
- [ ] AC3 — Teste que **panica** sem o fix e passa com ele, para cada construção afetada.
- [ ] AC4 — Espelhos Node e Python verificados quanto ao padrão equivalente.
- [ ] AC5 — Cenário P4 com baseline e detecção.
- [ ] AC6 — `make quality` verde **e CI verde**.

## Riscos para quem executar

- **Não corrigir só o apontado.** A REQ existe porque o ML-2B fez isso e sobrou uma.
- **O caminho de repro exige `CLAUDE.md` presente** — sem ele a geração de regras é pulada e o panic
  não aparece. Foi o que me fez errar a primeira tentativa de reprodução.
- **Cuidado com o binário do `PATH`** — desatualizado, e `--version` não distingue o build.

## Linked ADR
ADR: <!-- avaliar: fechar a classe pode merecer registro -->

## Linked Roadmap
Roadmap: <!-- sem roadmap; backlog -->
