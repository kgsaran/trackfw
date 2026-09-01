"""Safe lifecycle and ownership manager for physical integration artifacts."""

from __future__ import annotations

import hashlib
import json
import os
import stat
import sys
import tempfile
from pathlib import Path
from typing import Any
# alias: o parametro `home_dir` sombreia o nome importado
from trackfw.homedir import home_dir as _user_home_dir


class IntegrationError(RuntimeError):
    pass


def _hash(content: bytes) -> str:
    return hashlib.sha256(content).hexdigest()


class IntegrationManager:
    def __init__(
        self,
        project_root: str | os.PathLike[str],
        home_dir: str | os.PathLike[str] | None = None,
        *,
        on_skip=None,
    ):
        self.project_root = Path(project_root).absolute()
        self.home_dir = Path(home_dir or _user_home_dir()).absolute()
        # Optional observer: called once per skipped artifact (outdated+owned+no-force)
        # in resolved order, never twice for the same destination.
        # Signature: on_skip(destination: str, reason: str) -> None.
        # ``destination`` is the tilde-abbreviated display path — global scope
        # yields "~/...", project scope yields the project-relative path (no "~/").
        # ``reason`` is the complete warning line, ready to print verbatim,
        # without a trailing newline.  Callers MUST NOT compose, abbreviate, or
        # derive anything from it — just write it to stderr.
        # Cannot import _tildeify from commands/update_harness.py here because
        # update_harness.py already imports from integrations/manager.py
        # (circular import); display formatting is inlined in _mutate instead.
        self.on_skip = on_skip
        # _after_manifest_persist, when set, is invoked immediately after
        # manifests are persisted and before artifact bytes are written
        # during install/update (never during uninstall, which is
        # intentionally not inverted — see the comment in _mutate). It
        # exists only so tests can simulate an interruption exactly at the
        # ADR-2026-08-18 ordering seam. Production code never assigns it.
        # Mirrors internal/integrations/manager.go's afterManifestPersist
        # package var.
        self._after_manifest_persist = None

    def _resolve(self, plan: dict[str, Any]) -> tuple[Path, Path, Path]:
        raw = plan["destination"]
        if "\x00" in raw:
            raise IntegrationError("destination contains NUL")
        scope = plan["claim"]["scope"]
        if scope not in {"project", "global"}:
            raise IntegrationError(f"unsupported scope {scope!r}")
        root = self.project_root if scope == "project" else self.home_dir
        if raw.startswith("~/"):
            if scope != "global":
                raise IntegrationError("home destination requires global scope")
            destination = root / raw[2:]
        else:
            candidate = Path(raw)
            destination = candidate if candidate.is_absolute() else root / candidate
        destination = Path(os.path.normpath(destination))
        try:
            relative = destination.relative_to(root)
        except ValueError as error:
            raise IntegrationError(f"destination {raw!r} escapes {scope} root") from error
        if str(relative) in {"", "."} or ".." in Path(raw).parts:
            raise IntegrationError(f"unsafe destination {raw!r}")
        self._reject_symlinks(root, destination)
        manifest = root / ".trackfw" / "integrations-manifest.json"
        self._reject_symlinks(root, manifest)
        return destination, manifest, root

    @staticmethod
    def _reject_symlinks(root: Path, destination: Path) -> None:
        current = destination
        while True:
            try:
                mode = current.lstat().st_mode
                if stat.S_ISLNK(mode):
                    raise IntegrationError(f"refusing symlink path {current}")
            except FileNotFoundError:
                pass
            if current == root:
                return
            if root not in current.parents:
                raise IntegrationError(f"path {destination} escapes root")
            current = current.parent

    @staticmethod
    def _empty_manifest() -> dict[str, Any]:
        return {"schema_version": 1, "artifacts": {}}

    def _load_manifest(self, filename: Path) -> dict[str, Any]:
        try:
            data = json.loads(filename.read_text(encoding="utf-8"))
        except FileNotFoundError:
            return self._empty_manifest()
        except (OSError, json.JSONDecodeError) as error:
            raise IntegrationError(f"read integration manifest: {error}") from error
        if data.get("schema_version") != 1 or not isinstance(data.get("artifacts"), dict):
            raise IntegrationError("unsupported integration manifest")
        return data

    @staticmethod
    def _atomic_write(filename: Path, content: bytes, mode: int) -> None:
        filename.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        descriptor, temporary = tempfile.mkstemp(prefix=".trackfw-tmp-", dir=filename.parent)
        try:
            fchmod = getattr(os, "fchmod", None)
            if fchmod is not None:
                fchmod(descriptor, mode)
            else:
                # os.fchmod is Unix-only (CPython docs: "Availability: Unix").
                # On platforms without it (Windows), fall back to chmod on the
                # temp file's own path. This reopens a narrow TOCTOU window
                # that fchmod(fd) does not have, but only on platforms where
                # fchmod never existed to begin with — os.fchmod continues to
                # be used unconditionally wherever it is available.
                os.chmod(temporary, mode)
            with os.fdopen(descriptor, "wb") as stream:
                stream.write(content)
                stream.flush()
                os.fsync(stream.fileno())
            os.replace(temporary, filename)
        except BaseException:
            try:
                os.close(descriptor)
            except OSError:
                pass
            try:
                os.unlink(temporary)
            except FileNotFoundError:
                pass
            raise

    def _write_manifest(self, filename: Path, manifest: dict[str, Any]) -> None:
        content = (json.dumps(manifest, indent=2, sort_keys=True, ensure_ascii=False) + "\n").encode()
        self._atomic_write(filename, content, 0o600)

    def _inspect_core(self, plan: dict[str, Any]) -> dict[str, Any]:
        """Shared inspection core, returning every field both the public
        `inspect()` contract and doctor (ML-2A, `inspect_full`) need. Kept
        private and separate from `inspect()`'s return shape because
        integrations/command.py's `list --json` passes `inspect()`'s dict
        through verbatim (no field-picking, unlike Go/Node's dedicated
        output structs) — an exact-key contract test
        (test_list_json_has_exact_contract_and_deterministic_order) would
        break if new keys were added to that dict. `resolved_destination`
        and `registered` (doctor's needs) live only here and in
        `inspect_full()`.
        """
        destination, manifest_file, _ = self._resolve(plan)
        manifest = self._load_manifest(manifest_file)
        entry = manifest["artifacts"].get(str(destination))
        claim = plan["claim"]
        managed = bool(entry and claim in entry.get("claims", []))
        core = {
            "claim": claim,
            "support_level": plan["support_level"],
            "representation": plan["representation"],
            "destination": plan["destination"],
            "resolved_destination": str(destination),
            "state": "not-installed",
            "managed": managed,
            # registered reports whether the manifest has ANY entry for this
            # destination, regardless of claim ownership — unlike managed,
            # which additionally requires this exact claim to own that
            # entry. doctor needs this distinction: a destination registered
            # under a *different* claim must never be reported as an
            # "unregistered write" — the dominant false-positive doctor
            # exists to avoid. Mirrors Inspection.Registered
            # (internal/integrations/manager.go).
            "registered": bool(entry),
        }
        try:
            actual = _hash(destination.read_bytes())
        except FileNotFoundError:
            return core
        desired = _hash(plan["content"])
        if entry:
            if actual != entry["sha256"]:
                core["state"] = "modified"
            elif actual != desired or entry["catalog_version"] != plan["catalog_version"]:
                core["state"] = "outdated"
            else:
                core["state"] = "current"
        elif actual == desired:
            core["state"] = "current"
        elif actual in plan.get("legacy_hashes", []):
            core["state"] = "outdated"
        else:
            core["state"] = "modified"
        return core

    def inspect(self, plan: dict[str, Any]) -> dict[str, Any]:
        core = self._inspect_core(plan)
        claim = core["claim"]
        return {
            "target": claim["target"],
            "surface": claim["surface"],
            "scope": claim["scope"],
            "item": claim["item"],
            "support_level": core["support_level"],
            "representation": core["representation"],
            "destination": core["destination"],
            "state": core["state"],
            "managed": core["managed"],
        }

    def list(self, plans: list[dict[str, Any]]) -> list[dict[str, Any]]:
        return [self.inspect(plan) for plan in plans]

    def inspect_full(self, plan: dict[str, Any]) -> dict[str, Any]:
        """Like `inspect()`, but includes `claim`, `resolved_destination`
        and `registered` — doctor (ML-2A) needs all three and must not
        widen the public `inspect()`/`list()` JSON contract to get them."""
        return self._inspect_core(plan)

    def list_full(self, plans: list[dict[str, Any]]) -> list[dict[str, Any]]:
        return [self.inspect_full(plan) for plan in plans]

    def install(self, plans: list[dict[str, Any]], force: bool = False) -> None:
        self._mutate(plans, "install", force)

    def update(self, plans: list[dict[str, Any]], force: bool = False) -> None:
        self._mutate(plans, "update", force)

    def uninstall(self, plans: list[dict[str, Any]], force: bool = False) -> None:
        self._mutate(plans, "uninstall", force)

    def _mutate(self, plans: list[dict[str, Any]], operation: str, force: bool) -> None:
        resolved: list[tuple[dict[str, Any], Path, Path, Path]] = []
        manifests: dict[Path, dict[str, Any]] = {}
        for plan in plans:
            destination, manifest_file, root = self._resolve(plan)
            manifests.setdefault(manifest_file, self._load_manifest(manifest_file))
            resolved.append((plan, destination, manifest_file, root))

        # Phase 1 — conflict detection + preflight (all items).  Errors raise
        # immediately; skips are collected to call on_skip after all preflights
        # succeed, so the observer is never called when the batch will abort.
        desired_by_path: dict[Path, str] = {}
        skip_items: list[tuple[dict[str, Any], Path, Path, Path]] = []
        active: list[tuple[dict[str, Any], Path, Path, Path]] = []
        for plan, destination, manifest_file, root in resolved:
            desired = _hash(plan["content"])
            if destination in desired_by_path and desired_by_path[destination] != desired and operation != "uninstall":
                raise IntegrationError(f"conflicting content planned for {destination}")
            desired_by_path[destination] = desired
            skip = self._preflight(plan, destination, manifests[manifest_file], operation, force)
            if skip:
                skip_items.append((plan, destination, manifest_file, root))
            else:
                active.append((plan, destination, manifest_file, root))

        # Phase 2 — notify observer for each unique skipped destination in
        # resolved order; guard against None and against duplicate destinations.
        notified: set[Path] = set()
        for plan, destination, manifest_file, root in skip_items:
            if destination not in notified:
                notified.add(destination)
                if self.on_skip is not None:
                    scope = plan["claim"]["scope"]
                    # Display path mirrors Go/Node conventions:
                    #   global  → "~/..."  (tilde-abbreviated; relative to home_dir)
                    #   project → "..."    (relative to project_root; no leading "./")
                    # Cannot call _tildeify from commands/update_harness.py here:
                    # that module imports from integrations/manager.py, so importing
                    # back would be circular.  Inline equivalent 2-branch logic.
                    # .as_posix() ensures forward slashes on all platforms and makes
                    # the ML-6H double-slash guard explicit (Path normalisation in
                    # _resolve already removes trailing separators from home_dir,
                    # so "~/" is the only possible doubling site — as_posix() on
                    # relative_to() output never carries a leading separator).
                    if scope == "global":
                        display = "~/" + destination.relative_to(root).as_posix()
                        remediation = "trackfw update harness"
                    else:
                        display = destination.relative_to(root).as_posix()
                        remediation = "trackfw update"
                    reason = (
                        f"warning: skipping outdated artifact {display};"
                        f" run '{remediation}' to refresh it"
                    )
                    self.on_skip(display, reason)

        # Phase 3 — snapshot only active destinations (not skipped ones) plus all
        # manifest files (for rollback safety even when a scope had all items skipped).
        snapshots: dict[Path, tuple[bool, bytes, int]] = {}
        for filename in [entry[1] for entry in active] + list(manifests):
            if filename in snapshots:
                continue
            try:
                info = filename.lstat()
                if not stat.S_ISREG(info.st_mode):
                    raise IntegrationError(f"refusing non-regular file {filename}")
                snapshots[filename] = (True, filename.read_bytes(), stat.S_IMODE(info.st_mode))
            except FileNotFoundError:
                snapshots[filename] = (False, b"", 0)
        def persist_manifests() -> None:
            for filename in sorted(manifests, key=str):
                self._write_manifest(filename, manifests[filename])

        try:
            # ADR-2026-08-18: install/update persist the manifest before
            # writing artifact bytes (self-healing direction on
            # interruption); uninstall keeps the original
            # artifacts-before-manifest order. The two are deliberately NOT
            # symmetric.
            if operation == "uninstall":
                # Uninstall is not inverted. Removing bytes first and
                # persisting the manifest last means an interruption leaves
                # the manifest still declaring an artifact whose file is now
                # absent — inspect_with resolves that as "not-installed",
                # the same self-healing direction install/update get from
                # the inversion below. Inverting uninstall the same way
                # (drop the manifest entry, then remove bytes) would instead
                # leave, on interruption, a file on disk whose content still
                # matches the catalog template but with no manifest entry at
                # all — reported as an orphaned "current"/managed=False
                # artifact that looks legitimate and that nothing detects or
                # repairs automatically. That is exactly the "disk ahead of
                # manifest" bad direction the ADR exists to eliminate, so
                # uninstall must not be simetrized with install/update here.
                # Mirrors the comment in internal/integrations/manager.go:mutate.
                for plan, destination, manifest_file, root in active:
                    self._apply(plan, destination, manifests[manifest_file], root, operation, force)
                persist_manifests()
            else:
                # Install/Update: compute the manifest update for every
                # active item in memory (no bytes touched yet), persist all
                # manifests, and only then write the artifact bytes.
                pending_writes: list[tuple[Path, bytes, int]] = []
                for plan, destination, manifest_file, root in active:
                    write = self._plan_artifact_write(plan, destination, manifests[manifest_file], operation, force)
                    if write is not None:
                        pending_writes.append(write)
                persist_manifests()
                if self._after_manifest_persist is not None:
                    self._after_manifest_persist()
                for destination, content, mode in pending_writes:
                    self._atomic_write(destination, content, mode)
        except BaseException:
            # Best-effort restore: one snapshot failing to write back (e.g. the
            # very I/O condition that caused the batch to fail in the first
            # place is still in effect for that path) must not stop the
            # others — most importantly the manifest — from being restored.
            # Mirrors Go's `_ = atomicWrite(...)` (errors discarded) and
            # Node's `catch { /* preserve original error */ }` in rollback().
            # Surfaced by ADR-2026-08-18: with the manifest persisted before
            # artifact bytes, a write-phase failure now always needs the
            # manifest snapshot restored too, not just (previously, rarely)
            # the artifact one.
            for filename, (existed, content, mode) in snapshots.items():
                try:
                    if existed:
                        self._atomic_write(filename, content, mode)
                    else:
                        filename.unlink()
                except Exception:  # noqa: BLE001 - best-effort restore, mirrors Go's `_ = atomicWrite(...)` and Node's bare `catch {}`
                    pass
            raise

    def _preflight(self, plan, destination, manifest, operation, force) -> bool:
        """Returns True if the item should be skipped (not an error).

        The only skip condition is install on an outdated+owned artifact without
        --force: bytes are already on disk from a previous install; the user
        must run ``trackfw update`` (or ``trackfw update harness``) to refresh.
        All other abnormal states continue to raise IntegrationError.
        """
        if operation != "uninstall":
            self._detect_name_collision(plan, destination, force)
        state = self.inspect_with(plan, destination, manifest)
        entry = manifest["artifacts"].get(str(destination), {})
        owned = plan["claim"] in entry.get("claims", [])
        if operation == "install":
            if state == "modified" and not force:
                raise IntegrationError(f"artifact {destination} is modified; use force")
            if state == "outdated" and owned and not force:
                # Skip silently: artifact is managed and on-disk bytes are intact.
                # Caller notifies via on_skip observer; the batch continues.
                return True
        elif operation == "update":
            if not owned and state == "modified":
                raise IntegrationError(self._unmanaged_artifact_error(destination, plan["claim"]))
            if state == "modified" and not force:
                raise IntegrationError(f"artifact {destination} is modified; use force")
        elif operation == "uninstall" and owned and state == "modified" and not force:
            raise IntegrationError(f"artifact {destination} is modified; use force")
        return False

    @staticmethod
    def _unmanaged_artifact_error(destination: Path, claim: dict[str, Any]) -> str:
        """Error message for update/uninstall (or, defensively, install)
        refusing bytes trackfw did not write. Names the remedy — trackfw did
        not write these bytes, so the only safe way to bring the artifact
        under management is ``<kind> install --force``, which explicitly
        authorizes adopting/replacing unmanaged content — with the exact
        flags to reproduce this plan's claim (item, target, scope), ready to
        copy-paste. Mirrors internal/integrations/manager.go:
        unmanagedArtifactError — canonical, byte-identical source of truth
        across the 3 CLIs.
        """
        return (
            f'unmanaged artifact "{destination}" does not match a trackfw template'
            " — trackfw did not write these bytes.\n"
            f"Adopt it with: trackfw {claim['kind']} install --force"
            f" --items {claim['item']} --targets {claim['target']} --scope {claim['scope']}"
        )

    @staticmethod
    def _frontmatter_name(content: bytes) -> str | None:
        """Extracts only the "name" field of a "---"-delimited YAML
        frontmatter, without markdownParts' default values. Returns None
        when the file has no recognizable frontmatter or no "name". Mirrors
        Go's internal/integrations/render.go frontmatterName."""
        text = content.decode("utf-8", errors="replace").strip()
        if not text.startswith("---\n"):
            return None
        end = text.find("\n---", 4)
        if end < 0:
            return None
        frontmatter = text[4:end]
        for line in frontmatter.split("\n"):
            if ":" not in line:
                continue
            key, value = line.split(":", 1)
            if key.strip() != "name":
                continue
            value = value.strip().strip('"')
            if not value:
                return None
            return value
        return None

    def _detect_name_collision(self, plan: dict[str, Any], destination: Path, force: bool) -> None:
        """Guards against two distinct managed agent artifacts declaring the
        same frontmatter "name" inside the same destination directory (ADR
        ADR-2026-07-25-identidade-personalizavel-de-agentes, secao D4).

        Limitation: only scans ".md" siblings, mirroring Go's manager.go —
        JSON (cli-agent-json/agent-json) and TOML (custom-agent-toml)
        artifacts are not scanned for collisions.
        """
        if plan["claim"].get("kind") != "agents":
            return
        if destination.suffix != ".md":
            return
        desired_name = self._frontmatter_name(plan["content"])
        if desired_name is None:
            return
        directory = destination.parent
        try:
            entries = sorted(directory.iterdir())
        except FileNotFoundError:
            return
        for candidate in entries:
            if candidate.is_dir() or candidate.suffix != ".md":
                continue
            if candidate == destination:
                continue
            try:
                data = candidate.read_bytes()
            except OSError:
                continue
            candidate_name = self._frontmatter_name(data)
            if candidate_name is None or candidate_name != desired_name:
                continue
            if force:
                print(
                    f"aviso: {candidate} declara o mesmo name {desired_name!r} que "
                    f"{destination}; prosseguindo por --force",
                    file=sys.stderr,
                )
                continue
            raise IntegrationError(
                f"artifact {destination} declares name {desired_name!r} which collides "
                f"with existing file {candidate}"
            )

    @staticmethod
    def inspect_with(plan, destination: Path, manifest) -> str:
        entry = manifest["artifacts"].get(str(destination))
        try:
            actual = _hash(destination.read_bytes())
        except FileNotFoundError:
            return "not-installed"
        desired = _hash(plan["content"])
        if entry:
            if actual != entry["sha256"]:
                return "modified"
            if actual != desired or entry["catalog_version"] != plan["catalog_version"]:
                return "outdated"
            return "current"
        if actual == desired:
            return "current"
        if actual in plan.get("legacy_hashes", []):
            return "outdated"
        return "modified"

    def _apply(self, plan, destination: Path, manifest, root: Path, operation, force) -> None:
        """Uninstall only: removes ownership of one artifact from the
        manifest, and — once no claim remains — the artifact's bytes and any
        empty ancestor directories it managed. Mutates disk directly (not
        deferred), because uninstall deliberately keeps the
        pre-ADR-2026-08-18 ordering: see the comment in _mutate for why this
        is not simetrized with _plan_artifact_write. Mirrors
        internal/integrations/manager.go:applyUninstall.
        """
        artifacts = manifest["artifacts"]
        entry = artifacts.get(str(destination))
        owned = bool(entry and plan["claim"] in entry.get("claims", []))
        if not owned:
            return
        entry["claims"] = [claim for claim in entry["claims"] if claim != plan["claim"]]
        if entry["claims"]:
            return
        try:
            destination.unlink()
        except FileNotFoundError:
            pass
        del artifacts[str(destination)]
        self._remove_empty(destination.parent, root)

    def _plan_artifact_write(
        self, plan, destination: Path, manifest, operation, force
    ) -> tuple[Path, bytes, int] | None:
        """Computes the manifest update for one install/update item entirely
        in memory — never touches the artifact's bytes on disk. The caller
        (_mutate) persists every manifest in the batch first, and only then
        applies the returned pending write (ADR-2026-08-18). Returns None
        when no byte write is needed.

        The manifest values it stores are deliberately *optimistic* when a
        write is pending: sha256/catalog_version describe the content this
        call is about to write, not what is currently on disk. If
        interrupted before the pending write lands, the manifest already
        declares the target state and inspect_with resolves the (absent or
        stale) file to "not-installed"/"modified", both self-repairable by a
        later install/update, never "modified"+unowned ("unmanaged").
        Mirrors internal/integrations/manager.go:planArtifactWrite.
        """
        artifacts = manifest["artifacts"]
        entry = artifacts.get(str(destination))
        owned = bool(entry and plan["claim"] in entry.get("claims", []))

        try:
            actual = destination.read_bytes()
            exists = True
        except FileNotFoundError:
            actual = b""
            exists = False
        actual_hash = _hash(actual)
        desired_hash = _hash(plan["content"])
        known = actual_hash in plan.get("legacy_hashes", [])
        write = not exists
        if exists and not owned:
            if actual_hash != desired_hash and not known and not force:
                # Defense-in-depth: _preflight already rejects this exact case
                # for "update" (unconditionally) and for "install" without
                # --force (any state == "modified" is blocked before an
                # active item ever reaches _plan_artifact_write). This branch
                # is therefore not reachable via install/update today, but
                # stays as a second line of defense in case _preflight's
                # guard is ever loosened — hence the identical remediation
                # text, so a user who somehow hits it still gets the same
                # actionable message. No manifest mutation happens on this
                # path. Mirrors internal/integrations/manager.go:planArtifactWrite.
                raise IntegrationError(self._unmanaged_artifact_error(destination, plan["claim"]))
            write = actual_hash != desired_hash and (operation == "update" or force)
        elif exists and owned:
            write = actual_hash != desired_hash
        if entry is None:
            entry = {"destination": str(destination), "claims": []}
        if plan["claim"] not in entry["claims"]:
            entry["claims"].append(plan["claim"])
        pending: tuple[Path, bytes, int] | None = None
        if write:
            # Optimistic: bytes have not moved yet, but the manifest must
            # already describe the content we are about to write (see doc
            # comment above).
            actual_hash = desired_hash
            pending = (destination, plan["content"], 0o644)
        entry["sha256"] = actual_hash
        entry["catalog_version"] = plan["catalog_version"] if actual_hash == desired_hash else "legacy"
        artifacts[str(destination)] = entry
        return pending

    def _remove_empty(self, directory: Path, root: Path) -> None:
        while directory != root and root in directory.parents:
            try:
                if stat.S_ISLNK(directory.lstat().st_mode):
                    raise IntegrationError(f"refusing symlink directory {directory}")
                directory.rmdir()
            except FileNotFoundError:
                pass
            except OSError:
                return
            directory = directory.parent
