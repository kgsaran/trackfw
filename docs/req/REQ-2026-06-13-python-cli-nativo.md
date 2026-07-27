---
status: Done
date: 2026-06-13
author: zeus
adr: ""
roadmap: "docs/roadmaps/done/v2.2-python-cli-nativo-2026-06-13.md"
---

# REQ: Python CLI Nativo — Reimplementação em Python Puro

> Date: 2026-06-13 | Status: Done

## Motivação

O CLI Python atual (`pypi/trackfw/_cli.py`) é um wrapper que baixa o binário Go do GitHub em runtime.
Esse modelo falha em ambientes corporativos por dois motivos cumulativos:

1. **Mirror interno de PyPI** — o pacote precisa estar no Artifactory/Nexus interno
2. **GitHub bloqueado** — o download do binário em runtime é bloqueado por firewalls/EDR corporativos

O CLI Node.js (`npm/src/`) prova que a arquitetura correta é uma **reimplementação nativa** sem dependência de binário externo. O Python CLI deve seguir o mesmo modelo.

---

## Regra Dura: Paridade 3 CLIs

A partir desta REQ, toda feature implementada no Go CLI ou Node.js CLI **DEVE ter paridade completa no Python CLI** e vice-versa. Nenhum critério de aceite está satisfeito sem implementação nos **3 CLIs**.

---

## Critérios de Aceite

### Bloco A — Infraestrutura do pacote
- [ ] `pypi/trackfw/` substituído por implementação Python pura (sem `_cli.py` wrapper)
- [ ] `pyproject.toml` sem dependências externas (stdlib apenas: `argparse`, `pathlib`, `re`, `os`, `sys`)
- [ ] `pip install .` no diretório `pypi/` instala o comando `trackfw` funcional
- [ ] `python -m trackfw` também funciona

### Bloco B — Config (`pypi/trackfw/config.py`)
- [ ] `load(cwd)` lê `trackfw.yaml` com parse linha a linha (mesma lógica do Node.js)
- [ ] Defaults retrocompatíveis: `adr_dirs: ["docs/adr"]`, `req_dir: "docs/req"`, `roadmap_dir: "docs/roadmaps"`, `roadmap_namespacing: "flat"`, `wip_limit: 1`
- [ ] Singleton com `reset()` para testes

### Bloco C — Comandos (paridade com `npm/src/commands/`)
- [ ] `trackfw init` — scaffold de projeto com wizard interativo
- [ ] `trackfw adr new` — gera ADR com frontmatter
- [ ] `trackfw req new` — gera REQ com frontmatter
- [ ] `trackfw roadmap new/move/list/show` — gestão de roadmaps (flat e by_agent)
- [ ] `trackfw validate` — valida cadeia ADR→REQ→ROADMAP, WIP limit, stale WIP
- [ ] `trackfw status` — exibe resumo de governança (contagens por estado, agente)
- [ ] `trackfw log` — registra entrada no `.trackfw-log`
- [ ] `trackfw discover [--init] [--bootstrap-log]` — escaneia estrutura e gera `trackfw.yaml`
- [ ] `trackfw metrics` — métricas de throughput e cycle time
- [ ] `trackfw context` — exporta contexto para agentes de IA
- [ ] `trackfw sync` — sincroniza com fontes externas
- [ ] `trackfw plugins` — dispatch de plugins externos

### Bloco D — Validador (`pypi/trackfw/validator.py`)
- [ ] Mesma lógica do `npm/src/validator/index.js`: WIP limit, stale WIP, REQ linkada ao ADR, frontmatter obrigatório
- [ ] `governance_mode: lenient` não bloqueia — apenas warnings

### Bloco E — i18n (`pypi/trackfw/i18n/`)
- [ ] Suporte a pt-BR, en-US, es-ES (mesmos arquivos de locale do npm)
- [ ] Detecção automática via `LANG`/`LANGUAGE` env vars

### Bloco F — Qualidade
- [ ] `python -m pytest pypi/` verde (cobertura dos comandos principais)
- [ ] Nenhuma dependência externa (zero `pip install X`)
- [ ] Compatível com Python 3.8+

---

## Fora de Escopo
- `trackfw serve` — exclusivo do CLI Go (HTTP server)
- Migração automática do wrapper antigo para usuários existentes
- Publicação no PyPI (tarefa operacional separada)

## Linked ADR

ADR:

## Linked Roadmap

Roadmap: v2.2-python-cli-nativo-2026-06-13.md
