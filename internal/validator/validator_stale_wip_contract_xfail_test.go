package validator

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kgsaran/trackfw/internal/config"
)

// ML-1A — REQ-2026-07-27-debitos-tecnicos-pos-release:
// testes negativos strict para o contrato documentado de stale_wip.

func TestStaleWIPUsesTransitionLogEntryIntoWIP(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req", "docs/adr")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-old-wip.md",
		"---\nstatus: wip\n---\n# Roadmap\nREQ: docs/req/REQ-001.md\n## Acceptance Criteria\n- [ ] ok\n")
	writeFile(t, dir, "docs/roadmaps/.trackfw-log",
		"2026-07-10 10:00  ROADMAP-old-wip.md                                backlog → wip\n")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\nreq_dir: docs/req\nadr_dirs:\n  - docs/adr\n")

	recent := time.Now()
	if err := os.Chtimes(filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-old-wip.md"), recent, recent); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	staleWIPNow = func() time.Time {
		return time.Date(2026, 7, 27, 12, 0, 0, 0, time.Local)
	}
	t.Cleanup(func() { staleWIPNow = time.Now })

	warnings, err := validateStaleWIP()
	if err != nil {
		t.Fatalf("validateStaleWIP erro: %v", err)
	}
	if !hasWarning(warnings, "ROADMAP-old-wip.md") {
		t.Fatalf("esperava stale_wip pela entrada antiga do .trackfw-log; warnings=%v", warnings)
	}
}

func TestStaleWIPUsesLatestTransitionIntoWIPBoundary(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req", "docs/adr")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-boundary.md",
		"---\nstatus: wip\n---\n# Roadmap\nREQ: docs/req/REQ-001.md\n## Acceptance Criteria\n- [ ] ok\n")
	writeFile(t, dir, "docs/roadmaps/.trackfw-log",
		"2026-07-01 10:00  ROADMAP-boundary.md                               backlog → wip\n"+
			"2026-07-26 10:01  ROADMAP-boundary.md                               blocked → wip\n")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\nstale_wip_days: 2\n")
	old := time.Date(2026, 6, 1, 10, 0, 0, 0, time.Local)
	if err := os.Chtimes(filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-boundary.md"), old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	staleWIPNow = func() time.Time {
		return time.Date(2026, 7, 28, 10, 1, 0, 0, time.Local)
	}
	t.Cleanup(func() { staleWIPNow = time.Now })

	warnings, err := validateStaleWIP()
	if err != nil {
		t.Fatalf("validateStaleWIP erro: %v", err)
	}
	if !hasWarning(warnings, "ROADMAP-boundary.md") {
		t.Fatalf("boundary de 2 dias deveria gerar warning; warnings=%v", warnings)
	}
	staleWIPNow = func() time.Time {
		return time.Date(2026, 7, 28, 10, 0, 59, 0, time.Local)
	}
	warnings, err = validateStaleWIP()
	if err != nil {
		t.Fatalf("validateStaleWIP erro: %v", err)
	}
	if hasWarning(warnings, "ROADMAP-boundary.md") {
		t.Fatalf("abaixo do boundary não deveria gerar warning; warnings=%v", warnings)
	}
}

func TestStaleWIPFallsBackToMTimeWhenLogMissing(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps/wip", "docs/req", "docs/adr")
	roadmap := filepath.Join(dir, "docs/roadmaps/wip/ROADMAP-mtime.md")
	writeFile(t, dir, "docs/roadmaps/wip/ROADMAP-mtime.md",
		"---\nstatus: wip\n---\n# Roadmap\nREQ: docs/req/REQ-001.md\n## Acceptance Criteria\n- [ ] ok\n")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\nstale_wip_days: 3\n")
	mtime := time.Date(2026, 7, 20, 9, 0, 0, 0, time.Local)
	if err := os.Chtimes(roadmap, mtime, mtime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)
	staleWIPNow = func() time.Time {
		return time.Date(2026, 7, 24, 9, 0, 0, 0, time.Local)
	}
	t.Cleanup(func() { staleWIPNow = time.Now })

	warnings, err := validateStaleWIP()
	if err != nil {
		t.Fatalf("validateStaleWIP erro: %v", err)
	}
	if !hasWarning(warnings, "ROADMAP-mtime.md") {
		t.Fatalf("fallback por mtime deveria gerar warning; warnings=%v", warnings)
	}
}

func TestStaleWIPReportsWIPWalkError(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "docs/roadmaps", "docs/req", "docs/adr")
	writeFile(t, dir, "docs/roadmaps/wip", "not a directory\n")
	writeFile(t, dir, "trackfw.yaml", "roadmap_dir: docs/roadmaps\nreq_dir: docs/req\nadr_dirs:\n  - docs/adr\n")
	chdir(t, dir)
	config.Reset()
	t.Cleanup(config.Reset)

	warnings, err := validateStaleWIP()
	if err != nil {
		t.Fatalf("validateStaleWIP erro: %v", err)
	}
	if !hasWarning(warnings, "wip") {
		t.Fatalf("esperava diagnostico para erro de walk/ENOTDIR em wip/; warnings=%v", warnings)
	}
}
