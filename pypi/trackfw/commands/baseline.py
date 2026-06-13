"""
baseline.py — Comando `trackfw baseline`.
Salva snapshot das violations atuais em .trackfw-baseline.json.
"""

from .. import validator as _validator


def register(subparsers):
    """Registra o subcomando 'baseline' no parser principal."""
    parser = subparsers.add_parser(
        "baseline",
        help="Grava snapshot das violations atuais em .trackfw-baseline.json",
    )
    parser.set_defaults(func=run)
    return parser


def run(args):
    """Executa validate_unfiltered() e salva o resultado como baseline."""
    result = _validator.validate_unfiltered()
    violations = result.get("violations", [])
    warnings = result.get("warnings", [])
    _validator.save_baseline(violations, warnings)
    print(f"Baseline gravado: {len(violations)} violations, {len(warnings)} warnings")
