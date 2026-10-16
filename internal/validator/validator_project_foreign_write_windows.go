//go:build windows

package validator

// validateProjectForeignWriteWindows is the "project_foreign_write_windows" rule
// (D2/D3, ADR-2026-10-16-no-windows-o-validate-avisa-quando-o-projeto-concede-escrita-a-outro-usuario).
//
// Always-warning; routed via applyRuleWarnOnly — never blockable.
//
// Checks the project root and, if it exists and is not a reparse point, .trackfw\
// for ACEs that grant write-equivalent rights to a SID outside the trusted set:
//   - the current process user (from OpenCurrentProcessToken),
//   - SYSTEM (S-1-5-18),
//   - Administrators (S-1-5-32-544),
//   - CREATOR OWNER (S-1-3-0),
//   - OWNER RIGHTS (S-1-3-4).
//
// An ACE is counted if it is an ACCESS_ALLOWED_ACE that (R-1):
//   - applies to the object itself (INHERIT_ONLY_ACE flag absent), OR
//   - is inheritable to files (OBJECT_INHERIT_ACE flag present), OR
//   - is inheritable to containers (CONTAINER_INHERIT_ACE flag present).
//
// Remedy message distinguishes inherited ACEs (INHERITED_ACE flag) from explicit ACEs (R-5):
//   - explicit: icacls "<dir>" /remove:g *<SID>
//   - inherited: move project inside user profile or icacls "<dir>" /inheritance:d && /remove:g
//
// D3: any error reading or parsing the DACL is emitted as a warning (fail-visible, not silent).
// R-2: GetAce failures emit a D3 warning and continue (no silent break).
// R-3: non-IsNotExist Lstat errors for .trackfw emit a D3 warning. Reparse points
//
//	(junctions/symlinks) are warned about and the DACL scan is skipped — GetNamedSecurityInfo
//	by name follows the reparse point to its target, which is outside the project's control.
//
// Outside Windows this file is replaced by the no-op in validator_project_foreign_write_other.go.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// d2WriteMask is the union of all rights from D2 of ADR-2026-10-16-no-windows-o-validate-avisa-...
//
// Bit values (hex):
//
//	FILE_WRITE_DATA    / FILE_ADD_FILE          = 0x00000002
//	FILE_APPEND_DATA   / FILE_ADD_SUBDIRECTORY  = 0x00000004
//	fileDeleteChild                              = 0x00000040  (not in x/sys — defined below)
//	DELETE                                       = 0x00010000
//	WRITE_DAC                                    = 0x00040000
//	WRITE_OWNER                                  = 0x00080000
//	GENERIC_WRITE                                = 0x40000000
//	GENERIC_ALL                                  = 0x10000000
const (
	// fileDeleteChild is 0x40. Not exported by golang.org/x/sys/windows.
	fileDeleteChild windows.ACCESS_MASK = 0x00000040

	// d2WriteMask is the union of write-equivalent rights from D2 of
	// ADR-2026-10-16-no-windows-o-validate-avisa-quando-o-projeto-concede-escrita-a-outro-usuario.
	d2WriteMask windows.ACCESS_MASK = windows.FILE_WRITE_DATA |
		windows.FILE_APPEND_DATA |
		fileDeleteChild |
		windows.DELETE |
		windows.WRITE_DAC |
		windows.WRITE_OWNER |
		windows.GENERIC_WRITE |
		windows.GENERIC_ALL
)

// staticTrustedSIDs lists the well-known SIDs that are always trusted (D2).
// Parsed once; parse errors are silently skipped (running on a non-standard
// Windows installation would be a separate problem).
var staticTrustedSIDs []*windows.SID

func init() {
	for _, s := range []string{
		"S-1-5-18",     // SYSTEM
		"S-1-5-32-544", // Administrators
		"S-1-3-0",      // CREATOR OWNER
		"S-1-3-4",      // OWNER RIGHTS
	} {
		if sid, err := windows.StringToSid(s); err == nil {
			staticTrustedSIDs = append(staticTrustedSIDs, sid)
		}
	}
}

// validateProjectForeignWriteWindows is called by the validator via the
// public wrapper registered in validator.go.
func validateProjectForeignWriteWindows() []string {
	root, err := os.Getwd()
	if err != nil {
		return []string{fmt.Sprintf(
			"project_foreign_write_windows: cannot determine project root: %v",
			err,
		)}
	}
	return checkProjectForeignWrite(root)
}

// checkProjectForeignWrite is the injectable inner function used by tests.
// It checks root and, if present and not a reparse point, root\.trackfw.
func checkProjectForeignWrite(root string) []string {
	// Resolve the current user SID from the process token.
	ownerSID, ownerErr := currentUserSID()

	var msgs []string

	// Check root directory.
	msgs = append(msgs, checkDirForeignWrite(root, ownerSID, ownerErr)...)

	// Check .trackfw\ if it exists; a missing .trackfw is silent per D2.
	trackfwDir := filepath.Join(root, ".trackfw")
	if _, statErr := os.Lstat(trackfwDir); statErr != nil {
		if !os.IsNotExist(statErr) {
			// R-3: access-denied or other non-existence error is a D3 warning.
			msgs = append(msgs, fmt.Sprintf(
				"project_foreign_write_windows: cannot stat %q: %v — DACL check skipped",
				trackfwDir, statErr,
			))
		}
		// IsNotExist: .trackfw doesn't exist, silent per D2.
	} else {
		// Path exists. Use GetFileAttributes to detect reparse points (junctions, symlinks).
		// GetNamedSecurityInfo by path follows reparse points to their target;
		// the target is outside the project's control, so its ACL proves nothing
		// about the project's integrity. Warn and skip the DACL scan. (R-3)
		ptr, ptrErr := windows.UTF16PtrFromString(trackfwDir)
		if ptrErr != nil {
			msgs = append(msgs, fmt.Sprintf(
				"project_foreign_write_windows: cannot encode path %q: %v — DACL check skipped",
				trackfwDir, ptrErr,
			))
		} else {
			attrs, attrErr := windows.GetFileAttributes(ptr)
			if attrErr != nil {
				msgs = append(msgs, fmt.Sprintf(
					"project_foreign_write_windows: cannot get attributes of %q: %v — DACL check skipped",
					trackfwDir, attrErr,
				))
			} else if attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
				msgs = append(msgs, fmt.Sprintf(
					"project_foreign_write_windows: %q is a reparse point (junction or symlink); "+
						"DACL scan skipped — target is outside the project; verify target separately",
					trackfwDir,
				))
			} else {
				msgs = append(msgs, checkDirForeignWrite(trackfwDir, ownerSID, ownerErr)...)
			}
		}
	}

	return msgs
}

// checkDirForeignWrite checks a single directory path for foreign write ACEs.
// ownerSID and ownerErr come from currentUserSID(); ownerErr is propagated as a
// D3 warning if present.
// Returns a slice of warning strings (empty when no foreign ACE is found).
func checkDirForeignWrite(dirPath string, ownerSID *windows.SID, ownerErr error) []string {
	if ownerErr != nil {
		return []string{fmt.Sprintf(
			"project_foreign_write_windows: %q: cannot determine current user SID: %v",
			dirPath, ownerErr,
		)}
	}

	// D3: error reading the DACL is a warning, never silence.
	sd, err := windows.GetNamedSecurityInfo(dirPath, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return []string{fmt.Sprintf(
			"project_foreign_write_windows: %q: cannot read DACL: %v",
			dirPath, err,
		)}
	}

	dacl, _, err := sd.DACL()
	if err != nil {
		return []string{fmt.Sprintf(
			"project_foreign_write_windows: %q: cannot parse DACL: %v",
			dirPath, err,
		)}
	}

	if dacl == nil {
		// NULL DACL = everyone has full control.
		return []string{fmt.Sprintf(
			"project_foreign_write_windows: %q has a NULL DACL (everyone full control) — "+
				"move the project inside your user profile or set an explicit restrictive DACL",
			dirPath,
		)}
	}

	var msgs []string
	var explicitLabels, explicitSIDs []string
	var inheritedLabels, inheritedSIDs []string

	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			// R-2: D3 violation to break silently; emit warning and continue scanning.
			msgs = append(msgs, fmt.Sprintf(
				"project_foreign_write_windows: %q: GetAce[%d] failed: %v — DACL scan may be incomplete",
				dirPath, i, err,
			))
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}

		flags := uint32(ace.Header.AceFlags)
		// R-1: An ACE counts if it applies to the object itself (IO absent),
		// is inheritable to files (OI set), or is inheritable to containers (CI set).
		// The original rule omitted CI, allowing an attacker with only CI|IO rights
		// to pre-create .trackfw (FILE_ADD_SUBDIRECTORY via container inheritance).
		appliesToObject := (flags & windows.INHERIT_ONLY_ACE) == 0
		inheritableToFile := (flags & windows.OBJECT_INHERIT_ACE) != 0
		inheritableToContainer := (flags & windows.CONTAINER_INHERIT_ACE) != 0
		if !appliesToObject && !inheritableToFile && !inheritableToContainer {
			continue
		}

		if ace.Mask&d2WriteMask == 0 {
			continue
		}

		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if isTrustedSID(aceSID, ownerSID) {
			continue
		}

		// R-5: distinguish inherited ACEs (INHERITED_ACE flag) from explicit ACEs.
		// Inherited ACEs require disabling inheritance before removing; explicit ACEs
		// can be removed directly.
		label := sidLabel(aceSID)
		sidStr := aceSID.String()
		if flags&windows.INHERITED_ACE != 0 {
			inheritedLabels = append(inheritedLabels, label)
			inheritedSIDs = append(inheritedSIDs, sidStr)
		} else {
			explicitLabels = append(explicitLabels, label)
			explicitSIDs = append(explicitSIDs, sidStr)
		}
	}

	// R-5: build separate messages with accurate remediation per ACE type.
	// icacls requires a '*' prefix for SID strings (otherwise treats as account name).
	// Paths are passed as plain strings inside "..." (not %q — %q doubles backslashes).
	if len(explicitLabels) > 0 {
		sidArgs := make([]string, len(explicitSIDs))
		for i, s := range explicitSIDs {
			sidArgs[i] = "*" + s
		}
		msgs = append(msgs, fmt.Sprintf(
			"project_foreign_write_windows: %q grants write access to %s (explicit ACE) — "+
				"run: icacls \"%s\" /remove:g %s",
			dirPath,
			strings.Join(explicitLabels, ", "),
			dirPath,
			strings.Join(sidArgs, " "),
		))
	}
	if len(inheritedLabels) > 0 {
		sidArgs := make([]string, len(inheritedSIDs))
		for i, s := range inheritedSIDs {
			sidArgs[i] = "*" + s
		}
		msgs = append(msgs, fmt.Sprintf(
			"project_foreign_write_windows: %q grants write access to %s (inherited ACE) — "+
				"move the project inside your user profile or run: "+
				"icacls \"%s\" /inheritance:d && icacls \"%s\" /remove:g %s",
			dirPath,
			strings.Join(inheritedLabels, ", "),
			dirPath, dirPath,
			strings.Join(sidArgs, " "),
		))
	}

	return msgs
}

// isTrustedSID returns true if sid is in the trusted set (process owner + static list).
func isTrustedSID(sid *windows.SID, ownerSID *windows.SID) bool {
	if ownerSID != nil && sid.Equals(ownerSID) {
		return true
	}
	for _, trusted := range staticTrustedSIDs {
		if sid.Equals(trusted) {
			return true
		}
	}
	return false
}

// sidLabel returns the account name "DOMAIN\name (SID)" or just "SID" on
// LookupAccountSid failure. Localized names are fine — tests assert only the SID.
func sidLabel(sid *windows.SID) string {
	account, domain, _, err := sid.LookupAccount("")
	if err != nil || (account == "" && domain == "") {
		return sid.String()
	}
	if domain != "" {
		return fmt.Sprintf(`%s\%s (%s)`, domain, account, sid.String())
	}
	return fmt.Sprintf(`%s (%s)`, account, sid.String())
}

// currentUserSID returns the SID of the current process token user.
func currentUserSID() (*windows.SID, error) {
	tok, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return nil, fmt.Errorf("OpenCurrentProcessToken: %w", err)
	}
	defer tok.Close()
	tu, err := tok.GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("GetTokenUser: %w", err)
	}
	return tu.User.Sid.Copy()
}
