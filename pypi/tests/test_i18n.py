"""
Testes de unidade para pypi/trackfw/i18n/__init__.py
Usa unittest (stdlib) — sem dependências externas.
"""

import os
import sys
import unittest

# Garante que o pacote pypi/trackfw seja importável
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

import trackfw.i18n as i18n


class TestI18n(unittest.TestCase):

    def setUp(self):
        """Limpa o cache e env vars antes de cada teste."""
        i18n.reset()
        # Remove variáveis de locale que possam interferir
        for var in ("TRACKFW_LANG", "LANG", "LANGUAGE", "LC_ALL"):
            os.environ.pop(var, None)

    def tearDown(self):
        """Restaura estado limpo após cada teste."""
        i18n.reset()
        for var in ("TRACKFW_LANG", "LANG", "LANGUAGE", "LC_ALL"):
            os.environ.pop(var, None)

    # ------------------------------------------------------------------
    # test_fallback_en_us
    # ------------------------------------------------------------------
    def test_fallback_en_us(self):
        """Sem variável de locale definida, retorna valor en-US."""
        val = i18n.t("init.description")
        self.assertEqual(val, "Initialize trackfw governance in the current project")

    # ------------------------------------------------------------------
    # test_pt_br
    # ------------------------------------------------------------------
    def test_pt_br(self):
        """TRACKFW_LANG=pt-BR retorna valor em português."""
        os.environ["TRACKFW_LANG"] = "pt-BR"
        i18n.reset()
        val = i18n.t("init.description")
        self.assertEqual(val, "Inicializa a governança trackfw no projeto atual")

    # ------------------------------------------------------------------
    # test_pt_br_via_lang_env
    # ------------------------------------------------------------------
    def test_pt_br_via_lang_env(self):
        """LANG=pt_BR.UTF-8 (formato Unix) também ativa locale pt-BR."""
        os.environ["LANG"] = "pt_BR.UTF-8"
        i18n.reset()
        val = i18n.t("init.description")
        self.assertEqual(val, "Inicializa a governança trackfw no projeto atual")

    # ------------------------------------------------------------------
    # test_es_es
    # ------------------------------------------------------------------
    def test_es_es(self):
        """TRACKFW_LANG=es-ES retorna valor em espanhol."""
        os.environ["TRACKFW_LANG"] = "es-ES"
        i18n.reset()
        val = i18n.t("init.description")
        self.assertEqual(val, "Inicializa la gobernanza trackfw en el proyecto actual")

    # ------------------------------------------------------------------
    # test_chave_inexistente
    # ------------------------------------------------------------------
    def test_chave_inexistente(self):
        """Chave que não existe retorna a própria chave."""
        val = i18n.t("chave.que.nao.existe.em.nenhum.locale")
        self.assertEqual(val, "chave.que.nao.existe.em.nenhum.locale")

    # ------------------------------------------------------------------
    # test_chave_aninhada
    # ------------------------------------------------------------------
    def test_chave_aninhada(self):
        """Chaves aninhadas com ponto funcionam corretamente."""
        val = i18n.t("adr.new.description")
        self.assertEqual(val, "Create a new Architecture Decision Record")

    # ------------------------------------------------------------------
    # test_interpolacao_de_variaveis
    # ------------------------------------------------------------------
    def test_interpolacao_de_variaveis(self):
        """Variáveis {{var}} são substituídas pelos kwargs."""
        val = i18n.t("adr.new.created", path="docs/adr/ADR-001-test.md")
        self.assertEqual(val, "✓ ADR created: docs/adr/ADR-001-test.md")

    # ------------------------------------------------------------------
    # test_interpolacao_pt_br
    # ------------------------------------------------------------------
    def test_interpolacao_pt_br(self):
        """Interpolação funciona também no locale pt-BR."""
        os.environ["TRACKFW_LANG"] = "pt-BR"
        i18n.reset()
        val = i18n.t("adr.new.created", path="docs/adr/ADR-001-teste.md")
        self.assertEqual(val, "✓ ADR criado: docs/adr/ADR-001-teste.md")

    # ------------------------------------------------------------------
    # test_locale_detectado_en_us
    # ------------------------------------------------------------------
    def test_locale_detectado_en_us(self):
        """locale() retorna 'en-US' sem variáveis definidas."""
        lc = i18n.locale()
        self.assertEqual(lc, "en-US")

    # ------------------------------------------------------------------
    # test_locale_detectado_pt_br
    # ------------------------------------------------------------------
    def test_locale_detectado_pt_br(self):
        """locale() retorna 'pt-BR' quando TRACKFW_LANG=pt-BR."""
        os.environ["TRACKFW_LANG"] = "pt-BR"
        i18n.reset()
        lc = i18n.locale()
        self.assertEqual(lc, "pt-BR")

    # ------------------------------------------------------------------
    # test_fallback_chave_ausente_no_locale_atual
    # ------------------------------------------------------------------
    def test_fallback_chave_ausente_no_locale_atual(self):
        """Se chave não existir no locale atual, usa fallback en-US."""
        # Injeta um locale fictício cujas mensagens estarão vazias
        # Simulamos isso limpando o cache e injetando dict vazio
        os.environ["TRACKFW_LANG"] = "pt-BR"
        i18n.reset()
        # Temporariamente remove a chave do cache de pt-BR para simular ausência
        i18n._load("pt-BR")
        saved = i18n._cache.get("pt-BR", {}).get("status")
        if saved:
            i18n._cache["pt-BR"].pop("status", None)
            val = i18n.t("status.description")
            # Deve retornar o valor en-US como fallback
            self.assertEqual(val, "Show project governance status")
            # Restaura
            i18n._cache["pt-BR"]["status"] = saved
        else:
            # Se o campo não estava no cache, verifica que o fallback funciona mesmo assim
            val = i18n.t("status.description")
            self.assertIsInstance(val, str)
            self.assertTrue(len(val) > 0)


if __name__ == "__main__":
    unittest.main()
