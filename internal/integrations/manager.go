package integrations

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
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
}

type Manager struct {
	ProjectRoot string
	HomeDir     string
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
	desired := make(map[string]string)
	for _, item := range resolved {
		hash := contentHash(item.plan.Content)
		if prior, ok := desired[item.destination]; ok && prior != hash && operation != mutationUninstall {
			return fmt.Errorf("conflicting content planned for %q", item.destination)
		}
		desired[item.destination] = hash
		if err := preflight(item, manifests[item.manifest], force, operation); err != nil {
			return err
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
	for _, item := range resolved {
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

	for _, item := range resolved {
		manifest := manifests[item.manifest]
		if err := applyMutation(item, &manifest, force, operation); err != nil {
			return err
		}
		manifests[item.manifest] = manifest
	}
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
	committed = true
	return nil
}

func preflight(item resolvedPlan, manifest Manifest, force bool, operation mutation) error {
	inspection, err := inspectResolved(item.plan, item.destination, manifest)
	if err != nil {
		return err
	}
	owned := claimOwned(manifest.Artifacts[item.destination], item.plan.Claim)
	switch operation {
	case mutationInstall:
		if inspection.State == StateModified && !force {
			return fmt.Errorf("artifact %q is modified; use force to replace it", item.destination)
		}
		if inspection.State == StateOutdated && owned && !force {
			return fmt.Errorf("artifact %q is outdated; use update", item.destination)
		}
	case mutationUpdate:
		if inspection.State == StateModified && !force {
			return fmt.Errorf("artifact %q is modified; use force to update it", item.destination)
		}
	case mutationUninstall:
		if !owned {
			return nil
		}
		if inspection.State == StateModified && !force {
			return fmt.Errorf("artifact %q is modified; use force to remove it", item.destination)
		}
	}
	return nil
}

func applyMutation(item resolvedPlan, manifest *Manifest, force bool, operation mutation) error {
	entry, hasEntry := manifest.Artifacts[item.destination]
	owned := hasEntry && claimOwned(entry, item.plan.Claim)
	desiredHash := contentHash(item.plan.Content)

	if operation == mutationUninstall {
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
		delete(manifest.Artifacts, item.destination)
		return nil
	}

	data, err := os.ReadFile(item.destination)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	actualHash := contentHash(data)
	knownLegacy := hashIn(actualHash, item.plan.LegacyHashes)

	writeDesired := !exists
	if exists && !owned {
		if actualHash != desiredHash && !knownLegacy && !force {
			return fmt.Errorf("unmanaged artifact %q does not match a trackfw template", item.destination)
		}
		writeDesired = operation == mutationUpdate && actualHash != desiredHash || force && actualHash != desiredHash
	} else if exists && owned {
		writeDesired = actualHash != desiredHash
	}
	if writeDesired {
		if err := atomicWrite(item.destination, item.plan.Content, 0o600); err != nil {
			return fmt.Errorf("write managed artifact %q: %w", item.destination, err)
		}
		actualHash = desiredHash
	}

	if !hasEntry {
		entry = ManifestArtifact{Destination: item.destination}
	}
	entry.Claims = appendClaim(entry.Claims, item.plan.Claim)
	entry.Hash = actualHash
	if actualHash == desiredHash {
		entry.CatalogVersion = item.plan.CatalogVersion
	} else {
		entry.CatalogVersion = "legacy"
	}
	manifest.Artifacts[item.destination] = entry
	return nil
}

func inspectResolved(plan PlannedArtifact, destination string, manifest Manifest) (Inspection, error) {
	result := Inspection{Claim: plan.Claim, Destination: destination, SupportLevel: plan.SupportLevel}
	entry, managed := manifest.Artifacts[destination]
	result.Managed = managed && claimOwned(entry, plan.Claim)
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
		if filepath.Clean(destination) != destination || destination == "." || strings.HasPrefix(destination, ".."+string(filepath.Separator)) {
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
