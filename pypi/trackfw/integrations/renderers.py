"""Native renderers for catalog assets."""

from __future__ import annotations

import json
import re
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


# Mapa de modelos canônicos para os IDs aceitos pelo Codex CLI.
# opus→gpt-5.4, sonnet→gpt-5.4-mini; demais valores (ou ausente) são omitidos.
_MODEL_MAP_CODEX: dict[str, str] = {
    "opus": "gpt-5.4",
    "sonnet": "gpt-5.4-mini",
}


def _map_model_codex(model: str) -> str | None:
    """Converte modelo canônico para o valor aceito pelo Codex CLI.

    Retorna o modelo mapeado ou None se a linha model deve ser omitida.
    """
    return _MODEL_MAP_CODEX.get(model)


# Mapa de modelos canônicos para os valores aceitos pela Cursor (fonte:
# cursor.com/docs/subagents, ver ADR ADR-2026-08-14-roteamento-de-model-tier-
# por-alvo-no-render-de-agentes-para-codex-e-cursor). opus→claude-opus-5[...],
# sonnet→composer-2.5[...]; demais valores (ou ausente) são omitidos — a
# Cursor cai no default "inherit"/Auto.
_MODEL_MAP_CURSOR: dict[str, str] = {
    "opus": "claude-opus-5[effort=high]",
    "sonnet": "composer-2.5[fast=true]",
}


def _map_model_cursor(model: str) -> str | None:
    """Converte modelo canônico para o valor aceito pela Cursor.

    Retorna o modelo mapeado ou None se a linha "model:" deve ser removida
    do frontmatter (Cursor cai no default "inherit"/Auto).

    Espelha internal/integrations/render.go:mapModelCursor.
    """
    return _MODEL_MAP_CURSOR.get(model)


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


def _contains_control_char(s: str) -> bool:
    """Reports whether s contains any ASCII control character (U+0000–U+001F)
    or a Unicode line/paragraph separator (U+2028, U+2029).

    Mirrors internal/integrations/render.go:containsControlChar (ML-5C).
    """
    return any(ord(c) < 0x20 or ord(c) in (0x2028, 0x2029) for c in s)


def _rewrite_frontmatter_model_line(source: str, value: str) -> str:
    """Substitui a linha "model:" do frontmatter de um markdown cru por
    value, preservando as demais linhas do frontmatter e o corpo byte a
    byte. Se o frontmatter não tiver "model:", uma linha é anexada ao final
    do bloco de frontmatter. Se source não tem frontmatter reconhecível, é
    retornado sem alteração (trimmed) — espelha a detecção de fronteira de
    _rewrite_frontmatter_fields, escopada à chave "model".

    Raises ValueError if value contains any ASCII control character
    (U+0000–U+001F). Model IDs never require control characters; any such
    value is rejected to prevent frontmatter injection (ML-5A).

    Espelha internal/integrations/render.go:rewriteFrontmatterModelLine.
    """
    if _contains_control_char(value):
        raise ValueError(
            f"model value contains control character and was rejected: "
            f"model IDs never require newlines or other control characters "
            f"(got {value!r})"
        )
    trimmed = source.strip()
    if not trimmed.startswith("---\n"):
        return trimmed
    end = trimmed.find("\n---", 4)
    if end < 0:
        return trimmed
    frontmatter = trimmed[4:end]
    rest = trimmed[end:]  # começa com "\n---", seguido do corpo

    lines = frontmatter.split("\n")
    found = False
    for index, line in enumerate(lines):
        if ":" not in line:
            continue
        key, value2 = line.split(":", 1)
        if key.strip() != "model":
            continue
        trimmed_value = value2.strip()
        quoted = len(trimmed_value) >= 2 and trimmed_value.startswith('"') and trimmed_value.endswith('"')
        if quoted:
            lines[index] = f'model: "{value}"'
        else:
            lines[index] = f"model: {value}"
        found = True
        break
    if not found:
        lines.append(f"model: {value}")

    return "---\n" + "\n".join(lines) + rest


def _remove_frontmatter_model_line(source: str) -> str:
    """Remove a linha "model:" do frontmatter de um markdown cru, se
    presente, preservando as demais linhas e o corpo byte a byte. Se source
    não tem linha "model:" ou frontmatter reconhecível, é retornado sem
    alteração (trimmed).

    Espelha internal/integrations/render.go:removeFrontmatterModelLine.
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
    kept = [
        line
        for line in lines
        if not (":" in line and line.split(":", 1)[0].strip() == "model")
    ]
    if len(kept) == len(lines):
        return trimmed

    return "---\n" + "\n".join(kept) + rest


def _is_version_string(s: str) -> bool:
    """Reporta se s é uma string de versão pura (dígitos e pontos apenas, ex.:
    "5", "4.6", "1.0.2"). Valores que não correspondem — IDs pré-compostos como
    "claude-sonnet-4-5-20250929" (tem traço), "latest" (tem letra), "" (vazio) —
    retornam False e o chamador usa o valor literalmente (escape hatch,
    ADR-2026-08-21 §3). Espelha internal/integrations/render.go:isVersionString.
    """
    if not s:
        return False
    return bool(re.fullmatch(r"[0-9]+(\.[0-9]+)*", s))


def _compose_claude_model_id(tier: str, version: str) -> str:
    """Constrói um identificador de modelo Claude a partir de tier e versão,
    aplicando as três regras de composição (ADR-2026-08-21 §2):
      Regra 1: ponto vira traço ("4.6" → "claude-sonnet-4-6")
      Regra 2: versão maior nunca ganha "-0" ("5" → "claude-sonnet-5")
      Regra 3: tratada via escape hatch — IDs pré-compostos nunca chegam aqui
    Espelha internal/integrations/render.go:composeClaudeModelID.
    """
    hyphenated = version.replace(".", "-")
    return f"claude-{tier}-{hyphenated}"


def _normalize_markdown(source: str) -> str:
    return source.strip() + "\n"


def normalize_markdown(source: str) -> str:
    """Public alias of _normalize_markdown (D5/D9 extension point) — lets
    trackfw.thirdparty.references.normalize_third_party_content reuse the
    exact strip+single-trailing-newline convention render() uses for
    managed catalog skill content, without duplicating the rule.
    Mirrors internal/integrations/render.go's NormalizeThirdPartyContent
    calling the unexported normalizeMarkdown within the same Go package;
    Python needs a public (non-underscore) name for the cross-package
    call from trackfw.thirdparty, which cannot reach a private symbol."""
    return _normalize_markdown(source)


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
    agent_models: "dict[str, str] | None" = None,
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
        lines = [
            f"name = {json.dumps(name.replace('-', '_'), ensure_ascii=False)}",
            f"description = {json.dumps(description, ensure_ascii=False)}",
        ]
        mapped_codex = _map_model_codex(metadata.get("model", ""))
        if mapped_codex is not None:
            lines.append(f"model = {json.dumps(mapped_codex, ensure_ascii=False)}")
        lines.append(f"developer_instructions = {json.dumps(body, ensure_ascii=False)}")
        lines.append("")
        return "\n".join(lines)
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
    if target == "cursor":
        mapped_cursor = _map_model_cursor(metadata.get("model", ""))
        if mapped_cursor is not None:
            source = _rewrite_frontmatter_model_line(source, mapped_cursor)
        else:
            source = _remove_frontmatter_model_line(source)
    elif target == "claude" and agent_models:
        # Allowlist: somente o alvo "claude" recebe IDs de modelo compostos.
        # Codex, Cursor, Antigravity, OpenCode, Gemini, Kiro e qualquer outro
        # alvo são deixados sem alteração mesmo com agent_models populado —
        # ADR-2026-08-21 §4 (gate, não cuidado).
        tier = metadata.get("model", "")
        version = agent_models.get(tier, "")
        if version:
            # String vazia = sem pin: deixa source sem alteração (alias de tier preservado).
            if _is_version_string(version):
                # Regras 1 e 2: ponto→traço; major omite minor.
                model_id = _compose_claude_model_id(tier, version)
            else:
                # Escape hatch: valor com traço, letra ou outro char não-versão
                # é usado literalmente (ADR-2026-08-21 §3).
                model_id = version
            source = _rewrite_frontmatter_model_line(source, model_id)

    if agent is None:
        return _normalize_markdown(source)
    with_body = _insert_body_prefix(source, greeting)
    with_frontmatter = _rewrite_frontmatter_fields(with_body, name, description)
    with_signature = _rewrite_signature_line(with_frontmatter, agent.display_name)
    return _normalize_markdown(with_signature)


# ---------------------------------------------------------------------------
# Resolução de modelo efetivo — exposta para "trackfw agents models" (ML-2A)
# ---------------------------------------------------------------------------


def resolve_agent_model(
    tier: str,
    representation: str,
    target_id: str,
    agent_models: "dict[str, str] | None" = None,
) -> "tuple[str, bool]":
    """Retorna o valor de modelo que render() escreveria no campo model: do
    artefato com a representation e target_id dados, para um agente de tier tier.

    Retorna (resolved, present) — present=False significa que o formato do
    artefato omite o campo model inteiramente (ex.: cli-agent-json, agent-json,
    opencode-agent); o chamador deve exibir "—" em vez do alias de tier.

    agentModels só é aplicado para o alvo "claude" (ADR-2026-08-21 §4).
    Espelha internal/integrations/models.go:ResolveAgentModel e
    npm/src/integrations/render.js:resolveAgentModel.
    """
    if representation == "custom-agent-toml":
        v = _map_model_codex(tier)
        return (v, v is not None)
    if representation in ("cli-agent-json", "agent-json"):
        return ("", False)
    if representation == "agent-directory":
        v = _map_model(tier)
        return (v, v is not None)
    if representation == "opencode-agent":
        return ("", False)
    # default branch — espelha o case default de render()
    if target_id == "cursor":
        v = _map_model_cursor(tier)
        return (v, v is not None)
    if target_id == "claude":
        am = agent_models or {}
        version = am.get(tier, "")
        if version:
            if _is_version_string(version):
                model_id = _compose_claude_model_id(tier, version)
            else:
                model_id = version  # escape hatch: usa literalmente
            return (model_id, True)
        # sem pin → alias de tier inalterado
    return (tier, True)


def looks_like_suspect_model_value(v: str) -> bool:
    """Reporta se v é um valor de agent_models que aciona o escape hatch e
    provavelmente produz um identificador de modelo inválido no artefato gerado.

    Retorna True quando v não é uma string de versão pura E não começa com
    "claude-". Chamadores devem emitir um aviso por tier (não por linha) para
    stderr quando isso retornar True.

    Preferência por falso-negativo: "4.6-beta" avisa; "4.6", "5",
    "claude-sonnet-4-5-20250929" não avisam.
    Espelha internal/integrations/models.go:LooksLikeSuspectModelValue.
    """
    # ML-5A: values with control characters are always suspect —
    # _rewrite_frontmatter_model_line rejects them outright, so this function
    # must agree with the write path to keep "trackfw agents models" aligned
    # with "trackfw agents install/update" behavior.
    return _contains_control_char(v) or (not _is_version_string(v) and not v.startswith("claude-"))
