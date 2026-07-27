from pathlib import Path


def test_site_does_not_claim_validate_consumes_json_schema_automatically():
    root = Path(__file__).resolve().parents[2]
    pages = [
        root / "site" / "guide" / "ai-agents.md",
        root / "site" / "en" / "guide" / "ai-agents.md",
        root / "docs" / "cli-parity.md",
    ]
    forbidden = [
        "O frontmatter é validado contra JSON Schemas em `docs/schema/`",
        "Frontmatter is validated against JSON Schemas in `docs/schema/`",
        "trackfw validate` valida frontmatter contra",
        "trackfw validate` validates frontmatter against",
        "consumes JSON Schema automatically",
        "consome JSON Schema automaticamente",
    ]

    offenders = []
    for page in pages:
        text = page.read_text(encoding="utf-8")
        for phrase in forbidden:
            if phrase in text:
                offenders.append(f"{page.relative_to(root)}: {phrase}")

    assert not offenders, "Contrato documental falso sobre JSON Schema:\n" + "\n".join(offenders)


def test_json_schema_contract_is_external_and_bilingual():
    root = Path(__file__).resolve().parents[2]

    pt = (root / "site" / "guide" / "ai-agents.md").read_text(encoding="utf-8")
    en = (root / "site" / "en" / "guide" / "ai-agents.md").read_text(encoding="utf-8")
    parity = (root / "docs" / "cli-parity.md").read_text(encoding="utf-8")

    assert "não são\nconsumidos automaticamente pelo `trackfw validate`" in pt
    assert "they are not\nconsumed automatically by `trackfw validate`" in en
    assert "do not load or\nexecute those JSON Schemas automatically" in parity

    for text in (pt, en):
        assert "/tmp/adr-frontmatter.json" in text
        assert "npx ajv validate -s docs/schema/adr.schema.json -d /tmp/adr-frontmatter.json" in text
        assert "python3 -m jsonschema -i /tmp/adr-frontmatter.json docs/schema/adr.schema.json" in text
