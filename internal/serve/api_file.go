package serve

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
)

// fileHandler handles GET /api/file?path=<relative-path>.
// Returns the raw file content as text/plain.
//
// Security model — two-stage containment:
//
//  1. Lexical containment (no filesystem access): filepath.Clean + filepath.Join
//     resolves ".." sequences; the result must start with one of the configured
//     allowed roots. Paths outside the roots receive 403 before the filesystem is
//     touched, preventing 403 vs 404 from leaking existence of arbitrary system paths.
//
//  2. Physical containment (symlink resolution): filepath.EvalSymlinks is called on
//     both the requested path and each allowed root.  A symlink whose name lives
//     inside an allowed root but whose physical target does not is rejected with 403.
//     Any canonicalization failure (ENOENT, dangling symlink, permission error, …)
//     is mapped uniformly to 404 to avoid leaking path structure.
//
// A legitimate symlink whose target also lives inside an allowed root is served normally.
func fileHandler(w http.ResponseWriter, r *http.Request, cfg config.ProjectConfig) {
	setCORSHeaders(w)

	rawPath := r.URL.Query().Get("path")
	if rawPath == "" {
		http.Error(w, "missing path parameter", http.StatusBadRequest)
		return
	}

	// Resolve relative to the current working directory.
	workDir, err := os.Getwd()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// filepath.Clean eliminates any ".." sequences; filepath.Join absorbs absolute
	// path components so a caller cannot override workDir with an absolute rawPath.
	cleanedPath := filepath.Clean(rawPath)
	absPath := filepath.Join(workDir, cleanedPath)

	// ── Stage 1: Lexical containment ──────────────────────────────────────────
	// No filesystem operations.  Rejects obvious traversal or out-of-root paths
	// before touching the disk, preventing existence-oracle side-channels.
	lexicalAllowedDirs := buildAllowedDirs(workDir, cfg)
	if !filePathAllowed(absPath, lexicalAllowedDirs) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// ── Stage 2: Physical containment ─────────────────────────────────────────
	// Canonicalize both the allowed roots and the requested path by resolving
	// symlinks.  This defeats symlink-based escape: a symlink placed inside an
	// allowed root can point to an arbitrary physical target.

	// Canonicalize workDir (resolves /var → /private/var on macOS, etc.).
	realWorkDir, err := filepath.EvalSymlinks(workDir)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	physicalAllowedDirs := buildRealAllowedDirs(realWorkDir, cfg)

	// Canonicalize the requested file.  Any error — ENOENT, dangling symlink,
	// symlink loop, permission denied — is returned as 404.
	// Rationale: the lexical check already passed, so the nominal path is inside an
	// allowed root.  Leaking "exists vs forbidden" for paths inside the root is
	// acceptable (the board API already enumerates those).  Leaking it for paths
	// outside would be an oracle — prevented by the lexical check above.
	realAbsPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	// Physical containment: the canonical target must also be inside a canonical root.
	if !filePathAllowed(realAbsPath, physicalAllowedDirs) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data, err := os.ReadFile(realAbsPath)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}

// buildAllowedDirs returns the lexical (non-canonicalized) absolute allowed
// directories derived from workDir and the project configuration.
func buildAllowedDirs(workDir string, cfg config.ProjectConfig) []string {
	var dirs []string
	for _, d := range cfg.ADRDirs {
		dirs = append(dirs, filepath.Join(workDir, filepath.Clean(d)))
	}
	dirs = append(dirs, filepath.Join(workDir, filepath.Clean(cfg.REQDir)))
	dirs = append(dirs, filepath.Join(workDir, filepath.Clean(cfg.RoadmapDir)))
	return dirs
}

// buildRealAllowedDirs returns the canonicalized (symlinks resolved) absolute
// allowed directories.  Directories that do not exist are silently skipped;
// a file cannot reside in a non-existent directory.
func buildRealAllowedDirs(realWorkDir string, cfg config.ProjectConfig) []string {
	var dirs []string
	for _, d := range cfg.ADRDirs {
		if rd, err := filepath.EvalSymlinks(filepath.Join(realWorkDir, filepath.Clean(d))); err == nil {
			dirs = append(dirs, rd)
		}
	}
	if rd, err := filepath.EvalSymlinks(filepath.Join(realWorkDir, filepath.Clean(cfg.REQDir))); err == nil {
		dirs = append(dirs, rd)
	}
	if rd, err := filepath.EvalSymlinks(filepath.Join(realWorkDir, filepath.Clean(cfg.RoadmapDir))); err == nil {
		dirs = append(dirs, rd)
	}
	return dirs
}

// filePathAllowed reports whether absPath is inside one of the allowed directories.
// Uses a trailing separator to prevent /docs/adr from prefix-matching /docs/adr2.
func filePathAllowed(absPath string, allowedDirs []string) bool {
	for _, dir := range allowedDirs {
		prefix := dir
		if !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		if strings.HasPrefix(absPath, prefix) || absPath == dir {
			return true
		}
	}
	return false
}
