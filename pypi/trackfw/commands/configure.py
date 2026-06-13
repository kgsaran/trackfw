"""
configure.py — Comando `trackfw configure`.
Wizard interativo (stdlib input()) para gerar trackfw.yaml.
"""

import os


_DEFAULTS = {
    "adr_dirs": "docs/adr",
    "req_dir": "docs/req",
    "roadmap_dir": "docs/roadmaps",
    "wip_limit": "1",
    "req_marker": "REQ:",
}


def _prompt(label: str, default: str) -> str:
    """Exibe prompt com default entre colchetes e retorna valor informado ou default."""
    value = input(f"{label} [{default}]: ").strip()
    return value if value else default


def run(args):
    """Executa o wizard interativo de configuração."""
    cwd = os.getcwd()
    yaml_path = os.path.join(cwd, "trackfw.yaml")

    if os.path.exists(yaml_path):
        answer = input("trackfw.yaml já existe. Recriar do zero? [s/N]: ").strip().lower()
        if answer not in ("s", "sim", "y", "yes"):
            print("Operação cancelada.")
            return

    adr_dirs = _prompt("ADR dirs", _DEFAULTS["adr_dirs"])
    req_dir = _prompt("REQ dir", _DEFAULTS["req_dir"])
    roadmap_dir = _prompt("Roadmap dir", _DEFAULTS["roadmap_dir"])
    wip_limit = _prompt("WIP limit", _DEFAULTS["wip_limit"])
    req_marker = _prompt("Marcador de REQ", _DEFAULTS["req_marker"])

    # Identificar campos customizados (diferentes do default)
    custom = {}
    if adr_dirs != _DEFAULTS["adr_dirs"]:
        custom["adr_dirs"] = adr_dirs
    if req_dir != _DEFAULTS["req_dir"]:
        custom["req_dir"] = req_dir
    if roadmap_dir != _DEFAULTS["roadmap_dir"]:
        custom["roadmap_dir"] = roadmap_dir
    if wip_limit != _DEFAULTS["wip_limit"]:
        custom["wip_limit"] = wip_limit
    if req_marker != _DEFAULTS["req_marker"]:
        custom["req_marker"] = req_marker

    # Gerar conteúdo do YAML
    lines = ["# trackfw.yaml — gerado por trackfw configure"]

    if custom:
        if "adr_dirs" in custom:
            lines.append("adr_dirs:")
            for d in custom["adr_dirs"].split(","):
                lines.append(f"  - {d.strip()}")
        if "req_dir" in custom:
            lines.append(f"req_dir: {custom['req_dir']}")
        if "roadmap_dir" in custom:
            lines.append(f"roadmap_dir: {custom['roadmap_dir']}")
        if "wip_limit" in custom:
            lines.append(f"wip_limit: {custom['wip_limit']}")
        if "req_marker" in custom:
            lines.append("link_fields:")
            lines.append("  req:")
            lines.append(f"    - {custom['req_marker']}")

    content = "\n".join(lines) + "\n"

    with open(yaml_path, "w", encoding="utf-8") as f:
        f.write(content)

    print(f"trackfw.yaml gravado com {len(custom)} campos customizados")


def register(subparsers):
    """Registra o subcomando 'configure' no parser principal."""
    parser = subparsers.add_parser(
        "configure",
        help="Wizard interativo para criar/recriar o trackfw.yaml",
    )
    parser.set_defaults(func=run)
    return parser
