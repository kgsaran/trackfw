"""
barrier.py — Comando `trackfw barrier <roadmap> --wave <n>`.

Núcleo determinístico da wave-release barrier. Stack-agnostic: nunca assume build
tool, test runner ou regra de paridade — todo check executável vem do próprio
roadmap. Contrato completo em docs/cli-parity.md, seção "## `trackfw barrier`".

Este módulo NUNCA invoca agentes e NUNCA executa operações Git — isso é
responsabilidade exclusiva do slash-command `/trackfw:barrier` e do
`trackfw_architect`.
"""

from __future__ import annotations

import json
import os
import re
import subprocess
import sys
from datetime import datetime, timezone

from .. import config as _config
from .. import validator as _validator


class BarrierUsageError(Exception):
    """Erro de uso (exit 2) — roadmap/wave não resolvido ou entrada malformada."""


# ────────────────────────────────────────────────────────────────────────────
# Registro do subcomando
# ────────────────────────────────────────────────────────────────────────────

def register(subparsers):
    parser = subparsers.add_parser(
        "barrier",
        help="Avalia deterministicamente se uma wave do roadmap pode ser liberada",
        description=(
            "trackfw barrier <roadmap> --wave <n> avalia, de forma determinística e "
            "stack-agnostic, se uma wave do roadmap está pronta para liberação: "
            "MLs completos, evidência de aceite, gates declarados na wave e "
            "governança (`trackfw validate`)."
        ),
    )
    parser.add_argument(
        "roadmap",
        help="Basename do roadmap, com ou sem .md, resolvido em wip/ e depois done/",
    )
    parser.add_argument(
        "--wave",
        dest="wave",
        required=True,
        help="Número da wave a avaliar (inteiro >= 1)",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        default=False,
        help="Emite o documento de resultado em JSON em vez do relatório texto",
    )
    parser.set_defaults(func=run)
    return parser


# ────────────────────────────────────────────────────────────────────────────
# Resolução de roadmap
# ────────────────────────────────────────────────────────────────────────────

def _resolve_roadmap_path(cfg: dict, roadmap_arg: str) -> str:
    basename = roadmap_arg if roadmap_arg.endswith(".md") else roadmap_arg + ".md"
    dirs = _validator.resolve_wip_dirs(cfg) + _validator.resolve_done_dirs(cfg)
    for d in dirs:
        candidate = os.path.join(d, basename)
        if os.path.isfile(candidate):
            return candidate
    raise BarrierUsageError(
        f"roadmap not found: {roadmap_arg!r} (searched wip/ and done/ under "
        f"{cfg.get('roadmap_dir', 'docs/roadmaps')!r})"
    )


# ────────────────────────────────────────────────────────────────────────────
# Parsing string-level do roadmap (docs/cli-parity.md § Roadmap parsing rules)
# ────────────────────────────────────────────────────────────────────────────

_WAVE_HEADING_RE = re.compile(r"^## Wave (\S+) ")
_H2_BOUNDARY_RE = re.compile(r"^## ")
_ML_HEADING_RE = re.compile(r"^### (ML-\S+)")
_H3_OR_H2_BOUNDARY_RE = re.compile(r"^(?:### |## )")
_STATUS_LINE_RE = re.compile(r"^\*\*Status:\*\*(.*)$")
_ACCEPTANCE_HEADER_RE = re.compile(r"^\*\*Crit[ée]rios de aceite:\*\*")
_STAR_BOUNDARY_RE = re.compile(r"^\*\*")
_CRITERIA_ITEM_RE = re.compile(r"^- \[.\]")
_CRITERIA_UNMET_RE = re.compile(r"^- \[ \]")
_GATES_HEADER_RE = re.compile(r"^\*\*Gates da wave:\*\*")


def _find_wave(lines: list, wave_number: int, roadmap_arg: str) -> tuple:
    """Retorna (start, end) — índices (inclusivo/exclusivo) do corpo da wave
    solicitada. Lança BarrierUsageError se a wave não existir ou se um cabeçalho
    de wave malformado for encontrado antes dela."""
    n = len(lines)
    i = 0
    found = None
    while i < n:
        line = lines[i]
        m = _WAVE_HEADING_RE.match(line)
        if m:
            token = m.group(1)
            if not re.match(r"^\d+$", token):
                raise BarrierUsageError(
                    f"malformed wave heading at line {i + 1}: number {token!r} is not parseable"
                )
            j = i + 1
            while j < n and not _H2_BOUNDARY_RE.match(lines[j]):
                j += 1
            if int(token) == wave_number:
                found = (i, j)
                break
            i = j
        else:
            i += 1
    if found is None:
        raise BarrierUsageError(
            f"wave {wave_number} not found in roadmap {roadmap_arg!r}"
        )
    return found


def _find_mls(lines: list, start: int, end: int) -> list:
    mls = []
    i = start
    while i < end:
        line = lines[i]
        if _ML_HEADING_RE.match(line):
            m = _ML_HEADING_RE.match(line)
            ml_id = m.group(1)
            j = i + 1
            while j < end and not _H3_OR_H2_BOUNDARY_RE.match(lines[j]):
                j += 1
            mls.append({"id": ml_id, "start": i, "end": j})
            i = j
        else:
            i += 1
    return mls


def _ml_status(lines: list, start: int, end: int):
    """Retorna (complete: bool, marker: str|None). marker é None quando a linha
    **Status:** está ausente (rule 3)."""
    for idx in range(start, end):
        m = _STATUS_LINE_RE.match(lines[idx])
        if m:
            remainder = m.group(1).strip()
            return ("✅" in remainder), remainder
    return False, None


def _ml_acceptance(lines: list, start: int, end: int):
    """Retorna dict {"total": n, "unmet": n} ou None quando não há bloco de
    critérios de aceite (rule 4)."""
    block_start = None
    block_end = end
    for idx in range(start, end):
        if _ACCEPTANCE_HEADER_RE.match(lines[idx]):
            block_start = idx + 1
            j = idx + 1
            while j < end and not _STAR_BOUNDARY_RE.match(lines[j]):
                j += 1
            block_end = j
            break
    if block_start is None:
        return None
    items = [lines[idx] for idx in range(block_start, block_end) if _CRITERIA_ITEM_RE.match(lines[idx])]
    unmet = [it for it in items if _CRITERIA_UNMET_RE.match(it)]
    return {"total": len(items), "unmet": len(unmet)}


def _find_gates(lines: list, start: int, end: int):
    """Retorna lista de comandos (pode ser vazia) ou None quando não há bloco de
    gates declarado (rule 5 — ausência de bloco é legal e produz zero gates)."""
    for idx in range(start, end):
        if _GATES_HEADER_RE.match(lines[idx]):
            fence_idx = idx + 1
            if fence_idx >= end or not lines[fence_idx].strip().startswith("```"):
                raise BarrierUsageError(
                    f"malformed gates block at line {idx + 1}: "
                    "'**Gates da wave:**' must be immediately followed by a fenced code block"
                )
            j = fence_idx + 1
            while j < end and lines[j].strip() != "```":
                j += 1
            if j >= end:
                raise BarrierUsageError(
                    f"unterminated fenced code block starting at line {fence_idx + 1}"
                )
            commands = []
            for k in range(fence_idx + 1, j):
                stripped = lines[k].strip()
                if not stripped or stripped.startswith("#"):
                    continue
                commands.append(stripped)
            return commands
    return None


# ────────────────────────────────────────────────────────────────────────────
# Avaliação dos checks embutidos
# ────────────────────────────────────────────────────────────────────────────

def _check_mls_complete(mls: list) -> dict:
    evidence = []
    failures = []
    for ml in mls:
        complete, marker = _ml_status(_LINES_CACHE, ml["start"], ml["end"])
        if complete:
            evidence.append(f"{ml['id']}: ✅")
        else:
            failures.append(f"{ml['id']}: not complete (status: {marker if marker else 'missing'})")
    status = "passed" if (mls and not failures) else "blocked"
    if not mls:
        failures.append("wave contains no ML headings")
    return {"name": "mls_complete", "status": status, "evidence": evidence, "failures": failures}


def _check_acceptance_evidence(mls: list) -> dict:
    evidence = []
    failures = []
    for ml in mls:
        block = _ml_acceptance(_LINES_CACHE, ml["start"], ml["end"])
        if block is None or block["total"] == 0:
            failures.append(f"{ml['id']}: no acceptance block")
        elif block["unmet"] == 0:
            evidence.append(f"{ml['id']}: {block['total']} criteria met")
        else:
            failures.append(f"{ml['id']}: {block['unmet']} unmet acceptance criteria")
    status = "passed" if not failures else "blocked"
    return {"name": "acceptance_evidence", "status": status, "evidence": evidence, "failures": failures}


def _check_gates(commands) -> dict:
    if commands is None:
        return {"name": "gates", "status": "passed", "commands": [], "evidence": [], "failures": []}

    evidence = []
    failures = []
    for cmd in commands:
        result = subprocess.run(
            cmd,
            shell=True,
            cwd=os.getcwd(),
            capture_output=True,
            text=True,
        )
        if result.returncode == 0:
            evidence.append(f"{cmd}: exit 0")
        else:
            failures.append(f"{cmd}: exit {result.returncode}")
            if result.stdout:
                sys.stderr.write(result.stdout)
            if result.stderr:
                sys.stderr.write(result.stderr)
    status = "passed" if not failures else "blocked"
    return {"name": "gates", "status": status, "commands": list(commands), "evidence": evidence, "failures": failures}


def _check_validate() -> dict:
    result = _validator.validate()
    v = len(result.get("violations", []))
    w = len(result.get("warnings", []))
    msg = f"{v} violations, {w} warnings"
    if v == 0:
        return {"name": "validate", "status": "passed", "evidence": [msg], "failures": []}
    return {"name": "validate", "status": "blocked", "evidence": [], "failures": [msg]}


# ────────────────────────────────────────────────────────────────────────────
# Execução
# ────────────────────────────────────────────────────────────────────────────

_LINES_CACHE: list = []


def _parse_wave_int(raw: str, roadmap_arg: str) -> int:
    try:
        value = int(raw)
    except (TypeError, ValueError):
        raise BarrierUsageError(f"malformed --wave value: {raw!r} is not an integer")
    if value < 1:
        raise BarrierUsageError(f"malformed --wave value: {raw!r} must be an integer >= 1")
    return value


def _build_result_document(roadmap_arg: str, roadmap_path: str, wave_number: int) -> dict:
    global _LINES_CACHE
    content = open(roadmap_path, "r", encoding="utf-8").read()
    _LINES_CACHE = content.split("\n")

    started_at = _now_rfc3339()
    wave_start, wave_end = _find_wave(_LINES_CACHE, wave_number, roadmap_arg)
    mls = _find_mls(_LINES_CACHE, wave_start, wave_end)
    gate_commands = _find_gates(_LINES_CACHE, wave_start, wave_end)

    checks = [
        _check_mls_complete(mls),
        _check_acceptance_evidence(mls),
        _check_gates(gate_commands),
        _check_validate(),
    ]
    finished_at = _now_rfc3339()

    status = "passed" if all(c["status"] == "passed" for c in checks) else "blocked"
    top_failures = []
    for c in checks:
        for f in c["failures"]:
            top_failures.append(f"{c['name']}: {f}")

    doc = {
        "roadmap": os.path.basename(roadmap_path),
        "wave": wave_number,
        "status": status,
        "started_at": started_at,
        "finished_at": finished_at,
        "checks": checks,
        "failures": top_failures,
    }
    return doc


def _now_rfc3339() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def _print_text_report(doc: dict) -> None:
    print(f"Barrier — {doc['roadmap']} — Wave {doc['wave']}")
    print(f"Status: {doc['status']}")
    for c in doc["checks"]:
        marker = "✓" if c["status"] == "passed" else "✗"
        print(f"  {marker} {c['name']}: {c['status']}")
        for e in c["evidence"]:
            print(f"      {e}")
        for f in c["failures"]:
            print(f"      FAIL: {f}")
    if doc["failures"]:
        print("\nFailures:")
        for f in doc["failures"]:
            print(f"  - {f}")


def run(args):
    _config.reset()
    cfg = _config.load()

    try:
        wave_number = _parse_wave_int(args.wave, args.roadmap)
        roadmap_path = _resolve_roadmap_path(cfg, args.roadmap)
        doc = _build_result_document(args.roadmap, roadmap_path, wave_number)
    except BarrierUsageError as exc:
        sys.stderr.write(f"trackfw barrier: error: {exc}\n")
        sys.exit(2)
        return

    if getattr(args, "json", False):
        print(json.dumps(doc, ensure_ascii=False))
    else:
        _print_text_report(doc)

    sys.exit(0 if doc["status"] == "passed" else 1)
