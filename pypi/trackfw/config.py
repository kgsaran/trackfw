"""
config.py — Leitura de trackfw.yaml, espelhando npm/src/config/index.js.

Parseia com PyYAML e normaliza todo escalar para string na fronteira, para que Go
(gopkg.in/yaml.v3), Node (yaml 2.x) e Python (PyYAML) concordem byte a byte no que chega aos
consumidores — ver ADR-2026-08-02-parsing-de-config-por-biblioteca-yaml-com-normalizacao-para-
string-na-fronteira.md.
"""

import os
import sys

import yaml

NAMESPACING_FLAT = "flat"
NAMESPACING_BY_AGENT = "by_agent"

# MALFORMED_CONFIG_MESSAGE is written to stderr, verbatim, when trackfw.yaml exists but fails to
# parse as YAML. Kept identical, character-for-character, to Go's MalformedConfigMessage and
# Node's MALFORMED_CONFIG_MESSAGE — see the comment on _parse() below for why the text is static
# rather than built from the underlying library's error.
MALFORMED_CONFIG_MESSAGE = (
    'trackfw: erro ao carregar "trackfw.yaml": YAML malformado. '
    "Corrija a sintaxe do arquivo antes de continuar."
)


def _resolve_alias_node(node):
    """PyYAML's yaml.compose() já resolve aliases de forma transparente: o nó de 'b' em
    'a: &x 3 / b: *x' é o MESMO objeto de nó que 'a' (identidade compartilhada), não um nó
    de alias separado com um nome. Diferente de Go e Node, não há passo extra a fazer aqui —
    esta função existe para documentar essa garantia e manter o mesmo formato de chamada dos
    outros dois CLIs."""
    return node


def _normalize_node(node):
    """Converte um nó bruto (ScalarNode/SequenceNode/MappingNode) do yaml.compose() em uma
    string (escalar, usando o texto pré-coerção via ScalarNode.value), uma lista de strings
    (sequência) ou um dict (mapeamento) — recursivamente.

    ScalarNode.value já devolve o texto correto tanto para escalares "plain" (não processados —
    preserva "yes", "010", "2026-08-02" como estão no arquivo) quanto para escalares
    quoted/bloco (já des-escapados) — confirmado empiricamente no ML-1A: não há necessidade de
    tratar quoted e plain de formas diferentes.
    """
    node = _resolve_alias_node(node)
    if isinstance(node, yaml.ScalarNode):
        return node.value
    if isinstance(node, yaml.SequenceNode):
        return [_normalize_node(child) for child in node.value]
    if isinstance(node, yaml.MappingNode):
        result = {}
        for key_node, val_node in node.value:
            key_node = _resolve_alias_node(key_node)
            key = key_node.value if isinstance(key_node, yaml.ScalarNode) else str(key_node)
            result[key] = _normalize_node(val_node)
        return result
    return None


def _string_list(val):
    """Converte um valor normalizado (list) em lista de strings. Uma sequência
    presente-porém-vazia devolve lista vazia (não None), distinguindo "presente e vazio" de
    "ausente" — contrato herdado do fix de lista inline."""
    if not isinstance(val, list):
        return None
    return [v for v in val if isinstance(v, str)]


_instance = None


def defaults():
    """Retorna dict com valores padrão de configuração."""
    return {
        # campos existentes
        "adr_dirs": ["docs/adr"],
        "strict_ci_paths": False,
        "req_dir": "docs/req",
        "roadmap_dir": "docs/roadmaps",
        "roadmap_namespacing": "flat",
        "agents": [],
        "governance_mode": "",
        "lenient_until": "",
        "wip_limit": 1,
        "wip_by_squad": False,
        "stale_wip_days": 7,
        "require_req_in_commit": False,
        # novos campos
        "trace_id_field": "",
        "forge": "",
        "link_fields": {
            "req":     ["REQ:"],
            "adr":     ["ADR:"],
            "roadmap": ["Roadmap:"],
        },
        "acceptance_markers": ["## Acceptance Criteria", "## Critérios de Aceite"],
        # ML-1A namespaces — ver ADR-2026-08-02-caminho-unico-de-leitura-do-trackfw-yaml-com-
        # namespaces-tipados.md. Chaves continuam planas na raiz do YAML; estes são agrupamentos
        # em memória populados pelo mesmo _parse() único abaixo, sem segunda leitura do arquivo.
        "update": {
            "hooks": "",
            "ci": "",
            "backend": "",
            "frontend": "",
            "pkg_manager": "",
        },
        "sync": {
            "linear_api_key": "",
            "linear_team_id": "",
            "jira_base_url": "",
            "jira_email": "",
            "jira_token": "",
            "jira_project": "",
        },
        "rules": {
            "wip_has_req":          "error",
            "wip_acceptance":       "error",
            "wip_limit":            "error",
            "stale_wip":            "warning",
            "adr_orphan":           "warning",
            "ref_targets_exist":    "error",
            "folder_status":        "warning",
            "filename_uniqueness":  "error",
            "blocked_by_draft_adr": "error",
            "adr_accepted_when_req_done": "error",
        },
    }


def load(cwd=None):
    """
    Carrega trackfw.yaml do diretório cwd (default: os.getcwd()).
    Singleton: segunda chamada retorna o mesmo objeto.
    """
    global _instance
    if _instance is not None:
        return _instance

    _instance = defaults()
    yaml_path = os.path.join(cwd or os.getcwd(), "trackfw.yaml")
    if not os.path.exists(yaml_path):
        return _instance

    with open(yaml_path, "r", encoding="utf-8") as f:
        content = f.read()

    malformed = _parse(content, _instance)
    if malformed:
        print(MALFORMED_CONFIG_MESSAGE, file=sys.stderr)
        sys.exit(1)
    return _instance


def reset():
    """Zera o singleton (útil em testes)."""
    global _instance
    _instance = None


def _parse(content, cfg):
    """Parseia content com yaml.compose (árvore de nós brutos, pré-coerção) e aplica as ~20
    chaves conhecidas em cfg. Chaves desconhecidas são ignoradas.

    Retorna True quando content é YAML malformado (quem chama, load(), transforma isso em
    mensagem fatal em stderr + sys.exit(1)) e False caso contrário — incluindo os casos benignos
    de documento ausente/vazio/só-comentários (yaml.compose devolve None sem levantar) ou um
    documento cujo nó de topo é sintaticamente válido mas não é um mapeamento (YAML válido,
    formato inesperado): nenhum dos dois é falha de parsing, então ambos continuam no-ops
    silenciosos, como antes desta função ganhar um canal de erro.
    """
    try:
        root = yaml.compose(content, Loader=yaml.SafeLoader)
    except yaml.YAMLError:
        return True

    if root is None:
        return False
    if not isinstance(root, yaml.MappingNode):
        return False

    m = _normalize_node(root)
    if not isinstance(m, dict):
        return False

    if "adr_dirs" in m:
        items = _string_list(m["adr_dirs"])
        if items is not None:
            cfg["adr_dirs"] = [os.path.expanduser(v) for v in items]
    if isinstance(m.get("req_dir"), str):
        cfg["req_dir"] = m["req_dir"]
    if isinstance(m.get("roadmap_dir"), str):
        cfg["roadmap_dir"] = m["roadmap_dir"]
    if isinstance(m.get("roadmap_namespacing"), str):
        cfg["roadmap_namespacing"] = m["roadmap_namespacing"]
    if "agents" in m:
        items = _string_list(m["agents"])
        if items is not None:
            cfg["agents"] = items
    if isinstance(m.get("governance_mode"), str):
        cfg["governance_mode"] = m["governance_mode"]
    if isinstance(m.get("lenient_until"), str):
        cfg["lenient_until"] = m["lenient_until"]
    if isinstance(m.get("wip_limit"), str):
        try:
            n = int(m["wip_limit"])
            if n > 0:
                cfg["wip_limit"] = n
        except ValueError:
            pass
    if isinstance(m.get("wip_by_squad"), str):
        cfg["wip_by_squad"] = m["wip_by_squad"] == "true"
    if isinstance(m.get("stale_wip_days"), str):
        try:
            n = int(m["stale_wip_days"])
            if n > 0:
                cfg["stale_wip_days"] = n
        except ValueError:
            pass
    if isinstance(m.get("require_req_in_commit"), str):
        cfg["require_req_in_commit"] = m["require_req_in_commit"] == "true"
    if isinstance(m.get("strict_ci_paths"), str):
        cfg["strict_ci_paths"] = m["strict_ci_paths"] == "true"
    if isinstance(m.get("trace_id_field"), str):
        cfg["trace_id_field"] = m["trace_id_field"]
    if isinstance(m.get("forge"), str):
        cfg["forge"] = m["forge"]
    if "acceptance_markers" in m:
        items = _string_list(m["acceptance_markers"])
        if items is not None:
            cfg["acceptance_markers"] = items
    if isinstance(m.get("link_fields"), dict):
        lf = m["link_fields"]
        req_items = _string_list(lf.get("req"))
        if req_items is not None:
            cfg["link_fields"]["req"] = req_items
        adr_items = _string_list(lf.get("adr"))
        if adr_items is not None:
            cfg["link_fields"]["adr"] = adr_items
        roadmap_items = _string_list(lf.get("roadmap"))
        if roadmap_items is not None:
            cfg["link_fields"]["roadmap"] = roadmap_items
    if isinstance(m.get("rules"), dict):
        for k, v in m["rules"].items():
            if isinstance(v, str):
                cfg["rules"][k] = v

    # ML-1A — namespaces update e sync. Mesmo dict normalizado m, sem segunda leitura.
    if isinstance(m.get("hooks"), str):
        cfg["update"]["hooks"] = m["hooks"]
    if isinstance(m.get("ci"), str):
        cfg["update"]["ci"] = m["ci"]
    if isinstance(m.get("backend"), str):
        cfg["update"]["backend"] = m["backend"]
    if isinstance(m.get("frontend"), str):
        cfg["update"]["frontend"] = m["frontend"]
    if isinstance(m.get("pkg_manager"), str):
        cfg["update"]["pkg_manager"] = m["pkg_manager"]
    if isinstance(m.get("linear_api_key"), str):
        cfg["sync"]["linear_api_key"] = m["linear_api_key"]
    if isinstance(m.get("linear_team_id"), str):
        cfg["sync"]["linear_team_id"] = m["linear_team_id"]
    if isinstance(m.get("jira_base_url"), str):
        cfg["sync"]["jira_base_url"] = m["jira_base_url"]
    if isinstance(m.get("jira_email"), str):
        cfg["sync"]["jira_email"] = m["jira_email"]
    if isinstance(m.get("jira_token"), str):
        cfg["sync"]["jira_token"] = m["jira_token"]
    if isinstance(m.get("jira_project"), str):
        cfg["sync"]["jira_project"] = m["jira_project"]

    return False
