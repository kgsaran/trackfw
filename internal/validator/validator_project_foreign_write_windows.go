//go:build windows

package validator

// validateProjectForeignWriteWindows is the "project_foreign_write_windows" rule
// (D2/D3, ADR-2026-10-16-no-windows-o-validate-avisa-quando-o-projeto-concede-escrita-a-outro-usuario).
//
// Always-warning; routed via applyRuleWarnOnly — never blockable.
//
// Checks the project root and, if it exists, .trackfw\ for ACEs that grant
// write-equivalent rights to a SID outside the trusted set:
//   - the current process user (from OpenCurrentProcessToken),
//   - SYSTEM (S-1-5-18),
//   - Administrators (S-1-5-32-544),
//   - CREATOR OWNER (S-1-3-0),
//   - OWNER RIGHTS (S-1-3-4).
//
// An ACE is counted if it is an ACCESS_ALLOWED_ACE that:
//   - applies to the object itself (INHERIT_ONLY_ACE flag absent), OR
//   - is inheritable to files (OBJECT_INHERIT_ACE flag present).
//
// D3: any error reading a DACL is emitted as a warning (fail-visible, not silent).
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
			"project_foreign_write_windows: cannot determine project root: %v — "+
				"move the project to your user profile or remove inheritance",
			err,
		)}
	}
	return checkProjectForeignWrite(root)
}

// checkProjectForeignWrite is the injectable inner function used by tests.
// It checks root and, if present, root\.trackfw.
func checkProjectForeignWrite(root string) []string {
	// Resolve the current user SID from the process token.
	ownerSID, ownerErr := currentUserSID()

	var msgs []string

	// Check root directory.
	if warn := checkDirForeignWrite(root, root, ownerSID, ownerErr); warn != "" {
		msgs = append(msgs, warn)
	}

	// Check .trackfw\ if it exists; a missing .trackfw is silent per D2.
	trackfwDir := filepath.Join(root, ".trackfw")
	if _, statErr := os.Lstat(trackfwDir); statErr == nil {
		if warn := checkDirForeignWrite(trackfwDir, root, ownerSID, ownerErr); warn != "" {
			msgs = append(msgs, warn)
		}
	}

	return msgs
}

// checkDirForeignWrite checks a single directory path for foreign write ACEs.
// root is the project root (used only in the remediation message).
// ownerSID and ownerErr come from currentUserSID(); ownerErr is propagated as a
// D3 warning if present.
func checkDirForeignWrite(dirPath, root string, ownerSID *windows.SID, ownerErr error) string {
	if ownerErr != nil {
		return fmt.Sprintf(
			"project_foreign_write_windows: %q: cannot determine current user SID: %v — "+
				"move the project inside your user profile or remove inheritance",
			dirPath, ownerErr,
		)
	}

	// D3: error reading the DACL is a warning, never silence.
	sd, err := windows.GetNamedSecurityInfo(dirPath, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Sprintf(
			"project_foreign_write_windows: %q: cannot read DACL: %v — "+
				"move the project inside your user profile or remove inheritance",
			dirPath, err,
		)
	}

	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Sprintf(
			"project_foreign_write_windows: %q: cannot parse DACL: %v — "+
				"move the project inside your user profile or remove inheritance",
			dirPath, err,
		)
	}

	if dacl == nil {
		// NULL DACL = everyone has full control.
		return fmt.Sprintf(
			"project_foreign_write_windows: %q has a NULL DACL (everyone full control) — "+
				"move the project inside your user profile or remove inheritance",
			dirPath,
		)
	}

	var foreignSIDs []string

	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			break
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			continue
		}

		flags := uint32(ace.Header.AceFlags)
		// An ACE counts if it applies to the object itself (INHERIT_ONLY absent)
		// or is inheritable to files (OBJECT_INHERIT set).
		appliesToObject := (flags & windows.INHERIT_ONLY_ACE) == 0
		inheritableToFile := (flags & windows.OBJECT_INHERIT_ACE) != 0
		if !appliesToObject && !inheritableToFile {
			continue
		}

		if ace.Mask&d2WriteMask == 0 {
			continue
		}

		aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if isTrustedSID(aceSID, ownerSID) {
			continue
		}

		foreignSIDs = append(foreignSIDs, sidLabel(aceSID))
	}

	if len(foreignSIDs) == 0 {
		return ""
	}

	return fmt.Sprintf(
		"project_foreign_write_windows: %q grants write access to %s — "+
			"move the project inside your user profile or remove inheritance from %q",
		dirPath,
		strings.Join(foreignSIDs, ", "),
		root,
	)
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
