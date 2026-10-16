//go:build windows

package fsperm

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CreateRestrictedFile creates a new temporary file in dir whose name begins
// with the part of pattern before "*" and ends with the part after "*" (matching
// os.CreateTemp conventions). On NTFS volumes the file is born with a protected
// DACL that grants Full Control only to Owner, SYSTEM and Administrators — no
// Authenticated Users, Users or Everyone ACE. On volumes without ACL support
// (FAT, exFAT, SMB) the function silently falls back to os.CreateTemp and returns
// a valid *os.File with nil error (ADR D4 — silent fallback, declared limit).
//
// The race window between creation and DACL application that exists with
// os.CreateTemp + SetRestrictiveACL is eliminated here by construction:
// windows.CreateFile receives the SECURITY_ATTRIBUTES with the restrictive DACL
// already set, and the share mode is FILE_SHARE_READ (no concurrent write handle).
// The os.Rename that follows preserves the DACL because the temp and final files
// are in the same directory (same volume). (R-5)
func CreateRestrictedFile(dir, pattern string) (*os.File, error) {
	// Detect ACL support before taking any action. GetVolumeInformation needs a
	// root path that already exists, so we call MkdirAll's side-effect implicitly
	// by testing the directory directly. The caller ensures dir exists before us.
	aclOK, err := isACLSupported(dir)
	if err != nil || !aclOK {
		// Volume without ACL support (FAT/exFAT/SMB) — silent fallback.
		// write-containment-allowed: fsperm volume-without-ACL fallback; delegates to os.CreateTemp on FAT/exFAT/SMB
		return os.CreateTemp(dir, pattern)
	}

	// Build a restrictive SDDL: D:P = protected DACL; FA = File All / GENERIC_ALL.
	// Owner SID is obtained at runtime from the process token (R-3).
	ownerSID, err := currentOwnerSID()
	if err != nil {
		return nil, fmt.Errorf("fsperm: obtendo SID do proprietário: %w", err)
	}
	// File DACL: no OI/CI flags (applies only to this file, not propagated).
	sddl := "D:P(A;;FA;;;" + ownerSID.String() + ")(A;;FA;;;SY)(A;;FA;;;BA)"
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, fmt.Errorf("fsperm: construindo SDDL %q: %w", sddl, err)
	}
	sa := windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}

	// Split pattern the same way os.CreateTemp does: everything before the last
	// '*' is the prefix, everything after is the suffix. If there is no '*', the
	// whole pattern is the prefix.
	prefix, suffix, _ := strings.Cut(pattern, "*")

	for range 10000 {
		b := make([]byte, 5) // 10 hex chars, same as os.CreateTemp
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		name := filepath.Join(dir, prefix+hex.EncodeToString(b)+suffix)
		nameUTF16, err := windows.UTF16PtrFromString(name)
		if err != nil {
			return nil, err
		}
		h, err := windows.CreateFile(
			nameUTF16,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			windows.FILE_SHARE_READ,
			&sa,
			windows.CREATE_NEW,
			windows.FILE_ATTRIBUTE_NORMAL,
			0,
		)
		if err != nil {
			if errors.Is(err, windows.ERROR_FILE_EXISTS) {
				continue
			}
			// Check soft ACL-support errors: volume doesn't support security.
			errno := toErrno(err)
			if errno == windows.ERROR_NOT_SUPPORTED ||
				errno == windows.ERROR_INVALID_FUNCTION ||
				errno == syscall.Errno(0x546) { // ERROR_NO_SECURITY_ON_OBJECT
				// write-containment-allowed: fsperm volume-without-ACL fallback; delegates to os.CreateTemp on FAT/exFAT/SMB
				return os.CreateTemp(dir, pattern)
			}
			return nil, fmt.Errorf("fsperm: criando arquivo restrito %q: %w", name, err)
		}
		return os.NewFile(uintptr(h), name), nil
	}
	return nil, fmt.Errorf("fsperm: não foi possível criar arquivo temporário único em %q após 10000 tentativas", dir)
}

// SetRestrictiveDirACL applies a protected DACL to a directory, granting Full
// Control only to Owner, SYSTEM and Administrators, with OI|CI flags so that
// new children inherit the protection. This re-protects a .trackfw directory
// even when it pre-existed (ADR D3 ajuste).
//
// Soft-fail conditions: ERROR_NOT_SUPPORTED (50), ERROR_INVALID_FUNCTION (1),
// ERROR_NO_SECURITY_ON_OBJECT (0x546) — these indicate a volume that does not
// support ACLs. All other errors propagate (fail-closed, R-4).
func SetRestrictiveDirACL(path string) error {
	ownerSID, err := currentOwnerSID()
	if err != nil {
		return fmt.Errorf("fsperm: SetRestrictiveDirACL: obtendo SID do proprietário: %w", err)
	}
	// Directory DACL: OI|CI = ObjectInherit|ContainerInherit, so new files and
	// subdirectories inherit protection. D:P = protected (SE_DACL_PROTECTED, R-4).
	sddl := "D:P(A;OICI;FA;;;" + ownerSID.String() + ")(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)"
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return fmt.Errorf("fsperm: SetRestrictiveDirACL: construindo SDDL: %w", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("fsperm: SetRestrictiveDirACL: extraindo DACL: %w", err)
	}
	err = windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil,
	)
	if err != nil {
		errno := toErrno(err)
		if errno == windows.ERROR_NOT_SUPPORTED ||
			errno == windows.ERROR_INVALID_FUNCTION ||
			errno == syscall.Errno(0x546) {
			return nil // soft-fail for non-ACL volumes
		}
		return fmt.Errorf("fsperm: SetRestrictiveDirACL %q: %w", path, err)
	}
	return nil
}

// currentOwnerSID returns the SID of the current process owner by reading the
// process access token. Fail-closed: if the token cannot be read, an error is
// returned and callers must not substitute a static fallback SID (R-3).
func currentOwnerSID() (*windows.SID, error) {
	tok, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return nil, fmt.Errorf("abrindo token do processo: %w", err)
	}
	defer tok.Close()
	tu, err := tok.GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("GetTokenUser: %w", err)
	}
	// Copy the SID so it outlives the token handle.
	dup, err := tu.User.Sid.Copy()
	if err != nil {
		return nil, fmt.Errorf("copiando SID: %w", err)
	}
	return dup, nil
}

// isACLSupported returns true when the volume hosting path supports persistent
// ACLs (FILE_PERSISTENT_ACLS flag). FAT/exFAT/SMB without ACLs return false.
func isACLSupported(path string) (bool, error) {
	// Resolve to volume root via GetVolumePathName.
	abs, err := filepath.Abs(path)
	if err != nil {
		return false, err
	}
	absUTF16, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return false, err
	}
	volBuf := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(absUTF16, &volBuf[0], uint32(len(volBuf))); err != nil {
		return false, err
	}
	volRoot := windows.UTF16ToString(volBuf)
	volRootUTF16, err := windows.UTF16PtrFromString(volRoot)
	if err != nil {
		return false, err
	}
	var flags uint32
	if err := windows.GetVolumeInformation(volRootUTF16, nil, 0, nil, nil, &flags, nil, 0); err != nil {
		return false, err
	}
	return flags&windows.FILE_PERSISTENT_ACLS != 0, nil
}

// toErrno unwraps err down to a syscall.Errno, returning 0 if none found.
func toErrno(err error) syscall.Errno {
	var e syscall.Errno
	if errors.As(err, &e) {
		return e
	}
	return 0
}
