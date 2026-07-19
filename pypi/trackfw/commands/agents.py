"""The ``trackfw agents`` lifecycle command."""

from trackfw.integrations.command import add_lifecycle_parser


def register(subparsers):
    return add_lifecycle_parser(subparsers, "agents")
