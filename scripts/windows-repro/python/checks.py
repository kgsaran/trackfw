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
    elif proc.returncode != 0 and _is_startup_failure(stderr):
        # O codigo medido (cli.py --help de topo) nao chegou a rodar — o
        # processo morreu ANTES do ponto onde o UnicodeEncodeError apareceria
        # (ex.: ModuleNotFoundError por drift de dependencia em pip install,
        # ver quality.yml). ABSENT aqui seria vacuidade: declarar "defeito
        # ausente" sobre um caminho que nunca foi exercitado.
        print("VERDICT=INCONCLUSIVE (processo morreu antes de alcancar o codigo medido — ver stderr_tail)")
    else:
        print("VERDICT=ABSENT")


def _is_startup_failure(stderr: str) -> bool:
    """True se o stderr indica que o processo morreu ANTES de alcancar o
    codigo medido (falha de import/startup), nao por causa do defeito sob
    medicao. Sinal escolhido: ModuleNotFoundError/ImportError (drift de
    dependencia — o gatilho concreto identificado na barreira de qualidade
    de 2026-08-30, quality.yml:374) OU um traceback de import do proprio
    interpretador ANTES da primeira linha de output esperada (import de
    trackfw.cli falhando no topo do -c). `returncode != 0` sozinho NAO
    basta — o CLI pode legitimamente sair != 0 (ex.: argparse.error) sem
    que isso signifique "nao rodou". Combinar com a string do traceback de
    import e o sinal mais especifico disponivel sem instrumentar o
    subprocesso; qualquer caso genuinamente ambiguo cai em INCONCLUSIVE por
    este mesmo caminho (nao ha ramo que force ABSENT quando o sinal e
    incerto), que e o lado seguro exigido pela barreira.
    """
    return (
        "ModuleNotFoundError" in stderr
        or "ImportError" in stderr
        or "Traceback (most recent call last)" in stderr
    )


def _is_cp1252_cascade(stderr: str) -> bool:
    """True se o stderr indica que `init` morreu no MESMO UnicodeEncodeError
    do item 1 (print de checkmark/seta em console cp1252), nao por outro
    motivo. Usado para distinguir BLOCKED-BY-ITEM-1 (cascata conhecida) de
    INCONCLUSIVE genuino (falha por causa diferente).
    """
    return "UnicodeEncodeError" in stderr


def cmd_crlf():
    """item 5 — geradores Python escrevem CRLF (open(path,'w') sem
    newline=). Roda `trackfw init --identity-preset none` de verdade (nao
    chama a funcao do gerador isoladamente) num diretorio limpo e varre os
    BYTES crus dos scripts .sh gerados — mesma metodologia que o autor da
    issue usou para medir o defeito.

    ML-1C: a primeira medida (ML-1A) veio INCONCLUSIVE porque `init` morre
    no MESMO UnicodeEncodeError cp1252 do item 1 (init_gen.py imprime
    'checkmark'/checar-arquivo com caracteres fora de cp1252) ANTES de
    terminar de escrever os .sh — a cascata mascara a medicao do item 5, que
    e sobre CRLF, nao sobre encoding de console. Para medir o item 5
    ISOLADO do item 1, neutralizamos o item 1 SO NESTE subprocesso via
    PYTHONIOENCODING=utf-8 (forca stdout/stderr do filho a UTF-8,
    independente da codepage do console) — isso nao muda nada sobre CRLF,
    que e uma questao de open() sem newline=, ortogonal a encoding de
    console. Documentado aqui e no stdout do check: esta medicao do item 5
    foi feita com o item 1 neutralizado, nao com o ambiente cru.
    """
    tmp = tempfile.mkdtemp(prefix="trackfw-crlf-check-")
    try:
        env = dict(os.environ)
        env["PYTHONPATH"] = os.environ.get("TRACKFW_PYPI_SRC", "")
        env["PYTHONIOENCODING"] = "utf-8"
        print("NOTE: item 1 neutralizado para esta medicao via PYTHONIOENCODING=utf-8 (ver docstring)")
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
            stderr = proc.stderr.decode("utf-8", errors="replace")
            print(f"stderr_tail={stderr[-400:]!r}")
            if _is_cp1252_cascade(stderr):
                print("VERDICT=BLOCKED-BY-ITEM-1 (init ainda morreu em UnicodeEncodeError mesmo com PYTHONIOENCODING=utf-8 — dependencia nao desacoplavel por este mecanismo)")
            else:
                print("VERDICT=INCONCLUSIVE (init nao completou por motivo nao relacionado ao item 1, ver stderr acima)")
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

    ML-1C: mesma cascata do item 5 — `init` (sem --identity-preset) tambem
    passa por init_gen.py apos o wizard (ou antes de alcancar o ponto de
    leitura, dependendo do caminho), e pode morrer no MESMO
    UnicodeEncodeError cp1252 do item 1 antes do EOF do wizard ser
    observavel. Mesma neutralizacao do item 5: PYTHONIOENCODING=utf-8 SO
    neste subprocesso, documentada aqui e no stdout. Isto nao afeta a
    deteccao de isatty() em si (e uma questao de terminal/TTY, ortogonal a
    encoding de console) — so evita que o item 1 mascare a medicao.
    """
    tmp = tempfile.mkdtemp(prefix="trackfw-isatty-check-")
    try:
        env = dict(os.environ)
        env["PYTHONPATH"] = os.environ.get("TRACKFW_PYPI_SRC", "")
        env["PYTHONIOENCODING"] = "utf-8"
        print("NOTE: item 1 neutralizado para esta medicao via PYTHONIOENCODING=utf-8 (ver docstring)")
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
            # distingue "morreu por causa do wizard" (defeito) de "ainda
            # morreu no item 1 apesar da neutralizacao" de outra causa
            # qualquer de saida != 0.
            if "EOF" in stderr or "identity" in stderr.lower() or "wizard" in stderr.lower():
                print("VERDICT=REPRODUCED")
            elif _is_cp1252_cascade(stderr):
                print("VERDICT=BLOCKED-BY-ITEM-1 (init ainda morreu em UnicodeEncodeError mesmo com PYTHONIOENCODING=utf-8 — dependencia nao desacoplavel por este mecanismo)")
            else:
                print("VERDICT=INCONCLUSIVE (saida != 0 por motivo nao relacionado ao wizard nem ao item 1)")
        else:
            print("VERDICT=ABSENT (init completou sem entrar no wizard sob stdin=NUL)")
    finally:
        shutil.rmtree(tmp, ignore_errors=True)


# ROADMAP-2026-09-05-retarget-dos-checks-de-camada-2 (ML-2C, ML-2D):
# "cp1252-print" e "gatequote" foram REMOVIDOS. Ambos replicavam mecanismos
# de producao (o gate .sh real / pypi/trackfw/commands/barrier.py) fora do
# artefato real — exatamente o padrao que este roadmap corrige. O item 4 do
# run.ps1 agora invoca scripts/check-parity-contract-coverage.sh (o .sh
# real) via `bash`; o item 7 agora invoca `trackfw barrier` de verdade (via
# `python -m trackfw`), nao mais este arquivo.
COMMANDS = {
    "help": cmd_help,
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
