"""Comando ``trackfw init`` para o pacote Python."""

import os
import sys

from trackfw.generators.init_gen import scaffold


def _parse_agents(raw: str) -> list[str]:
    return [agent.strip() for agent in raw.split(",") if agent.strip()]


def register(subparsers):
    parser = subparsers.add_parser(
        "init",
        help="Initialize trackfw governance in the current project",
    )
    parser.add_argument("--project-name", default=None, help="Project name")
    parser.add_argument(
        "--namespacing",
        choices=["flat", "by_agent"],
        default="flat",
        help="Roadmap directory layout",
    )
    parser.add_argument(
        "--agents",
        default="",
        help="Comma-separated agents used with --namespacing by_agent",
    )
    parser.add_argument("--wip-limit", type=int, default=1, help="Maximum active roadmaps")
    parser.set_defaults(func=run)
    return parser


def run(args):
    agents = _parse_agents(args.agents)
    if args.namespacing == "by_agent" and not agents:
        print("--agents is required with --namespacing by_agent", file=sys.stderr)
        sys.exit(2)
    if args.wip_limit < 1:
        print("--wip-limit must be greater than zero", file=sys.stderr)
        sys.exit(2)

    cwd = os.getcwd()
    opts = {
        "project_name": args.project_name or os.path.basename(cwd),
        "namespacing": args.namespacing,
        "agents": agents,
        "wip_limit": args.wip_limit,
    }
    scaffold(cwd, opts)
    return 0
