"""commands/log.py — Subcomando `trackfw log`."""

import os


def register(subparsers):
    """Adiciona subcomando `log` ao parser principal."""
    log_parser = subparsers.add_parser(
        "log",
        help="Show trackfw transition log",
    )
    log_parser.add_argument("--tail", type=int, default=20, help="Number of lines to show")
    log_parser.set_defaults(func=_cmd_log)


def _cmd_log(args):
    """Mostra o .trackfw-log em roadmap_dir."""
    from trackfw.config import load as load_config

    cfg = load_config()
    log_path = os.path.join(cfg.get("roadmap_dir", "docs/roadmaps"), ".trackfw-log")

    if not os.path.exists(log_path):
        print("No transition log found")
        return

    with open(log_path, "r", encoding="utf-8") as f:
        lines = [line.rstrip("\n") for line in f if line.strip()]

    tail = max(args.tail, 0)
    visible = lines[-tail:] if tail else []

    print("── trackfw log ─────────────────────────")
    for line in visible:
        print(line)
