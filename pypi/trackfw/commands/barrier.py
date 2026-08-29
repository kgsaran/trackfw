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
import unicodedata
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
        help="Rótulo da wave a avaliar. Gramática: <inteiro>[-<sufixo>], inteiro >= 0. Ex: 0, 1, 2, 2-bis, 2-hotfix.",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        default=False,
        help="Emite o documento de resultado em JSON em vez do relatório texto",
    )
    parser.add_argument(
        "--trust-local-gates",
        action="store_true",
        default=False,
        dest="trust_local_gates",
        help="Trust the local roadmap content for gate execution without comparing to origin/main (used by the /trackfw:barrier slash command for WIP roadmaps)",
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
    # Pinned literally by docs/cli-parity.md (`## trackfw barrier`): roadmap_arg is
    # the argument exactly as the user typed it, with no .md normalization.
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    raise BarrierUsageError(
        f'roadmap "{roadmap_arg}" not found in wip/ nor done/ under {roadmap_dir}'
    )


# ────────────────────────────────────────────────────────────────────────────
# Parsing string-level do roadmap (docs/cli-parity.md § Roadmap parsing rules)
# ────────────────────────────────────────────────────────────────────────────

# Pinned grammar: ^## Wave (\d+(?:-[a-z0-9]+)?) — docs/cli-parity.md § "Wave label grammar"
# Trailing space is part of rule 1 and is preserved.
_WAVE_HEADING_RE = re.compile(r"^## Wave (\d+(?:-[a-z0-9]+)?) ")
# Broad detector: any ## Wave <token><space> line — used to identify malformed headings that
# don't satisfy the pinned grammar (e.g. ## Wave X — Title, ## Wave 2-BIS — Title).
_ANY_WAVE_H2_RE = re.compile(r"^## Wave (\S+) ")
_H2_BOUNDARY_RE = re.compile(r"^## ")
_ML_HEADING_RE = re.compile(r"^### (ML-\S+)")
_H3_OR_H2_BOUNDARY_RE = re.compile(r"^(?:### |## )")
_STATUS_LINE_RE = re.compile(r"^\*\*Status:\*\*(.*)$")
# _ACCEPTANCE_HEADER_RE accepts both the canonical English header (ADR
# 2026-08-29 decision 1) and the Portuguese one, which remains accepted with
# no removal date (decision 2 — 99/143 roadmaps in the corpus use it,
# including artifacts in done/). The `^` anchor is load-bearing: an
# unanchored match would casar dentro de prosa ou de cerca de código, turning
# a quoted literal into forged acceptance evidence.
_ACCEPTANCE_HEADER_RE = re.compile(r"^\*\*(?:Acceptance criteria|Crit[ée]rios de aceite):\*\*")
_STAR_BOUNDARY_RE = re.compile(r"^\*\*")
_CRITERIA_ITEM_RE = re.compile(r"^- \[.\]")
_CRITERIA_UNMET_RE = re.compile(r"^- \[ \]")
_GATES_HEADER_RE = re.compile(r"^\*\*Gates da wave:\*\*")

# _STATUS_VOCABULARY is the closed set of first-token markers recognised as
# "complete" (ADR decision 3): the checkmark emoji, and the English/Portuguese
# words, matched after case-folding and diacritics-folding. Deliberately
# closed and explicit — "feito", "ok", "finalizado" are out (ADR, Alternatives
# Considered — "accept any non-empty status" is the rejected no-op design).
_STATUS_VOCABULARY = {"✅", "done", "concluido"}


_VS16 = "\ufe0f"


def _strip_vs16(token: str) -> str:
    """Removes only U+FE0F occurrences from token. Deliberately narrower than
    "strip every variation selector" or "strip every Mn": VS16 is the one
    documented cosmetic exception (ADR 2026-08-29 decision 9) — the seletor
    emoji keyboards insert after "✅" to force text-style presentation,
    producing "✅️", visually identical to "✅". Anything else of category Mn
    is now a rejection, not a fold (see _has_disallowed_combining_mark)."""
    return token.replace(_VS16, "")


def _has_disallowed_combining_mark(token: str) -> bool:
    """Reports whether token (with VS16 already removed) contains any
    Unicode category Mn (Mark, Nonspacing) codepoint. ADR 2026-08-29
    decision 9: after the single VS16 exception, any combining mark on the
    first status token is rejected outright — the ML is NOT complete —
    rather than folded away. A vocabulary this small exists to refuse
    ambiguity; silently folding combining marks reopens exactly the
    ambiguity it exists to close ("d<U+1DC0>one" must not read as "done").
    Uses unicodedata.category(ch) == "Mn" rather than
    unicodedata.combining(ch), to match Go's unicode.Mn and Node's \\p{Mn}
    exactly (category, not canonical combining class).

    ORDER IS LOAD-BEARING: this check MUST run on the token BEFORE NFD
    decomposition, not after. "Concluído" in its authored (NFC) form has no
    literal Mn codepoint — the accented "í" is a single precomposed
    codepoint. NFD-decomposing it *produces* a trailing Mn (U+0301,
    COMBINING ACUTE ACCENT) that _normalize_status_token then strips for
    vocabulary matching. Running this rejection check on the
    already-decomposed string would treat that legitimate,
    vocabulary-sanctioned accent the same as an injected combining mark and
    reject "Concluído" outright, breaking AC15's own positive case. Checking
    the raw, pre-decomposition token instead lets it through here (no
    literal Mn present) while still catching a combining mark authored
    directly onto the token, which is category Mn in either form, decomposed
    or not."""
    return any(unicodedata.category(ch) == "Mn" for ch in token)


def _normalize_status_token(token: str) -> str:
    """Folds diacritics (NFD + strip combining marks by
    unicodedata.category(ch) == "Mn"), then lower-cases the token for
    vocabulary comparison. Callers MUST have already rejected via
    _has_disallowed_combining_mark before calling this — see that
    function's docstring for why. Never mutates the emoji base codepoint
    itself."""
    decomposed = unicodedata.normalize("NFD", token)
    stripped = "".join(ch for ch in decomposed if unicodedata.category(ch) != "Mn")
    return stripped.lower()


def _status_is_complete(marker: str) -> bool:
    """Implements rule 3 by FIRST TOKEN, not substring (ADR decision 3,
    AC8/AC9/AC14). marker is the already-trimmed remainder of the
    "**Status:**" line. str.split() with no arguments splits on any
    whitespace per str.isspace(), which treats U+00A0 (NBSP) as a separator —
    matching the accepted "NBSP separator" case — while a zero-width
    character (U+200B, not str.isspace()) stays glued to the token and safely
    causes rejection (a usability false-negative, not a security concern).

    This is the fix for
    vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md:
    `"✅" in marker` would classify "**Status:** ⬜ Pendente ✅" as complete
    (reproduced live against 7.3.0 — ADR decision 8). First-token comparison
    rejects it because the first token is "⬜", not the marker.

    ADR decision 9: VS16 is stripped first (the one cosmetic exception),
    then any remaining combining mark on the raw token rejects outright —
    see _has_disallowed_combining_mark for why this must happen before NFD.
    """
    trimmed = marker.strip()
    if not trimmed:
        return False
    first = _strip_vs16(trimmed.split()[0])
    if _has_disallowed_combining_mark(first):
        return False
    return _normalize_status_token(first) in _STATUS_VOCABULARY


def _detect_fence_marker(trimmed: str):
    """Inspects a whitespace-trimmed line and reports whether it opens or
    closes a CommonMark-style fence: a run of 3+ identical backtick (`) or
    tilde (~) characters at the start of the line. Returns (char, length) or
    None. ADR decision (ML-1B, achado 1): CommonMark defines a fence as 3+ of
    the SAME character (backtick or tilde), closed by a run of the same
    character with length >= the opening run — masking only "```" left both
    "~~~" fences and 4+-backtick fences (whose interior can nest a
    3-backtick block) unmasked, which is the escape route a hostile roadmap
    would use."""
    if not trimmed:
        return None
    first = trimmed[0]
    if first != "`" and first != "~":
        return None
    i = 0
    while i < len(trimmed) and trimmed[i] == first:
        i += 1
    if i < 3:
        return None
    return (first, i)


def _fence_mask(lines: list) -> list:
    """Returns, for each line index, whether that line lies strictly inside a
    fenced code block (``` ... ``` or ~~~ ... ~~~, per CommonMark: 3+ of the
    same fence character, closed by a run of the same character with length
    >= the opening run's length). A line that is itself a fence delimiter
    is never reported as "inside" — only the lines between an opening and a
    closing delimiter are masked. ADR decision 7 / AC13: ML detection, status
    and acceptance-header parsing must ignore documentation/examples inside a
    cerca — otherwise a roadmap that cites those literals (as this very
    roadmap, its REQ and its ADR do, repeatedly) is read as real ML content.
    _find_gates already has its own, independent fence-matching for the
    "```bash ... ```" gates block and is untouched by this mask."""
    mask = [False] * len(lines)
    fenced = False
    fence_char = None
    fence_len = 0
    for i, line in enumerate(lines):
        trimmed = line.strip()
        marker = _detect_fence_marker(trimmed)
        if not fenced:
            if marker:
                fenced = True
                fence_char, fence_len = marker
            continue
        # Currently inside a fence: only a marker of the SAME character with
        # length >= the opening run closes it (a nested shorter/different
        # marker stays masked as interior content) — AND, per CommonMark, a
        # closing fence line contains NOTHING besides the fence run and
        # (already-stripped) surrounding whitespace: marker[1] == len(trimmed).
        # Fix for hades-tf security review (2026-08-29, achado #1 /
        # vault/notes/barrier-fence-closing-trailing-content-bypass-2026-08-29.md):
        # a line like "```trailing-junk" found INSIDE an open fence does not
        # close it in real CommonMark — it stays interior content — but the
        # prior check treated any run of length >= fence_len as a valid close
        # regardless of trailing text. The opening branch above is unchanged:
        # CommonMark allows an info string after the opening run (` ```bash `).
        if marker and marker[0] == fence_char and marker[1] >= fence_len and marker[1] == len(trimmed):
            fenced = False
            continue
        mask[i] = True
    return mask


def _is_valid_wave_label(token: str) -> bool:
    """Valida o token de rótulo de wave contra a gramática pinada.

    Gramática: <inteiro>[-<sufixo>], inteiro >= 0, sufixo [a-z0-9]+.
    Válidos: "0", "1", "2", "2-bis", "2-hotfix", "10-a2". "0" é a convenção da
    Wave 0 de modelo de ameaça (docs/cli-parity.md § "Wave label grammar").
    Inválidos: "X", "2-BIS", "2-", "2-bis-ter", "-bis".

    Usa re.fullmatch para garantir correspondência completa do token —
    re.match sem $ aceitaria "2-bis-ter" ao corresponder apenas "2-bis".
    A exigência >= 0 rejeita negativos (embora a regex já não os aceite,
    por não ter sinal) e aceita "0" (Node.js: parseInt >= 0).
    Usado pelo pré-passo de heading E pela validação de --wave, para que
    as duas superfícies compartilhem exatamente a mesma regra.
    """
    m = re.fullmatch(r"(\d+)(?:-[a-z0-9]+)?", token)
    return bool(m) and int(m.group(1)) >= 0


def _find_wave(lines: list, wave_label: str, roadmap_basename: str) -> tuple:
    """Retorna (start, end) — índices (inclusivo/exclusivo) do corpo da wave
    solicitada. Lança BarrierUsageError se a wave não existir ou se qualquer
    cabeçalho de wave no documento for malformado. roadmap_basename é o basename
    resolvido (incluindo .md), usado apenas na mensagem de erro — pinned
    literalmente por docs/cli-parity.md.

    Gramática do rótulo: ^## Wave (\\d+(?:-[a-z0-9]+)?) — pinada em docs/cli-parity.md
    § "Wave label grammar". Um cabeçalho fora dessa gramática aborta o documento inteiro
    — decisão 16 do ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md.
    Labels são identidades distintas: '2' não casa com '## Wave 2-bis '.

    Detection is a full pre-pass (pinned by docs/cli-parity.md §
    "Detection is a full pre-pass — pinned"): TODAS as headings de wave são
    validadas antes de resolver o rótulo pedido, sem break antecipado. Uma heading
    malformada em qualquer posição do documento — inclusive depois da wave alvo —
    aborta imediatamente com exit 2. Isso espelha o comportamento do Go
    (parseWaves) e do Node.js (findWave two-phase), e impede que um documento
    malformado seja lido como "wave reprovada" (ADR decisão 12).
    """
    n = len(lines)

    # Fase 1 — pré-passo completo: percorre TODAS as linhas e valida cada
    # heading de wave. Aborta na primeira heading malformada, independentemente
    # de sua posição em relação à wave pedida.
    waves: list = []
    for i, line in enumerate(lines):
        m_loose = _ANY_WAVE_H2_RE.match(line)
        if not m_loose:
            continue
        token = m_loose.group(1)
        if not _is_valid_wave_label(token):
            raise BarrierUsageError(
                f'malformed wave heading at line {i + 1}: "{token}" is not a valid wave label'
            )
        # Heading válida — calcula o span.
        j = i + 1
        while j < n and not _H2_BOUNDARY_RE.match(lines[j]):
            j += 1
        waves.append((token, i, j))

    # Fase 2 — encontra a wave alvo por correspondência exata de rótulo.
    for token, start, end in waves:
        if token == wave_label:
            return (start, end)

    raise BarrierUsageError(
        f'wave {wave_label} not found in roadmap "{roadmap_basename}"'
    )


def _find_mls(lines: list, fenced: list, start: int, end: int) -> list:
    """fenced marks, per line index into lines, whether that line lies inside
    a fenced code block (_fence_mask) — a "### ML-XX" heading inside a cerca
    is documentation, not a real ML, and must not become a phantom ML (ADR
    decision 7, AC13-b)."""
    mls = []
    i = start
    while i < end:
        if fenced[i]:
            i += 1
            continue
        line = lines[i]
        if _ML_HEADING_RE.match(line):
            m = _ML_HEADING_RE.match(line)
            ml_id = m.group(1)
            j = i + 1
            while j < end and (fenced[j] or not _H3_OR_H2_BOUNDARY_RE.match(lines[j])):
                j += 1
            mls.append({"id": ml_id, "start": i, "end": j})
            i = j
        else:
            i += 1
    return mls


def _ml_status(lines: list, fenced: list, start: int, end: int):
    """Retorna (complete: bool, marker: str|None). marker é None quando a linha
    **Status:** está ausente (rule 3). Conclusão é por PRIMEIRO TOKEN, não
    substring (ADR decision 3, AC8/AC9/AC14). Linhas dentro de cerca de código
    são ignoradas — um "**Status:**" citado dentro de uma cerca (ex.: como
    documentação deste próprio defeito) não é o status real do ML (ADR
    decision 7, AC13-a)."""
    for idx in range(start, end):
        if fenced[idx]:
            continue
        m = _STATUS_LINE_RE.match(lines[idx])
        if m:
            remainder = m.group(1).strip()
            return _status_is_complete(remainder), remainder
    return False, None


def _ml_acceptance(lines: list, fenced: list, start: int, end: int):
    """Retorna dict {"total": n, "unmet": n} ou None quando não há bloco de
    critérios de aceite (rule 4). Linhas dentro de cerca de código são
    ignoradas em toda a função — na busca do cabeçalho, no limite "**" de fim
    de bloco, e na contagem de critérios — do contrário uma cerca citando o
    cabeçalho de aceite/"- [x]" como exemplo forjaria evidência de aceite (ADR
    decision 7, AC13-a)."""
    block_start = None
    block_end = end
    for idx in range(start, end):
        if fenced[idx]:
            continue
        if _ACCEPTANCE_HEADER_RE.match(lines[idx]):
            block_start = idx + 1
            j = idx + 1
            while j < end and (fenced[j] or not _STAR_BOUNDARY_RE.match(lines[j])):
                j += 1
            block_end = j
            break
    if block_start is None:
        return None
    items = [
        lines[idx]
        for idx in range(block_start, block_end)
        if not fenced[idx] and _CRITERIA_ITEM_RE.match(lines[idx])
    ]
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

def _check_mls_complete(mls: list, wave_label: str) -> dict:
    evidence = []
    failures = []
    for ml in mls:
        complete, marker = _ml_status(_LINES_CACHE, _FENCE_MASK_CACHE, ml["start"], ml["end"])
        if complete:
            evidence.append(f"{ml['id']}: ✅")
        else:
            failures.append(f"{ml['id']}: not complete (status: {marker if marker else 'missing'})")
    status = "passed" if (mls and not failures) else "blocked"
    if not mls:
        # Pinned literally by docs/cli-parity.md ("Wave contains zero MLs" case).
        failures.append(f"wave {wave_label}: no ML found")
    return {"name": "mls_complete", "status": status, "evidence": evidence, "failures": failures}


def _check_acceptance_evidence(mls: list) -> dict:
    evidence = []
    failures = []
    for ml in mls:
        block = _ml_acceptance(_LINES_CACHE, _FENCE_MASK_CACHE, ml["start"], ml["end"])
        if block is None or block["total"] == 0:
            failures.append(f"{ml['id']}: no acceptance block")
        elif block["unmet"] == 0:
            evidence.append(f"{ml['id']}: {block['total']} criteria met")
        else:
            failures.append(f"{ml['id']}: {block['unmet']} unmet acceptance criteria")
    status = "passed" if not failures else "blocked"
    return {"name": "acceptance_evidence", "status": status, "evidence": evidence, "failures": failures}


# ────────────────────────────────────────────────────────────────────────────
# Roadmap trust check (AC11, AC12 — docs/cli-parity.md § Trust and --trust-local-gates)
# ────────────────────────────────────────────────────────────────────────────

def _roadmap_trust_for_gates(roadmap_path: str) -> dict:
    """Determines whether the gates declared in a roadmap can be trusted for execution.

    Decision (AC4, AC11): the discriminant is git — a roadmap whose content
    differs from origin/main, or that is absent from origin/main, is untrusted.

    Returns {"trusted": True} or {"trusted": False, "failure_msg": str}.
    Fail-open when not in a git repo, origin/main not resolvable, or any git
    error other than "path absent from origin/main". See docs/cli-parity.md.
    """
    roadmap_dir = os.path.dirname(os.path.abspath(roadmap_path))

    # Step 1: check if we are inside a git repository.
    r = subprocess.run(
        ["git", "rev-parse", "--git-dir"],
        cwd=roadmap_dir,
        capture_output=True,
    )
    if r.returncode != 0:
        # Not a git repo → fail-open.
        return {"trusted": True}

    # Step 2: get the repository toplevel.
    r = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        cwd=roadmap_dir,
        capture_output=True,
        text=True,
    )
    if r.returncode != 0:
        return {"trusted": True}
    repo_root = r.stdout.strip()

    # Step 3: compute repo-relative path (git uses forward slashes).
    abs_roadmap = os.path.abspath(roadmap_path)
    rel_path = os.path.relpath(abs_roadmap, repo_root).replace(os.sep, "/")

    # Step 4: retrieve the file at origin/main.
    r = subprocess.run(
        ["git", "show", f"origin/main:{rel_path}"],
        cwd=repo_root,
        capture_output=True,
    )
    if r.returncode != 0:
        # If the path specifically does not exist in origin/main → untrusted.
        stderr = r.stderr.decode("utf-8", errors="replace")
        if "does not exist in" in stderr or "exists on disk, but not in" in stderr:
            return {
                "trusted": False,
                "failure_msg": "gates not evaluated: roadmap is not committed in origin/main — pass --trust-local-gates to evaluate local gates",
            }
        # Other failures (no remote, not fetched) → fail-open.
        return {"trusted": True}

    # Step 5: compare content byte-for-byte.
    main_content = r.stdout
    try:
        with open(roadmap_path, "rb") as f:
            local_content = f.read()
    except OSError:
        return {"trusted": True}

    if main_content != local_content:
        return {
            "trusted": False,
            "failure_msg": "gates not evaluated: roadmap content differs from origin/main — pass --trust-local-gates to evaluate local gates",
        }
    return {"trusted": True}


def _check_gates(commands, trust_result: dict | None = None) -> dict:
    """Evaluate gate commands, subject to trust check.

    trust_result: {"trusted": True} or {"trusted": False, "failure_msg": str}.
    When None, defaults to trusted (backward compatibility for unit tests).
    """
    if trust_result is None:
        trust_result = {"trusted": True}

    cmd_list = list(commands) if commands is not None else []

    if not trust_result.get("trusted", True):
        # Roadmap is not trusted: do not execute gates (AC3, AC14).
        # Report as not_evaluated — distinct from passed and blocked (AC6).
        return {
            "name": "gates",
            "status": "not_evaluated",
            "commands": cmd_list,
            "evidence": [],
            "failures": [trust_result["failure_msg"]],
        }

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
    return {"name": "gates", "status": status, "commands": cmd_list, "evidence": evidence, "failures": failures}


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
_FENCE_MASK_CACHE: list = []


def _split_roadmap_lines(content: str) -> list:
    """Single boundary where the raw file content becomes the list every
    marker regex operates on. Strips a trailing "\\r" from each line
    produced by splitting on "\\n" — every downstream marker (ML heading,
    "**Status:**", acceptance header, criterion lines, "**Gates da wave:**",
    the fence delimiter) then sees the same content it would see for an
    LF-only file.

    This runtime does not currently depend on this normalization to pass a
    CRLF roadmap end-to-end — ``open(path, "r", encoding="utf-8")`` already
    runs Python's universal-newlines translation (default ``newline=None``),
    so ``\\r\\n``/``\\r`` are converted to ``\\n`` before ``content`` ever
    reaches this function. This is defensive, not load-bearing today: it
    keeps the three runtimes symmetric (mirrors
    internal/commands/barrier.go splitRoadmapLines and
    npm/src/commands/barrier.js splitRoadmapLines) and keeps this codepath
    correct even if a future change reads the file in binary mode or with
    ``newline=''`` and loses the automatic translation.

    Only the trailing "\\r" immediately before the split point is stripped —
    this must never be confused with per-line indentation trimming, which
    ML-1B deliberately removed: leading whitespace on a marker line still
    fails to match (markers are anchored at column 0, untouched by this
    function).
    """
    return [line[:-1] if line.endswith("\r") else line for line in content.split("\n")]


def _parse_wave_label(raw: str) -> str:
    """Valida e retorna o rótulo de wave passado via --wave.

    Gramática: <inteiro>[-<sufixo>], sufixo [a-z0-9]+, inteiro >= 0.
    Exemplos válidos: "0", "1", "2", "2-bis", "2-hotfix", "10-a2".
    Exemplos inválidos: "abc", "2-BIS", "2-", "2-bis-ter".

    Mensagem de erro pinada literalmente por docs/cli-parity.md (quarta mensagem
    de exit-2): 'invalid --wave "<value>" — not a valid wave label'. O separador
    é travessão U+2014 (—), não hífen. <value> é o argumento exatamente como
    o usuário digitou. Usa _is_valid_wave_label para compartilhar a mesma regra
    da validação de heading (pré-passo).
    """
    if not _is_valid_wave_label(raw):
        raise BarrierUsageError(
            f'invalid --wave "{raw}" — not a valid wave label'
        )
    return raw


def _build_result_document(roadmap_arg: str, roadmap_path: str, wave_label: str, trust_local_gates: bool = False) -> dict:
    global _LINES_CACHE, _FENCE_MASK_CACHE
    content = open(roadmap_path, "r", encoding="utf-8").read()
    _LINES_CACHE = _split_roadmap_lines(content)
    _FENCE_MASK_CACHE = _fence_mask(_LINES_CACHE)

    roadmap_basename = os.path.basename(roadmap_path)
    started_at = _now_rfc3339()
    wave_start, wave_end = _find_wave(_LINES_CACHE, wave_label, roadmap_basename)
    mls = _find_mls(_LINES_CACHE, _FENCE_MASK_CACHE, wave_start, wave_end)
    gate_commands = _find_gates(_LINES_CACHE, wave_start, wave_end)

    # Determine trust for gate execution (AC11, AC12).
    trust_result = {"trusted": True} if trust_local_gates else _roadmap_trust_for_gates(roadmap_path)

    checks = [
        _check_mls_complete(mls, wave_label),
        _check_acceptance_evidence(mls),
        _check_gates(gate_commands, trust_result),
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
        "wave": wave_label,  # string, não int — mudança observável (ver relatório do ML-2C)
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
        wave_label = _parse_wave_label(args.wave)
        roadmap_path = _resolve_roadmap_path(cfg, args.roadmap)
        trust_local_gates = getattr(args, "trust_local_gates", False)
        doc = _build_result_document(args.roadmap, roadmap_path, wave_label, trust_local_gates=trust_local_gates)
    except BarrierUsageError as exc:
        # No argparse "error:" prefix — pinned literally by docs/cli-parity.md
        # (`## trackfw barrier`), so the message is byte-identical across runtimes.
        sys.stderr.write(f"trackfw barrier: {exc}\n")
        sys.exit(2)
        return

    if getattr(args, "json", False):
        print(json.dumps(doc, ensure_ascii=False))
    else:
        _print_text_report(doc)

    sys.exit(0 if doc["status"] == "passed" else 1)
