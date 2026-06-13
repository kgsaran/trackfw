"""
commands/context.py — Subcomando `trackfw context`.
Exporta contexto do projeto (ADRs, REQs, roadmaps WIP) para consumo por agentes de IA.
Espelho Python de npm/src/commands/context.js.
"""

import json
import os
import sys


def _extract_frontmatter_field(content, field):
    """
    Extrai valor de campo YAML dentro de bloco --- ... ---.
    Retorna string vazia se não encontrado.
    Espelha extractFrontmatterField do JS.
    """
    lines = content.split("\n")
    started = False
    in_frontmatter = False
    for line in lines:
        trimmed = line.strip()
        if trimmed == "---":
            if not started:
                started = True
                in_frontmatter = True
                continue
            break  # segundo --- fecha o bloco
        if not in_frontmatter:
            break
        key = field + ":"
        if trimmed.startswith(key):
            val = trimmed[len(key):].strip()
            val = val.strip("\"'")
            return val
    return ""


def _extract_inline_status(content):
    """
    Extrai status da linha '| Status: ...' do markdown.
    Espelha extractInlineStatus do JS.
    """
    for line in content.split("\n"):
        idx = line.find("| Status: ")
        if idx >= 0:
            rest = line[idx + len("| Status: "):]
            pipe_idx = rest.find(" |")
            if pipe_idx >= 0:
                rest = rest[:pipe_idx]
            rest = rest.rstrip(" >|").strip()
            return rest or "unknown"
    return "unknown"


def _collect_entries(dir_path, entry_type, state=None):
    """
    Lê diretório e retorna lista de entradas com type, file, status, state.
    Espelha collectEntries do JS.
    """
    entries = []
    try:
        files = [
            f for f in os.listdir(dir_path)
            if f.endswith(".md") and not os.path.isdir(os.path.join(dir_path, f))
        ]
    except OSError:
        return entries

    for filename in files:
        content = ""
        try:
            with open(os.path.join(dir_path, filename), "r", encoding="utf-8") as f:
                content = f.read()
        except OSError:
            pass

        status = _extract_frontmatter_field(content, "status")
        if not status:
            status = _extract_inline_status(content)
        if not status:
            status = state or "unknown"

        entry = {"type": entry_type, "file": filename, "status": status}
        if state is not None:
            entry["state"] = state
        entries.append(entry)

    return entries


def _get_context(fmt, output_file=None):
    """
    Coleta governança e imprime em markdown ou json.
    Espelha getContext do JS.
    """
    from trackfw import config as _config
    from trackfw.validator import validate

    cfg = _config.load()

    # ADRs
    adrs = []
    for adr_dir in cfg.get("adr_dirs", ["docs/adr"]):
        adrs.extend(_collect_entries(adr_dir, "ADR"))

    # REQs
    reqs = _collect_entries(cfg.get("req_dir", "docs/req"), "REQ")

    # Roadmaps
    roadmaps = []
    states = ["wip", "backlog", "blocked", "done", "abandoned"]
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")

    if cfg.get("roadmap_namespacing") == "by_agent":
        agents = cfg.get("agents") or []
        if not agents:
            try:
                agents = [
                    f for f in os.listdir(roadmap_dir)
                    if os.path.isdir(os.path.join(roadmap_dir, f))
                ]
            except OSError:
                agents = []
        for agent in agents:
            for state in states:
                d = os.path.join(roadmap_dir, agent, state)
                roadmaps.extend(_collect_entries(d, "ROADMAP", state))
    else:
        for state in states:
            d = os.path.join(roadmap_dir, state)
            roadmaps.extend(_collect_entries(d, "ROADMAP", state))

    # Validate
    result = validate()
    violations = [v["message"] for v in result.get("violations", [])]
    warnings = [w["message"] for w in result.get("warnings", [])]

    # Score
    score = 0
    if adrs:
        score += 20
    if reqs:
        score += 20
    if roadmaps:
        score += 20
    if not violations:
        score += 40

    # Saída
    out = sys.stdout
    if output_file:
        try:
            out = open(output_file, "w", encoding="utf-8")
        except OSError as e:
            print(f"Error opening output file: {e}", file=sys.stderr)
            sys.exit(1)

    try:
        if fmt == "json":
            data = {
                "score": score,
                "violations": violations,
                "warnings": warnings,
                "adrs": adrs,
                "reqs": reqs,
                "roadmaps": roadmaps,
            }
            print(json.dumps(data, indent=2), file=out)
            return

        # Markdown
        print("# trackfw governance context\n", file=out)
        print(f"**Governance score:** {score}/100\n", file=out)

        print(f"## ADRs ({len(adrs)})", file=out)
        if not adrs:
            print("- (none)", file=out)
        else:
            for a in adrs:
                print(f"- {a['file']} [{a['status']}]", file=out)

        print(f"\n## REQs ({len(reqs)})", file=out)
        if not reqs:
            print("- (none)", file=out)
        else:
            for r in reqs:
                print(f"- {r['file']} [{r['status']}]", file=out)

        print(f"\n## Roadmaps ({len(roadmaps)})", file=out)
        if not roadmaps:
            print("- (none)", file=out)
        else:
            for r in roadmaps:
                print(f"- {r['file']} [{r.get('state', r['status'])}]", file=out)

        if violations:
            print(f"\n## Violations ({len(violations)})", file=out)
            for v in violations:
                print(f"- {v}", file=out)

        if warnings:
            print(f"\n## Warnings ({len(warnings)})", file=out)
            for w in warnings:
                print(f"- {w}", file=out)
    finally:
        if output_file and out is not sys.stdout:
            out.close()


def register(subparsers):
    """Adiciona subcomando `context` ao parser principal."""
    parser = subparsers.add_parser(
        "context",
        help="Print governance context for LLM consumption",
    )
    parser.add_argument(
        "--format",
        choices=["markdown", "json", "md"],
        default="markdown",
        metavar="FORMAT",
        help="Output format: markdown or json (default: markdown)",
    )
    parser.add_argument(
        "--output",
        metavar="FILE",
        default=None,
        help="Write output to FILE instead of stdout",
    )
    parser.set_defaults(func=_cmd_context)


def _cmd_context(args):
    fmt = getattr(args, "format", "markdown")
    # normaliza "markdown" → "md" para compat com lógica interna
    if fmt == "markdown":
        fmt = "md"
    output_file = getattr(args, "output", None)
    _get_context(fmt, output_file)
