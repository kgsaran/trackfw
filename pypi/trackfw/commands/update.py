"""
commands/update.py — trackfw update (Python CLI).
Escopo reduzido: atualiza somente as regras de agente (blocos marker-delimited).
Gates (hooks/CI) e Claude commands requerem o CLI Go ou Node.js.
"""

import os
import argparse


def register(subparsers: argparse.ArgumentParser) -> None:
    parser = subparsers.add_parser(
        "update",
        help="Update trackfw rules in agent config files (agent rules only)",
    )
    parser.set_defaults(func=_run)


def _run(args: argparse.Namespace) -> None:
    cwd = os.getcwd()
    yaml_path = os.path.join(cwd, "trackfw.yaml")

    if not os.path.exists(yaml_path):
        print("Erro: trackfw.yaml não encontrado — execute trackfw init primeiro")
        raise SystemExit(1)

    print("trackfw update — atualizando regras de agente...\n")

    from trackfw.generators.init_gen import inject_rules_detected
    try:
        inject_rules_detected(cwd)
        print("  Regras de agente atualizadas (CLAUDE.md, GEMINI.md, etc.)")
    except Exception as e:
        print(f"  Aviso: falha ao atualizar regras: {e}")

    print()
    print("  Nota: este CLI Python atualiza apenas as regras de agente.")
    print("  Para atualizar gates (hooks/CI) e Claude commands, use:")
    print("    trackfw update   (CLI Go)")
    print("    npx trackfw update   (CLI Node.js)")

    print("\ntrackfw update concluído")
    try:
        from trackfw.generators.init_gen import print_architect_next_steps
        print_architect_next_steps(cwd)
    except Exception:
        pass
