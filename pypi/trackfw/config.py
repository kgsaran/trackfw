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

# MALFORMED_GLOBAL_CONFIG_MESSAGE is written to stderr (non-fatal) when ~/.trackfw/trackfw.yaml
# exists but fails to parse as valid YAML. Must be byte-identical to Go's MalformedGlobalConfigMessage
# and Node's MALFORMED_GLOBAL_CONFIG_MESSAGE — ADR-2026-08-23.
MALFORMED_GLOBAL_CONFIG_MESSAGE = (
    'trackfw: aviso: "~/.trackfw/trackfw.yaml" tem YAML malformado'
    " — config global de modelo ignorada; usando tier canônico."
)

# GLOBAL_AGENT_MODELS_NONE_MESSAGE is emitted to stderr when agent_models is not configured
# in the global config and not found in cwd's trackfw.yaml either.
# Must be byte-identical to Go's GlobalAgentModelsNoneMessage and Node's equivalent.
GLOBAL_AGENT_MODELS_NONE_MESSAGE = (
    "trackfw: agents global: agent_models não configurado em ~/.trackfw/trackfw.yaml"
    " — usando tier canônico. Configure em ~/.trackfw/trackfw.yaml para pinar versões."
)

# GLOBAL_AGENT_MODELS_PROJECT_ONLY_MESSAGE is emitted when agent_models is found in the
# project's trackfw.yaml but not in the global config (AC4/AC14). Value NOT applied.
# Must be byte-identical to Go's GlobalAgentModelsProjectOnlyMessage and Node's equivalent.
GLOBAL_AGENT_MODELS_PROJECT_ONLY_MESSAGE = (
    "trackfw: agents global: agent_models configurado em trackfw.yaml do projeto"
    " mas não vale para escopo global. Mova a chave para ~/.trackfw/trackfw.yaml."
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
            # ML-2B (agentes especialistas aceitam contexto de convenções) field
            "agent_conventions": "",
        },
        "sync": {
            "linear_api_key": "",
            "linear_team_id": "",
            "jira_base_url": "",
            "jira_email": "",
            "jira_token": "",
            "jira_project": "",
        },
        # credential_guard field — ver ADR-2026-08-05-hook-de-guarda-contra-materializacao-de-
        # credenciais-reais-por-subagentes.md.
        "credential_guard": {
            "mode": "warn",
        },
        # agent_models field — ver ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md.
        # Mapeia nome de tier (ex.: "opus", "sonnet") para string de versão (ex.: "5", "4.6").
        # Ausente ou vazio → comportamento idêntico ao atual (alias de tier usado verbatim).
        "agent_models": {},
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


def parse_rules_from_content(content):
    """Parseia só o mapeamento `rules:` de um conteúdo arbitrário de trackfw.yaml (ex.: um blob do
    git HEAD obtido via `git show HEAD:./trackfw.yaml`, não o arquivo do CWD que load() lê) e
    devolve como dict nome->severidade. Usado pelas regras de credential-guard ancoradas no HEAD do
    validator — ver ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-
    estrita-entre-head-e-disco.md — que precisam de `rules:` como existia numa ref específica do
    git, não no CWD, então não podem passar pelo singleton load() (sempre lê o arquivo do CWD e o
    cacheia por processo). Reaproveita _parse() sobre um cfg efêmero — espelha o
    ParseRulesFromContent do Go e o parseRulesFromContent do Node, para que este e load() nunca
    divirjam em como `rules:` em si é lido — puramente aditivo, não toca em load()/_parse().
    Conteúdo malformado (_parse() retornando True) é tratado como "sem rules:" — dict vazio —, já
    que o chamador só quer uma leitura best-effort de um blob histórico do git, não a saída fatal
    que load() tem para o arquivo do CWD ao vivo.
    """
    cfg = {
        "rules": {},
        "credential_guard": {},
        "update": {},
        "sync": {},
        "link_fields": {},
        "agent_models": {},
    }
    _parse(content, cfg)
    return cfg["rules"]


def read_agent_conventions(cwd=None):
    """Lê a chave `agent_conventions` diretamente de <cwd>/trackfw.yaml, contornando o
    singleton load() — mesmo padrão de isolamento de parse_rules_from_content, necessário
    porque os geradores (init_gen.py) injetam regras em arquivos de agente para um cwd que
    não é necessariamente o cwd do processo (contra o qual o singleton estaria cacheado).
    Arquivo ausente, ilegível, malformado ou simplesmente sem a chave são tratados da mesma
    forma: retorna "" silenciosamente, nunca uma exceção — é uma leitura best-effort de um
    campo opcional de texto livre. Espelha ReadAgentConventions (Go) e readAgentConventions
    (Node).
    """
    try:
        yaml_path = os.path.join(cwd or os.getcwd(), "trackfw.yaml")
        with open(yaml_path, "r", encoding="utf-8") as f:
            content = f.read()
        cfg = {
            "rules": {},
            "credential_guard": {},
            "update": {},
            "sync": {},
            "link_fields": {},
            "agent_models": {},
        }
        malformed = _parse(content, cfg)
        if malformed:
            return ""
        return cfg["update"].get("agent_conventions", "")
    except Exception:
        return ""


def _cwd_agent_models_source(cwd: str | None) -> str:
    """Returns 'project_only' if cwd's trackfw.yaml has agent_models configured, 'none' otherwise.
    Used for the AC14 diagnostic in load_global_agent_models.
    """
    if not cwd:
        return "none"
    try:
        yaml_path = os.path.join(cwd, "trackfw.yaml")
        with open(yaml_path, "r", encoding="utf-8") as f:
            content = f.read()
        cfg: dict = {
            "rules": {},
            "credential_guard": {},
            "update": {},
            "sync": {},
            "link_fields": {},
            "agent_models": {},
        }
        malformed = _parse(content, cfg)
        if not malformed and cfg.get("agent_models"):
            return "project_only"
    except Exception:
        pass
    return "none"


def load_global_agent_models(home_dir: str, cwd: str | None = None) -> tuple[dict, str]:
    """Reads agent_models from ~/.trackfw/trackfw.yaml, bypassing the load() singleton.

    home_dir is the user's home directory (e.g. os.path.expanduser('~')); cwd is the
    working directory used only for the AC14 diagnostic (detect 'configured in project,
    not global'). Never calls sys.exit. Pattern mirrors read_agent_conventions above.

    Returns (models, source) where source is one of:
      'global'            — agent_models resolved from ~/.trackfw/trackfw.yaml
      'none'              — global config absent or has no agent_models; cwd also has none
      'project_only'      — agent_models in cwd's trackfw.yaml but not global (AC14 trigger)
      'global_malformed'  — ~/.trackfw/trackfw.yaml exists but has invalid YAML (AC12 trigger)
    """
    global_path = os.path.join(home_dir, ".trackfw", "trackfw.yaml")
    try:
        with open(global_path, "r", encoding="utf-8") as f:
            data = f.read()
    except Exception:
        # Global file absent or unreadable → check cwd for AC14 diagnostic.
        return {}, _cwd_agent_models_source(cwd)

    # Validate YAML (non-fatal for global config, per AC12).
    cfg: dict = {
        "rules": {},
        "credential_guard": {},
        "update": {},
        "sync": {},
        "link_fields": {},
        "agent_models": {},
    }
    malformed = _parse(data, cfg)
    if malformed:
        return {}, "global_malformed"

    if cfg.get("agent_models"):
        return dict(cfg["agent_models"]), "global"

    # Global file exists but has no agent_models → check cwd (AC14).
    return {}, _cwd_agent_models_source(cwd)


def resolve_agent_models(scope: str, home_dir: str, cwd: str | None = None) -> tuple[dict, str]:
    """Returns (models, warn_msg) for the given scope.

    For global scope, reads from ~/.trackfw/trackfw.yaml via load_global_agent_models.
    For project scope, reads from the cwd's trackfw.yaml via load().
    warn_msg is non-empty when the caller should emit an advisory to stderr.
    resolve_agent_models never writes to stderr itself.
    """
    if scope != "global":
        return dict(load().get("agent_models", {})), ""

    models, source = load_global_agent_models(home_dir, cwd)
    if source == "none":
        return models, GLOBAL_AGENT_MODELS_NONE_MESSAGE
    if source == "project_only":
        return models, GLOBAL_AGENT_MODELS_PROJECT_ONLY_MESSAGE
    if source == "global_malformed":
        return models, MALFORMED_GLOBAL_CONFIG_MESSAGE
    return models, ""  # source == "global"


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
    if isinstance(m.get("agent_conventions"), str):
        cfg["update"]["agent_conventions"] = m["agent_conventions"]
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

    # credential_guard field — mapeamento aninhado, mesmo formato de link_fields acima. Um valor
    # de mode não reconhecido (ou um bloco credential_guard ausente/malformado) cai para o default
    # seguro ("warn") silenciosamente, igual ao comportamento de todo outro campo de formato não
    # reconhecido neste parser (ex.: roadmap_namespacing, forge) — sem caminho fatal, sem mensagem
    # em stderr, para uma única chave de enum.
    if isinstance(m.get("credential_guard"), dict):
        cg = m["credential_guard"]
        mode = cg.get("mode")
        if isinstance(mode, str) and mode in ("warn", "block"):
            cfg["credential_guard"]["mode"] = mode

    # agent_models field — mapeamento plano de nome de tier para string de versão.
    # Ver ADR-2026-08-21-versao-do-modelo-por-tier-com-composicao-por-alvo.md.
    # Um bloco ausente, malformado ou vazio deixa agent_models como o dict vazio dos defaults(),
    # preservando comportamento idêntico ao atual. Um valor de string vazia é armazenado como-está
    # (a camada de render trata string vazia como "sem pin" — cai no alias de tier).
    if isinstance(m.get("agent_models"), dict):
        for k, v in m["agent_models"].items():
            if isinstance(v, str):
                cfg["agent_models"][k] = v

    return False
