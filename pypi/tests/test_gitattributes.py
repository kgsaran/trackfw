"""ML-1A (ROADMAP-2026-09-02-gitattributes-com-merge-union-para-o-trackfw-log-nos-3-clis).

Os tres ramos de generate_gitattributes. O gate scripts/check-artifact-parity.sh
cobre so o ramo de CRIACAO cross-runtime; o de APPEND (projeto que ja tem
.gitattributes) so existe aqui e nos equivalentes Go/Node.
"""

import os

from trackfw.generators.init_gen import GITATTRIBUTES_BLOCK, generate_gitattributes

REPO_ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))


def _read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def _write(path, content):
    with open(path, "w", encoding="utf-8", newline="\n") as f:
        f.write(content)


def test_cria_quando_ausente_e_e_idempotente(tmp_path):
    generate_gitattributes(str(tmp_path))
    target = os.path.join(str(tmp_path), ".gitattributes")
    first = _read(target)
    assert first == GITATTRIBUTES_BLOCK
    generate_gitattributes(str(tmp_path))
    assert _read(target) == first, "init duas vezes nao pode duplicar a regra"


def test_append_nao_gruda_na_ultima_linha_sem_newline_final(tmp_path):
    target = os.path.join(str(tmp_path), ".gitattributes")
    _write(target, "* text=auto")
    generate_gitattributes(str(tmp_path))
    want = "* text=auto\n" + GITATTRIBUTES_BLOCK
    assert _read(target) == want
    generate_gitattributes(str(tmp_path))
    assert _read(target) == want, "segunda execucao duplicou a regra"


def test_nao_sobrescreve_regra_preexistente_com_outro_espacamento(tmp_path):
    target = os.path.join(str(tmp_path), ".gitattributes")
    existing = ".trackfw-log  merge=union\n"
    _write(target, existing)
    generate_gitattributes(str(tmp_path))
    assert _read(target) == existing


def test_comentario_nao_conta_como_regra(tmp_path):
    target = os.path.join(str(tmp_path), ".gitattributes")
    _write(target, "# .trackfw-log merge=union\n")
    generate_gitattributes(str(tmp_path))
    assert _read(target) == "# .trackfw-log merge=union\n" + GITATTRIBUTES_BLOCK


def test_bloco_contido_no_gitattributes_versionado_na_raiz():
    # CONTENCAO, nao igualdade: o `init` ANEXA o bloco a um `.gitattributes` que ja
    # exista, entao exigir o arquivo inteiro proibiria este repositorio de ter
    # qualquer regra propria — inclusive `*.go text eol=lf`, sem a qual um checkout
    # com core.autocrlf=true traz CRLF em todo .go. Medido: 0 de 213 com a regra,
    # 213 de 213 sem ela. A contencao continua pegando deriva do bloco.
    # Normaliza o fim de linha dos DOIS lados por simetria com Go e Node. Aqui o
    # open() em modo texto ja traduz sozinho — e era por isso que o Python passava
    # no Windows enquanto os outros dois reprovavam, sobre o MESMO arquivo.
    def _norm(text):
        return text.replace("\r\n", "\n")
    
    assert _norm(GITATTRIBUTES_BLOCK) in _norm(_read(os.path.join(REPO_ROOT, ".gitattributes")))
