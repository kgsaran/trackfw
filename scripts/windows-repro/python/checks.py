"""checks.py — verificacoes Python da suite de reproducao de defeito (camada
2) para ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-
sob-demanda, ML-1A.

Cada subcomando exercita o CAMINHO REAL de producao (pypi/trackfw), sem
monkeypatch/mock — diferente dos testes existentes que a Wave 0 (hades-tf)
identificou como vacuos porque substituem sys.stdin.isatty diretamente
(pypi/tests/test_init_identity.py:83,98).

Executado via `python scripts/windows-repro/python/checks.py <subcomando>`
com PYTHONPATH apontando para pypi/ (feito pelo chamador, run.ps1).
"""
import io
import os
import shutil
import subprocess
import sys
import tempfile


def cmd_help():
    """item 1 — UnicodeEncodeError no console cp1252, cli.py --help de topo.

    Wave 0 (hades-tf) verificou que o unico teste subprocess de --help
    hoje chama um SUBPARSER (roadmap --help), que nao renderiza a
    description= do parser raiz (onde esta o simbolo de seta). O caminho
    real que imprime essa string e args.command is None -> parser.print_help()
    (cli.py:179-180). Reproduzido aqui via subprocess real do interpretador
    Python, sem PYTHONUTF8/PYTHONIOENCODING setados (para nao mascarar a
    codepage cp1252 nativa do console Windows) e SEM capturar stdout num
    objeto que tenha .reconfigure (a producao usa exatamente esse guard).
    """
    env = dict(os.environ)
    env.pop("PYTHONUTF8", None)
    env.pop("PYTHONIOENCODING", None)
    env["PYTHONPATH"] = os.environ.get("TRACKFW_PYPI_SRC", "")
    encoding_probe = subprocess.run(
        [sys.executable, "-c", "import sys; print(sys.stdout.encoding, sys.stderr.encoding)"],
        env=env,
        capture_output=True,
        text=True,
    )
    print(f"child_stdout_stderr_encoding={encoding_probe.stdout.strip()!r}")
    proc = subprocess.run(
        [sys.executable, "-c", "from trackfw.cli import main; import sys; sys.argv=['trackfw']; main()"],
        env=env,
        capture_output=True,
        text=False,
    )
    stderr = proc.stderr.decode("utf-8", errors="replace")
    print(f"exit={proc.returncode}")
    print(f"stderr_tail={stderr[-400:]!r}")
    if "UnicodeEncodeError" in stderr:
        print("VERDICT=REPRODUCED")
    else:
        print("VERDICT=ABSENT")


def cmd_cp1252_print():
    """item 4 (mecanismo compartilhado com item 1) — o gate de cobertura
    (scripts/check-parity-contract-coverage.sh) crasha com o MESMO
    UnicodeEncodeError, lendo docs/cli-parity.md (65 ocorrencias de seta) e
    imprimindo no console cp1252. Como o script e .sh (dependeria de
    sh/bash no PATH — item 7, que este instrumento trata como incerto),
    reproduzimos aqui o mecanismo em isolamento: um `print()` real do
    interpretador Python do caractere que aparece em docs/cli-parity.md,
    sob as mesmas condicoes de console/encoding, SEM invocar o wrapper
    .sh (evita confundir o item 4 com o item 7 no mapeamento).
    """
    env = dict(os.environ)
    env.pop("PYTHONUTF8", None)
    env.pop("PYTHONIOENCODING", None)
    encoding_probe = subprocess.run(
        [sys.executable, "-c", "import sys; print(sys.stdout.encoding, sys.stderr.encoding)"],
        env=env,
        capture_output=True,
        text=True,
    )
    print(f"child_stdout_stderr_encoding={encoding_probe.stdout.strip()!r}")
    proc = subprocess.run(
        [sys.executable, "-c", "print('\\u2192')"],
        env=env,
        capture_output=True,
        text=False,
    )
    stderr = proc.stderr.decode("utf-8", errors="replace")
    print(f"exit={proc.returncode}")
    print(f"stderr_tail={stderr[-400:]!r}")
    if "UnicodeEncodeError" in stderr:
        print("VERDICT=REPRODUCED")
    else:
        print("VERDICT=ABSENT")


def cmd_crlf():
    """item 5 — geradores Python escrevem CRLF (open(path,'w') sem
    newline=). Roda `trackfw init --identity-preset none` de verdade (nao
    chama a funcao do gerador isoladamente) num diretorio limpo e varre os
    BYTES crus dos scripts .sh gerados — mesma metodologia que o autor da
    issue usou para medir o defeito.
    """
    tmp = tempfile.mkdtemp(prefix="trackfw-crlf-check-")
    try:
        env = dict(os.environ)
        env["PYTHONPATH"] = os.environ.get("TRACKFW_PYPI_SRC", "")
        proc = subprocess.run(
            [
                sys.executable,
                "-c",
                "from trackfw.cli import main; import sys; "
                "sys.argv=['trackfw','init','--identity-preset','none']; main()",
            ],
            cwd=tmp,
            env=env,
            capture_output=True,
            text=False,
        )
        print(f"init_exit={proc.returncode}")
        if proc.returncode != 0:
            print(f"stderr_tail={proc.stderr.decode('utf-8', errors='replace')[-400:]!r}")
            print("VERDICT=INCONCLUSIVE (init nao completou, ver stderr acima)")
            return

        sh_scripts = []
        for root, _dirs, files in os.walk(tmp):
            for name in files:
                if name.endswith(".sh"):
                    sh_scripts.append(os.path.join(root, name))

        if not sh_scripts:
            print("VERDICT=INCONCLUSIVE (nenhum .sh gerado por init nesta configuracao)")
            return

        crlf_count = 0
        for path in sh_scripts:
            with open(path, "rb") as fh:
                raw = fh.read()
            has_crlf = b"\r\n" in raw
            print(f"{os.path.relpath(path, tmp)}: crlf={has_crlf} bytes_sample={raw[:40]!r}")
            if has_crlf:
                crlf_count += 1

        print(f"scripts_checked={len(sh_scripts)} scripts_with_crlf={crlf_count}")
        print("VERDICT=REPRODUCED" if crlf_count > 0 else "VERDICT=ABSENT")
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


def cmd_isatty():
    """item 6 — sys.stdin.isatty() mente True para NUL no Windows. Roda
    `trackfw init` (SEM --identity-preset, SEM stdin conectado a um
    terminal real) e observa o crash real relatado na issue: entra no
    wizard de identidade em contexto nao interativo e morre com EOF ao ler
    uma linha. Nao usa monkeypatch nenhum — e a mesma condicao de um passo
    de CI real, onde stdin do processo filho vem de NUL/vazio.
    """
    tmp = tempfile.mkdtemp(prefix="trackfw-isatty-check-")
    try:
        env = dict(os.environ)
        env["PYTHONPATH"] = os.environ.get("TRACKFW_PYPI_SRC", "")
        proc = subprocess.run(
            [
                sys.executable,
                "-c",
                "from trackfw.cli import main; import sys; sys.argv=['trackfw','init']; main()",
            ],
            cwd=tmp,
            env=env,
            stdin=subprocess.DEVNULL,
            capture_output=True,
            text=False,
        )
        stderr = proc.stderr.decode("utf-8", errors="replace")
        print(f"exit={proc.returncode}")
        print(f"stderr_tail={stderr[-400:]!r}")
        if "EOF" in stderr or proc.returncode not in (0,):
            # distingue "morreu por causa do wizard" (defeito) de outra
            # causa qualquer de saida != 0 examinando a mensagem.
            if "EOF" in stderr or "identity" in stderr.lower() or "wizard" in stderr.lower():
                print("VERDICT=REPRODUCED")
            else:
                print("VERDICT=INCONCLUSIVE (saida != 0 por motivo nao relacionado ao wizard)")
        else:
            print("VERDICT=ABSENT (init completou sem entrar no wizard sob stdin=NUL)")
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


COMMANDS = {
    "help": cmd_help,
    "cp1252-print": cmd_cp1252_print,
    "crlf": cmd_crlf,
    "isatty": cmd_isatty,
}


def main():
    if len(sys.argv) < 2 or sys.argv[1] not in COMMANDS:
        print(f"uso: checks.py <{'|'.join(COMMANDS)}>", file=sys.stderr)
        sys.exit(2)
    COMMANDS[sys.argv[1]]()


if __name__ == "__main__":
    main()
