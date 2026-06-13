"""
help_cmd.py — Comando `trackfw help [key]`.
Lista todas as keys configuráveis ou exibe doc completa de uma key específica.
Arquivo nomeado help_cmd.py para evitar conflito com o builtin Python `help`.
"""

import sys

CONFIG_DOCS = {
    "adr_dirs": {
        "type": "list of strings",
        "default": '["docs/adr"]',
        "description": "Diretórios onde os ADRs são armazenados.",
        "example": "adr_dirs:\n  - docs/adr\n  - docs/architecture/decisions",
        "impact": "Afeta todas as verificações de ADR (órfão, draft, referência).",
    },
    "req_dir": {
        "type": "string",
        "default": '"docs/req"',
        "description": "Diretório onde as REQs são armazenadas.",
        "example": "req_dir: docs/requirements",
        "impact": "Afeta busca de REQs em validate, status e sync.",
    },
    "roadmap_dir": {
        "type": "string",
        "default": '"docs/roadmaps"',
        "description": "Diretório raiz dos roadmaps.",
        "example": "roadmap_dir: docs/planning",
        "impact": "Afeta listagem, movimentação e validação de roadmaps.",
    },
    "roadmap_namespacing": {
        "type": "flat|by_agent",
        "default": '"flat"',
        "description": "Estratégia de namespacing dos roadmaps.",
        "example": "roadmap_namespacing: by_agent",
        "impact": "by_agent cria subpastas por agente (ex: docs/roadmaps/zeus/wip/).",
    },
    "agents": {
        "type": "list of strings",
        "default": "[]",
        "description": "Lista de agentes ativos no projeto.",
        "example": "agents:\n  - zeus\n  - apolo\n  - afrodite",
        "impact": "Usado em namespacing by_agent e no relatório trackfw context.",
    },
    "governance_mode": {
        "type": "string",
        "default": '""',
        "description": "Modo de governança (strict, lenient).",
        "example": "governance_mode: lenient",
        "impact": "Em modo lenient, violations viram warnings e o exit code é 0.",
    },
    "lenient_until": {
        "type": "date (YYYY-MM-DD)",
        "default": '""',
        "description": "Data até quando o modo lenient está ativo.",
        "example": "lenient_until: 2026-07-01",
        "impact": "Após a data, o modo strict volta automaticamente.",
    },
    "wip_limit": {
        "type": "integer",
        "default": "1",
        "description": "Limite de itens WIP simultâneos.",
        "example": "wip_limit: 3",
        "impact": "Aumentar reduz a frequência de bloqueio.",
    },
    "wip_by_squad": {
        "type": "boolean",
        "default": "false",
        "description": "Aplicar limite WIP por squad individualmente.",
        "example": "wip_by_squad: true",
        "impact": "Cada squad tem seu próprio contador; roadmaps precisam do campo squad: no frontmatter.",
    },
    "require_req_in_commit": {
        "type": "boolean",
        "default": "false",
        "description": "Exigir referência de REQ em mensagens de commit.",
        "example": "require_req_in_commit: true",
        "impact": "Ativa hook commit-msg que rejeita commits sem REQ-YYYY-MM-DD-* em branches feat/fix.",
    },
    "link_fields.req": {
        "type": "list of strings",
        "default": '["REQ:"]',
        "description": "Marcadores que identificam link a REQ.",
        "example": "link_fields:\n  req:\n    - REQ:\n    - req_id:",
        "impact": "Afeta validate_wip_has_req e validate_reqs_have_adr.",
    },
    "link_fields.adr": {
        "type": "list of strings",
        "default": '["ADR:"]',
        "description": "Marcadores que identificam link a ADR.",
        "example": "link_fields:\n  adr:\n    - ADR:\n    - decision:",
        "impact": "Afeta validate_reqs_have_adr e validate_adrs_are_referenced.",
    },
    "link_fields.roadmap": {
        "type": "list of strings",
        "default": '["Roadmap:"]',
        "description": "Marcadores que identificam link a Roadmap.",
        "example": "link_fields:\n  roadmap:\n    - Roadmap:\n    - roadmap_ref:",
        "impact": "Afeta validate_reqs_have_roadmap e ref_targets_exist.",
    },
    "acceptance_markers": {
        "type": "list of strings",
        "default": '["## Acceptance Criteria", "## Critérios de Aceite"]',
        "description": "Marcadores de critério de aceite.",
        "example": "acceptance_markers:\n  - ## Done When\n  - ## Critérios",
        "impact": "Afeta validate_wip_has_acceptance_criteria.",
    },
    "rules.wip_has_req": {
        "type": "off|warning|error",
        "default": '"error"',
        "description": "Severidade: WIP sem REQ linkada.",
        "example": "rules:\n  wip_has_req: warning",
        "impact": "error → violation + exit 1; warning → aviso + exit 0; off → ignorado.",
    },
    "rules.wip_acceptance": {
        "type": "off|warning|error",
        "default": '"error"',
        "description": "Severidade: WIP sem critérios de aceite.",
        "example": "rules:\n  wip_acceptance: warning",
        "impact": "error → violation + exit 1; warning → aviso + exit 0; off → ignorado.",
    },
    "rules.wip_limit": {
        "type": "off|warning|error",
        "default": '"error"',
        "description": "Severidade: excesso de itens WIP.",
        "example": "rules:\n  wip_limit: warning",
        "impact": "error → violation + exit 1; warning → aviso + exit 0; off → ignorado.",
    },
    "rules.stale_wip": {
        "type": "off|warning|error",
        "default": '"warning"',
        "description": "Severidade: WIP sem atualização recente.",
        "example": "rules:\n  stale_wip: error",
        "impact": "Considera stale após 7 dias sem modificação (git log ou mtime).",
    },
    "rules.adr_orphan": {
        "type": "off|warning|error",
        "default": '"warning"',
        "description": "Severidade: ADR sem REQ vinculada.",
        "example": "rules:\n  adr_orphan: off",
        "impact": "error → violation + exit 1; warning → aviso + exit 0; off → ignorado.",
    },
    "rules.ref_targets_exist": {
        "type": "off|warning|error",
        "default": '"warning"',
        "description": "Severidade: referências com destino inexistente.",
        "example": "rules:\n  ref_targets_exist: error",
        "impact": "Verifica se os paths referenciados em REQ:/ADR:/Roadmap: existem no filesystem.",
    },
    "rules.folder_status": {
        "type": "off|warning|error",
        "default": '"warning"',
        "description": "Severidade: coerência entre pasta e status do arquivo.",
        "example": "rules:\n  folder_status: error",
        "impact": "Alerta quando o status: no frontmatter diverge da pasta (ex: wip/ com status: backlog).",
    },
    "rules.filename_uniqueness": {
        "type": "off|warning|error",
        "default": '"error"',
        "description": "Severidade: nomes de arquivo duplicados.",
        "example": "rules:\n  filename_uniqueness: warning",
        "impact": "Detecta o mesmo basename em múltiplos estados (ex: wip/ e done/).",
    },
    "rules.blocked_by_draft_adr": {
        "type": "off|warning|error",
        "default": '"error"',
        "description": "Severidade: REQ bloqueada por ADR em rascunho.",
        "example": "rules:\n  blocked_by_draft_adr: warning",
        "impact": "Verifica REQs Open com ADR em Status: Draft na seção Blocked by ADRs.",
    },
    "trace_id_field": {
        "type": "string",
        "default": '""',
        "description": "Campo de frontmatter usado como ID de rastreabilidade estável entre REQ e Roadmap. Vazio = desativado.",
        "example": "trace_id_field: req_id",
        "impact": "Ativa verificação bidirecional REQ↔Roadmap (traceid_orphan_*, traceid_state_mismatch, traceid_duplicate_*).",
    },
    "rules.traceid_orphan_roadmap": {
        "type": "off|warning|error",
        "default": '"error"',
        "description": "Roadmap com req_id sem REQ correspondente.",
        "example": "rules:\n  traceid_orphan_roadmap: warning",
        "impact": "Detecta Roadmaps sem REQ pareada.",
    },
    "rules.traceid_orphan_req": {
        "type": "off|warning|error",
        "default": '"error"',
        "description": "REQ com req_id sem Roadmap correspondente.",
        "example": "rules:\n  traceid_orphan_req: warning",
        "impact": "Detecta REQs sem Roadmap pareado.",
    },
    "rules.traceid_state_mismatch": {
        "type": "off|warning|error",
        "default": '"error"',
        "description": "REQ e Roadmap com mesmo req_id em estados divergentes.",
        "example": "rules:\n  traceid_state_mismatch: warning",
        "impact": "Garante consistência de estado.",
    },
    "rules.traceid_duplicate_req": {
        "type": "off|warning|error",
        "default": '"error"',
        "description": "Mesmo req_id em mais de uma REQ.",
        "example": "rules:\n  traceid_duplicate_req: warning",
        "impact": "Garante unicidade lógica de REQs.",
    },
    "rules.traceid_duplicate_roadmap": {
        "type": "off|warning|error",
        "default": '"error"',
        "description": "Mesmo req_id em mais de um Roadmap.",
        "example": "rules:\n  traceid_duplicate_roadmap: warning",
        "impact": "Garante unicidade lógica de Roadmaps.",
    },
}

_COL_KEY = 24
_COL_DEFAULT = 33
_SEP = "─" * 72


def list_keys() -> str:
    """Retorna a string formatada da tabela com todas as keys configuráveis."""
    header = f"{'KEY':<{_COL_KEY}} {'DEFAULT':<{_COL_DEFAULT}} DESCRIÇÃO"
    rows = [header, _SEP]
    for key, info in CONFIG_DOCS.items():
        desc = info["description"]
        rows.append(f"{key:<{_COL_KEY}} {info['default']:<{_COL_DEFAULT}} {desc}")
    return "\n".join(rows)


def describe_key(key: str):
    """Retorna a doc completa da key, ou None se desconhecida."""
    info = CONFIG_DOCS.get(key)
    if info is None:
        return None
    lines = [
        key,
        f"  Type:    {info['type']}",
        f"  Default: {info['default']}",
        f"  Desc:    {info['description']}",
        f"  Example:",
    ]
    for line in info["example"].split("\n"):
        lines.append(f"    {line}")
    lines.append(f"  Impact:  {info['impact']}")
    return "\n".join(lines)


def register(subparsers):
    """Registra o subcomando 'help' no parser principal."""
    parser = subparsers.add_parser(
        "help",
        help="Lista as keys configuráveis ou exibe doc de uma key específica",
    )
    parser.add_argument(
        "key",
        nargs="?",
        default=None,
        help="Nome da chave de configuração (opcional)",
    )
    parser.set_defaults(func=run)
    return parser


def run(args):
    """Executa o comando help."""
    if args.key is None:
        print(list_keys())
    else:
        result = describe_key(args.key)
        if result is None:
            print(f"chave desconhecida: {args.key}")
            sys.exit(1)
        print(result)
