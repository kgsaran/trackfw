from pathlib import Path

import pytest


@pytest.mark.xfail(
    strict=True,
    reason="ML-1A: site ainda afirma que trackfw validate consome JSON Schema automaticamente.",
)
def test_site_does_not_claim_validate_consumes_json_schema_automatically():
    root = Path(__file__).resolve().parents[2]
    pages = [
        root / "site" / "guide" / "ai-agents.md",
        root / "site" / "en" / "guide" / "ai-agents.md",
    ]
    forbidden = [
        "O frontmatter é validado contra JSON Schemas em `docs/schema/`",
        "Frontmatter is validated against JSON Schemas in `docs/schema/`",
    ]

    offenders = []
    for page in pages:
        text = page.read_text(encoding="utf-8")
        for phrase in forbidden:
            if phrase in text:
                offenders.append(f"{page.relative_to(root)}: {phrase}")

    assert not offenders, "Contrato documental falso sobre JSON Schema:\n" + "\n".join(offenders)
