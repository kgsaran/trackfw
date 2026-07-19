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

    from trackfw.generators.hooks import inject_hooks_detected
    try:
        inject_hooks_detected(cwd)
        print('  ✓ agent hooks atualizados')
    except Exception as e:
        print(f'  ⚠ agent hooks: {e}')

    if os.path.exists(os.path.join(cwd, "AGENTS.md")) or os.path.isdir(os.path.join(cwd, ".codex")):
        try:
            from trackfw.integrations.catalog import plan_deployments
            from trackfw.integrations.manager import IntegrationManager
            manager = IntegrationManager(cwd)
            _, plans = plan_deployments("agents", target_ids=["codex"], scope="project")
            plans = [plan for plan, status in zip(plans, manager.list(plans)) if status["state"] != "not-installed"]
            manager.update(plans)
            _, plans = plan_deployments("skills", target_ids=["codex"], scope="project")
            plans = [plan for plan, status in zip(plans, manager.list(plans)) if status["state"] != "not-installed"]
            manager.update(plans)
        except Exception as e:
            print(f"  ⚠ Codex integration: {e}")

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
