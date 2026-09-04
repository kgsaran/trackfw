"""Guarda de plataforma MEDIDA para os asserts de bit de execucao.

ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz, ML-4A.

## Por que existe

Em NTFS o bit de execucao nao existe: ``os.stat(p).st_mode & 0o111`` devolve 0 para TODO
arquivo, inclusive imediatamente depois de ``os.chmod(p, 0o755)``. Um assert "o artefato
gerado e executavel" nao mede o gerador ali -- mede uma propriedade que o sistema de
arquivos nao tem, e reprova sempre.

A decisao de suprimir a checagem apenas onde o bit nao e representavel esta tomada em
``vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01.md``:
o bit NUNCA foi discriminante em NTFS, e o WSL (kernel Linux, ext4) continua coberto porque
ali o bit e representavel de verdade.

## Detecao pela CONDICAO, nao por sys.platform

A sonda MEDE o filesystem em vez de inferir da plataforma -- mesmo idioma de
``_exec_bit_representavel`` em ``pypi/tests/test_validator.py``. Ela recebe o DIRETORIO onde
o teste escreve, porque e esse o filesystem sob medicao: um probe no ``tempfile.gettempdir()``
global mediria outro volume.

## Nao e skip, de proposito

O teste inteiro continua rodando; so o assert do bit e suprimido, e a supressao NOMEIA a
garantia que deixou de ser verificada. Um teste pulado nao mede mais que um teste que nao
existe.

## Toda saida e ASCII puro, de proposito

``vault/notes/gate-em-cp1252-tem-duas-falhas-distintas-crash-de-print-e-mismatch-por-
transcodificacao-2026-09-02``: sob ``cp1252`` (o default do console do Windows) um ``print``
com caractere fora da pagina crasha. A mensagem de supressao so tem sentido se ela sobreviver
exatamente no ambiente em que e emitida.
"""
import os
import tempfile
import warnings


class ExecBitNaoExercitado(UserWarning):
    """Categoria propria para a supressao do assert de bit de execucao.

    E `warnings.warn`, NAO `print`, e isso foi medido: o job de Windows roda
    `python -m pytest pypi/tests -q` (.github/workflows/quality.yml) SEM `-s`, e o pytest
    DESCARTA o stdout/stderr capturado de um teste que PASSA. Como estes testes passam no
    Windows justamente por causa da supressao, um `print` nunca chegaria ao log -- a garantia
    ficaria suprimida em silencio, que e o que este ML existe para impedir. O warnings summary
    do pytest aparece mesmo sob `-q` e mesmo com a suite inteira verde (medido).
    """


def exec_bit_representavel(diretorio=None):
    """Neste sistema de arquivos, um arquivo criado em `diretorio` e levado a 0o755 por
    os.chmod passa a ter st_mode & 0o111 != 0?

    Nao ha try/except em volta da medicao: se a propria sonda nao puder rodar, o erro sobe.
    "Nao consegui medir" nao pode virar supressao silenciosa dos asserts.
    """
    fd, p = tempfile.mkstemp(suffix=".sh", prefix="trackfw-execbit-probe-", dir=diretorio)
    os.close(fd)
    try:
        os.chmod(p, 0o755)
        return os.stat(p).st_mode & 0o111 != 0
    finally:
        os.remove(p)


def exec_bit_representavel_para(artefato):
    """Forma usada nos call sites: mede o filesystem do arquivo que esta sendo verificado."""
    return exec_bit_representavel(os.path.dirname(artefato) or None)


def exec_bit_nao_exercitado(artefato):
    """Registra, com tag grepavel, QUAL garantia deixou de ser verificada e por que.

    Quem ler o log do CI de Windows daqui a seis meses precisa do nome do artefato --
    nao de um "<script>" generico.
    """
    warnings.warn(
        'EXEC-BIT-NAO-EXERCITADO: {} -- garantia NAO verificada: "o artefato foi criado com o '
        'bit de execucao (0755)". Este sistema de arquivos devolve st_mode & 0o111 == 0 mesmo '
        "apos os.chmod(0o755) (NTFS nao representa o bit). O restante do teste continua "
        "medindo. Decisao: vault/notes/"
        "goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01.md".format(artefato),
        ExecBitNaoExercitado,
        stacklevel=2,
    )
