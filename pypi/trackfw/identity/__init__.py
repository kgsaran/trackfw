"""Customizable agent identity — parity port of internal/identity (Go).

Persisted at ``<home_dir>/.trackfw/identity.json``. All three trackfw CLIs
(Go, Node.js, Python) read and write this exact file/schema so a user who
configures their identity with one CLI gets identical results from the
others (ADR-2026-07-25-identidade-personalizavel-de-agentes).
"""

from __future__ import annotations

import json
import os
import re
import tempfile
import unicodedata
from dataclasses import dataclass, field
from typing import Any

_SCHEMA_VERSION = 1
_MAX_SLUG_LENGTH = 40
_SLUG_PATTERN = re.compile(r"^[a-z0-9]+(-[a-z0-9]+)*$")


class IdentityError(ValueError):
    """Raised for any identity-related failure (load/save/validate/slugify).

    Subclasses ValueError so callers that already catch
    ``(IntegrationError, OSError, ValueError)`` (see integrations/command.py)
    abort cleanly instead of leaking a traceback.
    """


@dataclass
class AgentIdentity:
    display_name: str
    slug: str


@dataclass
class Config:
    schema_version: int = _SCHEMA_VERSION
    user_nickname: str = ""
    agents: dict[str, AgentIdentity] = field(default_factory=dict)


def _identity_path(home_dir: str | os.PathLike[str]) -> str:
    return os.path.join(str(home_dir), ".trackfw", "identity.json")


def load(home_dir: str | os.PathLike[str]) -> Config:
    """Load the identity config from <home_dir>/.trackfw/identity.json.

    If the file does not exist, returns an empty Config — this is the
    non-regression path and must never surface as a failure to callers that
    have not yet customized their identity.
    """
    filename = _identity_path(home_dir)
    try:
        with open(filename, "r", encoding="utf-8") as stream:
            raw = stream.read()
    except FileNotFoundError:
        return Config()
    except OSError as error:
        raise IdentityError(f"identity: falha ao ler {filename}: {error}") from error

    try:
        data = json.loads(raw)
    except json.JSONDecodeError as error:
        raise IdentityError(f"identity: falha ao decodificar {filename}: {error}") from error

    schema_version = data.get("schema_version")
    if schema_version != _SCHEMA_VERSION:
        raise IdentityError(
            f"identity: versao de schema nao suportada em {filename}: "
            f"{schema_version!r} (esperado {_SCHEMA_VERSION})"
        )

    agents: dict[str, AgentIdentity] = {}
    for agent_id, entry in (data.get("agents") or {}).items():
        agents[agent_id] = AgentIdentity(
            display_name=entry.get("display_name", ""),
            slug=entry.get("slug", ""),
        )

    return Config(
        schema_version=schema_version,
        user_nickname=data.get("user_nickname", ""),
        agents=agents,
    )


def _atomic_write(filename: str, content: bytes, mode: int) -> None:
    """Writes content to filename via a temp file in the same directory
    followed by os.replace, so a reader never observes a partially
    written file. Shared shape with
    pypi/trackfw/thirdparty/quarantine.py's _atomic_write and
    pypi/trackfw/integrations/manager.py's
    IntegrationManager._atomic_write (replicated here rather than
    imported, to keep trackfw.identity independent of
    trackfw.integrations and trackfw.thirdparty — same rationale as the
    one documented in quarantine.py's own _atomic_write docstring)."""
    directory = os.path.dirname(filename)
    os.makedirs(directory, exist_ok=True, mode=0o700)
    descriptor, temporary = tempfile.mkstemp(prefix=".trackfw-tmp-", dir=directory)
    try:
        fchmod = getattr(os, "fchmod", None)
        if fchmod is not None:
            fchmod(descriptor, mode)
        else:
            # os.fchmod is Unix-only (CPython docs: "Availability: Unix").
            # On platforms without it (Windows), fall back to chmod on the
            # temp file's own path. This reopens a narrow TOCTOU window
            # that fchmod(fd) does not have, but only on platforms where
            # fchmod never existed to begin with — os.fchmod continues to
            # be used unconditionally wherever it is available.
            os.chmod(temporary, mode)
        with os.fdopen(descriptor, "wb") as stream:
            stream.write(content)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(temporary, filename)
    except BaseException:
        try:
            os.close(descriptor)
        except OSError:
            pass
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


def save(home_dir: str | os.PathLike[str], cfg: Config) -> None:
    """Persist cfg to <home_dir>/.trackfw/identity.json atomically.

    Mirrors Go's struct field order (schema_version, user_nickname, agents)
    and encoding/json's behavior of sorting map keys — so the top-level key
    order is fixed but the agents map keys are sorted.
    """
    payload: dict[str, Any] = {"schema_version": _SCHEMA_VERSION}
    if cfg.user_nickname:
        payload["user_nickname"] = cfg.user_nickname
    if cfg.agents:
        payload["agents"] = {
            agent_id: {"display_name": agent.display_name, "slug": agent.slug}
            for agent_id, agent in sorted(cfg.agents.items())
        }

    content = (json.dumps(payload, indent=2, ensure_ascii=False) + "\n").encode("utf-8")
    filename = _identity_path(home_dir)
    try:
        _atomic_write(filename, content, 0o600)
    except OSError as error:
        raise IdentityError(f"identity: falha ao gravar {filename}: {error}") from error


# ---------------------------------------------------------------------------
# Slugify
# ---------------------------------------------------------------------------


def slugify(value: str) -> str:
    """Convert value into a normalized slug matching ^[a-z0-9]+(-[a-z0-9]+)*$.

    Steps, in exact order (mirrors internal/identity/slug.go):
      1. Unicode NFD normalization + diacritics (category Mn) removal.
      2. Lowercase.
      3. Spaces and underscores become '-'.
      4. Any character outside [a-z0-9-] is dropped.
      5. Runs of '-' collapse into a single '-'.
      6. Leading/trailing '-' are trimmed.
      7. Empty result is an error.
      8. Result longer than 40 characters is an error.

    Never silently "fixes" degenerate input — always raises IdentityError
    instead of guessing.
    """
    folded = "".join(
        char for char in unicodedata.normalize("NFD", value) if unicodedata.category(char) != "Mn"
    )
    lowered = folded.lower()
    replaced = "".join("-" if char in (" ", "_") else char for char in lowered)
    filtered = "".join(char for char in replaced if ("a" <= char <= "z") or ("0" <= char <= "9") or char == "-")

    collapsed = re.sub(r"-{2,}", "-", filtered)
    trimmed = collapsed.strip("-")

    if trimmed == "":
        raise IdentityError(f"identity: slug vazio para {value!r}")
    if len(trimmed) > _MAX_SLUG_LENGTH:
        raise IdentityError(
            f"identity: slug {trimmed!r} excede o tamanho maximo de "
            f"{_MAX_SLUG_LENGTH} caracteres (tem {len(trimmed)})"
        )
    return trimmed


def agent_name(slug: str) -> str:
    """Return the display name suffixed with '-tf'. The only place the
    '-tf' suffix is applied to a slug."""
    return f"{slug}-tf"


def validate(cfg: Config, known_ids: list[str]) -> None:
    """Check cfg for structural and referential integrity:
    - every agent id in cfg.agents must be present in known_ids
    - display_name must not be empty
    - slug must match ^[a-z0-9]+(-[a-z0-9]+)*$
    - slugs must be unique across agents
    - slug must not end with the "-tf" suffix (it is appended automatically
      by agent_name)
    """
    known = set(known_ids)
    seen_slugs: dict[str, str] = {}

    for agent_id, agent in cfg.agents.items():
        if agent_id not in known:
            raise IdentityError(
                f"identity: agente desconhecido {agent_id!r} nao esta na lista de agentes conhecidos"
            )
        if not agent.display_name:
            raise IdentityError(f"identity: display_name vazio para o agente {agent_id!r}")
        if not _SLUG_PATTERN.match(agent.slug):
            raise IdentityError(
                f"identity: slug invalido {agent.slug!r} para o agente {agent_id!r} "
                f"(esperado padrao {_SLUG_PATTERN.pattern})"
            )
        if agent.slug in seen_slugs:
            raise IdentityError(
                f"identity: slug duplicado {agent.slug!r} entre os agentes "
                f"{seen_slugs[agent.slug]!r} e {agent_id!r}"
            )
        if agent.slug.endswith("-tf"):
            raise IdentityError(
                f"identity: slug {agent.slug!r} do agente {agent_id!r} nao deve incluir "
                f'o sufixo "-tf"; ele e acrescentado automaticamente '
                f"(use {agent.slug[:-3]!r} em vez de {agent.slug!r})"
            )
        seen_slugs[agent.slug] = agent_id


def lookup(cfg: Config, agent_id: str) -> AgentIdentity | None:
    """Return the identity configured for agent_id, if any."""
    return cfg.agents.get(agent_id)


# ---------------------------------------------------------------------------
# Known agent ids
# ---------------------------------------------------------------------------


def known_agent_ids() -> list[str]:
    """Canonical list of agent ids known to trackfw, in stable order."""
    return [
        "architect",
        "backend",
        "frontend",
        "qa",
        "infra",
        "security",
        "dba",
        "ux",
        "code-quality",
        "data",
        "iac",
        "tooling",
    ]


# ---------------------------------------------------------------------------
# Presets — HARDCODED tables (not derived via slugify at runtime), matching
# internal/identity/preset.go. Hardcoding avoids depending on Unicode
# normalization behaving identically across the Go, Node.js and Python CLIs.
# ---------------------------------------------------------------------------

_PRESETS: dict[str, dict[str, tuple[str, str]]] = {
    "greek": {
        "architect": ("Zeus", "zeus"),
        "backend": ("Apolo", "apolo"),
        "frontend": ("Afrodite", "afrodite"),
        "qa": ("Ártemis", "artemis"),
        "infra": ("Ares", "ares"),
        "security": ("Hades", "hades"),
        "dba": ("Poseidon", "poseidon"),
        "ux": ("Atena", "atena"),
        "code-quality": ("Hefesto", "hefesto"),
        "data": ("Métis", "metis"),
        "iac": ("Dédalo", "dedalo"),
        "tooling": ("Prometeu", "prometeu"),
    },
    "norse": {
        "architect": ("Odin", "odin"),
        "backend": ("Thor", "thor"),
        "frontend": ("Freya", "freya"),
        "qa": ("Heimdall", "heimdall"),
        "infra": ("Tyr", "tyr"),
        "security": ("Vidar", "vidar"),
        "dba": ("Njord", "njord"),
        "ux": ("Idun", "idun"),
        "code-quality": ("Bragi", "bragi"),
        "data": ("Mimir", "mimir"),
        "iac": ("Ivaldi", "ivaldi"),
        "tooling": ("Loki", "loki"),
    },
    "potter": {
        "architect": ("Dumbledore", "dumbledore"),
        "backend": ("Snape", "snape"),
        "frontend": ("Luna", "luna"),
        "qa": ("Moody", "moody"),
        "infra": ("Hagrid", "hagrid"),
        "security": ("Kingsley", "kingsley"),
        "dba": ("Flitwick", "flitwick"),
        "ux": ("Tonks", "tonks"),
        "code-quality": ("Hermione", "hermione"),
        "data": ("Trelawney", "trelawney"),
        "iac": ("Rowena", "rowena"),
        "tooling": ("Ollivander", "ollivander"),
    },
    "thrones": {
        "architect": ("Tyrion", "tyrion"),
        "backend": ("Jon", "jon"),
        "frontend": ("Sansa", "sansa"),
        "qa": ("Arya", "arya"),
        "infra": ("Brienne", "brienne"),
        "security": ("Varys", "varys"),
        "dba": ("Samwell", "samwell"),
        "ux": ("Margaery", "margaery"),
        "code-quality": ("Stannis", "stannis"),
        "data": ("Bran", "bran"),
        "iac": ("Gendry", "gendry"),
        "tooling": ("Qyburn", "qyburn"),
    },
    "chaves": {
        "architect": ("Girafales", "girafales"),
        "backend": ("Madruga", "madruga"),
        "frontend": ("Chiquinha", "chiquinha"),
        "qa": ("Florinda", "florinda"),
        "infra": ("Barriga", "barriga"),
        "security": ("Quico", "quico"),
        "dba": ("Clotilde", "clotilde"),
        "ux": ("Popis", "popis"),
        "code-quality": ("Nhonho", "nhonho"),
        "data": ("Godinez", "godinez"),
        "iac": ("Chaves", "chaves"),
        "tooling": ("Chapolin", "chapolin"),
    },
    "pioneers": {
        "architect": ("Turing", "turing"),
        "backend": ("Ritchie", "ritchie"),
        "frontend": ("Berners-Lee", "berners-lee"),
        "qa": ("Hamilton", "hamilton"),
        "infra": ("Torvalds", "torvalds"),
        "security": ("Diffie", "diffie"),
        "dba": ("Codd", "codd"),
        "ux": ("Norman", "norman"),
        "code-quality": ("Knuth", "knuth"),
        "data": ("Hopper", "hopper"),
        "iac": ("Hashimoto", "hashimoto"),
        "tooling": ("McCarthy", "mccarthy"),
    },
    "starwars": {
        "architect": ("Yoda", "yoda"),
        "backend": ("Han", "han"),
        "frontend": ("Leia", "leia"),
        "qa": ("Ahsoka", "ahsoka"),
        "infra": ("Chewbacca", "chewbacca"),
        "security": ("Vader", "vader"),
        "dba": ("R2-D2", "r2-d2"),
        "ux": ("Padmé", "padme"),
        "code-quality": ("Obi-Wan", "obi-wan"),
        "data": ("C-3PO", "c-3po"),
        "iac": ("Rey", "rey"),
        "tooling": ("Babu Frik", "babu-frik"),
    },
    "tolkien": {
        "architect": ("Gandalf", "gandalf"),
        "backend": ("Aragorn", "aragorn"),
        "frontend": ("Arwen", "arwen"),
        "qa": ("Legolas", "legolas"),
        "infra": ("Gimli", "gimli"),
        "security": ("Boromir", "boromir"),
        "dba": ("Elrond", "elrond"),
        "ux": ("Galadriel", "galadriel"),
        "code-quality": ("Faramir", "faramir"),
        "data": ("Bilbo", "bilbo"),
        "iac": ("Aulë", "aule"),
        "tooling": ("Celebrimbor", "celebrimbor"),
    },
    "turma": {
        "architect": ("Franjinha", "franjinha"),
        "backend": ("Cebolinha", "cebolinha"),
        "frontend": ("Magali", "magali"),
        "qa": ("Mônica", "monica"),
        "infra": ("Cascão", "cascao"),
        "security": ("Bidu", "bidu"),
        "dba": ("Marocas", "marocas"),
        "ux": ("Anjinho", "anjinho"),
        "code-quality": ("Titi", "titi"),
        "data": ("Chico", "chico"),
        "iac": ("Piteco", "piteco"),
        "tooling": ("Nimbus", "nimbus"),
    },
    "egyptian": {
        "architect": ("Thoth", "thoth"),
        "backend": ("Rá", "ra"),
        "frontend": ("Ísis", "isis"),
        "qa": ("Hórus", "horus"),
        "infra": ("Ptah", "ptah"),
        "security": ("Anúbis", "anubis"),
        "dba": ("Seshat", "seshat"),
        "ux": ("Bastet", "bastet"),
        "code-quality": ("Maat", "maat"),
        "data": ("Osíris", "osiris"),
        "iac": ("Imhotep", "imhotep"),
        "tooling": ("Khnum", "khnum"),
    },
}

_PRESET_ORDER = [
    "greek",
    "norse",
    "potter",
    "thrones",
    "chaves",
    "pioneers",
    "starwars",
    "tolkien",
    "turma",
    "egyptian",
]

assert set(_PRESET_ORDER) == set(_PRESETS), "identity: presetOrder e presets fora de sincronia"


def preset(name: str) -> Config:
    """Return the identity Config for the given themed preset name.

    Returns a fresh Config on every call: mutating it does not affect the
    module-level preset table or any other Config returned previously.
    """
    table = _PRESETS.get(name)
    if table is None:
        raise IdentityError(
            f"identity: preset desconhecido {name!r} (validos: {', '.join(preset_names())})"
        )
    agents = {
        agent_id: AgentIdentity(display_name=display_name, slug=slug)
        for agent_id, (display_name, slug) in table.items()
    }
    return Config(schema_version=_SCHEMA_VERSION, agents=agents)


def preset_names() -> list[str]:
    """Names of all known presets, in stable order."""
    return list(_PRESET_ORDER)
