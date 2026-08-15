"""Third-party artifact gate: network fetch, marker-based rejection,
quarantine, provenance and catalog-agent reference composition, for the
two-phase third-party artifact install flow (skills/agents installed
from a URL). See:
docs/adr/ADR-2026-08-15-gate-de-duas-fases-para-artefatos-de-terceiro-quarentena-parecer-vinculado-por-checksum-e-deteccao-por-proveniencia-versionada.md
(D1-D10). Native Python port of internal/thirdparty (Go) and the D9/D5
extension points added to internal/integrations/render.go and plan.go.
"""

from __future__ import annotations

from .fetch import ThirdPartyFetchError, fetch
from .markers import LITERAL_MARKERS, check_markers, checksum, redact_url
from .provenance import (
    PROVENANCE_SCHEMA_VERSION,
    load_provenance,
    provenance_path,
    upsert_provenance_entry,
    verify_approval,
    write_provenance,
)
from .quarantine import (
    QUARANTINE_SCHEMA_VERSION,
    decode_content,
    new_quarantine_entry,
    quarantine_path,
    read_quarantine,
    write_quarantine,
)
from .references import (
    REFERENCES_SCHEMA_VERSION,
    THIRDPARTY_REF_END,
    THIRDPARTY_REF_START,
    apply_third_party_references,
    normalize_third_party_content,
    references_path,
    resolve_third_party_skill_destination,
    upsert_third_party_reference,
)

__all__ = [
    "ThirdPartyFetchError",
    "fetch",
    "LITERAL_MARKERS",
    "check_markers",
    "checksum",
    "redact_url",
    "PROVENANCE_SCHEMA_VERSION",
    "load_provenance",
    "provenance_path",
    "upsert_provenance_entry",
    "verify_approval",
    "write_provenance",
    "QUARANTINE_SCHEMA_VERSION",
    "decode_content",
    "new_quarantine_entry",
    "quarantine_path",
    "read_quarantine",
    "write_quarantine",
    "REFERENCES_SCHEMA_VERSION",
    "THIRDPARTY_REF_END",
    "THIRDPARTY_REF_START",
    "apply_third_party_references",
    "normalize_third_party_content",
    "references_path",
    "resolve_third_party_skill_destination",
    "upsert_third_party_reference",
]
