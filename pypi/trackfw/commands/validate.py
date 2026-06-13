"""
validate.py — Comando `trackfw validate`.

Executa as validações de governança e reporta violations e warnings.
Espelho Python de npm/src/commands/validate.js.
"""

import json
import sys

from .. import validator as _validator
from .. import config as _config

# Códigos ANSI
_RED = "\033[31m"
_YELLOW = "\033[33m"
_GREEN = "\033[32m"
_RESET = "\033[0m"


def _supports_color() -> bool:
    """Retorna True se o terminal suporta cores ANSI."""
    return hasattr(sys.stdout, "isatty") and sys.stdout.isatty()


def _red(text: str) -> str:
    return f"{_RED}{text}{_RESET}" if _supports_color() else text


def _yellow(text: str) -> str:
    return f"{_YELLOW}{text}{_RESET}" if _supports_color() else text


def _green(text: str) -> str:
    return f"{_GREEN}{text}{_RESET}" if _supports_color() else text


def _item_to_json_dict(item) -> dict:
    """Converte um item de violation/warning para dict JSON estruturado.

    Extrai 'rule', 'file' e 'message' quando disponíveis no dict original.
    Campos ausentes ficam como None no output JSON.
    """
    if isinstance(item, dict):
        return {
            "rule": item.get("rule"),
            "file": item.get("file"),
            "message": item.get("message", str(item)),
        }
    return {"rule": None, "file": None, "message": str(item)}


def register(subparsers):
    """Registra o subcomando 'validate' no parser principal."""
    parser = subparsers.add_parser(
        "validate",
        help="Valida a conformidade de governança do projeto",
    )
    parser.add_argument(
        "--json",
        action="store_true",
        default=False,
        help="Emite o resultado em formato JSON estruturado",
    )
    parser.set_defaults(func=run)
    return parser


def run(args):
    """Executa a validação e imprime o resultado."""
    result = _validator.validate()
    violations = result.get("violations", [])
    warnings = result.get("warnings", [])

    gm = _validator._read_governance_mode()
    mode = "lenient" if gm["mode"] == "lenient" else "strict"
    exit_code = 1 if violations else 0

    # Modo JSON: emite apenas JSON estruturado e sai
    if getattr(args, "json", False):
        output = {
            "summary": {
                "violations": len(violations),
                "warnings": len(warnings),
                "mode": mode,
                "exit_code": exit_code,
            },
            "violations": [_item_to_json_dict(v) for v in violations],
            "warnings": [_item_to_json_dict(w) for w in warnings],
        }
        print(json.dumps(output, indent=2))
        sys.exit(exit_code)
        return

    # Modo texto: comportamento original inalterado
    if gm["mode"] == "lenient":
        until = gm.get("lenient_until")
        if until:
            print(f"[LENIENT MODE] Governance em modo permissivo até {until}")
        else:
            print("[LENIENT MODE] Governance em modo permissivo")

    if not violations and not warnings:
        print(_green("✓ Governance OK"))
        return 0

    if violations:
        print(f"\n{len(violations)} violation(s) encontrada(s):")
        for v in violations:
            msg = v["message"] if isinstance(v, dict) else str(v)
            print(f"  {_red('✗')} {msg}")

    if warnings:
        print(f"\n{len(warnings)} warning(s):")
        for w in warnings:
            msg = w["message"] if isinstance(w, dict) else str(w)
            print(f"  {_yellow('⚠')} {msg}")

    # Exit code 1 apenas se há violations (modo lenient já converte violations em warnings)
    if violations:
        sys.exit(1)

    return 0
