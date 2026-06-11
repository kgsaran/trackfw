"""Entry point para o CLI trackfw via PyPI."""

import os
import sys
import platform
import urllib.request
import tarfile
import zipfile
import tempfile
import shutil
from pathlib import Path

VERSION = "0.1.0"
REPO = "kgsaran/trackfw"


def _platform_info():
    system = platform.system().lower()
    machine = platform.machine().lower()
    os_map = {"linux": "linux", "darwin": "darwin", "windows": "windows"}
    arch_map = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}
    return os_map.get(system), arch_map.get(machine)


def _binary_path():
    pkg_dir = Path(__file__).parent
    is_windows = platform.system() == "Windows"
    name = "trackfw-bin.exe" if is_windows else "trackfw-bin"
    return pkg_dir / name


def _download_binary(dest: Path):
    os_name, arch = _platform_info()
    if not os_name or not arch:
        print(
            f"trackfw: plataforma não suportada ({platform.system()}/{platform.machine()})",
            file=sys.stderr,
        )
        sys.exit(1)

    is_windows = os_name == "windows"
    ext = ".zip" if is_windows else ".tar.gz"
    filename = f"trackfw_{VERSION}_{os_name}_{arch}{ext}"
    url = f"https://github.com/{REPO}/releases/download/v{VERSION}/{filename}"

    print(f"trackfw: baixando binário v{VERSION} para {os_name}/{arch}...", file=sys.stderr)

    with tempfile.TemporaryDirectory() as tmp:
        tmp_archive = os.path.join(tmp, filename)
        urllib.request.urlretrieve(url, tmp_archive)

        extracted_bin_name = "trackfw.exe" if is_windows else "trackfw"
        if is_windows:
            with zipfile.ZipFile(tmp_archive) as zf:
                zf.extract(extracted_bin_name, tmp)
        else:
            with tarfile.open(tmp_archive, "r:gz") as tf:
                member = tf.getmember(extracted_bin_name)
                tf.extract(member, tmp, filter="data")

        extracted = os.path.join(tmp, extracted_bin_name)
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.move(extracted, str(dest))

    if not is_windows:
        dest.chmod(0o755)

    print("trackfw: binário instalado.", file=sys.stderr)


def main():
    binary = _binary_path()

    if not binary.exists():
        _download_binary(binary)

    if platform.system() == "Windows":
        import subprocess
        result = subprocess.run([str(binary)] + sys.argv[1:])
        sys.exit(result.returncode)
    else:
        os.execv(str(binary), [str(binary)] + sys.argv[1:])
