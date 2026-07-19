"""Safe lifecycle and ownership manager for physical integration artifacts."""

from __future__ import annotations

import hashlib
import json
import os
import stat
import tempfile
from pathlib import Path
from typing import Any


class IntegrationError(RuntimeError):
    pass


def _hash(content: bytes) -> str:
    return hashlib.sha256(content).hexdigest()


class IntegrationManager:
    def __init__(self, project_root: str | os.PathLike[str], home_dir: str | os.PathLike[str] | None = None):
        self.project_root = Path(project_root).absolute()
        self.home_dir = Path(home_dir or Path.home()).absolute()

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
            os.fchmod(descriptor, mode)
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

    def inspect(self, plan: dict[str, Any]) -> dict[str, Any]:
        destination, manifest_file, _ = self._resolve(plan)
        manifest = self._load_manifest(manifest_file)
        entry = manifest["artifacts"].get(str(destination))
        claim = plan["claim"]
        managed = bool(entry and claim in entry.get("claims", []))
        result = {
            "target": claim["target"],
            "surface": claim["surface"],
            "scope": claim["scope"],
            "item": claim["item"],
            "support_level": plan["support_level"],
            "representation": plan["representation"],
            "destination": plan["destination"],
            "state": "not-installed",
            "managed": managed,
        }
        try:
            actual = _hash(destination.read_bytes())
        except FileNotFoundError:
            return result
        desired = _hash(plan["content"])
        if entry:
            if actual != entry["sha256"]:
                result["state"] = "modified"
            elif actual != desired or entry["catalog_version"] != plan["catalog_version"]:
                result["state"] = "outdated"
            else:
                result["state"] = "current"
        elif actual == desired:
            result["state"] = "current"
        elif actual in plan.get("legacy_hashes", []):
            result["state"] = "outdated"
        else:
            result["state"] = "modified"
        return result

    def list(self, plans: list[dict[str, Any]]) -> list[dict[str, Any]]:
        return [self.inspect(plan) for plan in plans]

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
        desired_by_path: dict[Path, str] = {}
        for plan, destination, manifest_file, _ in resolved:
            desired = _hash(plan["content"])
            if destination in desired_by_path and desired_by_path[destination] != desired and operation != "uninstall":
                raise IntegrationError(f"conflicting content planned for {destination}")
            desired_by_path[destination] = desired
            self._preflight(plan, destination, manifests[manifest_file], operation, force)

        snapshots: dict[Path, tuple[bool, bytes, int]] = {}
        for filename in [entry[1] for entry in resolved] + list(manifests):
            if filename in snapshots:
                continue
            try:
                info = filename.lstat()
                if not stat.S_ISREG(info.st_mode):
                    raise IntegrationError(f"refusing non-regular file {filename}")
                snapshots[filename] = (True, filename.read_bytes(), stat.S_IMODE(info.st_mode))
            except FileNotFoundError:
                snapshots[filename] = (False, b"", 0)
        try:
            for plan, destination, manifest_file, root in resolved:
                self._apply(plan, destination, manifests[manifest_file], root, operation, force)
            for filename in sorted(manifests, key=str):
                self._write_manifest(filename, manifests[filename])
        except BaseException:
            for filename, (existed, content, mode) in snapshots.items():
                if existed:
                    self._atomic_write(filename, content, mode)
                else:
                    try:
                        filename.unlink()
                    except FileNotFoundError:
                        pass
            raise

    def _preflight(self, plan, destination, manifest, operation, force) -> None:
        state = self.inspect_with(plan, destination, manifest)
        entry = manifest["artifacts"].get(str(destination), {})
        owned = plan["claim"] in entry.get("claims", [])
        if operation == "install":
            if state == "modified" and not force:
                raise IntegrationError(f"artifact {destination} is modified; use force")
            if state == "outdated" and owned and not force:
                raise IntegrationError(f"artifact {destination} is outdated; use update")
        elif operation == "update":
            if not owned and state == "modified":
                raise IntegrationError(f"unmanaged artifact {destination} does not match a trackfw template")
            if state == "modified" and not force:
                raise IntegrationError(f"artifact {destination} is modified; use force")
        elif operation == "uninstall" and owned and state == "modified" and not force:
            raise IntegrationError(f"artifact {destination} is modified; use force")

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
        artifacts = manifest["artifacts"]
        entry = artifacts.get(str(destination))
        owned = bool(entry and plan["claim"] in entry.get("claims", []))
        if operation == "uninstall":
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
            return

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
                raise IntegrationError(f"unmanaged artifact {destination} does not match a trackfw template")
            write = actual_hash != desired_hash and (operation == "update" or force)
        elif exists and owned:
            write = actual_hash != desired_hash
        if write:
            self._atomic_write(destination, plan["content"], 0o644)
            actual_hash = desired_hash
        if entry is None:
            entry = {"destination": str(destination), "claims": []}
        if plan["claim"] not in entry["claims"]:
            entry["claims"].append(plan["claim"])
        entry["sha256"] = actual_hash
        entry["catalog_version"] = plan["catalog_version"] if actual_hash == desired_hash else "legacy"
        artifacts[str(destination)] = entry

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
