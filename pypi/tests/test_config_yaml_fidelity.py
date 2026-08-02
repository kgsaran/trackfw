"""
Testes de unidade para pypi/trackfw/config.py — fidelidade textual da normalização YAML.

Cobre o AC3 (ADR-2026-08-02-parsing-de-config-por-biblioteca-yaml-com-normalizacao-para-string-
na-fronteira.md), teste por chave das ~20 chaves suportadas, as formas antes não suportadas
(mapa inline, lista aninhada inline, âncora) e o comportamento com config ausente/vazio ou
malformado.
"""

import contextlib
import io
import os
import sys
import tempfile
import shutil
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from trackfw import config


class ConfigFidelityTestCase(unittest.TestCase):

    def setUp(self):
        self.tmpdir = tempfile.mkdtemp()
        config.reset()

    def tearDown(self):
        config.reset()
        shutil.rmtree(self.tmpdir, ignore_errors=True)

    def _write(self, content):
        with open(os.path.join(self.tmpdir, "trackfw.yaml"), "w", encoding="utf-8") as f:
            f.write(content)

    def _load(self):
        return config.load(cwd=self.tmpdir)


class TestAC3FidelidadeTextual(ConfigFidelityTestCase):

    def test_lenient_until_data_nua_permanece_string(self):
        self._write("lenient_until: 2026-08-02\n")
        self.assertEqual(self._load()["lenient_until"], "2026-08-02")

    def test_wip_limit_octal_010_vira_10_nao_8(self):
        self._write("wip_limit: 010\n")
        self.assertEqual(self._load()["wip_limit"], 10)

    def test_wip_limit_decimal_simples(self):
        self._write("wip_limit: 3\n")
        self.assertEqual(self._load()["wip_limit"], 3)

    def test_wip_by_squad_true(self):
        self._write("wip_by_squad: true\n")
        self.assertTrue(self._load()["wip_by_squad"])

    def test_governance_mode_yes_permanece_string_yes(self):
        self._write("governance_mode: yes\n")
        self.assertEqual(self._load()["governance_mode"], "yes")

    def test_governance_mode_1_0_preserva_ponto_decimal(self):
        self._write("governance_mode: 1.0\n")
        self.assertEqual(self._load()["governance_mode"], "1.0")

    def test_governance_mode_null(self):
        self._write("governance_mode: null\n")
        self.assertEqual(self._load()["governance_mode"], "null")

    def test_governance_mode_til(self):
        self._write("governance_mode: ~\n")
        self.assertEqual(self._load()["governance_mode"], "~")


class TestAncoras(ConfigFidelityTestCase):

    def test_ancora_chega_com_valor_nao_o_nome(self):
        self._write("governance_mode: &gm strict\nforge: *gm\n")
        cfg = self._load()
        self.assertEqual(cfg["governance_mode"], "strict")
        self.assertEqual(cfg["forge"], "strict")

    def test_ancora_dentro_de_lista_resolve(self):
        self._write("agents: [&a zeus, apolo, *a]\n")
        self.assertEqual(self._load()["agents"], ["zeus", "apolo", "zeus"])


class TestFormasAntesNaoSuportadas(ConfigFidelityTestCase):

    def test_mapa_inline_rules(self):
        self._write("rules: {stale_wip: error, adr_orphan: warning}\n")
        cfg = self._load()
        self.assertEqual(cfg["rules"]["stale_wip"], "error")
        self.assertEqual(cfg["rules"]["adr_orphan"], "warning")
        self.assertEqual(cfg["rules"]["wip_has_req"], "error")  # default preservado

    def test_lista_aninhada_inline_link_fields(self):
        self._write('link_fields:\n  req: ["REQ:", "req_id"]\n')
        self.assertEqual(self._load()["link_fields"]["req"], ["REQ:", "req_id"])


class TestConfigAusenteVazioMalformado(ConfigFidelityTestCase):

    def test_config_vazio_cai_nos_defaults(self):
        self._write("")
        self.assertEqual(self._load(), config.defaults())

    def test_config_so_com_comentario_cai_nos_defaults(self):
        self._write("# apenas um comentário\n\n")
        self.assertEqual(self._load(), config.defaults())

    # Fixture confirmada como erro real de parse nos 3 CLIs (não vacua sob nenhum schema): Go
    # yaml.v3 devolve "did not find expected ',' or ']'"; PyYAML levanta YAMLError aqui mesmo
    # (while parsing a flow sequence); yaml (Node) popula doc.errors (não lança, mas o array
    # não fica vazio).
    #
    # ML-1B: load() agora falha alto (stderr + sys.exit(1)) em YAML malformado, em vez do
    # fallback silencioso anterior — o fallback silencioso era regressão frente ao parser
    # artesanal (que nunca via a config inteira ser descartada por um erro de sintaxe local).
    def _assert_load_fails_loud(self):
        stderr = io.StringIO()
        with self.assertRaises(SystemExit) as ctx:
            with contextlib.redirect_stderr(stderr):
                self._load()
        self.assertEqual(ctx.exception.code, 1)
        self.assertEqual(stderr.getvalue(), config.MALFORMED_CONFIG_MESSAGE + "\n")

    def test_yaml_malformado_falha_alto(self):
        self._write("agents: [zeus, apolo\nwip_limit: 3\n")
        self._assert_load_fails_loud()

    # Divergência encontrada na auditoria cruzada do ML-1B: yaml.compose() já rejeita stream com
    # mais de um documento ("expected a single document in the stream"), igual ao Node
    # (MULTIPLE_DOCS); sem hasMultipleDocuments no lado Go, yaml.Unmarshal decodificaria
    # silenciosamente só o primeiro documento e sairia com exit 0 — este teste guarda o lado
    # Python dessa convergência de 3 vias.
    def test_multiplos_documentos_falha_alto(self):
        self._write("wip_limit: 3\n---\nwip_limit: 5\n")
        self._assert_load_fails_loud()

    # Segunda divergência da mesma auditoria: referência de âncora antes da definição
    # (b: *x / a: &x 3) é inválida pela spec YAML — PyYAML levanta ComposerError, igual ao
    # yaml.v3 do Go ("unknown anchor 'x' referenced"). O Node só fecha essa mesma divergência
    # porque resolveAlias() nele passou a detectar Alias#resolve(doc) === undefined.
    def test_referencia_de_ancora_antes_da_definicao_falha_alto(self):
        self._write("b: *x\na: &x 3\n")
        self._assert_load_fails_loud()

    def test_referencia_de_ancora_depois_da_definicao_continua_valida(self):
        self._write("a: &x 3\nb: *x\n")
        self.assertEqual(self._load(), config.defaults())  # nao lanca; chave nao consumida

    # Divergência (direção oposta) da mesma auditoria: chaves top-level duplicadas SÃO aceitas
    # (last-wins) por PyYAML yaml.compose() e por gopkg.in/yaml.v3 (decodificando em *yaml.Node,
    # não struct) — só a lib `yaml` do Node marca isso como erro (DUPLICATE_KEY), filtrado lá via
    # NON_FATAL_ERROR_CODES para não divergir. Este teste guarda o lado Python.
    def test_chaves_top_level_duplicadas_nao_sao_malformado(self):
        self._write("wip_limit: 3\nwip_limit: 4\n")
        self.assertEqual(self._load()["wip_limit"], 4)


class TestPorChaveAsVinteChaves(ConfigFidelityTestCase):

    def test_adr_dirs(self):
        self._write("adr_dirs:\n  - docs/adr/x\n")
        self.assertEqual(self._load()["adr_dirs"], ["docs/adr/x"])

    def test_agents(self):
        self._write("agents:\n  - zeus\n")
        self.assertEqual(self._load()["agents"], ["zeus"])

    def test_req_dir(self):
        self._write("req_dir: docs/req2\n")
        self.assertEqual(self._load()["req_dir"], "docs/req2")

    def test_roadmap_dir(self):
        self._write("roadmap_dir: docs/rm2\n")
        self.assertEqual(self._load()["roadmap_dir"], "docs/rm2")

    def test_roadmap_namespacing(self):
        self._write("roadmap_namespacing: by_agent\n")
        self.assertEqual(self._load()["roadmap_namespacing"], "by_agent")

    def test_acceptance_markers(self):
        self._write('acceptance_markers:\n  - "## Done"\n')
        self.assertEqual(self._load()["acceptance_markers"], ["## Done"])

    def test_link_fields_req(self):
        self._write('link_fields:\n  req:\n    - "req_id"\n')
        self.assertEqual(self._load()["link_fields"]["req"], ["req_id"])

    def test_link_fields_adr(self):
        self._write('link_fields:\n  adr:\n    - "adr_id"\n')
        self.assertEqual(self._load()["link_fields"]["adr"], ["adr_id"])

    def test_link_fields_roadmap(self):
        self._write('link_fields:\n  roadmap:\n    - "rm_id"\n')
        self.assertEqual(self._load()["link_fields"]["roadmap"], ["rm_id"])

    def test_rules(self):
        self._write("rules:\n  stale_wip: error\n")
        self.assertEqual(self._load()["rules"]["stale_wip"], "error")

    def test_wip_limit(self):
        self._write("wip_limit: 4\n")
        self.assertEqual(self._load()["wip_limit"], 4)

    def test_wip_by_squad(self):
        self._write("wip_by_squad: true\n")
        self.assertTrue(self._load()["wip_by_squad"])

    def test_stale_wip_days(self):
        self._write("stale_wip_days: 14\n")
        self.assertEqual(self._load()["stale_wip_days"], 14)

    def test_lenient_until(self):
        self._write("lenient_until: 2026-09-01\n")
        self.assertEqual(self._load()["lenient_until"], "2026-09-01")

    def test_governance_mode(self):
        self._write("governance_mode: strict\n")
        self.assertEqual(self._load()["governance_mode"], "strict")

    def test_require_req_in_commit(self):
        self._write("require_req_in_commit: true\n")
        self.assertTrue(self._load()["require_req_in_commit"])

    def test_strict_ci_paths(self):
        self._write("strict_ci_paths: true\n")
        self.assertTrue(self._load()["strict_ci_paths"])

    def test_trace_id_field(self):
        self._write("trace_id_field: req_id\n")
        self.assertEqual(self._load()["trace_id_field"], "req_id")

    def test_forge(self):
        self._write("forge: gitlab\n")
        self.assertEqual(self._load()["forge"], "gitlab")

    def test_squad_chave_nao_consumida_nao_deve_quebrar(self):
        self._write("squad: platform\nreq_dir: docs/req\n")
        self.assertEqual(self._load()["req_dir"], "docs/req")


if __name__ == "__main__":
    unittest.main()
