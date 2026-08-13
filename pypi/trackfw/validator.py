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
    "MARKER \n" ou "MARKER \r\n" (espaço + newline/CRLF) — P3: detecta
    campos vazios em arquivos CRLF além de arquivos LF.
    """
    for marker in markers:
        if marker in content and (marker + " \n") not in content and (marker + " \r\n") not in content:
            return True
    return False


# _RULE_DEFAULTS mapeia regras cujo default NÃO é 'error'.
_RULE_DEFAULTS = {
    "note_orphan": "warning",
    # ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A,
    # ADR-2026-08-12 Emenda 3: o script não carrega marcador de versão, então esta regra não
    # consegue distinguir drift legítimo (trackfw não atualizado ainda) de adulteração — fica
    # warning, nunca error. credential_guard_mode_downgrade fica deliberadamente ausente daqui:
    # cai no default "error" de _rule_severity.
    "credential_guard_script_integrity": "warning",
}


# ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1A.
# ADR: docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-
# estrita-entre-head-e-disco.md.
#
# As 3 regras abaixo resolvem severidade de forma DIFERENTE de todas as outras ~38: comparam HEAD
# contra disco e adotam a MAIS ESTRITA das duas, em vez de ler só o disco. Deliberado, não bug —
# sem isso, estas 3 regras podem ser desligadas pela mesma edição NÃO COMMITADA que elas deveriam
# denunciar (`rules: credential_guard_mode_downgrade: off` em trackfw.yaml, nunca commitado). Toda
# outra regra continua passando por _disk_rule_severity, byte-idêntico a antes deste ADR.
_CREDENTIAL_GUARD_ANCHORED_RULES = {
    "credential_guard_hook_resolvable",
    "credential_guard_script_integrity",
    "credential_guard_mode_downgrade",
}


def _credential_guard_severity_rank(s: str) -> int:
    """Ordena severidades da menos para a mais estrita, para a comparação "mais estrita vence" de
    _credential_guard_rule_severity. Qualquer valor fora de 'off'/'warning' só significa 'error' na
    prática — _apply_rule já trata qualquer valor não reconhecido como violation, então este
    ranking espelha esse mesmo fallback em vez de introduzir um contrato mais rígido.
    """
    if s == "off":
        return 0
    if s == "warning":
        return 1
    return 2


def _credential_guard_stricter_severity(a: str, b: str) -> str:
    """Retorna a mais estrita entre a e b ('error' > 'warning' > 'off')."""
    return a if _credential_guard_severity_rank(a) >= _credential_guard_severity_rank(b) else b


def _credential_guard_default_severity(name: str) -> str:
    """Mesmo fallback "_RULE_DEFAULTS > error" que _disk_rule_severity usa quando trackfw.yaml não
    tem rules: <name> — extraído para _credential_guard_rule_severity poder aplicá-lo igualmente ao
    lado HEAD (que não tem equivalente de _RULE_DEFAULTS próprio, já que
    config.parse_rules_from_content só devolve o que rules: em si contém).
    """
    return _RULE_DEFAULTS.get(name, "error")


def _credential_guard_rule_severity(name: str, cfg: dict, cwd: str = None) -> str:
    """Resolve a severidade de uma das 3 _CREDENTIAL_GUARD_ANCHORED_RULES como a MAIS ESTRITA entre
    HEAD e disco — direcional, não "ignora disco e usa só HEAD" (ver o parecer §2 e o ADR — o caso
    comum, HEAD sem menção à regra, precisa resolver para o default, ou seja o valor mais estrito
    possível, senão o disco venceria de volta silenciosamente sempre).

    Sem HEAD (não é git worktree, sem commits, ou trackfw.yaml não versionado no HEAD —
    _head_trackfw_yaml's 3 casos de "sem âncora"): cai no disco puro, igual a qualquer outra regra.
    ADR ponto de decisão 4: limite aceito, não um bypass acionável por adversário — nenhum desses 3
    casos é alcançável por uma edição não commitada de trackfw.yaml sozinha.
    """
    disk_severity = _disk_rule_severity(name, cfg)

    root = cwd or os.getcwd()
    head_content, ok = _head_trackfw_yaml(root)
    if not ok:
        return disk_severity

    head_rules = _config.parse_rules_from_content(head_content)
    head_severity = head_rules.get(name) or _credential_guard_default_severity(name)

    return _credential_guard_stricter_severity(head_severity, disk_severity)


def _rule_severity(name: str, cfg: dict, cwd: str = None) -> str:
    """Retorna severidade da regra: 'off' | 'warning' | 'error'.
    Prioridade: trackfw.yaml rules: > _RULE_DEFAULTS > 'error'.

    Para as 3 _CREDENTIAL_GUARD_ANCHORED_RULES, delega a _credential_guard_rule_severity acima —
    ver o comentário logo antes dessa constante para o porquê. Toda outra regra segue para
    _disk_rule_severity, textualmente idêntico ao corpo desta função antes do ADR-2026-08-12.
    """
    if name in _CREDENTIAL_GUARD_ANCHORED_RULES:
        return _credential_guard_rule_severity(name, cfg, cwd)
    return _disk_rule_severity(name, cfg)


def _disk_rule_severity(name: str, cfg: dict) -> str:
    """Resolução ordinária, só-disco, usada por toda regra exceto as 3
    _CREDENTIAL_GUARD_ANCHORED_RULES: trackfw.yaml rules: (CWD) > _RULE_DEFAULTS > 'error'.
    """
    rules = cfg.get("rules", {})
    if name in rules:
        return rules[name]
    if name in _RULE_DEFAULTS:
        return _RULE_DEFAULTS[name]
    return "error"


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


def _apply_rule(rule_name: str, msgs: list, violations: list, warnings: list, cfg: dict, cwd: str = None):
    """
    Distribui msgs (lista de dicts) conforme a severidade configurada da regra.
    - 'off'     → descarta
    - 'warning' → adiciona a warnings
    - 'error'   → adiciona a violations (default)
    Enriquece cada item com 'rule' e 'file' antes de distribuir.

    cwd é repassado a _rule_severity só é consultado pelas 3 regras de credential-guard
    ancoradas no HEAD (ver _credential_guard_rule_severity) — toda outra regra o ignora.
    """
    if not msgs:
        return
    severity = _rule_severity(rule_name, cfg, cwd)
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


def _try_list_dir(dir_path: str):
    """
    Tenta listar o diretório distinguindo "não existe" de outros erros.
    Retorna (entries: list, error: OSError|None).
    - error=None: sucesso, ou diretório ausente (ENOENT) — esperado para estados não usados.
    - error não-None: diretório EXISTE mas não pôde ser lido (ENOTDIR, EPERM…) — P2: reportar.
    """
    try:
        entries = []
        for name in os.listdir(dir_path):
            try:
                full = os.path.join(dir_path, name)
                if not os.path.isdir(full):
                    entries.append(name)
            except OSError:
                pass
        return entries, None
    except FileNotFoundError:
        return [], None  # diretório ausente — esperado
    except OSError as e:
        return [], e  # existe mas inacessível (ENOTDIR, EPERM…)


def _inspection_item(rule: str, target: str, err) -> dict:
    return {"type": "violation", "message": f'{rule}: could not inspect "{target}": {err}'}


def _list_dir_for_rule(rule: str, dir_path: str, messages: list) -> list:
    entries, err = _try_list_dir(dir_path)
    if err is not None:
        messages.append(_inspection_item(rule, dir_path, err))
    return entries


def _read_file_for_rule(rule: str, file_path: str, messages: list):
    try:
        with open(file_path, "r", encoding="utf-8") as f:
            return f.read()
    except OSError as e:
        messages.append(_inspection_item(rule, file_path, e))
        return None


def _walk_dir_md(dir_path: str) -> list:
    """Retorna basenames de todos .md recursivamente em dir_path."""
    return [os.path.basename(p) for p in _walk_dir_md_paths_for_rule("", dir_path, None)]


def _walk_dir_md_paths_for_rule(rule: str, dir_path: str, messages: list | None) -> list:
    """Retorna paths de todos .md recursivamente em dir_path e reporta falhas de walk."""
    result = []

    def onerror(err):
        if messages is not None and not isinstance(err, FileNotFoundError):
            messages.append(_inspection_item(rule, getattr(err, "filename", dir_path), err))

    for root, _, files in os.walk(dir_path, onerror=onerror):
        for name in files:
            if name.endswith(".md"):
                result.append(os.path.join(root, name))
    return result


def _is_subpath(path: str, parent: str) -> bool:
    """Retorna True se path está contido dentro do diretório parent (ambos absolutos)."""
    try:
        path_abs = os.path.abspath(path)
        parent_abs = os.path.abspath(parent)
        return os.path.commonpath([path_abs, parent_abs]) == parent_abs
    except ValueError:
        return False


def _find_adr_file(basename: str, adr_dirs: list) -> str:
    """Busca basename recursivamente em todos os adr_dirs. Retorna caminho completo ou ''."""
    for adr_dir in adr_dirs:
        expanded_dir = os.path.expanduser(adr_dir)
        try:
            for root, dirs, files in os.walk(expanded_dir):
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


_REF_DELIMITERS = ("\"", "'", "`")


def _strip_ref_delimiters(value: str) -> str:
    """
    Remove um delimitador (aspas duplas, simples ou backtick) de cada ponta,
    independentemente, sem exigir par casado — alinhado a Go (strings.Trim
    com cutset) e Node (regex de borda única). Uso contido ao caminho de
    extração de referência (_extract_ref_path); não afeta
    normalize_yaml_flat_value, que segue exigindo par casado para todo o
    resto do frontmatter (contrato do PR #104).
    """
    if value and value[0] in _REF_DELIMITERS:
        value = value[1:]
    if value and value[-1] in _REF_DELIMITERS:
        value = value[:-1]
    return value


def _extract_ref_path(content: str, field: str) -> str:
    """
    Extrai o caminho .md após 'field: valor' na mesma linha.
    Retorna '' se não encontrado ou não terminar em .md.
    """
    for line in content.split("\n"):
        trimmed = line.strip()
        if ":" not in trimmed:
            continue
        key, val = trimmed.split(":", 1)
        if key.strip().lower() == field.lower():
            val = val.strip()
            if not val or val in ("—", "-", "–"):
                return ""
            # Primeira "palavra" (antes de espaço)
            val = val.split()[0] if val.split() else ""
            val = _strip_ref_delimiters(val)
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
        states = ["backlog", "analyzing", "wip", "blocked", "done", "abandoned"]
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


def _resolve_state_dirs(cfg: dict, state: str) -> list:
    """
    Fonte única de resolução de caminho por estado (ex: 'wip', 'done') conforme o modo de
    namespacing. resolve_wip_dirs e resolve_done_dirs são wrappers finos sobre esta função.
    Duplicar a lógica aqui foi a causa raiz de defeitos anteriores (roadmap_dir divergente entre
    runtimes).
    flat     → [cfg["roadmap_dir"] + "/" + state]
    by_agent → [cfg["roadmap_dir"] + "/" + agent + "/" + state for agent in agents]
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
        return [roadmap_dir + "/" + agent + "/" + state for agent in agents]

    return [cfg.get("roadmap_dir", "docs/roadmaps") + "/" + state]


def resolve_wip_dirs(cfg: dict) -> list:
    """
    Retorna lista de diretórios wip/ conforme o modo de namespacing.
    flat     → [cfg["roadmap_dir"] + "/wip"]
    by_agent → [cfg["roadmap_dir"] + "/" + agent + "/wip" for agent in agents]
    """
    return _resolve_state_dirs(cfg, "wip")


def resolve_done_dirs(cfg: dict) -> list:
    """
    Retorna lista de diretórios done/ conforme o modo de namespacing.
    flat     → [cfg["roadmap_dir"] + "/done"]
    by_agent → [cfg["roadmap_dir"] + "/" + agent + "/done" for agent in agents]
    """
    return _resolve_state_dirs(cfg, "done")


def normalize_yaml_flat_value(value: str) -> str:
    """Normaliza valor YAML flat removendo apenas delimitador externo pareado (aspas)."""
    if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
        return value[1:-1]
    return value


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
                val = normalize_yaml_flat_value(stripped[colon_idx + 1:].strip())
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
    return _adr_draft_status_for_rule(basename, cfg, None)[0]


def _extract_adr_status(content: str) -> str:
    """
    Extrai o status declarado de um ADR. Tenta primeiro o frontmatter (`status:`),
    a fonte estruturada e canônica emitida por todos os geradores (`adr new` e
    `NewADRDraft` escrevem `status:` e a linha de cabeçalho em sincronia). Cai para a
    linha de cabeçalho ('> Date: ... | Status: X') quando não há frontmatter, para
    cobrir ADRs legados sem bloco YAML (ex.: ADR-001). Retorna '' se nenhum for encontrado.
    """
    fm = parse_frontmatter(content)
    fm_status = fm.get("status", "").strip()
    if fm_status:
        return fm_status
    marker = "| Status: "
    for line in content.split("\n"):
        trimmed = line.strip()
        idx = trimmed.find(marker)
        if idx >= 0:
            rest = trimmed[idx + len(marker):]
            pipe_idx = rest.find(" |")
            if pipe_idx >= 0:
                rest = rest[:pipe_idx]
            return rest.strip()
    return ""


def _adr_not_accepted(content: str) -> bool:
    """
    Helper canônico único: verdadeiro se o status do ADR for 'Draft' ou 'Proposed'
    (comparação case-insensitive, espelhando strings.EqualFold do CLI Go). 'Aceito' é
    definido por exclusão — qualquer outro status (Accepted, Superseded, Deprecated,
    Rejected, ...) conta como aceito e não deve ser enumerado aqui.
    """
    return _extract_adr_status(content).strip().lower() in ("draft", "proposed")


def _adr_draft_status_for_rule(basename: str, cfg: dict, messages: list | None):
    """
    Verifica se <basename> está em status não aceito (Draft ou Proposed, via
    _adr_not_accepted) em algum dos adrDirs configurados.
    Busca recursivamente nas subpastas via _find_adr_file.
    """
    adr_dirs = [os.path.expanduser(d) for d in cfg.get("adr_dirs", ["docs/adr"])]
    p = _find_adr_file(basename, adr_dirs)
    if not p:
        return False, True
    try:
        with open(p, "r", encoding="utf-8") as f:
            return _adr_not_accepted(f.read()), True
    except OSError as e:
        if messages is not None:
            messages.append(_inspection_item("blocked_by_draft_adr", p, e))
        return False, False


def _wip_config_from(cfg: dict) -> dict:
    """
    Deriva {"limit": int, "by_squad": bool} a partir do dict de config já normalizado por
    _config.load() — nenhuma releitura de trackfw.yaml acontece aqui.
    """
    limit = cfg.get("wip_limit", 1)
    if not isinstance(limit, int) or limit <= 0:
        limit = 1
    return {"limit": limit, "by_squad": bool(cfg.get("wip_by_squad", False))}


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
            return normalize_yaml_flat_value(trimmed[len("squad:"):].strip())
    return ""


def _governance_mode_from(cfg: dict) -> dict:
    """
    Deriva {"mode": str, "lenient_until": datetime|None} a partir do dict de config já
    normalizado por _config.load() — nenhuma releitura de trackfw.yaml acontece aqui.
    cfg["governance_mode"] chega como o valor bruto do campo (string vazia se ausente);
    cfg["lenient_until"] chega como a data literal (ex.: "2026-08-02"), convertida aqui
    para datetime.
    """
    result = {"mode": "strict", "lenient_until": None}
    mode = cfg.get("governance_mode")
    if mode:
        result["mode"] = mode
    lenient_until = cfg.get("lenient_until")
    if lenient_until:
        try:
            result["lenient_until"] = datetime.fromisoformat(lenient_until)
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
    gm = _governance_mode_from(_config.load(cwd))
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
        entries = _list_dir_for_rule("wip_has_req", wip_dir, violations)
        for name in entries:
            content = _read_file_for_rule("wip_has_req", os.path.join(wip_dir, name), violations)
            if content is None:
                continue
            if not _content_has_marker(content, req_markers):
                violations.append(
                    {"type": "violation", "message": f'roadmap "{name}" is in wip but has no linked REQ'}
                )
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
    req_markers = cfg.get("link_fields", {}).get("req", ["REQ:"])
    violations = []
    for blocked_dir in _resolve_state_dirs(cfg, "blocked"):
        for name in _list_dir_for_rule("blocked_has_req", blocked_dir, violations):
            content = _read_file_for_rule("blocked_has_req", os.path.join(blocked_dir, name), violations)
            if content is None:
                continue
            if not _content_has_marker(content, req_markers):
                violations.append(
                    {"type": "violation", "message": f'roadmap "{name}" is in blocked but has no linked REQ'}
                )
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


def validate_adrs_are_referenced(cfg: dict, cwd: str = None) -> list:
    """ADRs em adr_dirs não referenciados em nenhuma REQ → violation (busca recursiva).
    Isenta arquivos localizados fora do diretório raiz (cwd).
    """
    abs_cwd = os.path.realpath(os.path.abspath(cwd or os.getcwd()))
    violations = []
    adrs = []
    adr_dirs = [os.path.expanduser(d) for d in cfg.get("adr_dirs", ["docs/adr"])]
    for adr_dir in adr_dirs:
        expanded_dir = os.path.expanduser(adr_dir)
        for file_path in _walk_dir_md_paths_for_rule("adr_orphan", expanded_dir, violations):
            real_path = os.path.realpath(file_path)
            # Isenta arquivos localizados fora do CWD (ex.: ADRs globais compartilhados ou symlinks externos)
            if not _is_subpath(real_path, abs_cwd):
                continue
            adrs.append(os.path.basename(file_path))

    req_files = resolve_req_files(cfg)
    combined = ""
    for file_path in req_files:
        content = _read_file_for_rule("adr_orphan", file_path, violations)
        if content is not None:
            combined += content

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
        entries = _list_dir_for_rule("wip_acceptance", wip_dir, violations)
        for name in entries:
            content = _read_file_for_rule("wip_acceptance", os.path.join(wip_dir, name), violations)
            if content is None:
                continue
            if not _content_has_marker(content, acceptance_markers):
                violations.append(
                    {"type": "violation", "message": f'roadmap "{name}" is in wip but has no acceptance criteria block'}
                )
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

    wip_cfg = _wip_config_from(cfg)

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


def _roadmap_log_identity(cfg: dict, file_path: str) -> str:
    basename = os.path.basename(file_path)
    if cfg.get("roadmap_namespacing") != _config.NAMESPACING_BY_AGENT:
        return basename

    state_dir = os.path.dirname(file_path)
    agent_dir = os.path.dirname(state_dir)
    agent = os.path.basename(agent_dir)
    if agent:
        return f"{agent}/{basename}"
    return basename


def _parse_transition_log_line(line: str):
    fields = line.split()
    if len(fields) < 5:
        return None

    try:
        timestamp = datetime.strptime(f"{fields[0]} {fields[1]}", "%Y-%m-%d %H:%M").timestamp()
    except ValueError:
        return None

    arrow_idx = -1
    for idx in range(3, len(fields)):
        if fields[idx] in ("→", "->"):
            arrow_idx = idx
            break

    if arrow_idx < 0 or arrow_idx + 1 >= len(fields):
        return None

    return {
        "timestamp": timestamp,
        "name": fields[2],
        "to_state": fields[arrow_idx + 1],
    }


def _latest_wip_transition_time(cfg: dict, file_path: str):
    log_path = os.path.join(cfg.get("roadmap_dir", "docs/roadmaps"), ".trackfw-log")
    expected_name = _roadmap_log_identity(cfg, file_path)
    latest = None
    diagnostics = []

    try:
        with open(log_path, "r", encoding="utf-8") as f:
            for line in f:
                stripped = line.strip()
                if not stripped:
                    continue
                parsed = _parse_transition_log_line(stripped)
                if not parsed:
                    diagnostics.append({
                        "type": "warning",
                        "message": f'stale_wip: invalid support line in "{log_path}": "{stripped}"'
                    })
                    continue
                if parsed["name"] != expected_name or parsed["to_state"] != "wip":
                    continue
                if latest is None or parsed["timestamp"] > latest:
                    latest = parsed["timestamp"]
    except FileNotFoundError:
        return None, []
    except OSError as e:
        return None, [_inspection_item("stale_wip", log_path, e)]

    return latest, diagnostics


def validate_stale_wip(cfg: dict, days: int = None, now: float = None) -> list:
    """
    Arquivos em wip/ com idade desde a última transição para wip >= days dias → warning.
    Quando o log não existe ou não possui entrada parseável, usa mtime como fallback.
    Suporta modo by_agent via resolve_wip_dirs.
    """
    wip_dirs = resolve_wip_dirs(cfg)
    warnings = []
    threshold_days = days
    if threshold_days is None:
        try:
            threshold_days = int(cfg.get("stale_wip_days", STALE_WIP_DAYS))
        except (TypeError, ValueError):
            threshold_days = STALE_WIP_DAYS
    if threshold_days <= 0:
        threshold_days = STALE_WIP_DAYS

    now_ts = now if now is not None else datetime.now().timestamp()

    for wip_dir in wip_dirs:
        md_files = [
            os.path.join(wip_dir, f)
            for f in _list_dir_for_rule("stale_wip", wip_dir, warnings)
            if f.endswith(".md")
        ]

        for file_path in md_files:
            try:
                stat = os.stat(file_path)
                log_time, diagnostics = _latest_wip_transition_time(cfg, file_path)
                warnings.extend(diagnostics)
                ref_time = log_time if log_time is not None else stat.st_mtime
                age_seconds = now_ts - ref_time
                age_days = int(age_seconds / (60 * 60 * 24))
                if age_days >= threshold_days:
                    last_modified = datetime.fromtimestamp(ref_time).strftime("%Y-%m-%d")
                    basename = os.path.basename(file_path)
                    warnings.append({
                        "type": "warning",
                        "message": f"roadmap/wip/{basename} has been in WIP for {age_days} days (last modified {last_modified})"
                    })
            except OSError as e:
                warnings.append(_inspection_item("stale_wip", file_path, e))

    return warnings


def validate_reqs_not_blocked_by_draft_adrs(cfg: dict) -> list:
    """REQs Open com ADRs não aceitos (Draft ou Proposed, via _adr_not_accepted) na
    seção '## Blocked by ADRs' → violation. A regra deixou de ser cega a Proposed
    (ADR-2026-08-01), mas o **nome da regra permanece** blocked_by_draft_adr — é
    chave pública de config.
    """
    files = resolve_req_files(cfg)
    violations = []
    for file_path in files:
        name = os.path.basename(file_path)
        content = _read_file_for_rule("blocked_by_draft_adr", file_path, violations)
        if content is None:
            continue

        if "Status: Open" not in content:
            continue

        blocked_adrs = _parse_blocked_adrs(file_path)
        for adr_basename in blocked_adrs:
            if _adr_draft_status_for_rule(adr_basename, cfg, violations)[0]:
                # ML-1D (2026-08-01): reconciliação de paridade — "Draft" saiu porque a
                # regra cobre Proposed também; texto agora byte-idêntico ao Go/Node
                # ("is blocked by not-accepted ADR:").
                violations.append({
                    "type": "violation",
                    "message": f"REQ {name} is blocked by not-accepted ADR: {adr_basename}"
                })
    return violations


def validate_frontmatter_presence(cfg: dict) -> list:
    """Verifica presença de frontmatter em ADRs e REQs (busca recursiva em adr_dirs)."""
    violations = []
    adr_dirs = [os.path.expanduser(d) for d in cfg.get("adr_dirs", ["docs/adr"])]

    for adr_dir in adr_dirs:
        files = [f for f in _walk_dir_md(adr_dir) if f.endswith(".md")]
        for f in files:
            full_path = _find_adr_file(f, adr_dirs)
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
    dirs = resolve_wip_dirs(cfg) + _resolve_state_dirs(cfg, "blocked")
    for wip_dir in dirs:
        for name in _list_dir_for_rule("ref_targets_exist", wip_dir, warnings):
            content = _read_file_for_rule("ref_targets_exist", os.path.join(wip_dir, name), warnings)
            if content is None:
                continue
            ref = _extract_ref_path(content, "REQ")
            if ref and not _reference_exists(ref):
                warnings.append({
                    "type": "warning",
                    "message": f'roadmap "{name}" links to REQ "{ref}" which does not exist'
                })

    # REQs: verificar ADR: e Roadmap:
    for file_path in resolve_req_files(cfg):
        content = _read_file_for_rule("ref_targets_exist", file_path, warnings)
        if content is None:
            continue
        name = os.path.basename(file_path)
        adr_ref = _extract_ref_path(content, "ADR")
        if adr_ref and not _reference_exists(adr_ref):
            warnings.append({
                "type": "warning",
                "message": f'req "{name}" links to ADR "{adr_ref}" which does not exist'
            })
        roadmap_ref = _extract_ref_path(content, "Roadmap")
        if roadmap_ref and not _reference_exists(roadmap_ref):
            warnings.append({
                "type": "warning",
                "message": f'req "{name}" links to Roadmap "{roadmap_ref}" which does not exist'
            })

    return warnings


def _reference_exists(ref: str) -> bool:
    return os.path.exists(os.path.expanduser(ref))


def validate_req_roadmap_lifecycle(cfg: dict) -> list:
    """Sinaliza REQ Open cujo roadmap canônico referenciado já está em done/."""
    warnings = []
    for file_path in resolve_req_files(cfg):
        try:
            with open(file_path, "r", encoding="utf-8") as f:
                content = f.read()
            if not _req_status_is_open(content):
                continue
            ref = _extract_ref_path(content, "Roadmap")
            if not ref:
                continue
            expanded_ref = os.path.expanduser(ref)
            if not os.path.isfile(expanded_ref):
                continue
            if os.path.basename(os.path.dirname(expanded_ref)) == "done":
                warnings.append({
                    "type": "warning",
                    "message": f'req "{os.path.basename(file_path)}" is Open but linked Roadmap "{ref}" is in done/'
                })
        except OSError:
            pass
    return warnings


def _req_status_is_open(content: str) -> bool:
    for line in content.split("\n"):
        trimmed = line.strip()
        if ":" in trimmed:
            key, val = trimmed.split(":", 1)
            if key.strip().lower() == "status":
                return normalize_yaml_flat_value(val.strip()).lower() == "open"
        marker = "| Status: "
        idx = trimmed.find(marker)
        if idx >= 0:
            rest = trimmed[idx + len(marker):]
            pipe_idx = rest.find(" |")
            if pipe_idx >= 0:
                rest = rest[:pipe_idx]
            return rest.strip().lower() == "open"
    return False


def _req_status_is_done(content: str) -> bool:
    for line in content.split("\n"):
        trimmed = line.strip()
        if ":" in trimmed:
            key, val = trimmed.split(":", 1)
            if key.strip().lower() == "status":
                return normalize_yaml_flat_value(val.strip()).lower() == "done"
        marker = "| Status: "
        idx = trimmed.find(marker)
        if idx >= 0:
            rest = trimmed[idx + len(marker):]
            pipe_idx = rest.find(" |")
            if pipe_idx >= 0:
                rest = rest[:pipe_idx]
            return rest.strip().lower() == "done"
    return False


def validate_adr_accepted_when_req_done(cfg: dict) -> list:
    """
    REQ com Status: Done referenciando (campo 'ADR:') um ADR ainda não aceito
    (Draft ou Proposed, via _adr_not_accepted) -> violation. 'Aceito' é definido por
    exclusão: Superseded/Deprecated/Rejected (e qualquer status != Draft/Proposed)
    não disparam a regra — REQ Done apoiada em ADR posteriormente substituído é
    histórico legítimo. REQ que não está Done nunca dispara esta regra.
    """
    violations = []
    adr_dirs = [os.path.expanduser(d) for d in cfg.get("adr_dirs", ["docs/adr"])]
    for file_path in resolve_req_files(cfg):
        req_name = os.path.basename(file_path)
        content = _read_file_for_rule("adr_accepted_when_req_done", file_path, violations)
        if content is None:
            continue
        if not _req_status_is_done(content):
            continue
        adr_ref = _extract_ref_path(content, "ADR")
        if not adr_ref:
            continue
        adr_basename = os.path.basename(adr_ref)
        adr_path = _find_adr_file(adr_basename, adr_dirs)
        if not adr_path:
            continue
        adr_content = _read_file_for_rule("adr_accepted_when_req_done", adr_path, violations)
        if adr_content is None:
            continue
        if _adr_not_accepted(adr_content):
            status = _extract_adr_status(adr_content) or "unknown"
            # ML-1D (2026-08-01): reconciliação de paridade — texto agora byte-idêntico
            # ao Go/Node: aspas em torno dos dois basenames + sufixo "(status: X)".
            violations.append({
                "type": "violation",
                "message": f'REQ "{req_name}" is Done but linked ADR "{adr_basename}" is not accepted (status: {status})'
            })
    return violations


_FOLDER_TO_STATUS = {
    "wip":       ["WIP", "wip", "In Progress"],
    "backlog":   ["Backlog", "backlog"],
    "analyzing": ["Analyzing", "analyzing"],
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
    states = ["wip", "backlog", "analyzing", "blocked", "done", "abandoned"]
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
        # P2: distinguir "diretório ausente" (esperado) de outros erros (reportar).
        entries, read_error = _try_list_dir(dir_path)
        if read_error is not None:
            warnings.append({
                "type": "warning",
                "message": f'folder_status: could not read directory "{dir_path}": {read_error}'
            })
            continue
        for name in entries:
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
    states = ["wip", "backlog", "analyzing", "blocked", "done", "abandoned"]
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    seen = {}  # filename → [states]

    list_errors = []
    if cfg.get("roadmap_namespacing") == _config.NAMESPACING_BY_AGENT:
        agents = cfg.get("agents") or []
        if not agents:
            try:
                agents = [f for f in os.listdir(roadmap_dir) if os.path.isdir(os.path.join(roadmap_dir, f))]
            except OSError:
                agents = []
        for agent in agents:
            for state in states:
                dir_path = os.path.join(roadmap_dir, agent, state)
                entries, read_error = _try_list_dir(dir_path)
                if read_error is not None:
                    list_errors.append({
                        "type": "violation",
                        "message": f'filename_uniqueness: could not read directory "{dir_path}": {read_error}'
                    })
                    continue
                for name in entries:
                    key = agent + "/" + name
                    seen.setdefault(key, []).append(state)
    else:
        for state in states:
            dir_path = os.path.join(roadmap_dir, state)
            entries, read_error = _try_list_dir(dir_path)
            if read_error is not None:
                list_errors.append({
                    "type": "violation",
                    "message": f'filename_uniqueness: could not read directory "{dir_path}": {read_error}'
                })
                continue
            for name in entries:
                seen.setdefault(name, []).append(state)

    violations = list(list_errors)
    # P3: ordenar os nomes e os estados para saída determinística.
    for name in sorted(seen.keys()):
        state_list = seen[name]
        if len(state_list) > 1:
            sorted_states = sorted(state_list)
            violations.append({
                "type": "violation",
                "message": f'roadmap "{name}" appears in multiple states: {sorted_states}'
            })
    return violations


def normalize_branch_slug(value: str) -> str:
    """Normaliza um slug de branch para comparação (lowercase, runs de não-alfanumérico → '-',
    sem '-' nas pontas). Espelha internal/validator/validator.go normalizeBranchSlug /
    NormalizeBranchSlug. Reutilizada por validate_branch_has_wip_roadmap e pelo comando
    `trackfw branch new` — nunca duplicar esta lógica."""
    return re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")


def branch_slug_matches_roadmap(branch_slug: str, wip_dirs: list, done_dirs: list):
    """Verifica se branch_slug (já normalizado via normalize_branch_slug) casa com o nome de
    algum roadmap .md encontrado em wip_dirs ou done_dirs. Espelha
    internal/validator/validator.go BranchSlugMatchesRoadmap. Reutilizada por
    validate_branch_has_wip_roadmap e pelo comando `trackfw branch new` — nunca duplicar esta
    lógica.

    Retorna (matched: bool, candidates: list) — candidates lista todos os roadmaps .md
    encontrados em wip_dirs+done_dirs (para diagnóstico/mensagem de orientação quando matched é
    False).
    """
    matched = False
    candidates = []
    for search_dir in wip_dirs + done_dirs:
        if os.path.isdir(search_dir):
            for f in os.listdir(search_dir):
                if f.endswith('.md'):
                    candidates.append(f)
                    if branch_slug in normalize_branch_slug(f):
                        matched = True
    return matched, candidates


def branch_governance_orientation(branch: str) -> str:
    """Mensagem de orientação impressa quando uma branch feat/fix/refactor não tem nenhum
    roadmap em wip/ nem em done/ (candidates vazio). Espelha
    internal/validator/validator.go BranchGovernanceOrientation — byte-idêntica. Compartilhada
    por validate_branch_has_wip_roadmap e `trackfw branch new` — nunca duplicar esta string."""
    return (
        f'branch "{branch}" is a feat/fix/refactor branch but no roadmap is in wip/ nor done/ — '
        f'create governance artifacts first:\n'
        f'  trackfw req new "title"\n'
        f'  trackfw roadmap new "title"\n'
        f'  trackfw roadmap move <name> wip'
    )


def branch_no_matching_roadmap_message(branch: str, candidates: list) -> str:
    """Mensagem de orientação impressa quando existem roadmaps em wip/ ou done/ mas nenhum casa
    com o slug da branch. Espelha internal/validator/validator.go
    BranchNoMatchingRoadmapMessage — byte-idêntica. Compartilhada por
    validate_branch_has_wip_roadmap e `trackfw branch new` — nunca duplicar esta string. Não
    muta candidates."""
    # P3: sort for deterministic output regardless of filesystem ordering.
    sorted_candidates = sorted(candidates)
    display = sorted_candidates[:3]
    suffix = f", e mais {len(sorted_candidates) - 3}" if len(sorted_candidates) > 3 else ""
    return (
        f'branch "{branch}" has no matching roadmap in wip/ nor done/ '
        f'(found: {", ".join(display)}{suffix}) — include the branch slug in the roadmap filename '
        f'or set TRACKFW_BRANCH explicitly in CI'
    )


def validate_branch_has_wip_roadmap(cfg: dict) -> list:
    """Verifica que branch feat/fix/refactor tem ao menos um roadmap em wip/ antes de trabalhar."""
    import subprocess
    # Derive the working directory from roadmap_dir so tests using tmp dirs get
    # an isolated git context (a tmp dir outside the repo returns non-zero).
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    git_cwd = os.path.dirname(os.path.abspath(roadmap_dir)) if roadmap_dir else None
    branch = os.environ.get("TRACKFW_BRANCH") or ""
    if not branch and git_cwd and _is_git_worktree(git_cwd):
        try:
            result = subprocess.run(
                ['git', 'symbolic-ref', '--short', 'HEAD'],
                capture_output=True, text=True, timeout=5,
                cwd=git_cwd
            )
            branch = result.stdout.strip() if result.returncode == 0 else ""
        except Exception:
            branch = ""
        if not branch:
            branch = (
                os.environ.get("GITHUB_HEAD_REF")
                or os.environ.get("CI_COMMIT_REF_NAME")
                or os.environ.get("GITHUB_REF_NAME")
                or ""
            )

    if not branch:
        return []

    if not (branch.startswith('feat/') or branch.startswith('fix/') or branch.startswith('refactor/')):
        return []

    wip_dirs = resolve_wip_dirs(cfg)
    done_dirs = resolve_done_dirs(cfg)
    branch_slug = normalize_branch_slug(branch.split("/", 1)[1])
    matched, candidates = branch_slug_matches_roadmap(branch_slug, wip_dirs, done_dirs)
    if matched:
        return []

    if not candidates:
        return [branch_governance_orientation(branch)]
    return [branch_no_matching_roadmap_message(branch, candidates)]


def _is_git_worktree(cwd: str) -> bool:
    """Retorna True se cwd pertence a um worktree git."""
    try:
        result = subprocess.run(
            ['git', 'rev-parse', '--is-inside-work-tree'],
            capture_output=True, text=True, timeout=5,
            cwd=cwd,
        )
        return result.returncode == 0 and result.stdout.strip() == "true"
    except Exception:
        return False


def validate_note_orphan(cfg: dict, cwd: str = None) -> list:
    """
    Detecta notas em vault/notes/ não referenciadas pelo index.md.
    index.md não conta como nota órfã. Projeto sem vault/ retorna [].
    Aceita link markdown `[texto](arquivo.md)` ou wikilink `[[nome]]`.
    """
    base = cwd or os.getcwd()
    vault_dir = os.path.join(base, "vault", "notes")
    if not os.path.isdir(vault_dir):
        return []

    index_path = os.path.join(vault_dir, "index.md")
    index_content = ""
    if os.path.exists(index_path):
        with open(index_path, "r", encoding="utf-8") as f:
            index_content = f.read()

    msgs = []
    try:
        entries = os.listdir(vault_dir)
    except OSError:
        return []

    for filename in sorted(entries):
        if not filename.endswith(".md") or filename == "index.md":
            continue
        name_without_ext = re.sub(r"\.md$", "", filename)
        referenced = (
            f"({filename})" in index_content
            or f"[[{name_without_ext}]]" in index_content
            or f"[[{filename}]]" in index_content
        )
        if not referenced:
            msgs.append({
                "type": "warning",
                "message": f'note "{filename}" is not referenced in vault/notes/index.md',
                "rule": "note_orphan",
                "file": filename,
            })
    return msgs


# CREDENTIAL_GUARD_SCRIPT_MARKER é o nome do script que a regra
# credential_guard_hook_resolvable procura dentro dos comandos de hook de projeto.
_CREDENTIAL_GUARD_SCRIPT_MARKER = "trackfw-credential-guard.sh"

# _CREDENTIAL_GUARD_HOOK_FILES é a lista fechada dos arquivos de hook de PROJETO que o trackfw
# gera hoje e que podem conter uma entrada de credential-guard
# (ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard, ML-1A). Hooks de escopo GLOBAL
# (~/.trackfw/..., trackfw update harness) ficam fora — caso distinto, fora do repositório do
# usuário, e a checagem de dedup global_credential_guard_installed_*() já os pula de propósito nas
# entradas de projeto.
_CREDENTIAL_GUARD_HOOK_FILES = [
    (".claude/settings.json", "Claude Code"),
    (".codex/hooks.json", "Codex CLI"),
    (".gemini/settings.json", "Gemini CLI"),
    (".cursor/hooks.json", "Cursor"),
    (".github/hooks/trackfw-attention.json", "GitHub Copilot CLI"),
    (".kiro/hooks/trackfw-attention.json", "Kiro"),
]


def _resolve_credential_guard_hook_path(raw: str, root: str):
    """Resolve o valor bruto de um comando de hook (string extraída do JSON) para um caminho de
    arquivo absoluto, usando exatamente as 3 formas de prefixo que o trackfw emite hoje
    (docs/cli-parity.md, "Mecanismo de resolução de caminho dos hooks de projeto, por CLI"):

    1. "$CLAUDE_PROJECT_DIR/…" / "$GEMINI_PROJECT_DIR/…" — placeholder de env var expandido em
       runtime pelo próprio CLI, substituído aqui pela raiz do projeto.
    2. '"$(git rev-parse --show-toplevel)/…"' — substituição de shell entre aspas literais
       (Codex). As aspas fazem parte do valor emitido e são removidas antes de resolver contra a
       raiz do projeto.
    3. Caminho relativo puro, sem prefixo nenhum (Cursor/Copilot/Kiro) — resolvido diretamente
       contra a raiz do projeto.

    Retorna None quando o valor não bate em nenhuma das 3 formas — o chamador NÃO deve tratar
    isso como violação.
    """
    claude_prefix = "$CLAUDE_PROJECT_DIR/"
    gemini_prefix = "$GEMINI_PROJECT_DIR/"
    codex_prefix = '"$(git rev-parse --show-toplevel)/'

    if raw.startswith(claude_prefix):
        return os.path.join(root, raw[len(claude_prefix):])
    if raw.startswith(gemini_prefix):
        return os.path.join(root, raw[len(gemini_prefix):])
    if raw.startswith(codex_prefix) and raw.endswith('"'):
        inner = raw[len(codex_prefix):-1]
        return os.path.join(root, inner)
    if not raw.startswith("$") and not raw.startswith('"') and not os.path.isabs(raw):
        # Caminho relativo puro — Cursor (beforeShellExecution/preToolUse), GitHub Copilot CLI
        # (campo "bash"), Kiro (action.command).
        return os.path.join(root, raw)
    return None


def _collect_credential_guard_commands(value, out: list):
    """Percorre recursivamente um valor JSON já decodificado e coleta todo valor-string que
    referencia trackfw-credential-guard.sh, independentemente do nome do campo que o contém.

    Os 6 formatos de hook usam campos diferentes para o comando: "command" (Claude/Codex/
    Gemini/Cursor), "bash" (GitHub Copilot CLI), "action.command" (Kiro). Varrer por VALOR em vez
    de por caminho de chave evita acoplar esta regra à forma exata de cada schema.
    """
    if isinstance(value, str):
        if _CREDENTIAL_GUARD_SCRIPT_MARKER in value:
            out.append(value)
    elif isinstance(value, list):
        for item in value:
            _collect_credential_guard_commands(item, out)
    elif isinstance(value, dict):
        for val in value.values():
            _collect_credential_guard_commands(val, out)


def validate_credential_guard_hook_resolvable(cfg: dict, cwd: str = None) -> list:
    """Regra "credential_guard_hook_resolvable": para cada arquivo de hook de PROJETO que
    existir, extrai os comandos que referenciam trackfw-credential-guard.sh, resolve o caminho e
    verifica que o script existe e é executável.

    Riscos de regressão mapeados no roadmap:
    - A regra só avalia entradas que EXISTEM. Ausência de entrada de guard é estado legítimo
      (guard global instalado via `trackfw update harness`) — nunca é violação por si só.
    - Arquivo de hook ausente é pulado em silêncio.
    - Arquivo de hook presente mas com JSON inválido é pulado em silêncio — validar a forma do
      JSON não é escopo desta regra.
    """
    root = cwd or os.getcwd()
    msgs = []

    for rel_path, cli in _CREDENTIAL_GUARD_HOOK_FILES:
        full_path = os.path.join(root, rel_path)
        try:
            with open(full_path, "r", encoding="utf-8") as f:
                content = f.read()
        except OSError:
            continue

        try:
            parsed = json.loads(content)
        except (json.JSONDecodeError, ValueError):
            continue

        commands = []
        _collect_credential_guard_commands(parsed, commands)

        seen = set()
        for raw in commands:
            if raw in seen:
                continue
            seen.add(raw)

            resolved = _resolve_credential_guard_hook_path(raw, root)
            if resolved is None:
                continue

            if not os.path.exists(resolved):
                msgs.append({
                    "type": "violation",
                    "message": (
                        f'{rel_path} ({cli}) references trackfw-credential-guard.sh resolved to '
                        f'"{resolved}", but the script does not exist — run `trackfw update` to '
                        f'regenerate it'
                    ),
                })
            elif not os.access(resolved, os.X_OK):
                msgs.append({
                    "type": "violation",
                    "message": (
                        f'{rel_path} ({cli}) references trackfw-credential-guard.sh resolved to '
                        f'"{resolved}", but the script is not executable — run `trackfw update` to '
                        f'regenerate it'
                    ),
                })

    return msgs


# ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A.
# _CREDENTIAL_GUARD_SCRIPT_REFERENCE is a validator-local copy of the same template composed in
# generators/init_gen.py (_CREDENTIAL_GUARD_SH = _CG_HEADER + _CG_PROJECT_GUARD +
# _CG_DETECTION_CORE + _CG_PROJECT_TAIL). Kept as a literal copy -- same choice made for Go
# (internal/validator/validator_credential_guard_integrity_reference.go) and Node
# (npm/src/validator/index.js, CREDENTIAL_GUARD_SCRIPT_REFERENCE) -- for uniformity across the 3
# stacks, even though Python's _generate_credential_guard_script has no stdout side effect that
# would force this choice on its own (unlike Go/Node, whose generator functions print a success
# line on every call). Drift is caught by tests/test_credential_guard_integrity.py, which
# regenerates the real script via _generate_credential_guard_script() into a temp dir and asserts
# byte-equality against this constant. Raw string (r"""...""") to match the convention already
# used by _CG_HEADER/_CG_PROJECT_GUARD/_CG_DETECTION_CORE in generators/init_gen.py -- avoids
# Python interpreting the shell script's own backslash escapes (e.g. \. in the JWT regex).
_CREDENTIAL_GUARD_SCRIPT_REFERENCE = r"""#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

JWT_PATTERN='eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'
AWS_KEY_PATTERN='AKIA[0-9A-Z]{16}'

MATCH=""
if printf '%s' "$INPUT" | grep -qE "$JWT_PATTERN"; then
  MATCH="JWT"
elif printf '%s' "$INPUT" | grep -qE "$AWS_KEY_PATTERN"; then
  MATCH="AWS access key"
fi

# The raw payload is JSON: any double quote inside the underlying tool_input.command is
# escaped as \" -- unescape those before scanning for redirect targets, or a quoted target
# like "$TMPFILE" is seen as starting with a literal backslash instead of a variable
# reference.
RAW=$(printf '%s' "$INPUT" | sed 's/\\"/"/g')

# Ignore matches that are only ever written to an ephemeral destination
# (mktemp-derived path or /dev/null). A match with no redirect at all
# (printed to stdout, e.g.) or redirected to a plain file path still
# alerts -- that is the incident this hook guards against.
is_ephemeral_target() {
  local target
  target=$(printf '%s' "$1" | tr -d "\"'" | sed -E 's/[},]+$//')
  case "$target" in
    /dev/null) return 0 ;;
    *mktemp*) return 0 ;;
  esac
  if printf '%s' "$target" | grep -qE '^\$\{?[A-Za-z_][A-Za-z0-9_]*\}?$'; then
    local varname pattern
    varname=$(printf '%s' "$target" | sed -E 's/^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$/\1/')
    pattern="*${varname}="'$(mktemp'"*"
    case "$RAW" in
      $pattern) return 0 ;;
    esac
  fi
  return 1
}

REDIRECTS=$(printf '%s' "$RAW" | grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+' || true)

# Second detection layer: only runs when the payload scan above found nothing -- keeps the common
# case (match already found) cheap and avoids reading files unnecessarily. Files above the size cap
# are skipped silently.
scan_file_for_pattern() {
  local path size
  path=$(printf '%s' "$1" | tr -d "\"'" | sed -E 's/[},]+$//')
  [ -n "$path" ] && [ -f "$path" ] || return 1
  size=$(wc -c < "$path" 2>/dev/null | tr -d '[:space:]')
  size=${size:-0}
  [ "$size" -lt 1048576 ] || return 1
  if grep -qE "$JWT_PATTERN" "$path" 2>/dev/null; then
    MATCH="JWT"
    return 0
  fi
  if grep -qE "$AWS_KEY_PATTERN" "$path" 2>/dev/null; then
    MATCH="AWS access key"
    return 0
  fi
  return 1
}

if [ -z "$MATCH" ] && [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      scan_file_for_pattern "$target" && break
    fi
  done <<< "$REDIRECTS"
fi

if [ -z "$MATCH" ]; then
  CMD_LINE=$(printf '%s' "$RAW" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  if [ -n "$CMD_LINE" ]; then
    set -- $CMD_LINE
    cmd_name="${1:-}"
    case "$cmd_name" in
      cat|head|tail|jq|grep)
        shift
        for token in "$@"; do
          scan_file_for_pattern "$token" && break
        done
        ;;
    esac
  fi
fi

[ -n "$MATCH" ] || exit 0

HAS_REDIRECT=0
EXEMPT=1
if [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    HAS_REDIRECT=1
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      EXEMPT=0
    fi
  done <<< "$REDIRECTS"
fi

if [ "$HAS_REDIRECT" -eq 1 ] && [ "$EXEMPT" -eq 1 ]; then
  exit 0
fi

DEFAULT_MODE="warn"
MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
esac

if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$MSG_ESC" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
"""


_CREDENTIAL_GUARD_MODE_LOOKBEHIND_LINES = 5


def _extract_credential_guard_mode(content: str):
    """Mirrors the shell script's own resolution of credential_guard.mode (`grep -A 5
    '^credential_guard:'`): the mode key is found on the matched line or within the 5 lines
    following it. Deliberately the SAME lightweight line-scan the shipped script itself uses --
    not a full YAML parser -- so this rule's notion of "what credential_guard.mode resolves to"
    matches what actually runs at hook time.

    Returns (mode: str, ok: bool). ok is False when no "credential_guard:" line exists at all, OR
    when it exists but no "mode:" key is found within the lookbehind window.
    """
    lines = content.split("\n")
    start = -1
    for i, line in enumerate(lines):
        if line.startswith("credential_guard:"):
            start = i
            break
    if start == -1:
        return "", False

    end = min(start + 1 + _CREDENTIAL_GUARD_MODE_LOOKBEHIND_LINES, len(lines))
    for line in lines[start:end]:
        trimmed = line.strip()
        if "mode:" not in trimmed:
            continue
        rest = trimmed[len("mode:"):] if trimmed.startswith("mode:") else trimmed
        rest = rest.strip()
        hash_idx = rest.find("#")
        if hash_idx >= 0:
            rest = rest[:hash_idx].strip()
        rest = rest.strip("\"'")
        return rest, True

    return "", False


def _head_trackfw_yaml(cwd: str):
    """Returns the content of trackfw.yaml as committed at HEAD, resolved relative to cwd (not
    necessarily the git toplevel -- `trackfw validate` can run from a subdirectory). ok is False
    whenever there is no usable anchor: not a git worktree, no commits yet, or trackfw.yaml not
    tracked at HEAD -- every one of these is a "no anchor, stay silent" case, never an error.
    """
    if not _is_git_worktree(cwd):
        return "", False
    try:
        verify = subprocess.run(
            ["git", "rev-parse", "--verify", "HEAD"],
            capture_output=True, text=True, timeout=5, cwd=cwd,
        )
        if verify.returncode != 0:
            return "", False
    except Exception:
        return "", False

    try:
        show = subprocess.run(
            ["git", "show", "HEAD:./trackfw.yaml"],
            capture_output=True, text=True, timeout=5, cwd=cwd,
        )
        if show.returncode != 0:
            return "", False
        return show.stdout, True
    except Exception:
        return "", False


def _credential_guard_mode_downgrade_message() -> str:
    return (
        "trackfw.yaml sets credential_guard.mode: block at the git HEAD commit, but the "
        "current file does not resolve to block — if this was intentional, commit the change; "
        "otherwise investigate before treating the credential guard as active"
    )


def validate_credential_guard_script_integrity(cwd: str = None) -> list:
    """Regra "credential_guard_script_integrity": compara scripts/trackfw-credential-guard.sh em
    disco contra o template que esta versão do trackfw geraria (âncora: o binário/pacote, não o
    git). Silenciosa quando o script não existe -- ausência é escopo de
    credential_guard_hook_resolvable, não desta regra. Severidade default "warning" (ver
    _RULE_DEFAULTS): o script não carrega marcador de versão, então esta regra não consegue
    distinguir drift legítimo de adulteração real -- a mensagem é causalmente neutra por isso
    (ADR-2026-08-12 Emenda 3).
    """
    root = cwd or os.getcwd()
    rel_path = "scripts/trackfw-credential-guard.sh"
    full_path = os.path.join(root, rel_path)
    try:
        with open(full_path, "r", encoding="utf-8") as f:
            content = f.read()
    except FileNotFoundError:
        return []
    except OSError:
        return []

    if content == _CREDENTIAL_GUARD_SCRIPT_REFERENCE:
        return []

    return [{
        "type": "warning",
        "message": (
            f"{rel_path} content diverges from the template this version of trackfw generates — "
            f"if you did not edit this file by hand, run `trackfw update` to regenerate it"
        ),
    }]


def validate_credential_guard_mode_downgrade(cwd: str = None) -> list:
    """Regra "credential_guard_mode_downgrade": dispara apenas quando credential_guard.mode era
    explicitamente "block" no HEAD do git e o trackfw.yaml atual em disco não resolve mais para
    "block" (warn explícito, valor não reconhecido, ou chave/arquivo ausente -- todos os quais o
    próprio script resolveria como "warn", o DEFAULT_MODE da variante de projeto).

    Silenciosa sempre que HEAD não é "block": isso é "sem âncora para detectar downgrade", não
    "nada errado". A ausência da chave em DISCO NUNCA é tratada como silêncio -- é exatamente a
    via que esta regra existe para cobrir.
    """
    root = cwd or os.getcwd()
    head_content, ok = _head_trackfw_yaml(root)
    if not ok:
        return []

    head_mode, _ = _extract_credential_guard_mode(head_content)
    if head_mode != "block":
        return []

    disk_path = os.path.join(root, "trackfw.yaml")
    try:
        with open(disk_path, "r", encoding="utf-8") as f:
            disk_content = f.read()
    except FileNotFoundError:
        # trackfw.yaml deletado inteiramente enquanto HEAD tinha mode: block -- é o downgrade.
        return [{"type": "violation", "message": _credential_guard_mode_downgrade_message()}]
    except OSError:
        return [{"type": "violation", "message": _credential_guard_mode_downgrade_message()}]

    disk_mode, _ = _extract_credential_guard_mode(disk_content)
    if disk_mode == "block":
        return []

    return [{"type": "violation", "message": _credential_guard_mode_downgrade_message()}]



def validate_adr_dirs_exist(cfg: dict) -> dict:
    """
    Verifica se todos os diretórios configurados em adr_dirs existem.
    - Se strict_ci_paths for True → gera violation em violations.
    - Se strict_ci_paths for False (default) → gera Warning em warnings.
    """
    violations = []
    warnings = []
    strict_ci = cfg.get("strict_ci_paths", False)
    adr_dirs = [os.path.expanduser(d) for d in cfg.get("adr_dirs", ["docs/adr"])]
    for adr_dir in adr_dirs:
        if not os.path.exists(adr_dir) or not os.path.isdir(adr_dir):
            msg = f'adr_dir "{adr_dir}" does not exist'
            item = {
                "type": "violation" if strict_ci else "warning",
                "message": msg,
                "rule": "adr_dir_exists",
                "file": adr_dir,
            }
            if strict_ci:
                violations.append(item)
            else:
                warnings.append(item)
    return {"violations": violations, "warnings": warnings}


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

    # Verificação de existência de adr_dirs (Warning por padrão, Erro se strict_ci_paths: true)
    dir_check = validate_adr_dirs_exist(cfg)
    violations.extend(dir_check["violations"])
    warnings.extend(dir_check["warnings"])

    # Regras com severidade configurável via cfg["rules"]
    _apply_rule("wip_has_req",          validate_wip_has_req(cfg),                    violations, warnings, cfg)
    _apply_rule("adr_orphan",           validate_adrs_are_referenced(cfg, cwd),       violations, warnings, cfg)
    _apply_rule("wip_acceptance",       validate_wip_has_acceptance_criteria(cfg),    violations, warnings, cfg)
    _apply_rule("blocked_by_draft_adr", validate_reqs_not_blocked_by_draft_adrs(cfg), violations, warnings, cfg)
    _apply_rule("adr_accepted_when_req_done", validate_adr_accepted_when_req_done(cfg), violations, warnings, cfg)
    _apply_rule("filename_uniqueness",  validate_filename_uniqueness(cfg),            violations, warnings, cfg)
    _apply_rule("branch_has_wip_roadmap", validate_branch_has_wip_roadmap(cfg),      violations, warnings, cfg)
    _apply_rule("ref_targets_exist",    validate_ref_targets_exist(cfg),              violations, warnings, cfg)
    warnings += _enrich_items(validate_req_roadmap_lifecycle(cfg), "req_roadmap_lifecycle")
    _apply_rule("folder_status",        validate_folder_status_coherence(cfg),        violations, warnings, cfg)
    _apply_rule("stale_wip",            validate_stale_wip(cfg),                      violations, warnings, cfg)
    _apply_rule("note_orphan",          validate_note_orphan(cfg, cwd),               violations, warnings, cfg)
    _apply_rule("credential_guard_hook_resolvable", validate_credential_guard_hook_resolvable(cfg, cwd), violations, warnings, cfg, cwd)

    # ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A:
    # detecta adulteração do credential-guard, âncora por alvo (ADR-2026-08-12 Emenda 1).
    _apply_rule("credential_guard_script_integrity", validate_credential_guard_script_integrity(cwd), violations, warnings, cfg, cwd)
    _apply_rule("credential_guard_mode_downgrade", validate_credential_guard_mode_downgrade(cwd), violations, warnings, cfg, cwd)

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
    """Executa validações, filtra pelo baseline (ratchet) e aplica modo lenient.

    ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-
    head-e-disco: carve-out do baseline — violations/warnings de uma das 3
    _CREDENTIAL_GUARD_ANCHORED_RULES NUNCA são toleradas via .trackfw-baseline.json, não importa o
    que o arquivo contenha para elas. Mecanismo DIFERENTE do HEAD-vs-disco em
    _credential_guard_rule_severity: .trackfw-baseline.json é .gitignore'd DE PROPÓSITO ("baseline
    local de violations toleradas (nao versionado)"), então não há HEAD desse arquivo para
    comparar — "exigir commit" simplesmente não se aplica a um arquivo que o projeto decidiu nunca
    versionar. A única forma de fechar esse canal é excluir estas 3 regras da elegibilidade de
    ratchet, por nome, independente do conteúdo da mensagem — daí a checagem por item.get("rule")
    abaixo (populada por _enrich_items em _apply_rule).
    """
    result = validate_unfiltered(cwd)
    violations = result.get("violations", [])
    warnings = result.get("warnings", [])

    # Ratchet: filtrar violations e warnings que já estavam no baseline
    baseline = load_baseline()
    if baseline is not None:
        baseline_set = set(baseline.get("violations", []))
        net_new = [
            v for v in violations
            if _extract_messages([v])[0] not in baseline_set
            or (isinstance(v, dict) and v.get("rule") in _CREDENTIAL_GUARD_ANCHORED_RULES)
        ]
        violations = net_new
        baseline_warn_set = set(baseline.get("warnings", []))
        warnings = [
            w for w in warnings
            if _extract_messages([w])[0] not in baseline_warn_set
            or (isinstance(w, dict) and w.get("rule") in _CREDENTIAL_GUARD_ANCHORED_RULES)
        ]

    # Modo lenient: mover violations para warnings
    if _is_lenient(cwd):
        warnings = warnings + violations
        violations = []

    return {"violations": violations, "warnings": warnings}


# ---------------------------------------------------------------------------
# Aliases e exportações para compatibilidade com o CLI
# ---------------------------------------------------------------------------

validate_single_wip = validate_wip_limit
