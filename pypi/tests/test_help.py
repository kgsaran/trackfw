import argparse
import unittest
import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))
from trackfw.commands import help_cmd
from trackfw.commands.help_cmd import list_keys, describe_key, suggest_topic, list_commands


class TestHelp(unittest.TestCase):
    def test_list_keys_contains_adr_dirs(self):
        output = list_keys()
        self.assertIn("adr_dirs", output)

    def test_list_keys_contains_wip_limit(self):
        output = list_keys()
        self.assertIn("wip_limit", output)

    def test_describe_known_key(self):
        output = describe_key("wip_limit")
        self.assertIsNotNone(output)
        self.assertIn("integer", output)
        self.assertIn("1", output)  # default value

    def test_describe_unknown_key(self):
        output = describe_key("nao_existe")
        self.assertIsNone(output)

    def test_list_keys_has_all_rules(self):
        output = list_keys()
        for rule in ["rules.wip_has_req", "rules.stale_wip", "rules.adr_orphan"]:
            self.assertIn(rule, output)

    def test_list_keys_has_header(self):
        output = list_keys()
        self.assertIn("KEY", output)
        self.assertIn("DEFAULT", output)
        self.assertIn("DESCRIÇÃO", output)

    def test_list_keys_contains_all_config_keys(self):
        output = list_keys()
        expected_keys = [
            "adr_dirs", "req_dir", "roadmap_dir", "roadmap_namespacing",
            "agents", "governance_mode", "lenient_until", "wip_limit",
            "wip_by_squad", "require_req_in_commit", "link_fields.req",
            "link_fields.adr", "link_fields.roadmap", "acceptance_markers",
            "rules.wip_has_req", "rules.wip_acceptance", "rules.wip_limit",
            "rules.stale_wip", "rules.adr_orphan", "rules.ref_targets_exist",
            "rules.folder_status", "rules.filename_uniqueness",
            "rules.blocked_by_draft_adr",
        ]
        for key in expected_keys:
            self.assertIn(key, output, f"Key ausente na tabela: {key}")

    def test_describe_key_adr_dirs(self):
        output = describe_key("adr_dirs")
        self.assertIsNotNone(output)
        self.assertIn("list of strings", output)
        self.assertIn("docs/adr", output)
        self.assertIn("Impact:", output)
        self.assertIn("Example:", output)

    def test_describe_key_governance_mode(self):
        output = describe_key("governance_mode")
        self.assertIsNotNone(output)
        self.assertIn("lenient", output)

    def test_describe_key_rules_wip_has_req(self):
        output = describe_key("rules.wip_has_req")
        self.assertIsNotNone(output)
        self.assertIn("error", output)
        self.assertIn("off|warning|error", output)

    def test_describe_key_link_fields_req(self):
        output = describe_key("link_fields.req")
        self.assertIsNotNone(output)
        self.assertIn("REQ:", output)

    def test_describe_key_acceptance_markers(self):
        output = describe_key("acceptance_markers")
        self.assertIsNotNone(output)
        self.assertIn("Acceptance Criteria", output)

    def test_list_keys_contains_trace_id_field(self):
        """list_keys() deve incluir trace_id_field."""
        output = list_keys()
        self.assertIn("trace_id_field", output)

    def test_describe_key_trace_id_field(self):
        """describe_key('trace_id_field') deve retornar dados válidos."""
        output = describe_key("trace_id_field")
        self.assertIsNotNone(output)
        self.assertIn("req_id", output)
        self.assertIn("Impact:", output)
        self.assertIn("Example:", output)

    def test_list_keys_contains_traceid_rules(self):
        """list_keys() deve incluir todas as regras rules.traceid_*."""
        output = list_keys()
        for rule in [
            "rules.traceid_orphan_roadmap",
            "rules.traceid_orphan_req",
            "rules.traceid_state_mismatch",
            "rules.traceid_duplicate_req",
            "rules.traceid_duplicate_roadmap",
        ]:
            self.assertIn(rule, output, f"Regra ausente na tabela: {rule}")

    def test_describe_key_traceid_orphan_roadmap(self):
        """describe_key('rules.traceid_orphan_roadmap') deve retornar doc válida."""
        output = describe_key("rules.traceid_orphan_roadmap")
        self.assertIsNotNone(output)
        self.assertIn("off|warning|error", output)
        self.assertIn("error", output)


class TestHelpCommandResolution(unittest.TestCase):
    """Cobre a resolução por nome de comando registrada no ML-5B, e prova
    que apenas um subparser 'help' existe (sem duplicação)."""

    def setUp(self):
        self.parser = argparse.ArgumentParser(prog="trackfw")
        self.subparsers = self.parser.add_subparsers(dest="command")
        init_parser = self.subparsers.add_parser(
            "init", help="Initialize trackfw governance in the current project"
        )
        init_parser.add_argument("--forge")
        help_cmd.register(self.subparsers)

    def tearDown(self):
        help_cmd._subparsers = None

    def test_single_help_subparser_registered(self):
        occurrences = [
            a.dest for a in self.subparsers._choices_actions if a.dest == "help"
        ]
        self.assertEqual(len(occurrences), 1)

    def test_list_commands_includes_init(self):
        output = list_commands()
        self.assertIn("init", output)

    def test_run_with_known_command_prints_its_help(self, capsys=None):
        import io
        import contextlib

        args = argparse.Namespace(topic="init")
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            help_cmd.run(args)
        output = buf.getvalue()
        self.assertIn("trackfw init", output)

    def test_suggest_topic_no_match_for_distant_string(self):
        self.assertIsNone(suggest_topic("chave-que-nao-existe"))

    def test_suggest_topic_matches_near_miss_key(self):
        self.assertEqual(suggest_topic("wip_limi"), "wip_limit")


if __name__ == "__main__":
    unittest.main()
