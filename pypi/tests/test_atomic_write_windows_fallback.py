"""ML-1A of ROADMAP-2026-09-01-escrita-atomica-do-cli-python-funciona-no-windows:
os.fchmod is Unix-only (CPython docs: "Availability: Unix") and crashed the
three replicated _atomic_write implementations on Windows. The fix is a
*conditional* fallback (getattr(os, "fchmod", None)) — os.fchmod must keep
being used, unconditionally, wherever it exists.

Falsification, both directions, per REQ AC3 and the ML-0A correction:

- Direction (a): the three writers must not raise AttributeError regardless
  of whether os.fchmod exists on this platform, and the mode they were
  asked for must actually land on disk. `monkeypatch.delattr(..., raising=
  False)` is used (not True) precisely so these tests are not simulations
  on every platform: on POSIX it simulates absence exactly like before; on
  Windows the delattr is a no-op because os.fchmod is already absent, and
  the test exercises the real Windows fallback path natively. This is what
  makes these 4 tests actual CI evidence for AC1 on windows-full-suites,
  not something that has to be caveated as structurally unverifiable here.
- Direction (b), the symmetric one the REQ calls out as the one that
  worries most: the fallback must NOT fire on POSIX. A control asserts
  os.fchmod is still called (not bypassed) when it is present. This
  direction is inherently POSIX-only (there is nothing to spy on where
  os.fchmod does not exist), so it is gated on hasattr(os, "fchmod")
  rather than on os.name.

Per the ML-0A veredict, "os.fchmod was called" is a VACUOUS assertion on 5
of the 7 call sites, because tempfile.mkstemp() already hands back 0o600 by
default — identical to what 5 of the 7 sites request. The only site where
fchmod has an *observable* effect is IntegrationManager._atomic_write with
mode=0o644 (manager.py:343/:358, fed by the pending write at manager.py:585).
test_manager_0o644_fallback_produces_observable_mode below targets exactly
that site and asserts the resulting st_mode, not the call — st_mode
equality is itself gated to POSIX because NTFS only honors the write bit
(0o644 can legitimately read back as 0o666 there); the no-AttributeError /
file-was-written assertions stay unconditional on all platforms.
"""

from __future__ import annotations

import os
import stat

import pytest

from trackfw.identity import _atomic_write as identity_atomic_write
from trackfw.integrations.manager import IntegrationManager
from trackfw.thirdparty.quarantine import _atomic_write as quarantine_atomic_write

_HAS_FCHMOD = hasattr(os, "fchmod")


# ---------------------------------------------------------------------------
# Direction (a): os.fchmod absent (native on Windows, simulated on POSIX) ->
# no AttributeError, mode still applied.
# ---------------------------------------------------------------------------


def test_identity_atomic_write_survives_missing_fchmod(tmp_path, monkeypatch):
    monkeypatch.delattr(os, "fchmod", raising=False)
    target = tmp_path / "identity.json"
    identity_atomic_write(str(target), b"{}", 0o600)
    assert target.read_bytes() == b"{}"


def test_quarantine_atomic_write_survives_missing_fchmod(tmp_path, monkeypatch):
    monkeypatch.delattr(os, "fchmod", raising=False)
    target = tmp_path / "quarantine" / "abc123.json"
    quarantine_atomic_write(target, b"{}", 0o600)
    assert target.read_bytes() == b"{}"


def test_manager_atomic_write_survives_missing_fchmod(tmp_path, monkeypatch):
    monkeypatch.delattr(os, "fchmod", raising=False)
    target = tmp_path / "artifact.txt"
    IntegrationManager._atomic_write(target, b"payload", 0o644)
    assert target.read_bytes() == b"payload"


def test_manager_0o644_fallback_produces_observable_mode(tmp_path, monkeypatch):
    """The one site (of 7) where fchmod's effect is observable: mode=0o644
    differs from tempfile.mkstemp()'s 0o600 default. Asserting the final
    st_mode — not that a function was called — is what makes this control
    non-vacuous, per the ML-0A/AC3 correction. The exact-bits assertion is
    POSIX-only: NTFS only honors the write bit, so 0o644 can legitimately
    read back as 0o666 there — the write-succeeded assertion above already
    covers Windows unconditionally."""
    monkeypatch.delattr(os, "fchmod", raising=False)
    target = tmp_path / "artifact.txt"
    IntegrationManager._atomic_write(target, b"payload", 0o644)
    assert target.read_bytes() == b"payload"
    if os.name == "posix":
        observed = stat.S_IMODE(target.stat().st_mode)
        assert observed == 0o644, f"fallback did not apply requested mode: got {oct(observed)}"


# ---------------------------------------------------------------------------
# Direction (b), the symmetric/worrying one: fallback must NOT fire when
# os.fchmod IS present. A misused getattr, or fchmod shadowed by a mock,
# would silently weaken the POSIX guarantee without failing any test that
# only checks the end result — so this control asserts the *call*, on the
# one non-vacuous site, specifically to catch that failure mode. There is
# nothing to spy on where os.fchmod does not exist, so these are gated on
# hasattr(os, "fchmod"), not on platform name.
# ---------------------------------------------------------------------------


@pytest.mark.skipif(not _HAS_FCHMOD, reason="os.fchmod-present control: nothing to spy on without it")
def test_manager_0o644_uses_fchmod_not_chmod_on_posix(tmp_path, monkeypatch):
    calls: dict[str, list[tuple]] = {"fchmod": [], "chmod": []}
    real_fchmod = os.fchmod
    real_chmod = os.chmod

    def spy_fchmod(fd, mode):
        calls["fchmod"].append((fd, mode))
        return real_fchmod(fd, mode)

    def spy_chmod(path, mode):
        calls["chmod"].append((path, mode))
        return real_chmod(path, mode)

    monkeypatch.setattr(os, "fchmod", spy_fchmod)
    monkeypatch.setattr(os, "chmod", spy_chmod)

    target = tmp_path / "artifact.txt"
    IntegrationManager._atomic_write(target, b"payload", 0o644)

    assert len(calls["fchmod"]) == 1, "os.fchmod must be used when present"
    assert calls["chmod"] == [], "fallback (os.chmod) must not fire when os.fchmod is present"
    observed = stat.S_IMODE(target.stat().st_mode)
    assert observed == 0o644


@pytest.mark.skipif(not _HAS_FCHMOD, reason="os.fchmod-present control: nothing to spy on without it")
def test_identity_uses_fchmod_not_chmod_on_posix(tmp_path, monkeypatch):
    calls: dict[str, list[tuple]] = {"fchmod": [], "chmod": []}
    real_fchmod = os.fchmod
    real_chmod = os.chmod

    def spy_fchmod(fd, mode):
        calls["fchmod"].append((fd, mode))
        return real_fchmod(fd, mode)

    def spy_chmod(path, mode):
        calls["chmod"].append((path, mode))
        return real_chmod(path, mode)

    monkeypatch.setattr(os, "fchmod", spy_fchmod)
    monkeypatch.setattr(os, "chmod", spy_chmod)

    target = tmp_path / "identity.json"
    identity_atomic_write(str(target), b"{}", 0o600)

    assert len(calls["fchmod"]) == 1
    assert calls["chmod"] == []


@pytest.mark.skipif(not _HAS_FCHMOD, reason="os.fchmod-present control: nothing to spy on without it")
def test_quarantine_uses_fchmod_not_chmod_on_posix(tmp_path, monkeypatch):
    calls: dict[str, list[tuple]] = {"fchmod": [], "chmod": []}
    real_fchmod = os.fchmod
    real_chmod = os.chmod

    def spy_fchmod(fd, mode):
        calls["fchmod"].append((fd, mode))
        return real_fchmod(fd, mode)

    def spy_chmod(path, mode):
        calls["chmod"].append((path, mode))
        return real_chmod(path, mode)

    monkeypatch.setattr(os, "fchmod", spy_fchmod)
    monkeypatch.setattr(os, "chmod", spy_chmod)

    target = tmp_path / "quarantine" / "abc123.json"
    quarantine_atomic_write(target, b"{}", 0o600)

    assert len(calls["fchmod"]) == 1
    assert calls["chmod"] == []
