"""
changelog.py — parsing e extração de seções do CHANGELOG.md no formato
Keep a Changelog (https://keepachangelog.com/en/1.1.0/).

Port 1:1 de internal/changelog/changelog.go (fonte de verdade). Mesmas
funções, mesma semântica, mesmas mensagens de erro (byte-idênticas).
"""

import os
import re

# section_header_re casa cabeçalhos de seção no formato
# "## [x.y.z] - YYYY-MM-DD" ou "## [Unreleased]" (sem data).
_SECTION_HEADER_RE = re.compile(r"^## \[([^\]]+)\](?: - (\d{4}-\d{2}-\d{2}))?")


class Section:
    """
    Representa uma seção de versão do CHANGELOG.md.
    version é "Unreleased" (sem colchetes) ou "x.y.z". body é o texto
    completo da seção (incluindo as subseções ### Added etc.), sem a linha
    de cabeçalho "## [...]".
    """

    __slots__ = ("version", "date", "body")

    def __init__(self, version: str, date: str = "", body: str = ""):
        self.version = version
        self.date = date
        self.body = body


def parse_sections(content: str) -> list:
    """
    Separa o conteúdo de um CHANGELOG.md em Section, uma por cabeçalho
    "## [...]" encontrado. Texto antes da primeira seção (título do
    arquivo, preâmbulo) é descartado.
    """
    lines = content.split("\n")

    sections = []
    current = None
    body_lines = []

    def flush():
        if current is not None:
            current.body = "\n".join(body_lines)
            sections.append(current)

    for line in lines:
        m = _SECTION_HEADER_RE.match(line)
        if m:
            flush()
            current = Section(version=m.group(1), date=m.group(2) or "")
            body_lines = []
            continue
        if current is not None:
            body_lines.append(line)

    flush()

    return sections


def first_section(sections: list) -> Section:
    """Retorna a primeira seção da lista. Erro se a lista vier vazia."""
    if not sections:
        raise ValueError("CHANGELOG.md has no version sections")
    return sections[0]


def find_version(sections: list, version: str) -> Section:
    """
    Busca a seção com version igual ao argumento, normalizando um prefixo
    "v"/"V" opcional no argumento do usuário antes de comparar.
    """
    normalized = version
    if normalized and normalized[0] in ("v", "V"):
        normalized = normalized[1:]
    for s in sections:
        if s.version == normalized or s.version == version:
            return s
    raise ValueError(f'version "{version}" not found in CHANGELOG.md')


def format_section(s: Section) -> str:
    """Reconstrói o texto formatado de uma seção, reproduzindo o cabeçalho original."""
    date_suffix = ""
    if s.date:
        date_suffix = " - " + s.date
    body = s.body.lstrip("\n")
    return f"## [{s.version}]{date_suffix}\n\n{body.rstrip(chr(10))}\n"


def read(root: str) -> str:
    """Lê o CHANGELOG.md na raiz informada."""
    path = os.path.join(root, "CHANGELOG.md")
    if not os.path.exists(path):
        raise FileNotFoundError("CHANGELOG.md not found — nothing to show")
    with open(path, "r", encoding="utf-8") as f:
        return f.read()
