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

func TestStaleWIPUsesTransitionLogEntryIntoWIP_XFail(t *testing.T) {
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

	warnings, err := validateStaleWIP()
	if err != nil {
		t.Fatalf("validateStaleWIP erro: %v", err)
	}
	xfailExpect(t, "ML-2A", "stale_wip ainda usa git/mtime, não a entrada .trackfw-log backlog → wip", func() bool {
		return !hasWarning(warnings, "ROADMAP-old-wip.md")
	})
}

func TestStaleWIPReportsWIPWalkError_XFail(t *testing.T) {
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
	xfailExpect(t, "ML-2B", "stale_wip deve diagnosticar erro de walk/ENOTDIR em wip/ em vez de silenciar", func() bool {
		return !hasWarning(warnings, "wip")
	})
}
