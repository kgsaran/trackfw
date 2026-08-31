package validator

import "runtime"

// CurrentGOOS is the platform string used by the mode checks in this package.
// It is seeded from runtime.GOOS at init and exists so tests can exercise the
// Windows guard on any host:
//
//	validator.CurrentGOOS = "windows"
//	defer func() { validator.CurrentGOOS = runtime.GOOS }()
//
// Mirrors generators.CurrentGOOS (internal/generators/scaffold_doctor.go), which
// established the same seam for the scaffold artifact mode check.
//
// Why the mode checks need it: on Windows the POSIX execute bit is not
// representable on NTFS. os.Stat().Mode() reports -rw-rw-rw- for every regular
// file, and &0111 is 0 even immediately after os.Chmod(path, 0o755). A check
// written as "the script is not executable" is therefore ALWAYS true there, and
// no action the user takes can make it false — `trackfw update`, the remedy the
// message prescribes, regenerates the script with the same unrepresentable mode.
var CurrentGOOS = runtime.GOOS
