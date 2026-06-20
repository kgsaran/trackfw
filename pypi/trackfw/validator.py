"""
validator.py — Validações de governança do trackfw.
Espelho Python de npm/src/validator/index.js (paridade de comportamento).
Stdlib apenas: os, pathlib, re, datetime, subprocess.
"""

import glob as _glob
import json
import os
import re
import subprocess
from datetime import datetime, timezone

from . import config as _config
from .traceid import check_traceid

STALE_WIP_DAYS = 7


# ---------------------------------------------------------------------------
# Helpers de field mapping e severidade (F2 + F3 — v2.4)
# ---------------------------------------------------------------------------

def _content_has_marker(content: str, markers: list) -> bool:
    """
    Retorna True se content contém qualquer marcador com valor não-vazio.
    Um marcador é considerado "sem valor" se a linha for exatamente
    "MARKER \n" (espaço + newline) — espelhando a lógica anterior.
    """
    for marker in markers:
        if marker in content and (marker + " \n") not in content:
            return True
    return False


def _rule_severity(name: str, cfg: dict) -> str:
    """Retorna severidade da regra: 'off' | 'warning' | 'error'."""
    return cfg.get("rules", {}).get(name, "error")


def _extract_file(msg: str) -> str:
    """Extrai o primeiro filename entre aspas duplas de uma mensagem. Retorna '' se ausente."""
    m = re.search(r'"([^"]+)"', msg)
    return m.group(1) if m else ""


def _enrich_items(items: list, rule_name: str) -> list:
    """
    Adiciona os campos 'rule' e 'file' a cada dict da lista, se ainda não presentes.
    Não modifica itens que já possuam esses campos.
    """
    result = []
    for item in items:
        if isinstance(item, dict):
            enriched = dict(item)
            if "rule" not in enriched:
                enriched["rule"] = rule_name
            if "file" not in enriched:
                enriched["file"] = _extract_file(enriched.get("message", ""))
            result.append(enriched)
        else:
            result.append(item)
    return result


def _apply_rule(rule_name: str, msgs: list, violations: list, warnings: list, cfg: dict):
    """
    Distribui msgs (lista de dicts) conforme a severidade configurada da regra.
    - 'off'     → descarta
    - 'warning' → adiciona a warnings
    - 'error'   → adiciona a violations (default)
    Enriquece cada item com 'rule' e 'file' antes de distribuir.
    """
    if not msgs:
        return
    severity = _rule_severity(rule_name, cfg)
    if severity == "off":
        return
    enriched = _enrich_items(msgs, rule_name)
    if severity == "warning":
        warnings.extend(enriched)
    else:
        violations.extend(enriched)


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


def _walk_dir_md(dir_path: str) -> list:
    """Retorna basenames de todos .md recursivamente em dir_path."""
    result = []
    try:
        for root, dirs, files in os.walk(dir_path):
            for name in files:
                if name.endswith('.md'):
                    result.append(name)
    except OSError:
        pass
    return result


def _find_adr_file(basename: str, adr_dirs: list) -> str:
    """Busca basename recursivamente em todos os adr_dirs. Retorna caminho completo ou ''."""
    for adr_dir in adr_dirs:
        try:
            for root, dirs, files in os.walk(adr_dir):
                if basename in files:
                    return os.path.join(root, basename)
        except OSError:
            pass
    return ""


def _git_last_modified_time(file_path: str):
    """
    Retorna timestamp (float) do último commit que tocou o arquivo via git log.
    Retorna None se não for um repo git ou git não estiver disponível.
    """
    try:
        result = subprocess.run(
            ["git", "log", "-1", "--format=%ct", "--", file_path],
            capture_output=True, text=True, timeout=5
        )
        out = result.stdout.strip()
        if out:
            return float(out)
    except Exception:
        pass
    return None


def _extract_ref_path(content: str, field: str) -> str:
    """
    Extrai o caminho .md após 'field: valor' na mesma linha.
    Retorna '' se não encontrado ou não terminar em .md.
    """
    prefix = field + ":"
    for line in content.split("\n"):
        trimmed = line.strip()
        if trimmed.startswith(prefix):
            val = trimmed[len(prefix):].strip()
            if not val or val in ("—", "-", "–"):
                return ""
            # Primeira "palavra" (antes de espaço)
            val = val.split()[0] if val.split() else ""
            if val.endswith(".md"):
                return val
    return ""


def resolve_req_files(cfg: dict) -> list:
    """
    Retorna lista de paths completos de .md em req_dir,
    consciente de roadmap_namespacing: by_agent percorre req_dir/<agente>/<estado>/.
    """
    req_dir = cfg.get("req_dir", "docs/req")
    namespacing = cfg.get("roadmap_namespacing", "")
    if namespacing == "by_agent":
        states = ["backlog", "wip", "blocked", "done", "abandoned"]
        agents = cfg.get("agents", [])
        if not agents:
            try:
                agents = [e for e in os.listdir(req_dir)
                          if os.path.isdir(os.path.join(req_dir, e))]
            except OSError:
                return []
        files = []
        for agent in agents:
            for state in states:
                pattern = os.path.join(req_dir, agent, state, "*.md")
                files.extend(_glob.glob(pattern))
        return files
    # flat (comportamento anterior)
    return _glob.glob(os.path.join(req_dir, "*.md"))


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
    Busca recursivamente nas subpastas via _find_adr_file.
    """
    p = _find_adr_file(basename, cfg.get("adr_dirs", ["docs/adr"]))
    if not p:
        return False
    try:
        with open(p, "r", encoding="utf-8") as f:
            return "Status: Draft" in f.read()
    except OSError:
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


_BASELINE_FILE = ".trackfw-baseline.json"


def _extract_messages(items: list) -> list:
    """Extrai campo 'message' de uma lista de dicts de violation/warning."""
    result = []
    for item in items:
        if isinstance(item, dict):
            result.append(item.get("message", str(item)))
        else:
            result.append(str(item))
    return result


def load_baseline() -> dict | None:
    """Lê .trackfw-baseline.json do CWD. Retorna None se não existir."""
    try:
        with open(_BASELINE_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    except FileNotFoundError:
        return None
    except (json.JSONDecodeError, OSError) as e:
        raise RuntimeError(f"Erro ao ler baseline: {e}") from e


def save_baseline(violations: list, warnings: list) -> None:
    """Salva violations e warnings como baseline em .trackfw-baseline.json.
    Aceita lista de dicts ou strings — normaliza para strings.
    """
    bf = {
        "created": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "violations": _extract_messages(violations),
        "warnings": _extract_messages(warnings),
    }
    with open(_BASELINE_FILE, "w", encoding="utf-8") as f:
        json.dump(bf, f, indent=2, ensure_ascii=False)


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
    Roadmaps em wip/ sem marcador req no conteúdo → violation.
    Suporta modo by_agent via resolve_wip_dirs.
    Usa cfg["link_fields"]["req"] para os marcadores configuráveis.
    """
    wip_dirs = resolve_wip_dirs(cfg)
    req_markers = cfg.get("link_fields", {}).get("req", ["REQ:"])
    violations = []
    for wip_dir in wip_dirs:
        entries = list_dir(wip_dir)
        for name in entries:
            try:
                with open(os.path.join(wip_dir, name), "r", encoding="utf-8") as f:
                    content = f.read()
                if not _content_has_marker(content, req_markers):
                    violations.append(
                        {"type": "violation", "message": f'roadmap "{name}" is in wip but has no linked REQ'}
                    )
            except OSError:
                pass
    return violations


def validate_reqs_have_adr(cfg: dict) -> list:
    """REQs em req_dir/ sem marcador adr no conteúdo → violation."""
    files = resolve_req_files(cfg)
    adr_markers = cfg.get("link_fields", {}).get("adr", ["ADR:"])
    violations = []
    for file_path in files:
        try:
            with open(file_path, "r", encoding="utf-8") as f:
                content = f.read()
            if not _content_has_marker(content, adr_markers):
                name = os.path.basename(file_path)
                violations.append(
                    {"type": "violation", "message": f'req "{name}" has no linked ADR'}
                )
        except OSError:
            pass
    return violations


def validate_blocked_has_req(cfg: dict) -> list:
    """Roadmaps em blocked/ sem marcador req → violation."""
    blocked_dir = cfg.get("roadmap_dir", "docs/roadmaps") + "/blocked"
    entries = list_dir(blocked_dir)
    req_markers = cfg.get("link_fields", {}).get("req", ["REQ:"])
    violations = []
    for name in entries:
        try:
            with open(os.path.join(blocked_dir, name), "r", encoding="utf-8") as f:
                content = f.read()
            if not _content_has_marker(content, req_markers):
                violations.append(
                    {"type": "violation", "message": f'roadmap "{name}" is in blocked but has no linked REQ'}
                )
        except OSError:
            pass
    return violations


def validate_reqs_have_roadmap(cfg: dict) -> list:
    """REQs sem marcador roadmap → violation."""
    files = resolve_req_files(cfg)
    roadmap_markers = cfg.get("link_fields", {}).get("roadmap", ["Roadmap:"])
    violations = []
    for file_path in files:
        try:
            with open(file_path, "r", encoding="utf-8") as f:
                content = f.read()
            if not _content_has_marker(content, roadmap_markers):
                name = os.path.basename(file_path)
                violations.append(
                    {"type": "violation", "message": f'req "{name}" has no linked Roadmap'}
                )
        except OSError:
            pass
    return violations


def validate_adrs_are_referenced(cfg: dict) -> list:
    """ADRs em adr_dirs não referenciados em nenhuma REQ → violation (busca recursiva)."""
    adrs = []
    for adr_dir in cfg.get("adr_dirs", ["docs/adr"]):
        adrs.extend(_walk_dir_md(adr_dir))

    req_files = resolve_req_files(cfg)
    combined = ""
    for file_path in req_files:
        try:
            with open(file_path, "r", encoding="utf-8") as f:
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
    """Roadmaps wip sem bloco de critérios de aceite → violation.
    Usa cfg["acceptance_markers"] para os marcadores configuráveis.
    """
    wip_dirs = resolve_wip_dirs(cfg)
    acceptance_markers = cfg.get("acceptance_markers", ["## Acceptance Criteria", "## Critérios de Aceite"])
    violations = []
    for wip_dir in wip_dirs:
        entries = list_dir(wip_dir)
        for name in entries:
            try:
                with open(os.path.join(wip_dir, name), "r", encoding="utf-8") as f:
                    content = f.read()
                if not _content_has_marker(content, acceptance_markers):
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
                git_time = _git_last_modified_time(file_path)
                ref_time = git_time if git_time is not None else stat.st_mtime
                age_seconds = now - ref_time
                age_days = int(age_seconds / (60 * 60 * 24))
                if age_days >= days:
                    last_modified = datetime.fromtimestamp(ref_time).strftime("%Y-%m-%d")
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
    files = resolve_req_files(cfg)
    violations = []
    for file_path in files:
        name = os.path.basename(file_path)
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
    """Verifica presença de frontmatter em ADRs e REQs (busca recursiva em adr_dirs)."""
    violations = []

    for adr_dir in cfg.get("adr_dirs", ["docs/adr"]):
        files = [f for f in _walk_dir_md(adr_dir) if f.endswith(".md")]
        for f in files:
            full_path = _find_adr_file(f, [adr_dir])
            if not full_path:
                continue
            try:
                with open(full_path, "r", encoding="utf-8") as fh:
                    content = fh.read()
                if not content.startswith("---"):
                    violations.append({
                        "type": "violation",
                        "message": f'adr "{f}" has no frontmatter block'
                    })
            except OSError:
                pass

    req_files = [p for p in resolve_req_files(cfg) if p.endswith(".md")]
    for file_path in req_files:
        try:
            with open(file_path, "r", encoding="utf-8") as fh:
                content = fh.read()
            if not content.startswith("---"):
                f = os.path.basename(file_path)
                violations.append({
                    "type": "violation",
                    "message": f'req "{f}" has no frontmatter block'
                })
        except OSError:
            pass

    return violations


def validate_ref_targets_exist(cfg: dict) -> list:
    """Verifica se arquivos referenciados em REQ:, ADR:, Roadmap: existem. Retorna warnings."""
    warnings = []

    # Roadmaps em wip e blocked: verificar REQ:
    dirs = resolve_wip_dirs(cfg) + [cfg.get("roadmap_dir", "docs/roadmaps") + "/blocked"]
    for wip_dir in dirs:
        for name in list_dir(wip_dir):
            try:
                with open(os.path.join(wip_dir, name), "r", encoding="utf-8") as f:
                    content = f.read()
                ref = _extract_ref_path(content, "REQ")
                if ref and not os.path.exists(ref):
                    warnings.append({
                        "type": "warning",
                        "message": f'roadmap "{name}" links to REQ "{ref}" which does not exist'
                    })
            except OSError:
                pass

    # REQs: verificar ADR: e Roadmap:
    for file_path in resolve_req_files(cfg):
        try:
            with open(file_path, "r", encoding="utf-8") as f:
                content = f.read()
            name = os.path.basename(file_path)
            adr_ref = _extract_ref_path(content, "ADR")
            if adr_ref and not os.path.exists(adr_ref):
                warnings.append({
                    "type": "warning",
                    "message": f'req "{name}" links to ADR "{adr_ref}" which does not exist'
                })
            roadmap_ref = _extract_ref_path(content, "Roadmap")
            if roadmap_ref and not os.path.exists(roadmap_ref):
                warnings.append({
                    "type": "warning",
                    "message": f'req "{name}" links to Roadmap "{roadmap_ref}" which does not exist'
                })
        except OSError:
            pass

    return warnings


_FOLDER_TO_STATUS = {
    "wip":       ["WIP", "wip", "In Progress"],
    "backlog":   ["Backlog", "backlog"],
    "blocked":   ["Blocked", "blocked"],
    "done":      ["Done", "done"],
    "abandoned": ["Abandoned", "abandoned"],
}


def validate_folder_status_coherence(cfg: dict) -> list:
    """
    Verifica que o campo status: no frontmatter bate com a pasta onde o arquivo está.
    Divergência → warning.
    """
    warnings = []
    states = ["wip", "backlog", "blocked", "done", "abandoned"]
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")

    dirs = []
    if cfg.get("roadmap_namespacing") == _config.NAMESPACING_BY_AGENT:
        agents = cfg.get("agents") or []
        if not agents:
            try:
                agents = [f for f in os.listdir(roadmap_dir) if os.path.isdir(os.path.join(roadmap_dir, f))]
            except OSError:
                agents = []
        for agent in agents:
            for state in states:
                dirs.append((os.path.join(roadmap_dir, agent, state), state))
    else:
        for state in states:
            dirs.append((os.path.join(roadmap_dir, state), state))

    for dir_path, state in dirs:
        for name in list_dir(dir_path):
            if not name.endswith(".md"):
                continue
            try:
                with open(os.path.join(dir_path, name), "r", encoding="utf-8") as f:
                    content = f.read()
                fm = parse_frontmatter(content)
                declared = fm.get("status", "")
                if not declared:
                    continue
                expected = _FOLDER_TO_STATUS.get(state, [])
                if not any(e.lower() == declared.lower() for e in expected):
                    warnings.append({
                        "type": "warning",
                        "message": f'roadmap "{name}": folder is "{state}" but status declares "{declared}"'
                    })
            except OSError:
                pass

    return warnings


def validate_filename_uniqueness(cfg: dict) -> list:
    """Detecta o mesmo filename de roadmap em dois ou mais estados. Duplicata → violation."""
    states = ["wip", "backlog", "blocked", "done", "abandoned"]
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    seen = {}  # filename → [states]

    if cfg.get("roadmap_namespacing") == _config.NAMESPACING_BY_AGENT:
        agents = cfg.get("agents") or []
        if not agents:
            try:
                agents = [f for f in os.listdir(roadmap_dir) if os.path.isdir(os.path.join(roadmap_dir, f))]
            except OSError:
                agents = []
        for agent in agents:
            for state in states:
                for name in list_dir(os.path.join(roadmap_dir, agent, state)):
                    key = agent + "/" + name
                    seen.setdefault(key, []).append(state)
    else:
        for state in states:
            for name in list_dir(os.path.join(roadmap_dir, state)):
                seen.setdefault(name, []).append(state)

    violations = []
    for name, state_list in seen.items():
        if len(state_list) > 1:
            violations.append({
                "type": "violation",
                "message": f'roadmap "{name}" appears in multiple states: {state_list}'
            })
    return violations


def validate_branch_has_wip_roadmap(cfg: dict) -> list:
    """Verifica que branch feat/fix/refactor tem ao menos um roadmap em wip/ antes de trabalhar."""
    import subprocess
    # Derive the working directory from roadmap_dir so tests using tmp dirs get
    # an isolated git context (a tmp dir outside the repo returns non-zero).
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    git_cwd = os.path.dirname(os.path.abspath(roadmap_dir)) if roadmap_dir else None
    try:
        result = subprocess.run(
            ['git', 'symbolic-ref', '--short', 'HEAD'],
            capture_output=True, text=True, timeout=5,
            cwd=git_cwd
        )
        if result.returncode != 0:
            return []
        branch = result.stdout.strip()
    except Exception:
        return []

    if not (branch.startswith('feat/') or branch.startswith('fix/') or branch.startswith('refactor/')):
        return []

    wip_dirs = resolve_wip_dirs(cfg)
    total = 0
    for wip_dir in wip_dirs:
        if os.path.isdir(wip_dir):
            total += sum(1 for f in os.listdir(wip_dir) if f.endswith('.md'))

    if total == 0:
        return [
            f'branch "{branch}" is a feat/fix/refactor branch but no roadmap is in wip/ — '
            f'create governance artifacts first:\n'
            f'  trackfw req new "title"\n'
            f'  trackfw roadmap new "title"\n'
            f'  trackfw roadmap move <name> wip'
        ]
    return []


# ---------------------------------------------------------------------------
# validate() — ponto de entrada principal
# ---------------------------------------------------------------------------

def validate_unfiltered(cwd: str = None) -> dict:
    """
    Executa todas as validações sem filtro de baseline.
    Retorna {"violations": [...], "warnings": [...]} onde cada item é um dict com "message".
    Usa _apply_rule para distribuir resultados conforme severidade configurada (F3 — v2.4).
    """
    _config.reset()
    cfg = _config.load(cwd)

    violations = []
    warnings = []

    # Regras com severidade configurável via cfg["rules"]
    _apply_rule("wip_has_req",          validate_wip_has_req(cfg),                    violations, warnings, cfg)
    _apply_rule("adr_orphan",           validate_adrs_are_referenced(cfg),            violations, warnings, cfg)
    _apply_rule("wip_acceptance",       validate_wip_has_acceptance_criteria(cfg),    violations, warnings, cfg)
    _apply_rule("blocked_by_draft_adr", validate_reqs_not_blocked_by_draft_adrs(cfg), violations, warnings, cfg)
    _apply_rule("filename_uniqueness",  validate_filename_uniqueness(cfg),            violations, warnings, cfg)
    _apply_rule("branch_has_wip_roadmap", validate_branch_has_wip_roadmap(cfg),      violations, warnings, cfg)
    _apply_rule("ref_targets_exist",    validate_ref_targets_exist(cfg),              violations, warnings, cfg)
    _apply_rule("folder_status",        validate_folder_status_coherence(cfg),        violations, warnings, cfg)
    _apply_rule("stale_wip",            validate_stale_wip(cfg),                      violations, warnings, cfg)

    # Regras com severidade configurável (req_has_adr, blocked_has_req, req_has_roadmap)
    _apply_rule("req_has_adr",     validate_reqs_have_adr(cfg),     violations, warnings, cfg)
    _apply_rule("blocked_has_req", validate_blocked_has_req(cfg),   violations, warnings, cfg)
    _apply_rule("req_has_roadmap", validate_reqs_have_roadmap(cfg), violations, warnings, cfg)
    violations += _enrich_items(validate_frontmatter_presence(cfg),    "frontmatter_presence")

    # wip_limit: violations e warnings já separados internamente
    wip_limit_result = validate_wip_limit(cfg)
    _apply_rule("wip_limit", wip_limit_result["violations"], violations, warnings, cfg)
    warnings += _enrich_items(wip_limit_result["warnings"], "wip_limit")

    # Verificação bidirecional de req_id (desativada se trace_id_field não configurado)
    violations += _enrich_items(check_traceid(cfg), "traceid")

    return {"violations": violations, "warnings": warnings}


def validate(cwd: str = None) -> dict:
    """Executa validações, filtra pelo baseline (ratchet) e aplica modo lenient."""
    result = validate_unfiltered(cwd)
    violations = result.get("violations", [])
    warnings = result.get("warnings", [])

    # Ratchet: filtrar violations e warnings que já estavam no baseline
    baseline = load_baseline()
    if baseline is not None:
        baseline_set = set(baseline.get("violations", []))
        net_new = [v for v in violations
                   if _extract_messages([v])[0] not in baseline_set]
        violations = net_new
        baseline_warn_set = set(baseline.get("warnings", []))
        warnings = [w for w in warnings
                    if _extract_messages([w])[0] not in baseline_warn_set]

    # Modo lenient: mover violations para warnings
    if _is_lenient(cwd):
        warnings = warnings + violations
        violations = []

    return {"violations": violations, "warnings": warnings}


# ---------------------------------------------------------------------------
# Aliases e exportações para compatibilidade com o CLI
# ---------------------------------------------------------------------------

validate_single_wip = validate_wip_limit
