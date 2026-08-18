package integrations

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

type LifecycleState string

const (
	StateNotInstalled LifecycleState = "not-installed"
	StateCurrent      LifecycleState = "current"
	StateOutdated     LifecycleState = "outdated"
	StateModified     LifecycleState = "modified"
)

type PlannedArtifact struct {
	Claim          Claim
	Destination    string
	Content        []byte
	CatalogVersion string
	SupportLevel   string
	LegacyHashes   []string
}

type Inspection struct {
	Claim        Claim          `json:"claim"`
	Destination  string         `json:"destination"`
	State        LifecycleState `json:"state"`
	SupportLevel string         `json:"support_level"`
	Managed      bool           `json:"managed"`
	// Registered reports whether the manifest has ANY entry for this
	// destination, regardless of claim ownership — unlike Managed, which
	// additionally requires this exact claim to own that entry (see
	// claimOwned). doctor (ML-2A) needs this distinction: a destination
	// registered under a *different* claim must never be reported as an
	// "unregistered write" — that would be the dominant false-positive the
	// command exists to avoid. Additive field; existing JSON consumers
	// (list --json) build their own output struct and are unaffected.
	Registered bool `json:"registered"`
}

type Manager struct {
	ProjectRoot string
	HomeDir     string

	// OnSkip is called once per artifact skipped during Install when the
	// artifact is outdated+owned and --force is not set. destination is the
	// tilde-abbreviated path; reason is the full warning line ready to print
	// to stderr. Nil → no-op. Called in the order of resolved items; never
	// called twice for the same destination (deduped inside mutate).
	OnSkip func(destination, reason string)
}

// tildeAbbrev converts an absolute destination path to a tilde-abbreviated
// display path.
//
// There is no pre-existing Go helper for this in the codebase: update.go
// uses hardcoded "~/.…" string constants, and integrations.GlobalGroupPath
// derives tilde paths by truncating catalog *template* strings — neither can
// abbreviate an arbitrary resolved absolute path. This helper is introduced
// for ML-2A; the Node.js equivalent is tildeify in
// npm/src/lib/update-engine.js (read for byte-parity reference).
//
// Both operands are run through filepath.Clean before comparison, which
// strips any redundant separators (including the ML-6H double-slash that
// arises when $HOME or a test tempdir already ends with a path separator).
// filepath.Clean never adds a trailing separator to a non-root path, so the
// "startsWith(normalizedHome + sep)" check is safe without a separate
// trailing-sep strip.
func (m Manager) tildeAbbrev(destination string) string {
	cleanDest := filepath.Clean(destination)
	if m.HomeDir != "" {
		cleanHome := filepath.Clean(m.HomeDir)
		if cleanDest == cleanHome {
			return "~"
		}
		if strings.HasPrefix(cleanDest, cleanHome+string(filepath.Separator)) {
			return "~" + cleanDest[len(cleanHome):]
		}
	}
	if m.ProjectRoot != "" {
		cleanRoot := filepath.Clean(m.ProjectRoot)
		if strings.HasPrefix(cleanDest, cleanRoot+string(filepath.Separator)) {
			return cleanDest[len(cleanRoot)+1:]
		}
	}
	return destination
}

func (m Manager) Inspect(plan PlannedArtifact) (Inspection, error) {
	resolved, manifestFile, err := m.resolve(plan)
	if err != nil {
		return Inspection{}, err
	}
	manifest, err := loadManifest(manifestFile)
	if err != nil {
		return Inspection{}, err
	}
	return inspectResolved(plan, resolved, manifest)
}

func (m Manager) List(plans []PlannedArtifact) ([]Inspection, error) {
	result := make([]Inspection, 0, len(plans))
	for _, plan := range plans {
		inspection, err := m.Inspect(plan)
		if err != nil {
			return nil, err
		}
		result = append(result, inspection)
	}
	return result, nil
}

func (m Manager) Install(plans []PlannedArtifact, force bool) error {
	return m.mutate(plans, force, mutationInstall)
}

func (m Manager) Update(plans []PlannedArtifact, force bool) error {
	return m.mutate(plans, force, mutationUpdate)
}

func (m Manager) Uninstall(plans []PlannedArtifact, force bool) error {
	return m.mutate(plans, force, mutationUninstall)
}

type mutation int

const (
	mutationInstall mutation = iota
	mutationUpdate
	mutationUninstall
)

type resolvedPlan struct {
	plan        PlannedArtifact
	destination string
	manifest    string
}

type fileSnapshot struct {
	exists bool
	data   []byte
	mode   os.FileMode
}

// pendingWrite is a byte write to an artifact destination computed by
// planArtifactWrite but not yet applied to disk. It is executed only after
// the manifest carrying its (optimistic) target Hash/CatalogVersion has
// already been persisted — see the ADR-2026-08-18 ordering in mutate.
type pendingWrite struct {
	destination string
	content     []byte
	mode        os.FileMode
}

// afterManifestPersist, when non-nil, runs immediately after manifests are
// persisted and before artifact bytes are written during install/update
// (never during uninstall, which is intentionally not inverted — see the
// comment in mutate). It exists only so tests can simulate an interruption
// exactly at the ADR-2026-08-18-ordering seam (manifest declares the target
// state, no artifact bytes have moved yet) without goroutine trickery.
// Production code never assigns it.
var afterManifestPersist func()

func (m Manager) mutate(plans []PlannedArtifact, force bool, operation mutation) (retErr error) {
	resolved := make([]resolvedPlan, 0, len(plans))
	manifests := make(map[string]Manifest)
	for _, plan := range plans {
		destination, manifestFile, err := m.resolve(plan)
		if err != nil {
			return err
		}
		if _, ok := manifests[manifestFile]; !ok {
			manifest, err := loadManifest(manifestFile)
			if err != nil {
				return err
			}
			manifests[manifestFile] = manifest
		}
		resolved = append(resolved, resolvedPlan{plan: plan, destination: destination, manifest: manifestFile})
	}

	// Preflight every operation before touching disk. This also catches duplicate
	// destinations with incompatible desired content.
	//
	// Items that should be skipped (outdated+owned Install without --force) are
	// collected in skippedDests for dedup, separated into `active`, and handled
	// via OnSkip. They are excluded from the snapshot and applyMutation phases so
	// that (a) their bytes are never overwritten and (b) their manifest entry is
	// not updated to the new desired hash.
	desired := make(map[string]string)
	active := make([]resolvedPlan, 0, len(resolved))
	skippedDests := make(map[string]struct{})
	for _, item := range resolved {
		hash := contentHash(item.plan.Content)
		if prior, ok := desired[item.destination]; ok && prior != hash && operation != mutationUninstall {
			return fmt.Errorf("conflicting content planned for %q", item.destination)
		}
		desired[item.destination] = hash
		skip, err := preflight(item, manifests[item.manifest], force, operation)
		if err != nil {
			return err
		}
		if skip {
			// Fire OnSkip once per destination (contract: "nunca duas vezes para
			// o mesmo destino"). Filtering by destination rather than by item
			// preserves the adoption path for a second claim that is not yet owned.
			if _, already := skippedDests[item.destination]; !already {
				skippedDests[item.destination] = struct{}{}
				if m.OnSkip != nil {
					abbrev := m.tildeAbbrev(item.destination)
					remediation := "trackfw update"
					if item.plan.Claim.Scope == "global" {
						remediation = "trackfw update harness"
					}
					reason := fmt.Sprintf("warning: skipping outdated artifact %s; run '%s' to refresh it", abbrev, remediation)
					m.OnSkip(abbrev, reason)
				}
			}
		} else {
			active = append(active, item)
		}
	}

	snapshots := make(map[string]fileSnapshot)
	remember := func(filename string) error {
		if _, ok := snapshots[filename]; ok {
			return nil
		}
		info, err := os.Lstat(filename)
		if os.IsNotExist(err) {
			snapshots[filename] = fileSnapshot{}
			return nil
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-regular file %q", filename)
		}
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		snapshots[filename] = fileSnapshot{exists: true, data: data, mode: info.Mode().Perm()}
		return nil
	}
	for _, item := range active {
		if err := remember(item.destination); err != nil {
			return err
		}
	}
	for filename := range manifests {
		if err := remember(filename); err != nil {
			return err
		}
	}

	committed := false
	defer func() {
		if committed || retErr == nil {
			return
		}
		for filename, snapshot := range snapshots {
			if snapshot.exists {
				_ = atomicWrite(filename, snapshot.data, snapshot.mode)
			} else {
				_ = os.Remove(filename)
			}
		}
	}()

	// ADR-2026-08-18: install/update persist the manifest before writing
	// artifact bytes (self-healing direction on interruption); uninstall
	// keeps the original artifacts-before-manifest order. The two are
	// deliberately NOT symmetric — see the comment below.
	if operation == mutationUninstall {
		// Uninstall is not inverted. Removing bytes first and persisting the
		// manifest last means an interruption leaves the manifest still
		// declaring an artifact whose file is now absent — inspectResolved
		// resolves that as StateNotInstalled, the same self-healing
		// direction install/update get from the inversion below. Inverting
		// uninstall the same way (drop the manifest entry, then remove
		// bytes) would instead leave, on interruption, a file on disk whose
		// content still matches the catalog template but with no manifest
		// entry at all — inspectResolved reports StateCurrent/managed=false,
		// an orphaned artifact that looks legitimate and that nothing
		// detects or repairs automatically. That is exactly the "disk ahead
		// of manifest" bad direction the ADR exists to eliminate, so
		// uninstall must not be simetrized with install/update here.
		for _, item := range active {
			manifest := manifests[item.manifest]
			if err := applyUninstall(item, &manifest); err != nil {
				return err
			}
			manifests[item.manifest] = manifest
		}
		if err := persistManifests(manifests); err != nil {
			return err
		}
	} else {
		// Install/Update: compute the manifest update for every active item
		// in memory (no bytes touched yet), persist all manifests, and only
		// then write the artifact bytes. Each write is already atomic
		// (atomicWrite); the window between the two phases is purely one of
		// ordering. If interrupted after the manifest is on disk but before
		// an artifact's bytes are written, the manifest already declares the
		// artifact's target state while the file is absent or stale —
		// inspectResolved resolves that to StateNotInstalled/StateModified,
		// which `install`/`update --install-missing` repairs on its own,
		// instead of the pre-ADR order's StateModified/`unmanaged` outcome
		// that required a human to run `install --force`.
		pendingWrites := make([]pendingWrite, 0, len(active))
		for _, item := range active {
			manifest := manifests[item.manifest]
			write, err := planArtifactWrite(item, &manifest, force, operation)
			if err != nil {
				return err
			}
			manifests[item.manifest] = manifest
			if write != nil {
				pendingWrites = append(pendingWrites, *write)
			}
		}
		if err := persistManifests(manifests); err != nil {
			return err
		}
		if afterManifestPersist != nil {
			afterManifestPersist()
		}
		for _, write := range pendingWrites {
			if err := atomicWrite(write.destination, write.content, write.mode); err != nil {
				return fmt.Errorf("write managed artifact %q: %w", write.destination, err)
			}
		}
	}
	committed = true
	return nil
}

// persistManifests writes every manifest in the batch, in deterministic
// (sorted) filename order.
func persistManifests(manifests map[string]Manifest) error {
	manifestFiles := make([]string, 0, len(manifests))
	for filename := range manifests {
		manifestFiles = append(manifestFiles, filename)
	}
	sort.Strings(manifestFiles)
	for _, filename := range manifestFiles {
		if err := writeManifest(filename, manifests[filename]); err != nil {
			return err
		}
	}
	return nil
}

// preflight validates the planned mutation for one artifact. It returns
// (skip=true, nil) when the artifact should be silently skipped (Install on
// an outdated+owned artifact without --force — bytes belong to a previous
// trackfw template, not to the user, so skipping is safe and non-destructive).
// It returns (false, err) for hard failures, and (false, nil) when the
// mutation may proceed normally.
//
// The Modified case in mutationInstall remains a hard error: bytes modified
// by the user must not be silently ignored. The two cases are intentionally
// asymmetric — do not simetrize them.
func preflight(item resolvedPlan, manifest Manifest, force bool, operation mutation) (skip bool, err error) {
	if operation != mutationUninstall {
		if err := detectNameCollision(item, force); err != nil {
			return false, err
		}
	}
	inspection, err := inspectResolved(item.plan, item.destination, manifest)
	if err != nil {
		return false, err
	}
	owned := claimOwned(manifest.Artifacts[item.destination], item.plan.Claim)
	switch operation {
	case mutationInstall:
		if inspection.State == StateModified && !force {
			return false, fmt.Errorf("artifact %q is modified; use force to replace it", item.destination)
		}
		if inspection.State == StateOutdated && owned && !force {
			// Skip: the artifact is a previous trackfw-owned template. Bytes are
			// not the user's, so skipping is safe. The caller is notified via
			// Manager.OnSkip and the rest of the batch continues.
			return true, nil
		}
	case mutationUpdate:
		// Force only authorizes replacing a modified artifact whose ownership is
		// already proven. Unknown unmanaged bytes must go through install --force;
		// update may adopt only the desired or a declared legacy template.
		if !owned && inspection.State == StateModified {
			return false, unmanagedArtifactError(item.destination, item.plan.Claim)
		}
		if inspection.State == StateModified && !force {
			return false, fmt.Errorf("artifact %q is modified; use force to update it", item.destination)
		}
	case mutationUninstall:
		if !owned {
			return false, nil
		}
		if inspection.State == StateModified && !force {
			return false, fmt.Errorf("artifact %q is modified; use force to remove it", item.destination)
		}
	}
	return false, nil
}

// detectNameCollision guards against two distinct managed agent artifacts
// declaring the same frontmatter "name" inside the same destination
// directory (ADR ADR-2026-07-25-identidade-personalizavel-de-agentes, seção
// D4). This matters because with customizable identities two different
// item.IDs could resolve to the same rendered name (e.g. two agents both
// pointed at slug "zeus"), and some surfaces key agent discovery off that
// name field.
//
// Limitation: the scan only inspects ".md" siblings, because that is the
// only format where this package has a cheap, dependency-free way
// (frontmatterName) to read the declared name back out of already-rendered
// bytes. JSON (cli-agent-json/agent-json) and TOML (custom-agent-toml)
// artifacts are not scanned for collisions — doing so would require a
// generic JSON/TOML parser here purely for this check, which is out of
// scope for this microlote.
func detectNameCollision(item resolvedPlan, force bool) error {
	if item.plan.Claim.Kind != KindAgents {
		return nil
	}
	if filepath.Ext(item.destination) != ".md" {
		return nil
	}
	desiredName, ok := frontmatterName(item.plan.Content)
	if !ok {
		return nil
	}
	directory := filepath.Dir(item.destination)
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("scan %q for name collisions: %w", directory, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		candidate := filepath.Join(directory, entry.Name())
		if candidate == item.destination {
			continue
		}
		data, err := os.ReadFile(candidate)
		if err != nil {
			continue
		}
		candidateName, ok := frontmatterName(data)
		if !ok || candidateName != desiredName {
			continue
		}
		if force {
			fmt.Fprintf(os.Stderr, "aviso: %q declara o mesmo name %q que %q; prosseguindo por --force\n", candidate, desiredName, item.destination)
			continue
		}
		return fmt.Errorf("artifact %q declares name %q which collides with existing file %q", item.destination, desiredName, candidate)
	}
	return nil
}

// applyUninstall removes ownership of one artifact from manifest, and — once
// no claim remains — the artifact's bytes and any empty ancestor directories
// this managed. It mutates disk directly (not deferred), because uninstall
// deliberately keeps the pre-ADR-2026-08-18 ordering: see the comment in
// mutate for why this is not simetrized with planArtifactWrite.
func applyUninstall(item resolvedPlan, manifest *Manifest) error {
	entry, hasEntry := manifest.Artifacts[item.destination]
	owned := hasEntry && claimOwned(entry, item.plan.Claim)
	if !owned {
		return nil
	}
	entry.Claims = removeClaim(entry.Claims, item.plan.Claim)
	if len(entry.Claims) != 0 {
		manifest.Artifacts[item.destination] = entry
		return nil
	}
	if err := os.Remove(item.destination); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove managed artifact %q: %w", item.destination, err)
	}
	root := filepath.Dir(filepath.Dir(item.manifest))
	if err := removeEmptyAncestors(filepath.Dir(item.destination), root); err != nil {
		return fmt.Errorf("clean managed artifact directories: %w", err)
	}
	delete(manifest.Artifacts, item.destination)
	return nil
}

// planArtifactWrite computes the manifest update for one install/update item
// entirely in memory — it never touches the artifact's bytes on disk. The
// caller (mutate) persists every manifest in the batch first, and only then
// applies the returned *pendingWrite (ADR-2026-08-18). It returns (nil, nil)
// when no byte write is needed.
//
// The manifest values it stores are deliberately *optimistic* when a write is
// pending: Hash/CatalogVersion describe the content this call is about to
// write, not what is currently on disk. That is the point of the inversion —
// if interrupted before the pending write lands, the manifest already
// declares the target state and inspectResolved resolves the (absent or
// stale) file to StateNotInstalled/StateModified, both self-repairable by a
// later install/update, never StateModified+unowned ("unmanaged").
func planArtifactWrite(item resolvedPlan, manifest *Manifest, force bool, operation mutation) (*pendingWrite, error) {
	entry, hasEntry := manifest.Artifacts[item.destination]
	owned := hasEntry && claimOwned(entry, item.plan.Claim)
	desiredHash := contentHash(item.plan.Content)

	data, err := os.ReadFile(item.destination)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	actualHash := contentHash(data)
	knownLegacy := hashIn(actualHash, item.plan.LegacyHashes)

	writeDesired := !exists
	if exists && !owned {
		if actualHash != desiredHash && !knownLegacy && !force {
			// Defense-in-depth: preflight already rejects this exact case for
			// mutationUpdate (unconditionally) and for mutationInstall without
			// --force (any State == StateModified is blocked before an active
			// item ever reaches planArtifactWrite). This branch is therefore
			// not reachable via Manager.Install/Update today, but it stays as
			// a second line of defense in case preflight's guard is ever
			// loosened — hence the identical remediation text, so a user who
			// somehow hits it still gets the same actionable message. No
			// manifest mutation happens on this path.
			return nil, unmanagedArtifactError(item.destination, item.plan.Claim)
		}
		writeDesired = operation == mutationUpdate && actualHash != desiredHash || force && actualHash != desiredHash
	} else if exists && owned {
		writeDesired = actualHash != desiredHash
	}

	if !hasEntry {
		entry = ManifestArtifact{Destination: item.destination}
	}
	entry.Claims = appendClaim(entry.Claims, item.plan.Claim)

	var write *pendingWrite
	if writeDesired {
		// Optimistic: bytes have not moved yet, but the manifest must already
		// describe the content we are about to write (see doc comment above).
		entry.Hash = desiredHash
		entry.CatalogVersion = item.plan.CatalogVersion
		write = &pendingWrite{destination: item.destination, content: item.plan.Content, mode: 0o644}
	} else {
		// No pending write: either the desired content already matches disk,
		// or a known-legacy artifact is being adopted without rewriting its
		// bytes (see TestManagerLegacyAdoptionAndUpdate). The manifest must
		// reflect the actual, already-on-disk hash.
		entry.Hash = actualHash
		if actualHash == desiredHash {
			entry.CatalogVersion = item.plan.CatalogVersion
		} else {
			entry.CatalogVersion = "legacy"
		}
	}
	manifest.Artifacts[item.destination] = entry
	return write, nil
}

// removeEmptyAncestors removes only empty real directories below root. It
// stops at the first non-empty directory and never removes or follows a
// symlink, including one introduced concurrently after path resolution.
func removeEmptyAncestors(directory, root string) error {
	for directory != root && beneath(root, directory) {
		info, err := os.Lstat(directory)
		if os.IsNotExist(err) {
			directory = filepath.Dir(directory)
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink directory %q", directory)
		}
		if !info.IsDir() {
			return nil
		}
		if err := os.Remove(directory); err != nil {
			if errors.Is(err, os.ErrExist) || isDirectoryNotEmpty(err) {
				return nil
			}
			return err
		}
		directory = filepath.Dir(directory)
	}
	return nil
}

func isDirectoryNotEmpty(err error) bool {
	// os.Remove returns a PathError whose platform-specific error string is
	// ENOTEMPTY on Unix and "directory not empty" on Windows.
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "directory not empty") || strings.Contains(message, "not empty")
}

func inspectResolved(plan PlannedArtifact, destination string, manifest Manifest) (Inspection, error) {
	result := Inspection{Claim: plan.Claim, Destination: destination, SupportLevel: plan.SupportLevel}
	entry, managed := manifest.Artifacts[destination]
	result.Managed = managed && claimOwned(entry, plan.Claim)
	result.Registered = managed
	data, err := os.ReadFile(destination)
	if os.IsNotExist(err) {
		result.State = StateNotInstalled
		return result, nil
	}
	if err != nil {
		return Inspection{}, fmt.Errorf("read artifact %q: %w", destination, err)
	}
	actual := contentHash(data)
	desired := contentHash(plan.Content)
	if managed {
		if actual != entry.Hash {
			result.State = StateModified
		} else if actual != desired || entry.CatalogVersion != plan.CatalogVersion {
			result.State = StateOutdated
		} else {
			result.State = StateCurrent
		}
		return result, nil
	}
	if actual == desired {
		result.State = StateCurrent
	} else if hashIn(actual, plan.LegacyHashes) {
		result.State = StateOutdated
	} else {
		result.State = StateModified
	}
	return result, nil
}

func (m Manager) resolve(plan PlannedArtifact) (string, string, error) {
	if strings.ContainsRune(plan.Destination, 0) {
		return "", "", errors.New("destination contains NUL")
	}
	if plan.Claim.Scope != "project" && plan.Claim.Scope != "global" {
		return "", "", fmt.Errorf("unsupported scope %q", plan.Claim.Scope)
	}
	root := m.ProjectRoot
	if plan.Claim.Scope == "global" {
		root = m.HomeDir
	}
	if root == "" {
		return "", "", fmt.Errorf("%s root is required", plan.Claim.Scope)
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	destination := plan.Destination
	if strings.HasPrefix(destination, "~/") {
		if plan.Claim.Scope != "global" {
			return "", "", errors.New("home destination requires global scope")
		}
		destination = filepath.Join(root, strings.TrimPrefix(destination, "~/"))
	} else if filepath.IsAbs(destination) {
		destination = filepath.Clean(destination)
	} else {
		if path.Clean(destination) != destination || destination == "." || strings.HasPrefix(destination, "../") {
			return "", "", fmt.Errorf("unsafe destination %q", plan.Destination)
		}
		destination = filepath.Join(root, destination)
	}
	if !beneath(root, destination) {
		return "", "", fmt.Errorf("destination %q is outside %s root", plan.Destination, plan.Claim.Scope)
	}
	if err := rejectSymlinks(root, destination); err != nil {
		return "", "", err
	}
	manifestFile := manifestPath(root)
	if err := rejectSymlinks(root, manifestFile); err != nil {
		return "", "", err
	}
	return destination, manifestFile, nil
}

func beneath(root, filename string) bool {
	relative, err := filepath.Rel(root, filename)
	return err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative)
}

func rejectSymlinks(root, filename string) error {
	current := filename
	for {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink path %q", current)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if current == root {
			return nil
		}
		parent := filepath.Dir(current)
		if parent == current || !beneath(root, current) {
			return fmt.Errorf("path %q escapes root", filename)
		}
		current = parent
	}
}

func atomicWrite(filename string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(filename)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".trackfw-tmp-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, filename)
}

// unmanagedArtifactError builds the error returned when update/uninstall (or,
// defensively, install) refuses to touch bytes trackfw did not write. The
// message names the remedy — trackfw did not write these bytes, so the only
// safe way to bring the artifact under management is `<kind> install
// --force`, which explicitly authorizes adopting/replacing unmanaged content
// — with the exact flags to reproduce this plan's claim (item, target,
// scope), so the user can copy-paste it instead of guessing.
//
// Mirrors npm/src/integrations/manager.js and pypi/trackfw/integrations/
// manager.py — the wording here is the canonical, byte-identical source of
// truth for the other two CLIs.
func unmanagedArtifactError(destination string, claim Claim) error {
	return fmt.Errorf(
		"unmanaged artifact %q does not match a trackfw template — trackfw did not write these bytes.\nAdopt it with: trackfw %s install --force --items %s --targets %s --scope %s",
		destination, claim.Kind, claim.Item, claim.Target, claim.Scope,
	)
}

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

func hashIn(hash string, hashes []string) bool {
	for _, candidate := range hashes {
		if strings.EqualFold(hash, candidate) {
			return true
		}
	}
	return false
}

func claimOwned(entry ManifestArtifact, claim Claim) bool {
	for _, current := range entry.Claims {
		if current == claim {
			return true
		}
	}
	return false
}

func appendClaim(claims []Claim, claim Claim) []Claim {
	for _, current := range claims {
		if current == claim {
			return claims
		}
	}
	return append(claims, claim)
}

func removeClaim(claims []Claim, claim Claim) []Claim {
	result := claims[:0]
	for _, current := range claims {
		if current != claim {
			result = append(result, current)
		}
	}
	return result
}
