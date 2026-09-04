"""Resolução de `bash` por CAMINHO ABSOLUTO PROVADO para os sítios de teste que lançam
os scripts de guard via `subprocess`.

ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz, ML-0C.

## Por que existe

Medido no CI de Windows (run `33875124523`, ITEM 12 da sonda do ML-0B):

    shutil_which_bash = 'C:\\Program Files\\Git\\bin\\bash.EXE'   <- GNU bash, --version rc=0
    bare_rc           = 1
    bare_is_gnu_bash  = False
    bare_out          = UTF-16: "Windows Subsystem for Linux has no installed distributions."

`C:\\Windows\\System32\\bash.exe` é o **stub do WSL** e VENCE a resolução por nome nu. Sem
distribuição instalada ele sai **1** e fala em **UTF-16 pelo stdout** — o canal que os testes
descartam. Daí a assinatura que derrubou ~50 testes Python de Windows: `exit 1` uniforme (o caso
que devia sair 0 e o que devia sair 2 davam o MESMO 1) com `stderr` vazio. O guard nunca era
invocado: é defeito de HARNESS, não de segurança.

Go e Node passam porque entregam caminho absoluto ao `CreateProcess`; o CPython passa
`lpApplicationName = NULL` e cai na ordem implícita, onde `System32` vem antes de `Git\\bin`.

## Duas travas que decidem se o remédio funciona

1. **`shutil.which` sozinho NÃO é o remédio.** Ele varre o `%PATH%` na ORDEM DO PATH e devolve o
   binário certo — mas essa não é a ordem do `CreateProcess` com `lpApplicationName=NULL`. Ele
   serve para ACHAR o candidato; o que corrige é **passar o caminho absoluto** ao `subprocess`.
   Usar `which` e continuar passando `"bash"` não mudaria nada.

2. **Prove a IDENTIDADE, não a existência.** O discriminante entre "não achou" e "achou o errado"
   NÃO é o exit code — é `--version` contendo `GNU bash`. Um `bash.exe` que existe e não é bash é
   exatamente o defeito que se está corrigindo. A sonda do `--version` roda em BYTES de propósito:
   a saída UTF-16 do stub não casa com `b"GNU bash"` e o candidato é recusado sem depender de
   decodificação.

A exclusão explícita de `System32\\bash.exe` da lista de candidatos é cinto-e-suspensório: o portão
que carrega o peso é o de IDENTIDADE — o stub seria recusado mesmo sem a exclusão por caminho.

## POSIX

Em POSIX a resolução por nome nu já é inequívoca, e resolver por caminho absoluto não muda
comportamento observável (mesmo binário, mesma saída). O caminho de código é ÚNICO nas duas
plataformas de propósito: é o que torna a correção falsificável fora do Windows — dá para provar
localmente que o valor entregue ao `subprocess` deixou de ser a string `"bash"`.

## Falha

Se nenhum candidato for GNU bash, `bash_executable()` levanta `BashNotFound` NOMEANDO cada
candidato tentado e o que ele respondeu. Não existe `skip`: pular aqui trocaria um bloqueio visível
por um invisível. A resolução é preguiçosa (não em tempo de import) para que a falha apareça como
erro dos testes que realmente lançam bash, e não como erro de coleta da suíte inteira.
"""

import os
import shutil
import subprocess

__all__ = ["BashNotFound", "bash_executable", "bash_cmd"]

_VERSION_PROBE_TIMEOUT = 15


class BashNotFound(RuntimeError):
    """Nenhum candidato provou ser GNU bash. Carrega a lista do que foi tentado."""


def _system32_bash():
    """O stub do WSL. Nunca entra na lista de candidatos."""
    root = os.environ.get("SystemRoot") or os.environ.get("windir") or r"C:\Windows"
    return os.path.join(root, "System32", "bash.exe")


def _is_excluded(path):
    if os.name != "nt":
        return False
    return os.path.normcase(os.path.abspath(path)) == os.path.normcase(
        os.path.abspath(_system32_bash())
    )


def _path_dirs():
    raw = os.environ.get("PATH", "")
    return [d for d in raw.split(os.pathsep) if d]


def _candidates():
    """Candidatos em ordem, sem duplicatas e sem o stub do WSL.

    No Windows as instalações do Git for Windows vêm primeiro (é onde mora o GNU bash real do
    runner, conforme medido), depois a varredura do %PATH% — que ainda passa pelo portão de
    identidade, então a ordem é conveniência, não garantia.
    """
    seen = set()
    out = []

    def add(p):
        if not p:
            return
        p = os.path.abspath(p)
        key = os.path.normcase(p)
        if key in seen or _is_excluded(p) or not os.path.isfile(p):
            return
        seen.add(key)
        out.append(p)

    if os.name == "nt":
        for base in (
            os.environ.get("ProgramFiles", r"C:\Program Files"),
            os.environ.get("ProgramFiles(x86)", r"C:\Program Files (x86)"),
        ):
            add(os.path.join(base, "Git", "bin", "bash.exe"))
            add(os.path.join(base, "Git", "usr", "bin", "bash.exe"))
        names = ("bash.exe",)
    else:
        names = ("bash",)

    which = shutil.which("bash")
    if which and not _is_excluded(which):
        add(which)

    for d in _path_dirs():
        for name in names:
            add(os.path.join(d, name))

    if os.name != "nt":
        for p in ("/bin/bash", "/usr/bin/bash", "/usr/local/bin/bash", "/opt/homebrew/bin/bash"):
            add(p)

    return out


def _identity(path):
    """(is_gnu_bash, evidência curta). Roda em BYTES: a saída UTF-16 do stub do WSL não casa."""
    try:
        proc = subprocess.run(
            [path, "--version"],
            capture_output=True,
            timeout=_VERSION_PROBE_TIMEOUT,
        )
    except subprocess.TimeoutExpired:
        return False, "--version excedeu %ds" % _VERSION_PROBE_TIMEOUT
    except OSError as exc:
        return False, "OSError: %s" % exc
    blob = (proc.stdout or b"") + (proc.stderr or b"")
    first = blob.split(b"\n", 1)[0][:120]
    evidence = "rc=%d out=%r" % (proc.returncode, first)
    return (proc.returncode == 0 and b"GNU bash" in blob), evidence


_resolved = None
_failure = None


def bash_executable():
    """Caminho ABSOLUTO de um bash cuja identidade foi provada por `--version` (`GNU bash`).

    Levanta `BashNotFound` — nomeando cada candidato e sua resposta — se nenhum provar identidade.
    """
    global _resolved, _failure
    if _resolved is not None:
        return _resolved
    if _failure is not None:
        raise BashNotFound(_failure)

    tried = []
    for cand in _candidates():
        ok, evidence = _identity(cand)
        tried.append("  %s -> %s" % (cand, evidence))
        if ok:
            _resolved = cand
            return _resolved

    if not tried:
        tried.append("  (nenhum arquivo bash encontrado no %PATH% nem nos caminhos conhecidos)")
    _failure = (
        "nenhum candidato provou ser GNU bash (identidade por `--version`, nao por existencia).\n"
        "System32\\bash.exe (stub do WSL) e excluido por projeto.\n"
        "Candidatos tentados:\n" + "\n".join(tried)
    )
    raise BashNotFound(_failure)


def bash_cmd(*args):
    """argv para `subprocess`: bash absoluto provado + os argumentos dados.

    Substitui `["bash", script, ...]`, que no Windows resolve para o stub do WSL.
    """
    return [bash_executable(), *args]
