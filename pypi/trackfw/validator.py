"""
validator.py — Validações de governança do trackfw.
Espelho Python de npm/src/validator/index.js (paridade de comportamento).
Stdlib apenas: os, pathlib, re, datetime.
"""

import os
from datetime import datetime, timezone

from . import config as _config

STALE_WIP_DAYS = 7


# ---------------------------------------------------------------------------
# Utilitários internos
# ---------------------------------------------------------------------------

def list_dir(path: str) -> list:
    """
    Retorna lista de nomes de arquivo (não-diretórios) em path.
    Retorna [] se o diretório não existir ou ocorrer erro.
    """
    try:
        entries = []
        for name in os.listdir(path):
            try:
                full = os.path.join(path, name)
                if not os.path.isdir(full):
                    entries.append(name)
            except OSError:
                pass
        return entries
    except OSError:
        return []


def resolve_wip_dirs(cfg: dict) -> list:
    """
    Retorna lista de diretórios wip/ conforme o modo de namespacing.
    flat     → [cfg["roadmap_dir"] + "/wip"]
    by_agent → [cfg["roadmap_dir"] + "/" + agent + "/wip" for agent in agents]
    """
    if cfg.get("roadmap_namespacing") == _config.NAMESPACING_BY_AGENT:
        agents = cfg.get("agents") or []
        if not agents:
            roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
            try:
                agents = [
                    f for f in os.listdir(roadmap_dir)
                    if os.path.isdir(os.path.join(roadmap_dir, f))
                ]
            except OSError:
                agents = []
        roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
        return [roadmap_dir + "/" + agent + "/wip" for agent in agents]

    return [cfg.get("roadmap_dir", "docs/roadmaps") + "/wip"]


def parse_frontmatter(content: str) -> dict:
    """
    Extrai campos entre --- e --- do início do arquivo.
    Retorna dict com chaves em snake_case.
    """
    result = {}
    if not content.startswith("---"):
        return result
    lines = content.split("\n")
    in_block = False
    for i, line in enumerate(lines):
        stripped = line.strip()
        if i == 0 and stripped == "---":
            in_block = True
            continue
        if in_block:
            if stripped == "---":
                break
            colon_idx = stripped.find(":")
            if colon_idx >= 0:
                key = stripped[:colon_idx].strip().replace("-", "_")
                val = stripped[colon_idx + 1:].strip()
                result[key] = val
    return result


def _parse_blocked_adrs(file_path: str) -> list:
    """
    Extrai basenames de ADRs da seção '## Blocked by ADRs' de um arquivo REQ.
    Espelha parseBlockedADRs do JS.
    """
    try:
        with open(file_path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return []

    lines = content.split("\n")
    adrs = []
    in_section = False
    for line in lines:
        if line == "## Blocked by ADRs":
            in_section = True
            continue
        if in_section:
            if line.startswith("## "):
                break
            if line.startswith("- "):
                item = line[2:].strip()
                parts = item.split()
                if parts and parts[0].endswith(".md"):
                    adrs.append(parts[0])
    return adrs


def _adr_is_draft(basename: str, cfg: dict) -> bool:
    """
    Verifica se <basename> contém 'Status: Draft' em algum dos adrDirs configurados.
    """
    for adr_dir in cfg.get("adr_dirs", ["docs/adr"]):
        p = os.path.join(adr_dir, basename)
        if os.path.exists(p):
            try:
                with open(p, "r", encoding="utf-8") as f:
                    return "Status: Draft" in f.read()
            except OSError:
                pass
    return False


def _read_wip_config(cwd: str = None) -> dict:
    """
    Lê wip_limit e wip_by_squad do trackfw.yaml no CWD.
    Retorna {"limit": 1, "by_squad": False} se o arquivo não existir.
    """
    cfg_result = {"limit": 1, "by_squad": False}
    yaml_path = os.path.join(cwd or os.getcwd(), "trackfw.yaml")
    try:
        with open(yaml_path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return cfg_result

    for line in content.split("\n"):
        trimmed = line.strip()
        if trimmed.startswith("wip_limit:"):
            val = trimmed[len("wip_limit:"):].strip().split()[0] if trimmed[len("wip_limit:"):].strip() else ""
            try:
                n = int(val)
                if n > 0:
                    cfg_result["limit"] = n
            except (ValueError, IndexError):
                pass
        if trimmed.startswith("wip_by_squad:"):
            val = trimmed[len("wip_by_squad:"):].strip().split()[0] if trimmed[len("wip_by_squad:"):].strip() else ""
            if val == "true":
                cfg_result["by_squad"] = True

    return cfg_result


def _parse_squad_from_frontmatter(file_path: str) -> str:
    """
    Extrai o valor do campo 'squad:' de um arquivo markdown.
    Retorna string vazia se ausente.
    """
    try:
        with open(file_path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return ""

    for line in content.split("\n"):
        trimmed = line.strip()
        if trimmed.startswith("squad:"):
            return trimmed[len("squad:"):].strip()
    return ""


def _read_governance_mode(cwd: str = None) -> dict:
    """
    Lê governance_mode e lenient_until do trackfw.yaml.
    Retorna {"mode": "strict", "lenient_until": None} por padrão.
    """
    result = {"mode": "strict", "lenient_until": None}
    yaml_path = os.path.join(cwd or os.getcwd(), "trackfw.yaml")
    try:
        with open(yaml_path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return result

    for line in content.split("\n"):
        trimmed = line.strip()
        if trimmed.startswith("governance_mode:"):
            val_part = trimmed[len("governance_mode:"):].strip()
            vals = val_part.split()
            if vals:
                result["mode"] = vals[0]
        if trimmed.startswith("lenient_until:"):
            val_part = trimmed[len("lenient_until:"):].strip()
            vals = val_part.split()
            if vals:
                try:
                    d = datetime.fromisoformat(vals[0])
                    # Garantir que é aware ou naive consistente
                    result["lenient_until"] = d
                except ValueError:
                    pass

    return result


def _is_lenient(cwd: str = None) -> bool:
    """Retorna True se o projeto está em modo lenient e o prazo não expirou."""
    gm = _read_governance_mode(cwd)
    if gm["mode"] != "lenient":
        return False
    if gm["lenient_until"] is None:
        return True
    # Comparação sem timezone
    now = datetime.now()
    lu = gm["lenient_until"]
    # Remove tzinfo se presente para comparação homogênea
    if lu.tzinfo is not None:
        now = datetime.now(timezone.utc)
    return now < lu


# ---------------------------------------------------------------------------
# Funções de validação públicas (assinatura: cfg como parâmetro)
# ---------------------------------------------------------------------------

def validate_wip_has_req(cfg: dict) -> list:
    """
    Roadmaps em wip/ sem 'REQ:' no conteúdo → violation.
    Suporta modo by_agent via resolve_wip_dirs.
    """
    wip_dirs = resolve_wip_dirs(cfg)
    violations = []
    for wip_dir in wip_dirs:
        entries = list_dir(wip_dir)
        for name in entries:
            try:
                with open(os.path.join(wip_dir, name), "r", encoding="utf-8") as f:
                    content = f.read()
                if "REQ:" not in content or "REQ: \n" in content:
                    violations.append(
                        {"type": "violation", "message": f'roadmap "{name}" is in wip but has no linked REQ'}
                    )
            except OSError:
                pass
    return violations


def validate_reqs_have_adr(cfg: dict) -> list:
    """REQs em req_dir/ sem 'ADR:' no conteúdo → violation."""
    entries = list_dir(cfg.get("req_dir", "docs/req"))
    violations = []
    for name in entries:
        try:
            with open(os.path.join(cfg["req_dir"], name), "r", encoding="utf-8") as f:
                content = f.read()
            if "ADR:" not in content or "ADR: \n" in content:
                violations.append(
                    {"type": "violation", "message": f'req "{name}" has no linked ADR'}
                )
        except OSError:
            pass
    return violations


def validate_blocked_has_req(cfg: dict) -> list:
    """Roadmaps em blocked/ sem 'REQ:' → violation."""
    blocked_dir = cfg.get("roadmap_dir", "docs/roadmaps") + "/blocked"
    entries = list_dir(blocked_dir)
    violations = []
    for name in entries:
        try:
            with open(os.path.join(blocked_dir, name), "r", encoding="utf-8") as f:
                content = f.read()
            if "REQ:" not in content or "REQ: \n" in content:
                violations.append(
                    {"type": "violation", "message": f'roadmap "{name}" is in blocked but has no linked REQ'}
                )
        except OSError:
            pass
    return violations


def validate_reqs_have_roadmap(cfg: dict) -> list:
    """REQs sem 'Roadmap:' → violation."""
    entries = list_dir(cfg.get("req_dir", "docs/req"))
    violations = []
    for name in entries:
        try:
            with open(os.path.join(cfg["req_dir"], name), "r", encoding="utf-8") as f:
                content = f.read()
            if "Roadmap:" not in content or "Roadmap: \n" in content:
                violations.append(
                    {"type": "violation", "message": f'req "{name}" has no linked Roadmap'}
                )
        except OSError:
            pass
    return violations


def validate_adrs_are_referenced(cfg: dict) -> list:
    """ADRs em adr_dirs não referenciados em nenhuma REQ → violation."""
    adrs = []
    for adr_dir in cfg.get("adr_dirs", ["docs/adr"]):
        adrs.extend(list_dir(adr_dir))

    req_entries = list_dir(cfg.get("req_dir", "docs/req"))
    combined = ""
    for name in req_entries:
        try:
            with open(os.path.join(cfg["req_dir"], name), "r", encoding="utf-8") as f:
                combined += f.read()
        except OSError:
            pass

    violations = []
    for adr in adrs:
        if adr not in combined:
            violations.append(
                {"type": "violation", "message": f'adr "{adr}" is not referenced by any REQ'}
            )
    return violations


def validate_wip_has_acceptance_criteria(cfg: dict) -> list:
    """Roadmaps wip sem bloco de critérios de aceite → violation."""
    wip_dirs = resolve_wip_dirs(cfg)
    violations = []
    for wip_dir in wip_dirs:
        entries = list_dir(wip_dir)
        for name in entries:
            try:
                with open(os.path.join(wip_dir, name), "r", encoding="utf-8") as f:
                    content = f.read()
                has_block = (
                    "## Acceptance Criteria" in content
                    or "## Critérios de Aceite" in content
                    or "acceptance criteria" in content
                    or "Acceptance Criteria:" in content
                )
                if not has_block:
                    violations.append(
                        {"type": "violation", "message": f'roadmap "{name}" is in wip but has no acceptance criteria block'}
                    )
            except OSError:
                pass
    return violations


def validate_wip_limit(cfg: dict) -> dict:
    """
    Verifica o WIP limit por agente, por squad ou global conforme trackfw.yaml.
    Retorna {"violations": [], "warnings": []}.
    """
    violations = []
    warnings = []

    if cfg.get("roadmap_namespacing") == _config.NAMESPACING_BY_AGENT:
        agents = cfg.get("agents") or []
        if not agents:
            roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
            try:
                agents = [
                    f for f in os.listdir(roadmap_dir)
                    if os.path.isdir(os.path.join(roadmap_dir, f))
                ]
            except OSError:
                agents = []
        limit = cfg.get("wip_limit", 1)
        if limit <= 0:
            limit = 1
        for agent in agents:
            entries = list_dir(cfg["roadmap_dir"] + "/" + agent + "/wip")
            if len(entries) > limit:
                warnings.append({
                    "type": "warning",
                    "message": f'{len(entries)} roadmaps in wip/ for agent "{agent}" (limit: {limit}) — consider focusing'
                })
        return {"violations": violations, "warnings": warnings}

    # modo flat (global ou por squad)
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    wip_path = os.path.join(roadmap_dir, "wip")
    files = []
    try:
        files = [
            os.path.join(wip_path, f)
            for f in os.listdir(wip_path)
            if not os.path.isdir(os.path.join(wip_path, f))
        ]
    except OSError:
        return {"violations": violations, "warnings": warnings}

    wip_cfg = _read_wip_config()

    if not wip_cfg["by_squad"]:
        if len(files) > wip_cfg["limit"]:
            warnings.append({
                "type": "warning",
                "message": f'{len(files)} roadmaps in wip/ (limit: {wip_cfg["limit"]}) — consider focusing'
            })
        return {"violations": violations, "warnings": warnings}

    # por squad
    by_squad = {}
    for f in files:
        squad = _parse_squad_from_frontmatter(f)
        if not squad:
            squad = "(no squad)"
        by_squad.setdefault(squad, []).append(os.path.basename(f))

    for squad, items in by_squad.items():
        if len(items) > wip_cfg["limit"]:
            warnings.append({
                "type": "warning",
                "message": f'squad "{squad}" has {len(items)} roadmaps in wip/ (limit: {wip_cfg["limit"]})'
            })

    return {"violations": violations, "warnings": warnings}


def validate_stale_wip(cfg: dict, days: int = STALE_WIP_DAYS) -> list:
    """
    Arquivos em wip/ com mtime >= days dias → warning.
    Suporta modo by_agent via resolve_wip_dirs.
    """
    wip_dirs = resolve_wip_dirs(cfg)
    warnings = []
    now = datetime.now().timestamp()

    for wip_dir in wip_dirs:
        try:
            md_files = [
                os.path.join(wip_dir, f)
                for f in os.listdir(wip_dir)
                if f.endswith(".md")
            ]
        except OSError:
            continue

        for file_path in md_files:
            try:
                stat = os.stat(file_path)
                age_seconds = now - stat.st_mtime
                age_days = int(age_seconds / (60 * 60 * 24))
                if age_days >= days:
                    last_modified = datetime.fromtimestamp(stat.st_mtime).strftime("%Y-%m-%d")
                    basename = os.path.basename(file_path)
                    warnings.append({
                        "type": "warning",
                        "message": f"roadmap/wip/{basename} has been in WIP for {age_days} days (last modified {last_modified})"
                    })
            except OSError:
                pass

    return warnings


def validate_reqs_not_blocked_by_draft_adrs(cfg: dict) -> list:
    """REQs Open com ADRs Draft na seção '## Blocked by ADRs' → violation."""
    entries = list_dir(cfg.get("req_dir", "docs/req"))
    violations = []
    for name in entries:
        file_path = os.path.join(cfg["req_dir"], name)
        try:
            with open(file_path, "r", encoding="utf-8") as f:
                content = f.read()
        except OSError:
            continue

        if "Status: Open" not in content:
            continue

        blocked_adrs = _parse_blocked_adrs(file_path)
        for adr_basename in blocked_adrs:
            if _adr_is_draft(adr_basename, cfg):
                violations.append({
                    "type": "violation",
                    "message": f"REQ {name} is blocked by Draft ADR: {adr_basename}"
                })
    return violations


def validate_frontmatter_presence(cfg: dict) -> list:
    """Verifica presença de frontmatter em ADRs e REQs."""
    violations = []

    for adr_dir in cfg.get("adr_dirs", ["docs/adr"]):
        files = [f for f in list_dir(adr_dir) if f.endswith(".md")]
        for f in files:
            try:
                with open(os.path.join(adr_dir, f), "r", encoding="utf-8") as fh:
                    content = fh.read()
                if not content.startswith("---"):
                    violations.append({
                        "type": "violation",
                        "message": f'adr "{f}" has no frontmatter block'
                    })
            except OSError:
                pass

    req_dir = cfg.get("req_dir", "docs/req")
    req_files = [f for f in list_dir(req_dir) if f.endswith(".md")]
    for f in req_files:
        try:
            with open(os.path.join(req_dir, f), "r", encoding="utf-8") as fh:
                content = fh.read()
            if not content.startswith("---"):
                violations.append({
                    "type": "violation",
                    "message": f'req "{f}" has no frontmatter block'
                })
        except OSError:
            pass

    return violations


# ---------------------------------------------------------------------------
# validate() — ponto de entrada principal
# ---------------------------------------------------------------------------

def validate(cwd: str = None) -> dict:
    """
    Executa todas as validações e retorna {"violations": [...], "warnings": [...]}.
    Se governance_mode == "lenient": move violations para warnings.
    Cada item é um dict {"type": "violation"|"warning", "message": "..."}.
    """
    _config.reset()
    cfg = _config.load(cwd)

    wip_limit_result = validate_wip_limit(cfg)

    violations = (
        validate_wip_has_req(cfg)
        + validate_reqs_have_adr(cfg)
        + validate_blocked_has_req(cfg)
        + validate_reqs_have_roadmap(cfg)
        + validate_adrs_are_referenced(cfg)
        + validate_wip_has_acceptance_criteria(cfg)
        + validate_reqs_not_blocked_by_draft_adrs(cfg)
        + validate_frontmatter_presence(cfg)
        + wip_limit_result["violations"]
    )

    warnings = wip_limit_result["warnings"] + validate_stale_wip(cfg)

    if _is_lenient(cwd):
        warnings = warnings + violations
        violations = []

    return {"violations": violations, "warnings": warnings}


# ---------------------------------------------------------------------------
# Aliases e exportações para compatibilidade com o CLI
# ---------------------------------------------------------------------------

validate_single_wip = validate_wip_limit
