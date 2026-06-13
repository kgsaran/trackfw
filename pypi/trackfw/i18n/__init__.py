"""
trackfw.i18n — internacionalização do CLI Python.

Detecta o locale via variáveis de ambiente e carrega o arquivo JSON correspondente
do diretório pypi/trackfw/i18n/locales/.

Ordem de detecção:
  TRACKFW_LANG → LANG → LANGUAGE → LC_ALL → fallback en-US

Normalização de locale:
  pt_BR* → pt-BR | es_* → es-ES | qualquer outro → en-US

Uso:
  from trackfw.i18n import t
  print(t("init.description"))
  print(t("adr.new.created", path="docs/adr/ADR-001.md"))
"""

import json
import os
import re
from pathlib import Path

_LOCALES_DIR = Path(__file__).parent / "locales"

_SUPPORTED = {"pt-BR", "en-US", "es-ES"}

_cache: dict[str, dict] = {}
_detected_locale: str | None = None


def _detect_locale() -> str:
    """Detecta o locale a partir das variáveis de ambiente."""
    raw = (
        os.environ.get("TRACKFW_LANG")
        or os.environ.get("LANG")
        or os.environ.get("LANGUAGE")
        or os.environ.get("LC_ALL")
        or ""
    )
    # Remove sufixo de encoding: pt_BR.UTF-8 → pt_BR
    raw = raw.split(".")[0].split(":")[0]  # LANGUAGE pode ter lista separada por ":"

    # Normaliza separador: pt_BR → pt-BR
    normalized = raw.replace("_", "-")

    # Mapeamento por prefixo de idioma
    lang_prefix = normalized.split("-")[0].lower()
    if lang_prefix == "pt":
        return "pt-BR"
    if lang_prefix == "es":
        return "es-ES"

    return "en-US"


def _load(locale: str) -> dict:
    """Carrega e armazena em cache o arquivo JSON do locale especificado."""
    if locale in _cache:
        return _cache[locale]

    path = _LOCALES_DIR / f"{locale}.json"
    if not path.exists():
        path = _LOCALES_DIR / "en-US.json"

    try:
        with open(path, encoding="utf-8") as fh:
            data = json.load(fh)
    except Exception:
        data = {}

    _cache[locale] = data
    return data


def _get_nested(data: dict, key: str):
    """Navega em dicionário aninhado usando chave com pontos: 'a.b.c'."""
    parts = key.split(".")
    val = data
    for part in parts:
        if not isinstance(val, dict):
            return None
        val = val.get(part)
    return val


def locale() -> str:
    """Retorna o locale detectado."""
    global _detected_locale
    if _detected_locale is None:
        _detected_locale = _detect_locale()
    return _detected_locale


def t(key: str, **vars: str) -> str:
    """
    Retorna a string localizada para a chave informada.

    Suporta chaves aninhadas com ponto: t("init.prompt.projectName")
    Suporta interpolação via kwargs: t("adr.new.created", path="docs/adr/...")

    Fallback em cascata:
      locale atual → en-US → retorna a própria chave
    """
    current = locale()
    messages = _load(current)
    val = _get_nested(messages, key)

    # Fallback para en-US se não encontrado no locale atual
    if val is None and current != "en-US":
        fallback = _load("en-US")
        val = _get_nested(fallback, key)

    # Se ainda não encontrado, retorna a própria chave
    if not isinstance(val, str):
        return key

    # Interpolação de variáveis: {{var}} → valor
    if vars:
        for k, v in vars.items():
            val = val.replace("{{" + k + "}}", str(v))

    return val


def reset() -> None:
    """Reseta o cache de locale (útil em testes para trocar TRACKFW_LANG)."""
    global _detected_locale, _cache
    _detected_locale = None
    _cache = {}
