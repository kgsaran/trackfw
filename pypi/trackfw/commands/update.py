"""
commands/update.py — trackfw update (Python CLI).

Scope: the current repository only (docs/cli-parity.md, "`trackfw update`
vs `trackfw update harness`"). This command never mutates global state —
every write below is rooted at `cwd`, and the Codex integration block below
plans/applies with `scope="project"` explicitly. `trackfw update harness`
(trackfw/commands/update_harness.py) is the counterpart that refreshes the
user's global harness (`~/.claude`, `~/.codex`, etc.) and runs from
anywhere, without a `trackfw.yaml`.

Escopo reduzido: atualiza somente as regras de agente (blocos marker-delimited)
e a integração Codex de projeto já instalada. Gates (hooks/CI) e Claude
commands requerem o CLI Go ou Node.js — ver docs/cli-parity.md.
"""

import os
import argparse

from trackfw.commands import update_harness


def register(subparsers: argparse.ArgumentParser) -> None:
    parser = subparsers.add_parser(
        "update",
        help="Update trackfw rules in agent config files (agent rules only)",
    )
    # `update_action` is optional (required=False, the argparse default) so
    # bare `trackfw update` keeps running `_run` below via `set_defaults`.
    # Only when the user types `trackfw update harness` does the child
    # parser's own `set_defaults(func=...)` override it.
    update_actions = parser.add_subparsers(dest="update_action")
    update_harness.register(update_actions)
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
        from trackfw import identity
        from trackfw.identity import IdentityError

        # Identity errors must abort the command — never fall back silently
        # to the neutral default, which would revert the user's identity.
        try:
            ident = identity.load(os.path.expanduser("~"))
        except IdentityError as e:
            print(f"update: identidade invalida: {e}")
            raise SystemExit(2) from e

        try:
            from trackfw.integrations.catalog import plan_deployments
            from trackfw.integrations.manager import IntegrationManager
            manager = IntegrationManager(cwd)
            _, plans = plan_deployments("agents", target_ids=["codex"], scope="project", identity_cfg=ident)
            plans = [plan for plan, status in zip(plans, manager.list(plans)) if status["state"] != "not-installed"]
            manager.update(plans)
            _, plans = plan_deployments("skills", target_ids=["codex"], scope="project", identity_cfg=ident)
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
