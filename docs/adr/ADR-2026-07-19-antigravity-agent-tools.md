---
status: Accepted
date: 2026-07-19
author: "Zeus"
---

# ADR: Adapter de render do Antigravity — tools validos e model tier

> Date: 2026-07-19 | Status: Accepted
>
> Refina o ADR pai `docs/adr/ADR-2026-07-18-catalogo-canonico-e-adapters-para-integracoes-de-agentes.md` para o adapter do alvo `antigravity`.

## Context

O adapter de agents do alvo `antigravity` surface `current` (representacao `agent-directory`) emite o markdown canonico do asset **verbatim**. Verificacao empirica no Antigravity CLI (`agy`) mostrou dois defeitos:

1. **`model: opus|sonnet`** (nomes de modelo Anthropic) faz o `agy` **rejeitar silenciosamente** o agente — nao aparece em `agy agent`.
2. **Ausencia de `tools:`** faz o agente carregar em **modo read-only** — sem `write_to_file`/`run_command`, portanto incapaz de escrever arquivos ou rodar comandos.

Alem disso, IDs de tool invalidos **quebram** o agente (`no tool converter registered for <id>`), nao sao ignorados. O `agy` injeta ferramentas MCP automaticamente via `mcp_config.json` (sem ID no frontmatter).

## Decision

No adapter de agents do `antigravity` surface `current` (e apenas nele — `agent-directory` e exclusivo desse alvo), reconstruir o frontmatter:

1. **Mapear `model`** para tiers validos do agy: `opus -> pro`, `sonnet -> flash`. Ausente ou nao mapeavel -> omitir `model`.
2. **Injetar `tools:`** com IDs validos do agy, por papel do agente:
   - `trackfw-architect` (orquestrador) -> **SET_ARCH (14)**: `view_file, list_dir, grep_search, search_web, read_url_content, write_to_file, replace_file_content, run_command, command_status, generate_image, send_message, define_subagent, invoke_subagent, schedule`.
   - Demais especialistas -> **SET_IMPL (10)**: os 10 primeiros acima.
3. **IDs proibidos** (nunca emitir): `edit_file, read_file, find, view_code_item, view_file_outline, call_mcp_tool`.
4. **Nao alterar os assets** `assets/agents/*.md` — a transformacao vive no adapter, para nao vazar IDs especificos do agy para claude/gemini/cursor.
5. **Paridade** obrigatoria nos 3 CLIs (Go/Node/Python) com testes de contrato.

## Consequences

- Agentes `trackfw-*` injetados no Antigravity passam a ser aceitos pelo `agy` e ganham capacidade de escrita/execucao.
- O mapeamento de model e um ponto de manutencao: se o agy mudar os tiers (`flash_lite|flash|pro`), atualizar o mapa.
- A lista de IDs validos e acoplada a build do agy; documentada aqui e nos testes de contrato como fonte de verdade ate o agy publicar um schema estavel.
