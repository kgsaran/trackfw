package integrations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestClaimOrigin_LegacyManifestReadsAsCatalog is the retrocompatibility
// test for ADR-2026-08-15 D11 (Claim.Origin): a manifest written before
// this field existed has NO "origin" key at all in its claim JSON — not
// even an empty string. It must decode to the zero value and be treated
// exactly as a catalog claim: Managed stays true and the state stays
// StateCurrent after `update`, never StateModified. A regression here
// would mean D11 broke every project's existing manifest the moment they
// upgraded trackfw, which is the one thing D11 was designed to avoid.
func TestClaimOrigin_LegacyManifestReadsAsCatalog(t *testing.T) {
	manager, project, _ := testManager(t)
	destination := filepath.Join(project, ".claude/agents/trackfw-backend.md")
	content := []byte("legacy content\n")

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, content, 0o644); err != nil {
		t.Fatal(err)
	}

	// Hand-authored, exactly as trackfw wrote manifests before D11 —
	// literally no "origin" key anywhere in the claim object.
	legacyManifest := `{
  "schema_version": 1,
  "artifacts": {
    ` + jsonQuoteForTest(destination) + `: {
      "destination": ` + jsonQuoteForTest(destination) + `,
      "sha256": "` + contentHash(content) + `",
      "catalog_version": "v1",
      "claims": [
        {"target": "claude", "surface": "code", "scope": "project", "kind": "agents", "item": "backend"}
      ]
    }
  }
}
`
	manifestFile := manifestPath(project)
	if err := os.MkdirAll(filepath.Dir(manifestFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestFile, []byte(legacyManifest), 0o644); err != nil {
		t.Fatal(err)
	}

	// A freshly built plan for the SAME catalog claim never sets Origin
	// (it is the zero value "") — exactly like every plan built before
	// D11 existed.
	plan := PlannedArtifact{
		Claim:          Claim{Target: "claude", Surface: "code", Scope: "project", Kind: KindAgents, Item: "backend"},
		Destination:    destination,
		Content:        content,
		CatalogVersion: "v1",
		SupportLevel:   "native",
	}

	inspection, err := manager.Inspect(plan)
	if err != nil {
		t.Fatal(err)
	}
	if !inspection.Managed {
		t.Fatal("expected legacy (pre-origin) claim to be recognized as Managed")
	}
	if inspection.State != StateCurrent {
		t.Fatalf("expected StateCurrent for an unmodified legacy artifact, got %q", inspection.State)
	}

	// `trackfw agents update` (Update with force=false) must be a no-op
	// here — no drift, no StateModified — the regression D11 must not
	// introduce.
	if err := manager.Update([]PlannedArtifact{plan}, false); err != nil {
		t.Fatalf("update on unmodified legacy artifact must succeed, got: %v", err)
	}
	inspection, err = manager.Inspect(plan)
	if err != nil {
		t.Fatal(err)
	}
	if inspection.State != StateCurrent {
		t.Fatalf("expected StateCurrent after update, got %q", inspection.State)
	}
}

func jsonQuoteForTest(s string) string {
	// destination is built with filepath.Join, so on Windows it carries
	// native '\' separators; a raw `"` + s + `"` concatenation (the
	// previous body of this helper) produced invalid JSON escapes like
	// `\U`, `\A`, `\T` there — manifestPath's json.Unmarshal would then
	// fail, and the legacy-manifest read path (D11) treats an unreadable
	// manifest as fail-open/absent, silently defeating this
	// retrocompatibility test on Windows. json.Marshal escapes '\' per
	// spec, matching how the product actually serializes claims.
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
