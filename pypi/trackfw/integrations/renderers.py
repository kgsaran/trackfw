"""Native renderers for catalog assets."""

from __future__ import annotations

import json
from typing import Any

from trackfw import identity

# ---------------------------------------------------------------------------
# Constantes para renderização no formato agent-directory (Antigravity IDE/CLI)
# ---------------------------------------------------------------------------

# Mapa de modelos canônicos para valores aceitos pelo Antigravity CLI.
# opus→pro, sonnet→flash; flash_lite/flash/pro mantêm-se; demais são omitidos.
_MODEL_MAP: dict[str, str] = {
    "opus": "pro",
    "sonnet": "flash",
    "flash_lite": "flash_lite",
    "flash": "flash",
    "pro": "pro",
}

# SET_IMPL — conjunto base de 10 ferramentas (agentes não-architect)
_SET_IMPL: list[str] = [
    "view_file",
    "list_dir",
    "grep_search",
    "search_web",
    "read_url_content",
    "write_to_file",
    "replace_file_content",
    "run_command",
    "command_status",
    "generate_image",
]

# SET_ARCH — SET_IMPL + 4 ferramentas de orquestração (agente architect)
_SET_ARCH: list[str] = _SET_IMPL + [
    "send_message",
    "define_subagent",
    "invoke_subagent",
    "schedule",
]


def _map_model(model: str) -> str | None:
    """Converte modelo canônico para valor aceito pelo Antigravity CLI.

    Retorna o modelo mapeado ou None se a linha model deve ser omitida.
    """
    return _MODEL_MAP.get(model)


def _agent_tools(item_id: str) -> list[str]:
    """Retorna SET_ARCH se item_id == "architect", caso contrário SET_IMPL.

    A decisão é feita pelo id canônico do catálogo (ex.: "architect"), não
    pelo nome renderizado — que pode ser customizado por identidade (ex.:
    "zeus-tf") e não deve influenciar a seleção do toolset (ADR D8).
    """
    if item_id == "architect":
        return _SET_ARCH
    return _SET_IMPL


# ---------------------------------------------------------------------------
# Parser de frontmatter
# ---------------------------------------------------------------------------


def _parts(source: str) -> tuple[dict[str, str], str]:
    metadata: dict[str, str] = {}
    if not source.startswith("---\n"):
        return metadata, source
    marker = source.find("\n---\n", 4)
    if marker < 0:
        return metadata, source
    for line in source[4:marker].splitlines():
        if ":" in line:
            key, value = line.split(":", 1)
            metadata[key.strip()] = value.strip().strip('"')
    return metadata, source[marker + 5 :].lstrip()


# ---------------------------------------------------------------------------
# Injeção de identidade — Rota A (name/description/body já separados) e
# Rota B (frontmatter cru, usada pela representação default)
# ---------------------------------------------------------------------------


def _greeting_line(agent: identity.AgentIdentity, nickname: str) -> str:
    """Primeira linha injetada no corpo do agente quando há identidade
    configurada. Sem apelido configurado, menciona só o display_name."""
    if not nickname:
        return f"You are {agent.display_name}."
    return f"You are {agent.display_name}. Address the user as {nickname}."


def _insert_body_prefix(source: str, prefix: str) -> str:
    """Insere prefix como nova primeira linha do corpo de um markdown cru
    (frontmatter + body), seguido de linha em branco. Se source não tem
    frontmatter reconhecível, prefix é inserido no topo."""
    trimmed = source.strip()
    if not prefix:
        return trimmed
    if not trimmed.startswith("---\n"):
        return f"{prefix}\n\n{trimmed}"
    end = trimmed.find("\n---", 4)
    if end < 0:
        return f"{prefix}\n\n{trimmed}"
    insert_at = end + 4
    head = trimmed[:insert_at]
    rest = trimmed[insert_at:].lstrip("\n")
    if not rest:
        return f"{head}\n\n{prefix}"
    return f"{head}\n\n{prefix}\n\n{rest}"


def _rewrite_signature_line(source: str, display_name: str) -> str:
    """Reescreve a última linha da seção de corpo de um markdown cru que casa
    com o padrão de assinatura ``^— <nome>, <título>$`` (travessão em-dash
    U+2014, espaço, nome, vírgula, espaço, título). Apenas o primeiro grupo
    (o nome do agente) é substituído por display_name; o título é preservado
    byte a byte.

    Escopo: opera somente no corpo (após o fechamento do frontmatter). Uma
    linha de assinatura dentro do frontmatter nunca é tocada — a detecção de
    fronteira espelha _rewrite_frontmatter_fields exatamente.

    Se nenhuma linha do corpo casar com o padrão, source é retornado inalterado.
    Se display_name for vazio, source é retornado inalterado. A função nunca
    inventa uma assinatura que não estava presente.

    Espelha internal/integrations/render.go:rewriteSignatureLine.
    """
    if not display_name:
        return source
    trimmed = source.strip()

    # Localiza o início do corpo — espelha _rewrite_frontmatter_fields para que
    # o escopo de ambas as funções coincida.
    body_start = 0
    if trimmed.startswith("---\n"):
        end = trimmed.find("\n---", 4)
        if end >= 0:
            body_start = end + 4  # char imediatamente após "\n---"

    head = trimmed[:body_start]
    body_section = trimmed[body_start:]

    lines = body_section.split("\n")
    # Percorre de trás para frente para encontrar a ÚLTIMA linha candidata.
    for i in range(len(lines) - 1, -1, -1):
        line = lines[i]
        prefix = "— "  # em-dash + espaço
        if not line.startswith(prefix):
            continue
        rest = line[len(prefix):]  # pula "— "
        comma_idx = rest.find(", ")
        if comma_idx < 0:
            continue
        title = rest[comma_idx + 2:]
        if not title:
            continue
        lines[i] = f"— {display_name}, {title}"
        return head + "\n".join(lines)

    # Nenhuma linha de assinatura encontrada — retorna source inalterado.
    return source


def _rewrite_frontmatter_fields(source: str, name: str, description: str) -> str:
    """Substitui as linhas "name:" e "description:" do frontmatter de um
    markdown cru por name e description, preservando as demais linhas
    (ordem, espaçamento, estilo de aspas) e o corpo intocado.

    Escopo estritamente limitado ao bloco de frontmatter (entre o "---\\n"
    de abertura e o "\\n---" de fechamento): um "name:" que apareça no corpo
    nunca é tocado. Se o frontmatter não tiver "name:" ou "description:",
    essa chave é simplesmente deixada ausente — nunca inventa uma chave que
    não existia. Se source não tem frontmatter reconhecível, é retornado
    sem alteração (trimmed).
    """
    trimmed = source.strip()
    if not trimmed.startswith("---\n"):
        return trimmed
    end = trimmed.find("\n---", 4)
    if end < 0:
        return trimmed
    frontmatter = trimmed[4:end]
    rest = trimmed[end:]  # começa com "\n---", seguido do corpo

    lines = frontmatter.split("\n")
    for index, line in enumerate(lines):
        if ":" not in line:
            continue
        key, value = line.split(":", 1)
        key_name = key.strip()
        if key_name == "name":
            replacement = name
        elif key_name == "description":
            replacement = description
        else:
            continue
        trimmed_value = value.strip()
        quoted = len(trimmed_value) >= 2 and trimmed_value.startswith('"') and trimmed_value.endswith('"')
        if quoted:
            lines[index] = f'{key}: "{replacement}"'
        else:
            lines[index] = f"{key}: {replacement}"

    return "---\n" + "\n".join(lines) + rest


def _normalize_markdown(source: str) -> str:
    return source.strip() + "\n"


# ---------------------------------------------------------------------------
# Renderer principal
# ---------------------------------------------------------------------------


def render(
    kind: str,
    target: str,
    surface: str,
    item: dict[str, Any],
    source: str,
    capability: dict[str, str],
    identity_cfg: "identity.Config | None" = None,
) -> str:
    if kind == "skills":
        return _normalize_markdown(source)

    cfg = identity_cfg or identity.Config()
    agent = identity.lookup(cfg, item["id"])

    metadata, body = _parts(source)
    description = metadata.get("description", item["description"])
    name = metadata.get("name", f"trackfw-{item['id']}")
    body = body.strip()

    greeting = ""
    if agent is not None:
        greeting = _greeting_line(agent, cfg.user_nickname)
        name = identity.agent_name(agent.slug)
        description = f"{agent.display_name} — {description}"
        body = f"{greeting}\n\n{body}"

    representation = capability.get("representation")

    if representation == "custom-agent-toml":
        return "\n".join(
            [
                f"name = {json.dumps(name.replace('-', '_'), ensure_ascii=False)}",
                f"description = {json.dumps(description, ensure_ascii=False)}",
                f"developer_instructions = {json.dumps(body, ensure_ascii=False)}",
                "",
            ]
        )
    if representation in ("cli-agent-json", "agent-json"):
        # Go's encoding/json sorts map keys; keep byte-stable parity with the
        # canonical renderer as well as semantic JSON compatibility.
        payload = {"description": description, "name": name, "prompt": body}
        return json.dumps(payload, indent=2, ensure_ascii=False) + "\n"
    if representation == "agent-directory":
        # Reconstrói o frontmatter para o Antigravity CLI:
        # - mapeia model canônico para o valor aceito (opus→pro, sonnet→flash)
        # - injeta tools: SET_IMPL ou SET_ARCH dependendo do item.id (não do
        #   nome renderizado, que pode ser customizado pela identidade)
        # - omite campos não suportados pelo agy
        model = metadata.get("model", "")
        lines = ["---", f"name: {name}", f"description: {description}"]
        mapped = _map_model(model)
        if mapped is not None:
            lines.append(f"model: {mapped}")
        lines.append("tools:")
        for tool in _agent_tools(item["id"]):
            lines.append(f"  - {tool}")
        lines.append("---")
        result = "\n".join(lines) + "\n"
        if body:
            result += body + "\n"
        return result
    if representation == "opencode-agent":
        # Reconstrói o frontmatter para o OpenCode CLI (opencode.ai), seguindo
        # o mesmo padrão de reconstrução-do-zero do ramo "agent-directory".
        # Decisão registrada na Wave 1 do roadmap
        # ROADMAP-2026-08-04-compatibilidade-com-opencode-opencode-ai (achado
        # #3, pesquisa contra o binário real 1.18.13):
        #   - "tools:" é uma chave RESERVADA no schema de agente do OpenCode
        #     (espera um objeto de overrides por-ferramenta, ex. {bash: false},
        #     não uma lista de nomes estilo Claude Code) — reutilizar o
        #     frontmatter original faz o OpenCode recusar o carregamento
        #     INTEIRO do projeto ("Configuration is invalid"), não só daquele
        #     agente. Por isso "tools:" nunca é emitido aqui.
        #   - sem "mode:" explícito, o OpenCode assume mode "all" (agente
        #     selecionável como persona primária de chat) — os agentes
        #     trackfw devem ser sempre subagentes puros, nunca primários,
        #     para paridade com o comportamento nos demais targets. Por isso
        #     "mode: subagent" é sempre fixo, nunca omitido.
        #   - "model:" é deliberadamente OMITIDO (decisão de produto do
        #     orquestrador, não uma limitação técnica): o OpenCode espera
        #     "provider/model-id" (ex. "anthropic/claude-sonnet-4-5"), não os
        #     aliases curtos do catálogo canônico ("opus"/"sonnet"), e mapear
        #     para um provider fixo contradiria a motivação de negócio do REQ
        #     (permitir que o usuário roteie os agentes trackfw para o
        #     modelo open-source/local que ele já configurou em
        #     opencode.json). Omitir deixa o OpenCode resolver pelo default
        #     já configurado pelo usuário.
        #   - "memory:" também não faz sentido no schema do OpenCode e é
        #     descartado junto com "tools:".
        result = f"---\ndescription: {description}\nmode: subagent\n---\n"
        if body:
            result += body + "\n"
        return result

    # Rota B (default) — usada pela representação "subagent" e demais
    # representações que consomem o frontmatter cru (agent-markdown,
    # custom-agent, skill). Sem identidade, retorna a mesma expressão usada
    # antes de identity existir — a saída sem identidade é garantida
    # byte-a-byte idêntica por construção, não por coincidência. Com
    # identidade, além de reescrever name:/description: no frontmatter, também
    # reescreve a última linha de assinatura do corpo (ver
    # _rewrite_signature_line).
    if agent is None:
        return _normalize_markdown(source)
    with_body = _insert_body_prefix(source, greeting)
    with_frontmatter = _rewrite_frontmatter_fields(with_body, name, description)
    with_signature = _rewrite_signature_line(with_frontmatter, agent.display_name)
    return _normalize_markdown(with_signature)
