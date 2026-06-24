"""
generators/init_gen.py — scaffold de governança trackfw em Python puro.
Espelha npm/src/generators/init.js com suporte a namespacing flat e by_agent.
Depende apenas de stdlib.
"""

import os
import stat
from datetime import date, timedelta


# ---------------------------------------------------------------------------
# Constantes
# ---------------------------------------------------------------------------

RULES_START = '<!-- trackfw:rules:start -->'
RULES_END = '<!-- trackfw:rules:end -->'

AGENT_FILES = {
    'claude':   'CLAUDE.md',
    'codex':    'AGENTS.md',
    'gemini':   'GEMINI.md',
    'copilot':  '.github/copilot-instructions.md',
    'windsurf': '.windsurfrules',
    'amazonq':  '.amazonq/developer/guidelines.md',
    'cursor':   '.cursor/rules/trackfw.mdc',
}

AGENT_HEADERS = {
    'claude':   '# Project Instructions\n',
    'codex':    '# Project Instructions\n',
    'gemini':   '# Project Instructions\n',
    'copilot':  '# GitHub Copilot Instructions\n',
    'windsurf': '# Windsurf Rules\n',
    'amazonq':  '# Amazon Q Developer Guidelines\n',
    'cursor':   '---\ndescription: trackfw governance rules\nglob: "**/*"\nalwaysApply: true\n---\n',
}

GOV_DIRS_FLAT = [
    'docs/adr',
    'docs/req',
    'docs/roadmaps/backlog',
    'docs/roadmaps/analyzing',
    'docs/roadmaps/wip',
    'docs/roadmaps/blocked',
    'docs/roadmaps/done',
    'docs/roadmaps/abandoned',
]

ROADMAP_STATES = ['backlog', 'analyzing', 'wip', 'blocked', 'done', 'abandoned']


# ---------------------------------------------------------------------------
# Função principal
# ---------------------------------------------------------------------------

def scaffold(cwd: str, opts: dict) -> None:
    """
    Cria a estrutura de governança trackfw no diretório cwd.

    opts esperado:
    {
        "project_name": str,
        "namespacing": "flat" | "by_agent",
        "agents": list[str],   # usado somente se namespacing == "by_agent"
        "wip_limit": int,
    }
    """
    namespacing = opts.get('namespacing', 'flat')
    agents = opts.get('agents', [])
    wip_limit = opts.get('wip_limit', 1)

    if namespacing == 'by_agent':
        dirs = _gov_dirs_by_agent(agents)
    else:
        dirs = GOV_DIRS_FLAT

    for d in dirs:
        abs_dir = os.path.join(cwd, d)
        os.makedirs(abs_dir, exist_ok=True)
        print(f'  checkmark {d}')

    _write_trackfw_yaml(cwd, opts)
    _write_example_adr(cwd, opts)
    generate_claude_commands(cwd)
    _generate_attention_scripts(cwd)
    print_architect_next_steps(cwd)


# ---------------------------------------------------------------------------
# Helpers de estrutura de diretórios
# ---------------------------------------------------------------------------

def _gov_dirs_by_agent(agents: list) -> list:
    """
    Retorna a lista de diretórios para o modo by_agent.
    docs/req é sempre flat (não por agente).
    """
    dirs = []
    for agent in agents:
        dirs.append(f'docs/adr/{agent}')
    dirs.append('docs/req')
    for agent in agents:
        for state in ROADMAP_STATES:
            dirs.append(f'docs/roadmaps/{agent}/{state}')
    return dirs


# ---------------------------------------------------------------------------
# trackfw.yaml
# ---------------------------------------------------------------------------

def _write_trackfw_yaml(cwd: str, opts: dict) -> None:
    namespacing = opts.get('namespacing', 'flat')
    agents = opts.get('agents', [])
    wip_limit = opts.get('wip_limit', 1)
    today = date.today().isoformat()

    lines = [
        '# trackfw configuration',
        f'# generated: {today}',
        '',
    ]

    if namespacing == 'by_agent':
        lines.append('adr_dirs:')
        for agent in agents:
            lines.append(f'  - docs/adr/{agent}')
    else:
        lines.append('adr_dirs:')
        lines.append('  - docs/adr')

    lines.append('req_dir: docs/req')
    lines.append('roadmap_dir: docs/roadmaps')
    lines.append(f'roadmap_namespacing: {namespacing}')

    if namespacing == 'by_agent' and agents:
        lines.append('agents:')
        for agent in agents:
            lines.append(f'  - {agent}')

    lines.append(f'wip_limit: {wip_limit}')
    lines.append('')  # newline final

    content = '\n'.join(lines)
    dest = os.path.join(cwd, 'trackfw.yaml')
    with open(dest, 'w', encoding='utf-8') as f:
        f.write(content)
    print('  checkmark trackfw.yaml')


# ---------------------------------------------------------------------------
# ADR exemplo
# ---------------------------------------------------------------------------

def _write_example_adr(cwd: str, opts: dict) -> None:
    """
    Cria docs/adr/ADR-001-inicio-do-projeto.md como arquivo exemplo.
    No modo by_agent cria no diretório do primeiro agente (se houver).
    """
    namespacing = opts.get('namespacing', 'flat')
    agents = opts.get('agents', [])

    if namespacing == 'by_agent' and agents:
        adr_dir = os.path.join(cwd, 'docs', 'adr', agents[0])
    else:
        adr_dir = os.path.join(cwd, 'docs', 'adr')

    os.makedirs(adr_dir, exist_ok=True)

    today = date.today().isoformat()
    filename = 'ADR-001-inicio-do-projeto.md'
    filepath = os.path.join(adr_dir, filename)

    # Idempotente: não sobrescreve se já existir
    if os.path.exists(filepath):
        return

    content = f"""---
name: ADR-001-inicio-do-projeto
title: "Início do projeto"
status: Proposed
date: {today}
---

# ADR-001: Início do projeto

## Status
Proposed

## Context
<!-- Descreva o contexto e o problema que motivou esta decisão -->

## Decision
<!-- Descreva a decisão tomada -->

## Consequences
<!-- Descreva as consequências desta decisão -->
"""

    with open(filepath, 'w', encoding='utf-8') as f:
        f.write(content)

    rel = os.path.relpath(filepath, cwd)
    print(f'  checkmark {rel}')


# ---------------------------------------------------------------------------
# trackfw rules inject-or-update
# ---------------------------------------------------------------------------

def _trackfw_rules_block() -> str:
    return (
        RULES_START + '\n'
        '## trackfw — Governance Rules\n\n'
        'This project uses **trackfw** for AI-native delivery governance.\n'
        'Chain: `ADR → REQ → ROADMAP` · States: `backlog / analyzing / wip / blocked / done / abandoned`\n\n'
        '### Agent Protocol\n'
        '1. **Before starting:** run `trackfw context` · read `docs/agents-working-context.md`\n'
        '2. **After finishing:** update `docs/agents-working-context.md` with what changed\n'
        '3. **Before PR:** `trackfw validate` must pass\n'
        '4. **ML lifecycle — mandatory:**\n'
        '   - Starting a ML: edit roadmap `**Status:** ⬜ Pendente` → `**Status:** 🔄 Em andamento` + commit.\n'
        '   - Completing a ML: edit roadmap → `**Status:** ✅ Concluído` + include in ML commit.\n'
        '   - Analyzing a roadmap: move from `backlog/` to `analyzing/`; to `wip/` only when coding starts.\n\n'
        '### Attention Signal (when you need user input during a task)\n'
        'Write `docs/roadmaps/.trackfw-attention.json`:\n'
        '```json\n'
        '{"roadmap":"file.md","ml":"ML-1A","message":"what you need","level":"action_required","timestamp":"ISO8601Z"}\n'
        '```\n'
        'Delete the file when resolved. Visible as a live banner in `trackfw serve`.\n'
        '\n### Architecture Directives (mandatory)\n'
        '- **3-layer arch + no in-memory data:** frontend / backend / database; always DB + ORM — never arrays/globals\n'
        '- **Auth + Docker + .env from day 1:** never defer auth; containerize early; all config via env vars\n'
        '- **2-layer validation + API-first:** frontend (UX) + backend (security); define OpenAPI contract first\n'
        '- **Security wave + test coverage:** red-team review in every roadmap; TDD for critical; min 60%/80%\n'
        '- Use `/trackfw:architect` to define stack before the first REQ\n'
        + RULES_END
    )


def _inject_or_update_rules(file_path: str, header_if_new: str) -> None:
    os.makedirs(os.path.dirname(os.path.abspath(file_path)), exist_ok=True)

    block = _trackfw_rules_block()

    if not os.path.exists(file_path):
        content = header_if_new or ''
        if content and not content.endswith('\n'):
            content += '\n'
        content += '\n' + block + '\n'
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(content)
        return

    with open(file_path, 'r', encoding='utf-8') as f:
        content = f.read()

    start = content.find(RULES_START)
    if start == -1:
        if not content.endswith('\n'):
            content += '\n'
        content += '\n' + block + '\n'
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(content)
        return

    end = content.find(RULES_END, start)
    if end == -1:
        content += '\n' + block + '\n'
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(content)
        return

    new_content = content[:start] + block + content[end + len(RULES_END):]
    with open(file_path, 'w', encoding='utf-8') as f:
        f.write(new_content)


def inject_rules_for_tool(tool: str, cwd: str) -> None:
    rel_path = AGENT_FILES.get(tool)
    if not rel_path:
        return
    header = AGENT_HEADERS.get(tool, '')
    _inject_or_update_rules(os.path.join(cwd, rel_path), header)


def inject_rules_detected(cwd: str) -> None:
    for tool, rel_path in AGENT_FILES.items():
        if tool == 'cursor':
            if os.path.isdir(os.path.join(cwd, '.cursor')):
                try:
                    inject_rules_for_tool('cursor', cwd)
                except Exception:
                    pass
            continue
        if os.path.exists(os.path.join(cwd, rel_path)):
            try:
                inject_rules_for_tool(tool, cwd)
            except Exception:
                pass


def generate_claude_commands(cwd: str) -> None:
    """Instala os slash commands do trackfw em .claude/commands/trackfw/."""
    cmd_dir = os.path.join(cwd, '.claude', 'commands', 'trackfw')
    os.makedirs(cmd_dir, exist_ok=True)

    _install_not_found = (
        '\n\nSe o comando falhar com `trackfw: command not found` ou similar, informe ao usuário:\n\n'
        '```\n'
        'trackfw não está instalado. Instale com uma das opções:\n\n'
        '  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh\n'
        '  npm install -g trackfw\n'
        '  pip install trackfw\n'
        '```'
    )

    commands = {
        'adr.md': (
            'Execute o seguinte comando bash: `trackfw adr new "$ARGUMENTS"`'
            + _install_not_found
        ),
        'req.md': (
            'Execute o seguinte comando bash: `trackfw req new "$ARGUMENTS"`'
            + _install_not_found
        ),
        'validate.md': (
            'Execute o seguinte comando bash: `trackfw validate`'
            + _install_not_found
        ),
        'status.md': (
            'Execute o seguinte comando bash: `trackfw status`'
            + _install_not_found
        ),
        'move.md': (
            'Execute o seguinte comando bash: `trackfw roadmap move $ARGUMENTS`\n\n'
            'O formato esperado é: `<nome-do-roadmap> <estado>`\n\n'
            'Estados válidos: `backlog`, `wip`, `blocked`, `done`, `abandoned`\n\n'
            'Exemplo: `/trackfw:move meu-roadmap wip`\n\n'
            'Se o comando falhar com `trackfw: command not found` ou similar, informe ao usuário:\n'
            'trackfw não está instalado. Instale com:\n'
            '  curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh\n'
            '  npm install -g trackfw\n'
            '  pip install trackfw'
        ),
        'roadmap.md': (
            'Gere um roadmap de implementação em microlotes para uma REQ do projeto.\n\n'
            '## Passos\n\n'
            '1. **Listar REQs disponíveis**\n'
            '   Use Glob para listar `docs/req/*.md`. Se nenhum arquivo encontrado, informe:\n'
            '   > Nenhuma REQ encontrada em `docs/req/`. Crie uma primeiro com `/trackfw:req`.\n\n'
            '2. **Selecionar a REQ**\n'
            '   - Se `$ARGUMENTS` foi fornecido: use como filtro (substring case-insensitive) para encontrar o arquivo\n'
            '   - Se não foi fornecido ou o filtro não encontrar exatamente um: liste os arquivos disponíveis e pergunte ao usuário qual usar\n'
            '   - Leia o conteúdo completo do arquivo REQ selecionado\n\n'
            '3. **Gerar o roadmap**\n'
            '   Com base no conteúdo da REQ, gere um roadmap seguindo **estritamente** este formato:\n\n'
            '   ```markdown\n'
            '   # Roadmap: <título derivado da REQ>\n\n'
            '   > Criado em: <YYYY-MM-DD> | Status: ⬜ Backlog\n\n'
            '   ## Diagnóstico / Contexto\n'
            '   <resumo do problema, motivação e escopo extraídos da REQ>\n\n'
            '   ## Wave 1 — <nome descritivo> (<N> MLs em paralelo)\n'
            '   > Dependências: Independente\n\n'
            '   ### ML-1A — <título>\n'
            '   **Status:** ⬜ Pendente\n'
            '   **Arquivos afetados:**\n'
            '   - `caminho/exato/do/arquivo`\n'
            '   **Ações:**\n'
            '   - Descrição detalhada da ação com valores, chaves e comandos exatos\n'
            '   **Critérios de aceite:**\n'
            '   - [ ] build sem erros\n'
            '   - [ ] testes verdes\n'
            '   **Comandos de validação:** `<comando de build e teste do projeto>`\n'
            '   ```\n\n'
            '   **Princípios obrigatórios:**\n'
            '   - MLs dentro da mesma Wave são **independentes** (arquivos distintos, sem conflito)\n'
            '   - Cada ML deve ser detalhado o suficiente para execução por um agente sem contexto extra\n'
            '   - Maximizar paralelismo: agrupe em paralelo tudo que não compartilhar arquivos\n'
            '   - Waves sequenciais apenas quando há dependência real de resultado\n'
            '   - Critérios de aceite mensuráveis em cada ML\n\n'
            '4. **Salvar o arquivo**\n'
            '   - Calcule o slug: título em lowercase, espaços → hifens, remova caracteres especiais\n'
            '   - Crie o arquivo em `docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`\n'
            '   - Use a data de hoje\n\n'
            '5. **Confirmar**\n'
            '   Informe o caminho do arquivo criado e um resumo das Waves e total de MLs gerados.'
        ),
        'implement.md': (
            'Você é o orquestrador de implementação do trackfw. Siga o fluxo abaixo **sem pular etapas**.\n\n'
            '## Argumento\n\n'
            '`$ARGUMENTS` é opcional. Se fornecido, é usado como filtro (substring case-insensitive) sobre os nomes de arquivo das REQs.\n\n'
            '---\n\n'
            '## Passo 1 — Selecionar a REQ\n\n'
            'Use Glob para listar `docs/req/*.md`.\n\n'
            '- Se **nenhum arquivo encontrado**: informe que não há REQs disponíveis e sugira criar com `/trackfw:req`.\n'
            '- Se **`$ARGUMENTS` foi fornecido** e filtra para exatamente uma REQ: use-a diretamente.\n'
            '- Em **todos os outros casos** (sem argumento, ou argumento ambíguo): apresente a lista de REQs disponíveis e pergunte ao usuário qual deseja implementar.\n\n'
            'Leia o conteúdo completo da REQ selecionada.\n\n'
            '---\n\n'
            '## Passo 2 — Encontrar ou gerar o Roadmap\n\n'
            'Verifique se existe um roadmap vinculado à REQ buscando em `docs/roadmaps/` (backlog, wip, blocked, done, abandoned) por arquivo cujo nome contenha o slug da REQ.\n\n'
            '**Se o roadmap ainda não existe:**\n'
            '- Informe o usuário: "Nenhum roadmap encontrado para esta REQ. Gerando agora..."\n'
            '- Execute o fluxo completo de geração do `/trackfw:roadmap` (leia o arquivo `.claude/commands/trackfw/roadmap.md` para seguir as instruções exatas), passando a REQ já selecionada — não pergunte novamente.\n'
            '- Salve o roadmap gerado em `docs/roadmaps/backlog/ROADMAP-<YYYY-MM-DD>-<slug>.md`.\n\n'
            '**Se o roadmap existe e já está em `done/` ou `abandoned/`:**\n'
            '- Informe o usuário e pergunte se deseja criar um novo roadmap ou encerrar.\n\n'
            '**Se o roadmap existe em `backlog/` ou `blocked/`:**\n'
            '- Prossiga para o Passo 3.\n\n'
            '**Se já está em `wip/`:**\n'
            '- Prossiga diretamente para o Passo 4 (já está em execução).\n\n'
            '---\n\n'
            '## Passo 3 — Mover roadmap para WIP\n\n'
            'Execute:\n'
            '```bash\n'
            'trackfw roadmap move <nome-do-roadmap> wip\n'
            '```\n\n'
            'Confirme que o arquivo foi movido para `docs/roadmaps/wip/`.\n\n'
            '---\n\n'
            '## Passo 4 — Ler e apresentar o plano\n\n'
            'Leia o roadmap (agora em `wip/`). Apresente ao usuário:\n'
            '- Título do roadmap\n'
            '- Total de Waves e MLs\n'
            '- Lista resumida dos MLs por Wave\n\n'
            'Confirme: "Iniciando implementação. Vou executar cada ML em ordem e atualizar o roadmap a cada conclusão."\n\n'
            '---\n\n'
            '## Passo 5 — Executar cada ML em ordem\n\n'
            'Para cada Wave (em sequência), execute os MLs da Wave:\n\n'
            '### Para cada ML:\n\n'
            '**5a. Anunciar:** informe qual ML está sendo executado (ex: "Executando ML-1A — Criar client.go").\n\n'
            '**5b. Implementar:** execute as ações descritas no ML usando suas ferramentas (Read, Write, Edit, Bash). Siga exatamente os arquivos afetados, ações e critérios de aceite listados no roadmap.\n\n'
            '**5c. Validar:** execute os comandos de validação do ML. Se falhar, corrija antes de avançar.\n\n'
            '**5d. Atualizar o roadmap:** edite o arquivo de roadmap em `docs/roadmaps/wip/` substituindo o status do ML:\n'
            '- `**Status:** ⬜ Pendente` → `**Status:** ✅ Concluído`\n\n'
            '**5e. Commitar:**\n'
            '```bash\n'
            'git add -A\n'
            'git commit -m "feat(<escopo>): <descrição do ML>"\n'
            '```\n\n'
            'Só avance para a próxima Wave após todos os MLs da Wave atual estarem ✅.\n\n'
            '---\n\n'
            '## Passo 6 — Finalizar\n\n'
            'Quando todos os MLs estiverem ✅:\n\n'
            '**6a.** Execute `trackfw validate` — deve passar com zero violations.\n\n'
            '**6b.** Mova o roadmap para done:\n'
            '```bash\n'
            'trackfw roadmap move <nome-do-roadmap> done\n'
            '```\n\n'
            '**6c.** Faça o commit final:\n'
            '```bash\n'
            'git add docs/roadmaps/\n'
            'git commit -m "docs(trackfw): roadmap <nome> → done"\n'
            '```\n\n'
            '**6d.** Informe o usuário:\n'
            '```\n'
            '✅ Implementação concluída.\n'
            'Roadmap: docs/roadmaps/done/<nome>.md\n'
            'Próximo passo: abrir PR com gh pr create\n'
            '```'
        ),
        'architect.md': (
            'Você é o guia de arquitetura do trackfw. Ajude o usuário a escolher a stack correta e arquitetar a aplicação em linguagem simples, acessível para times não técnicos.\n\n'
            '## Passo 1 — Descoberta de Negócio\n\n'
            'Faça ao usuário as seguintes perguntas em linguagem simples, uma por vez:\n\n'
            '1. "O que sua aplicação vai fazer? Descreva em 2-3 frases como se fosse explicar para alguém de fora da TI."\n'
            '2. "Quantas pessoas vão usar esse sistema simultaneamente? (< 10 pessoas / 10-100 pessoas / > 100 pessoas)"\n'
            '3. "Esse sistema vai para produção de verdade ou é um protótipo para validar uma ideia?"\n'
            '4. "Você precisa de login/autenticação de usuários? (Sim / Não / Não sei)"\n'
            '5. "Tem alguma restrição de tecnologia ou preferência da empresa? (ex: só Java, só Microsoft, etc.)"\n\n'
            '---\n\n'
            '## Passo 2 — Recomendação de Stack\n\n'
            'Com base nas respostas, escolha **UM** dos combos pré-validados:\n\n'
            '### Combo A — Protótipo Rápido\n'
            '**Quando usar:** prototipagem, validação de ideia, até ~10 usuários, sem pressão de produção.\n'
            '- **Frontend:** React + Vite\n'
            '- **Backend:** FastAPI (Python) ou Express (Node.js)\n'
            '- **Banco:** SQLite + SQLAlchemy / Prisma\n'
            '- **Auth:** JWT simples quando necessário\n'
            '- **Docker:** Dockerfile básico para o backend\n\n'
            '### Combo B — Sistema Pequeno/Médio em Produção\n'
            '**Quando usar:** sistema real, 10-100 usuários, robustez e manutenibilidade.\n'
            '- **Frontend:** Next.js (SSR + rotas prontas)\n'
            '- **Backend:** FastAPI (Python) ou NestJS (Node.js)\n'
            '- **Banco:** PostgreSQL + ORM (SQLAlchemy / Prisma / TypeORM)\n'
            '- **Auth:** OAuth2 com JWT (Supabase Auth ou Auth0)\n'
            '- **Docker:** docker-compose com frontend + backend + banco\n\n'
            '### Combo C — Enterprise / Java\n'
            '**Quando usar:** integração com sistemas corporativos, > 100 usuários, exigência de Java.\n'
            '- **Frontend:** Angular\n'
            '- **Backend:** Spring Boot\n'
            '- **Banco:** PostgreSQL + Hibernate\n'
            '- **Auth:** Spring Security + OAuth2 (Keycloak ou Azure AD)\n'
            '- **Docker:** docker-compose com todos os serviços\n\n'
            'Apresente o combo recomendado com explicação simples do motivo.\n\n'
            '---\n\n'
            '## Passo 3 — Arquitetura em Camadas (explicação simples)\n\n'
            'Explique a arquitetura com uma metáfora de negócio:\n\n'
            '"Pense na aplicação como um restaurante:\n'
            '- **Frontend** = o salão: o que o cliente vê e interage\n'
            '- **Backend** = a cozinha: onde as regras de negócio acontecem, nunca exposta diretamente\n'
            '- **Banco de dados** = a despensa: onde os dados ficam guardados, acessada só pela cozinha"\n\n'
            'Reforce as **Architecture Directives** já injetadas no CLAUDE.md deste projeto: separação em 3 camadas sem dados em memória (sempre DB + ORM), auth + Docker + .env desde o dia 1, validação em 2 camadas, contrato OpenAPI antes de codar, wave de segurança em todo roadmap e cobertura mínima de testes (60% protótipo / 80% produção).\n\n'
            '---\n\n'
            '## Passo 4 — Gerar o ADR de Stack\n\n'
            'Execute `/trackfw:adr` com o título: `"Stack e arquitetura em camadas — [nome do projeto]"`\n\n'
            'O ADR deve registrar a stack escolhida (combo e componentes), motivação baseada nas respostas, alternativas descartadas e princípios de arquitetura adotados.\n\n'
            '---\n\n'
            '## Passo 5 — Próximos Passos\n\n'
            'Oriente o usuário:\n\n'
            '```\n'
            '✅ Stack definida. Próximos passos:\n\n'
            '1. Crie a REQ da primeira feature com /trackfw:req\n'
            '2. Gere o roadmap em microlotes com /trackfw:roadmap\n'
            '3. Inicie a implementação com /trackfw:implement\n'
            '```'
        ),
    }

    created = 0
    skipped = 0
    for filename, content in commands.items():
        file_path = os.path.join(cmd_dir, filename)
        if os.path.exists(file_path):
            skipped += 1
            continue
        with open(file_path, 'w', encoding='utf-8') as f:
            f.write(content)
        created += 1

    if skipped > 0:
        print(f'  ✓ .claude/commands/trackfw/ ({created} slash commands criados, {skipped} já existiam)')
    else:
        print(f'  ✓ .claude/commands/trackfw/ ({created} slash commands)')


_ATTENTION_SIGNAL_SH = r"""#!/usr/bin/env bash
# trackfw attention signal — permission/notification hook
# Writes .trackfw-attention.json so trackfw serve board shows a banner.
set -euo pipefail

INPUT=$(cat)
HOOK_CWD=$(echo "$INPUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('cwd',''))" 2>/dev/null || echo "")
[ -n "$HOOK_CWD" ] && cd "$HOOK_CWD"
[ -f "trackfw.yaml" ] || exit 0

if command -v jq &>/dev/null; then
  TOOL=$(echo "$INPUT" | jq -r '.tool_name // .notification_type // ""')
  MSG=$(echo "$INPUT" | jq -r '(.message // .tool_input.description // .tool_input.question // .tool_input.command // ("Approval required for: " + (.tool_name // .notification_type // "unknown"))) | .[0:300]')
else
  TOOL=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('tool_name') or d.get('notification_type') or '')" 2>/dev/null || echo "")
  MSG=$(echo "$INPUT" | python3 -c "import sys,json; d=json.load(sys.stdin); ti=d.get('tool_input',{}); print((d.get('message') or ti.get('description') or ti.get('question') or ti.get('command') or 'Approval required for: '+(d.get('tool_name') or d.get('notification_type') or 'unknown'))[:300])" 2>/dev/null || echo "Agent needs attention")
fi

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | awk '{print $2}' | tr -d "\"'" | head -1)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"%s","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$(echo "$TOOL" | sed 's/"/\\"/g')" \
  "$(echo "$MSG"  | sed 's/"/\\"/g; s/$//' | tr -d '\n')" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-attention.json"

exit 0
"""

_ATTENTION_CLEANUP_SH = r"""#!/usr/bin/env bash
# trackfw attention cleanup — PostToolUse/AfterTool hook
set -euo pipefail

INPUT=$(cat)
HOOK_CWD=$(echo "$INPUT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('cwd',''))" 2>/dev/null || echo "")
[ -n "$HOOK_CWD" ] && cd "$HOOK_CWD"
[ -f "trackfw.yaml" ] || exit 0

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | awk '{print $2}' | tr -d "\"'" | head -1)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

rm -f "$ROADMAP_DIR/.trackfw-attention.json"
exit 0
"""


def _generate_attention_scripts(cwd: str) -> None:
    """Gera scripts shell de attention signal/cleanup em scripts/."""
    scripts_dir = os.path.join(cwd, 'scripts')
    os.makedirs(scripts_dir, exist_ok=True)

    signal_path = os.path.join(scripts_dir, 'trackfw-attention-signal.sh')
    with open(signal_path, 'w', encoding='utf-8') as f:
        f.write(_ATTENTION_SIGNAL_SH.lstrip('\n'))
    os.chmod(signal_path, 0o755)

    cleanup_path = os.path.join(scripts_dir, 'trackfw-attention-cleanup.sh')
    with open(cleanup_path, 'w', encoding='utf-8') as f:
        f.write(_ATTENTION_CLEANUP_SH.lstrip('\n'))
    os.chmod(cleanup_path, 0o755)


def print_architect_next_steps(cwd: str) -> None:
    """Exibe instruções de próximo passo após init/update."""
    candidates = [
        ('CLAUDE.md',                              'claude'),
        ('.cursor/rules/trackfw.mdc',              'cursor .'),
        ('.windsurfrules',                         'windsurf .'),
        ('.github/copilot-instructions.md',        'code . (Copilot)'),
        ('.amazonq/developer/guidelines.md',       'code . (Amazon Q)'),
        ('GEMINI.md',                              'gemini'),
        ('AGENTS.md',                              'codex'),
    ]

    detected = [cmd for f, cmd in candidates if os.path.exists(os.path.join(cwd, f))]
    if not detected:
        detected = ['claude']

    print()
    print('Próximo passo — inicie com o guia de arquitetura:')
    print()
    for cmd in detected:
        print(f'  {cmd}')
    print()
    print('  Execute /trackfw:architect no chat do seu assistente de IA.')
    print()
