"""
commands/log.py — Subcomando `trackfw log <message>`.
Faz append de uma mensagem no arquivo .trackfw-log na raiz do projeto.
"""

import os
import sys
from datetime import datetime


def register(subparsers):
    """Adiciona subcomando `log` ao parser principal."""
    log_parser = subparsers.add_parser(
        "log",
        help="Append a message to .trackfw-log",
    )
    log_parser.add_argument("message", help="Message to log")
    log_parser.set_defaults(func=_cmd_log)


def _cmd_log(args):
    """Faz append de args.message no .trackfw-log na raiz do projeto."""
    from trackfw.config import load as load_config

    # O .trackfw-log fica na raiz do projeto (mesmo dir que trackfw.yaml)
    cwd = os.getcwd()
    log_path = os.path.join(cwd, ".trackfw-log")

    timestamp = datetime.now().strftime("%Y-%m-%d %H:%M")
    line = f"{timestamp} {args.message}\n"

    with open(log_path, "a", encoding="utf-8") as f:
        f.write(line)

    print(f"logged: {args.message}")
