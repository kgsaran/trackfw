"""
generators/note.py — Gerador de notas de vault para trackfw.
Stdlib apenas — sem dependências externas.
"""

import os
import re
import unicodedata
from datetime import date

VAULT_DIR = "vault/notes"
INDEX_FILE = "vault/notes/index.md"


def slugify(title: str) -> str:
    """
    Converte título em slug kebab-case portável.
    NFKD + remoção de diacríticos + lowercase + [^a-z0-9]+ → hífen.
    Ex: "Autenticação e Sessão" → "autenticacao-e-sessao"
    """
    normalized = unicodedata.normalize("NFKD", title)
    ascii_str = normalized.encode("ascii", "ignore").decode("ascii")
    slug = ascii_str.lower()
    slug = re.sub(r"[^a-z0-9]+", "-", slug)
    slug = re.sub(r"-+", "-", slug)
    return slug.strip("-")


def new_note(title: str, cwd: str = None) -> str:
    """
    Cria vault/notes/<slug>-YYYY-MM-DD.md e linka no index.md.
    Idempotente: falha com erro claro se a nota já existir.

    Returns:
        Path do arquivo criado.
    """
    base = cwd or os.getcwd()
    vault_dir = os.path.join(base, VAULT_DIR)
    os.makedirs(vault_dir, exist_ok=True)

    slug = slugify(title)
    today = date.today().isoformat()
    filename = f"{slug}-{today}.md"
    note_path = os.path.join(vault_dir, filename)

    if os.path.exists(note_path):
        raise FileExistsError(f'nota "{filename}" já existe — não sobrescrita')

    body = (
        f"---\n"
        f'title: "{title}"\n'
        f"tags: []\n"
        f"date: {today}\n"
        f"related: []\n"
        f"---\n\n"
        f"# {title}\n\n"
        f"## Problem\n\n"
        f"<!-- Descreva o problema ou situação que motivou esta nota. -->\n\n"
        f"## Root cause\n\n"
        f"<!-- Qual foi a causa raiz identificada? -->\n\n"
        f"## Solution\n\n"
        f"<!-- Como foi resolvido ou mitigado? O que deve ser feito? -->\n"
    )

    with open(note_path, "w", encoding="utf-8", newline="\n") as f:
        f.write(body)

    append_note_to_index(filename, base)
    print(f"created {os.path.join(VAULT_DIR, filename)}")
    return note_path


def append_note_to_index(filename: str, cwd: str = None) -> None:
    """
    Acrescenta link para filename no vault/notes/index.md.
    Cria o index.md se não existir. Não duplica se já estiver linkado.
    Aceita link markdown `[texto](arquivo.md)` ou wikilink `[[nome]]`.
    """
    base = cwd or os.getcwd()
    index_path = os.path.join(base, INDEX_FILE)

    if not os.path.exists(index_path):
        initial = (
            "# Vault de Conhecimento\n\n"
            "> Ponto de entrada de conhecimento do projeto para agentes e pessoas.\n\n"
            "## Índice\n\n"
        )
        with open(index_path, "w", encoding="utf-8", newline="\n") as f:
            f.write(initial)

    with open(index_path, "r", encoding="utf-8") as f:
        content = f.read()

    name_without_ext = re.sub(r"\.md$", "", filename)

    # Verifica se já linkado
    if (
        f"({filename})" in content
        or f"[[{name_without_ext}]]" in content
        or f"[[{filename}]]" in content
    ):
        return

    link = f"- [{name_without_ext}]({filename})\n"
    with open(index_path, "a", encoding="utf-8", newline="\n") as f:
        f.write(link)


def note_files(cwd: str = None) -> list:
    """
    Retorna todos os arquivos .md em vault/notes/ exceto index.md.
    Retorna [] se o diretório não existir.
    """
    base = cwd or os.getcwd()
    vault_dir = os.path.join(base, VAULT_DIR)
    if not os.path.isdir(vault_dir):
        return []
    return [
        f
        for f in os.listdir(vault_dir)
        if f.endswith(".md") and f != "index.md"
    ]


def index_contains(filename: str, cwd: str = None) -> bool:
    """
    Retorna True se o index.md referencia filename.
    """
    base = cwd or os.getcwd()
    index_path = os.path.join(base, INDEX_FILE)
    if not os.path.exists(index_path):
        return False
    with open(index_path, "r", encoding="utf-8") as f:
        content = f.read()
    name_without_ext = re.sub(r"\.md$", "", filename)
    return (
        f"({filename})" in content
        or f"[[{name_without_ext}]]" in content
        or f"[[{filename}]]" in content
    )
