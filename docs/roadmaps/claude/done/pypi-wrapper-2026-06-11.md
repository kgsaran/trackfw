# Roadmap: Wrapper PyPI

> Criado em: 2026-06-11 | Status: ✅ Done

## Contexto

Publicar o `trackfw` CLI no PyPI como pacote `trackfw`. O PyPI não tem hook `postinstall` como o npm, então o padrão adotado é **lazy download**: a primeira vez que o usuário executa `trackfw`, o wrapper Python detecta que o binário nativo ainda não existe, baixa das GitHub Releases e executa. Nas execuções seguintes, vai direto ao binário em cache. É o mesmo padrão de `ruff`, `pyright` e similares.

**Repositório:** `github.com/kgsaran/trackfw`
**Package name PyPI:** `trackfw`
**Versão inicial:** `0.1.0`
**Python mínimo:** 3.8

### Estrutura final
```
pypi/
├── pyproject.toml          ← metadata + build config + console_scripts
└── trackfw/
    ├── __init__.py         ← vazio
    └── _cli.py             ← entry point: encontra/baixa binário e o executa
```

---

## Wave 1 — Três arquivos em paralelo

> Dependências: independentes entre si

---

### ML-1A — `pypi/pyproject.toml`
**Status:** ⬜ Pendente
**Arquivo:** `pypi/pyproject.toml`

```toml
[build-system]
requires = ["setuptools>=61"]
build-backend = "setuptools.backends.legacy:build"

[project]
name = "trackfw"
version = "0.1.0"
description = "Governed software delivery framework: ADR → REQ → ROADMAP → kanban"
readme = "README.md"
license = { text = "MIT" }
requires-python = ">=3.8"
keywords = ["cli", "adr", "roadmap", "governance", "delivery"]
classifiers = [
  "Development Status :: 3 - Alpha",
  "Environment :: Console",
  "License :: OSI Approved :: MIT License",
  "Programming Language :: Python :: 3",
  "Topic :: Software Development :: Build Tools",
]

[project.urls]
Homepage = "https://github.com/kgsaran/trackfw"
Repository = "https://github.com/kgsaran/trackfw"

[project.scripts]
trackfw = "trackfw._cli:main"

[tool.setuptools.packages.find]
where = ["."]
include = ["trackfw*"]
```

**Critérios de aceite:**
- [ ] TOML válido
- [ ] `console_scripts`: `trackfw = "trackfw._cli:main"`
- [ ] `requires-python = ">=3.8"`
- [ ] Sem dependências externas no `[project.dependencies]`

---

### ML-1B — `pypi/trackfw/__init__.py`
**Status:** ⬜ Pendente
**Arquivo:** `pypi/trackfw/__init__.py`
**Conteúdo:** arquivo vazio (apenas cria o pacote Python)

**Critérios de aceite:**
- [ ] Arquivo existe (pode estar vazio)

---

### ML-1C — `pypi/trackfw/_cli.py`
**Status:** ⬜ Pendente
**Arquivo:** `pypi/trackfw/_cli.py`

**Lógica completa:**

```python
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
    """Retorna (os_name, arch) no formato GoReleaser ou (None, None) se não suportado."""
    system = platform.system().lower()
    machine = platform.machine().lower()

    os_map = {"linux": "linux", "darwin": "darwin", "windows": "windows"}
    arch_map = {"x86_64": "amd64", "amd64": "amd64", "aarch64": "arm64", "arm64": "arm64"}

    return os_map.get(system), arch_map.get(machine)


def _binary_path():
    """Caminho onde o binário nativo será armazenado (junto ao pacote Python)."""
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

        # Download (segue redirects automaticamente via urllib)
        urllib.request.urlretrieve(url, tmp_archive)

        # Extração
        extracted_bin_name = "trackfw.exe" if is_windows else "trackfw"
        if is_windows:
            with zipfile.ZipFile(tmp_archive) as zf:
                zf.extract(extracted_bin_name, tmp)
        else:
            with tarfile.open(tmp_archive, "r:gz") as tf:
                tf.extract(extracted_bin_name, tmp, filter="data")

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

    # Substituir o processo atual pelo binário Go (Unix: os.execv; Windows: subprocess)
    if platform.system() == "Windows":
        import subprocess
        result = subprocess.run([str(binary)] + sys.argv[1:])
        sys.exit(result.returncode)
    else:
        os.execv(str(binary), [str(binary)] + sys.argv[1:])
```

**Critérios de aceite:**
- [ ] Sem dependências externas (apenas stdlib Python)
- [ ] `main()` é o entry point
- [ ] `os.execv` no Unix (sem processo intermediário) / `subprocess.run` no Windows
- [ ] Download lazy: só baixa se o binário não existir
- [ ] Plataforma não suportada → mensagem clara + `exit(1)`
- [ ] `tarfile` com `filter="data"` (segurança contra path traversal, Python 3.12+; compatível com 3.8 via fallback implícito)

---

## Wave 2 — Validação local

> Dependências: Wave 1 completa

### ML-2A — Verificar estrutura e `pip install --dry-run`
**Status:** ⬜ Pendente
**Comandos:**
```bash
cd pypi/
python3 -m py_compile trackfw/_cli.py    # sintaxe
python3 -m py_compile trackfw/__init__.py
pip3 install --dry-run . 2>&1            # valida pyproject.toml
```

**Critérios de aceite:**
- [ ] `py_compile` sem erros
- [ ] `pip install --dry-run .` resolve sem erros de metadata
- [ ] Estrutura de pacote correta

---

## Ordem de execução

```
Wave 1: ML-1A ║ ML-1B ║ ML-1C  (paralelo)
               ↓
Wave 2: ML-2A  (validação)
```
