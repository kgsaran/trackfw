#!/usr/bin/env python3
"""
build_wheel.py — constrói uma wheel binária do trackfw sem nenhum arquivo Python.

Layout produzido (seguindo o molde do gh-bin):
  trackfw-{version}-py3-none-{platform_tag}.whl
  ├── trackfw-{version}.dist-info/METADATA
  ├── trackfw-{version}.dist-info/WHEEL
  ├── trackfw-{version}.dist-info/RECORD
  └── trackfw-{version}.data/scripts/trackfw  (ou trackfw.exe no Windows)

Uso:
  python3 pypi/scripts/build_wheel.py \\
      --binary  bin/trackfw \\
      --version 8.0.0 \\
      --platform manylinux_2_17_x86_64 \\
      --output  dist/

Tags suportadas (PyPI aceita):
  manylinux_2_17_x86_64   linux x64 glibc ≥ 2.17
  manylinux_2_17_aarch64  linux arm64 glibc ≥ 2.17
  musllinux_1_2_x86_64    linux x64 musl ≥ 1.2
  musllinux_1_2_aarch64   linux arm64 musl ≥ 1.2
  macosx_11_0_arm64       macOS arm64 (Apple Silicon)
  macosx_10_9_x86_64      macOS x64
  win_amd64               Windows x64
  win_arm64               Windows arm64

  linux_x86_64 puro é REJEITADO pelo PyPI — use manylinux/musllinux.

ML-1C, ROADMAP-2026-09-12-v8-um-binario-muitos-canais.
"""

import argparse
import base64
import hashlib
import os
import stat
import sys
import zipfile
from pathlib import Path


def sha256_digest(data: bytes) -> str:
    """Retorna sha256=<base64url> como exigido pelo formato RECORD."""
    digest = hashlib.sha256(data).digest()
    return "sha256=" + base64.urlsafe_b64encode(digest).rstrip(b"=").decode()


def build_wheel(binary_path: Path, version: str, platform_tag: str, output_dir: Path) -> Path:
    binary_path = binary_path.resolve()
    output_dir = output_dir.resolve()
    output_dir.mkdir(parents=True, exist_ok=True)

    name = "trackfw"
    dist_name = f"{name}-{version}"
    # Wheel filename: {name}-{version}-{python_tag}-{abi_tag}-{platform_tag}.whl
    whl_name = f"{dist_name}-py3-none-{platform_tag}.whl"
    whl_path = output_dir / whl_name

    # Nome do binário dentro da wheel
    is_windows = platform_tag.startswith("win")
    bin_name = "trackfw.exe" if is_windows else "trackfw"

    # Caminhos internos da wheel
    dist_info_prefix = f"{dist_name}.dist-info"
    data_prefix = f"{dist_name}.data"
    scripts_prefix = f"{data_prefix}/scripts"
    bin_entry = f"{scripts_prefix}/{bin_name}"

    # Conteúdo dos arquivos de metadados
    metadata_content = (
        f"Metadata-Version: 2.1\n"
        f"Name: {name}\n"
        f"Version: {version}\n"
        f"Summary: Governance CLI for AI-native software delivery\n"
        f"Home-page: https://github.com/kgsaran/trackfw\n"
        f"License: MIT\n"
        f"Requires-Python: >=3.10\n"
    ).encode()

    wheel_content = (
        f"Wheel-Version: 1.0\n"
        f"Generator: build_wheel.py\n"
        f"Root-Is-Purelib: false\n"
        f"Tag: py3-none-{platform_tag}\n"
    ).encode()

    # Lê o binário
    binary_data = binary_path.read_bytes()

    # Calcula hashes para o RECORD
    meta_digest = sha256_digest(metadata_content)
    wheel_digest = sha256_digest(wheel_content)
    bin_digest = sha256_digest(binary_data)

    record_lines = [
        f"{dist_info_prefix}/METADATA,{meta_digest},{len(metadata_content)}",
        f"{dist_info_prefix}/WHEEL,{wheel_digest},{len(wheel_content)}",
        f"{bin_entry},{bin_digest},{len(binary_data)}",
        # RECORD auto-referência: sem hash e sem tamanho (PEP 627)
        f"{dist_info_prefix}/RECORD,,",
    ]
    record_content = "\n".join(record_lines).encode() + b"\n"

    # Monta a wheel (zip)
    with zipfile.ZipFile(whl_path, "w", compression=zipfile.ZIP_DEFLATED) as zf:
        def add_text(arcname: str, data: bytes) -> None:
            info = zipfile.ZipInfo(arcname)
            info.compress_type = zipfile.ZIP_DEFLATED
            # Arquivos de metadados: 644
            info.external_attr = (stat.S_IFREG | 0o644) << 16
            zf.writestr(info, data)

        def add_binary(arcname: str, data: bytes) -> None:
            info = zipfile.ZipInfo(arcname)
            info.compress_type = zipfile.ZIP_DEFLATED
            # Binário: 755 — sem isso pip instala sem +x e o comando falha com permission denied
            info.external_attr = (stat.S_IFREG | 0o755) << 16
            zf.writestr(info, data)

        add_text(f"{dist_info_prefix}/METADATA", metadata_content)
        add_text(f"{dist_info_prefix}/WHEEL", wheel_content)
        add_binary(bin_entry, binary_data)
        add_text(f"{dist_info_prefix}/RECORD", record_content)

    return whl_path


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Constrói wheel binária do trackfw (zero .py)"
    )
    parser.add_argument("--binary", required=True, help="Caminho para o binário Go compilado")
    parser.add_argument("--version", required=True, help="Versão do pacote (ex: 8.0.0)")
    parser.add_argument(
        "--platform",
        required=True,
        help="Tag de plataforma (ex: manylinux_2_17_x86_64, macosx_11_0_arm64, win_amd64)",
    )
    parser.add_argument("--output", default="dist", help="Diretório de saída (padrão: dist/)")
    args = parser.parse_args()

    binary_path = Path(args.binary)
    if not binary_path.exists():
        print(f"Erro: binário não encontrado: {binary_path}", file=sys.stderr)
        sys.exit(1)

    whl = build_wheel(
        binary_path=binary_path,
        version=args.version,
        platform_tag=args.platform,
        output_dir=Path(args.output),
    )
    print(f"Wheel criada: {whl}")


if __name__ == "__main__":
    main()
