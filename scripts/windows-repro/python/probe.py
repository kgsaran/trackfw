"""probe.py — sonda sob demanda (ADR-2026-08-30, decisao 3 / AC5 / AC6 / AC9),
braco Python. Espelha scripts/windows-repro/go/probe.go: imprime o valor
BRUTO que o SO devolveu para cada pergunta e para ai — sem veredito, sem
comparacao contra "esperado". Quem le decide o que significa.

NAO E checks.py (camada 2, mapeada aos 11 itens da issue #216, COM veredito
REPRODUCED/ABSENT) — ver windows-probe.yml para a distincao. checks.py nunca
deve importar as funcoes deste arquivo.

Introduzido por ROADMAP-2026-08-30-sonda-mede-junction-nos-3-runtimes-e-a-
pergunta-7-volta-a-responder.md, ML-1A, para responder AC4/AC5 da
REQ-2026-08-30 (junction em Python nunca havia sido medida).

Executado via `python scripts/windows-repro/python/probe.py <subcomando>`.
"""
import os
import stat
import subprocess
import sys
import tempfile
from pathlib import Path


def _print_tempdir_info(tmp):
    # Imprime o diretorio temporario que de fato foi resolvido ao lado de
    # RUNNER_TEMP — achado do ML-0A (hades-tf): tempfile.mkdtemp() resolve
    # via TMPDIR/TEMP/TMP, que NAO e RUNNER_TEMP. Medir em vez de presumir.
    print(f"tempdir_resolvido={tmp} runner_temp={os.environ.get('RUNNER_TEMP', '')}")


def _print_lstat(label, target):
    try:
        st = os.lstat(target)
    except OSError as err:
        print(f"{label} path={target} err={err}")
        return
    print(
        f"{label} path={target} islink={os.path.islink(target)} "
        f"S_ISLNK={stat.S_ISLNK(st.st_mode)} st_mode={oct(st.st_mode)}"
    )
    try:
        readlink_target = os.readlink(target)
        print(f"{label} readlink={readlink_target!r}")
    except OSError as err:
        print(f"{label} readlink_err={err}")


# Pergunta (referencia) — os.lstat sobre um arquivo comum, para comparacao
# com symlink e junction abaixo. Mesmo papel de cmdLstatCommon em probe.go.
def cmd_lstat_common():
    tmp = tempfile.mkdtemp(prefix="trackfw-probe-common-")
    common = os.path.join(tmp, "common.txt")
    with open(common, "w") as f:
        f.write("trackfw-probe-target\n")
    _print_lstat("lstat-common", common)


# Pergunta — os.lstat sobre um symlink REAL (os.symlink). Em windows-latest
# sem Developer Mode/admin, a criacao costuma falhar com OSError (privilegio
# ausente) — sinal em si, impresso cru, sem contornar (mesmo padrao de
# probe.go/cmdLstatSymlink).
def cmd_lstat_symlink():
    tmp = tempfile.mkdtemp(prefix="trackfw-probe-symlink-")
    _print_tempdir_info(tmp)
    target = os.path.join(tmp, "target.txt")
    with open(target, "w") as f:
        f.write("trackfw-probe-target\n")
    link = os.path.join(tmp, "link.txt")
    try:
        os.symlink(target, link)
    except OSError as err:
        print(
            f"lstat-symlink create_err={err} "
            "(esperado sem Developer Mode/admin — sinal em si, nao falha da sonda)"
        )
        return
    _print_lstat("lstat-symlink", link)


def _mklink_junction(junction, target_dir):
    """Cria junction via `cmd /c mklink /J` — Python nao tem primitivo nativo
    de junction (so os.symlink, que produz IO_REPARSE_TAG_SYMLINK, nao
    IO_REPARSE_TAG_MOUNT_POINT). Mesmo mecanismo usado por
    probe.go/cmdLstatJunction. Devolve o CompletedProcess bruto — quem chama
    decide o que o retorno significa."""
    try:
        return subprocess.run(
            ["cmd", "/c", "mklink", "/J", junction, target_dir],
            capture_output=True,
            text=True,
        )
    except OSError as err:
        # cmd.exe ausente (ex.: execucao fora do Windows) — sinal em si,
        # devolvido como um CompletedProcess sintetico em vez de derrubar a
        # sonda inteira.
        return subprocess.CompletedProcess(
            args=["cmd", "/c", "mklink", "/J", junction, target_dir],
            returncode=127,
            stdout="",
            stderr=f"err_spawn_cmd={err}",
        )


# Pergunta central desta extensao (AC4 da REQ) — os.lstat sobre uma
# JUNCTION criada por `mklink /J`, que nao exige privilegio, ao contrario do
# symlink real acima. A pergunta bruta: os.path.islink()/S_ISLNK() marcam
# esse reparse point como link, ou nao?
def cmd_lstat_junction():
    tmp = tempfile.mkdtemp(prefix="trackfw-probe-junction-")
    _print_tempdir_info(tmp)
    target_dir = os.path.join(tmp, "targetdir")
    os.mkdir(target_dir)
    junction = os.path.join(tmp, "junctionlink")
    proc = _mklink_junction(junction, target_dir)
    print(
        f"lstat-junction mklink_output={(proc.stdout + proc.stderr)!r} "
        f"mklink_returncode={proc.returncode}"
    )
    if proc.returncode != 0:
        print("lstat-junction create_failed — sonda nao pode medir lstat sobre a junction nesta execucao")
        return
    _print_lstat("lstat-junction", junction)

    # Comparacao direta: os.stat (segue o link) sobre o mesmo caminho — mesmo
    # formato comparativo que o braco Go usa (stat-junction).
    try:
        st = os.stat(junction)
        print(f"stat-junction(segue o link) path={junction} S_ISDIR={stat.S_ISDIR(st.st_mode)}")
    except OSError as err:
        print(f"stat-junction(segue o link) path={junction} err={err}")


# rmdir-junction — Pergunta 10 do workflow (ML-1A). Path.rmdir() sobre uma
# junction cujo alvo esta VAZIO — MESMO primitivo que a producao usa
# (pypi/trackfw/integrations/manager.py:589 _remove_empty chama
# `directory.rmdir()` sobre um pathlib.Path, nao `os.rmdir()` sobre uma
# string; embora seja o mesmo syscall por baixo, medir com o objeto exato da
# producao evita que alguem alegue "a sonda mediu uma chamada diferente").
# _remove_empty depende SO de `except OSError` ao redor do rmdir() para
# parar de subir removendo ancestrais — este e literalmente o dado que
# decide "Python para" vs "Python sobe removendo diretorios do usuario".
def cmd_rmdir_junction():
    tmp = tempfile.mkdtemp(prefix="trackfw-probe-rmdir-")
    _print_tempdir_info(tmp)
    target_dir = os.path.join(tmp, "targetdir")
    os.mkdir(target_dir)
    junction = os.path.join(tmp, "junctionlink")
    proc = _mklink_junction(junction, target_dir)
    if proc.returncode != 0:
        print(f"rmdir-junction create_err={(proc.stdout + proc.stderr)!r}")
        print("rmdir-junction create_failed — sonda nao pode medir rmdir sobre a junction nesta execucao")
        return

    try:
        Path(junction).rmdir()
        remove_err = None
    except OSError as err:
        remove_err = str(err)
    print(f"rmdir-junction Path(junction).rmdir()_err={remove_err}")

    junction_still_exists = os.path.lexists(junction)
    print(f"rmdir-junction junction_ainda_existe={junction_still_exists}")

    target_still_exists = os.path.isdir(target_dir)
    print(f"rmdir-junction alvo_ainda_existe={target_still_exists}")


def _print_table_row(target, path):
    try:
        st = os.lstat(path)
    except OSError as err:
        print(f"TABELA runtime=python target={target} err={err}")
        return
    print(
        f"TABELA runtime=python target={target} islink={os.path.islink(path)} "
        f"S_ISLNK={stat.S_ISLNK(st.st_mode)} S_ISDIR={stat.S_ISDIR(st.st_mode)}"
    )


# table — Pergunta 11 do workflow. Recria arquivo comum, symlink e junction
# do zero (fixture propria, isolada das perguntas anteriores) e imprime uma
# linha TABELA por alvo, mesmo prefixo/formato que probe.go e probe.js usam,
# para comparacao lado a lado no mesmo step do workflow (AC5). Sem veredito
# (AC6): so os bits crus que este runtime usa para decidir "e link?".
def cmd_table():
    tmp = tempfile.mkdtemp(prefix="trackfw-probe-table-")
    _print_tempdir_info(tmp)

    common = os.path.join(tmp, "common.txt")
    with open(common, "w") as f:
        f.write("x")
    _print_table_row("arquivo", common)

    target = os.path.join(tmp, "target.txt")
    with open(target, "w") as f:
        f.write("x")
    link = os.path.join(tmp, "link.txt")
    try:
        os.symlink(target, link)
        _print_table_row("symlink", link)
    except OSError as err:
        print(f"TABELA runtime=python target=symlink err_create={err}")

    target_dir = os.path.join(tmp, "targetdir")
    os.mkdir(target_dir)
    junction = os.path.join(tmp, "junctionlink")
    proc = _mklink_junction(junction, target_dir)
    if proc.returncode == 0:
        _print_table_row("junction", junction)
    else:
        print(f"TABELA runtime=python target=junction err_create={(proc.stdout + proc.stderr)!r}")


COMMANDS = {
    "lstat-common": cmd_lstat_common,
    "lstat-symlink": cmd_lstat_symlink,
    "lstat-junction": cmd_lstat_junction,
    "rmdir-junction": cmd_rmdir_junction,
    "table": cmd_table,
}


def main():
    if len(sys.argv) < 2 or sys.argv[1] not in COMMANDS:
        print(f"uso: probe.py <{'|'.join(COMMANDS)}>", file=sys.stderr)
        sys.exit(2)
    COMMANDS[sys.argv[1]]()


if __name__ == "__main__":
    main()
