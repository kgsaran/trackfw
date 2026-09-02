#!/usr/bin/env bash
# check-output-encoding-declared.sh — gate anti-reintroducao da Wave 2 do
# ROADMAP-2026-09-02-saida-nao-ascii-declara-codificacao-em-script-gerado-e-em-gate
# (ML-2A). Dois alvos, medidos separadamente, com guarda de vacuidade em cada um.
#
# ALVO 1 — FERRAMENTA. Todo scripts/check-*.sh que invoca python3 declara
# `export PYTHONIOENCODING=utf-8` ANTES da primeira invocacao. Sem isso, sob
# console cp1252 (Windows) o gate reprova por um motivo alheio ao que mede: ou
# crasha (UnicodeEncodeError no print de caractere fora do cp1252) ou — o modo
# perigoso — NAO crasha e devolve mismatch, porque um caractere definido em
# cp1252 (o em-dash U+2014 vira o byte 0x97) transcodifica no canal python->bash
# e quebra a comparacao byte-a-byte. Os dois modos foram medidos ao vivo no
# ML-1B; ver vault/notes/gate-em-cp1252-tem-duas-falhas-distintas-crash-de-
# print-e-mismatch-por-transcodificacao-2026-09-02.md.
#
# ALVO 2 — PRODUTO, e e o alvo que a paridade NAO cobre. O literal
# `attentionSignalScript`, embutido byte-identico nos 3 CLIs
# (internal/generators/scaffold.go, npm/src/generators/hooks.js,
# pypi/trackfw/generators/init_gen.py) e escrito em
# scripts/trackfw-attention-signal.sh na maquina de quem adota o trackfw,
# invoca `python3 -c` no ramo sem jq. Essas invocacoes tem de manter o prefixo
# PYTHONIOENCODING=utf-8 (ML-1A).
#   Por que isto NAO e redundante com check-attention-scripts-parity.sh: aquele
#   gate compara os 3 CLIs ENTRE SI. Se alguem remover o prefixo dos TRES, ele
#   continua verde — paridade mede se as implementacoes concordam, nao se o
#   contrato esta correto. Mesmo cego ja registrado em
#   vault/notes/barrier-so-casa-cabecalho-de-aceite-em-portugues-2026-08-28.md.
#   Falsificado por execucao: com o prefixo removido dos 3, o gate de paridade
#   devolve exit 0 e este devolve exit 1.
#
# ------------------------------------------------------------------------------
# FORMA ACEITA E FORMA RECUSADA — decisao explicita, nao acidente de regex.
#
# Le vault/notes/gate-literal-regex-syntax-equivalent-bypass-2026-09-01.md antes
# de mexer aqui: um gate de regex literal reprova quem esta certo (sintaxe
# equivalente) e passa quem esta errado (mencao morta em comentario/heredoc).
# As duas metades sao tratadas.
#
# ACEITO no alvo 1 (assignment exportado, em linha de CODIGO):
#   export PYTHONIOENCODING=utf-8        <- forma canonica da arvore hoje (37/37)
#   export   PYTHONIOENCODING=UTF-8      <- espacos extras, caixa alta NO VALOR
#   export PYTHONIOENCODING="utf-8"      <- aspas duplas ou simples
#   export PYTHONIOENCODING=utf8         <- aliases do codec utf_8 do Python:
#   export PYTHONIOENCODING=utf_8            utf-8 / utf8 / utf_8 / u8
#   export PYTHONIOENCODING=u8
#   export PYTHONIOENCODING=utf-8:strict <- unico handler de erro aceito, e ele
#       e o default. Ver a recusa dos demais handlers abaixo.
#
# RECUSADO no alvo 1, e o motivo de cada recusa:
#   - NOME da variavel em outra caixa (`export pythonioencoding=utf-8`). Nome de
#     variavel em shell POSIX e case-SENSITIVE: `pythonioencoding` e OUTRA
#     variavel, que o Python nunca le. A declaracao teria efeito ZERO. A
#     insensibilidade de caixa vale so para o VALOR — os aliases de codec do
#     Python sao case-insensitive —, e por isso o `(?i:...)` esta aplicado
#     apenas ao grupo de aliases, e nao a regex inteira. (Um `re.IGNORECASE`
#     global aqui aceitava, ate o ML-2B, `pythonioencoding=utf-8` como conforme
#     nos DOIS alvos, inclusive no alvo 2, que e produto.)
#   - QUALQUER handler de erro que nao seja `strict` (`:replace`,
#     `:backslashreplace`, `:surrogatepass`, `:surrogateescape`, ...). O
#     comentario anterior a este afirmava que o handler "nunca dispara, porque
#     com encoding utf-8 nenhum str do Python e inencodavel". ISSO E FALSO e foi
#     medido: `json.load` PRESERVA surrogate solto vindo de escape `\udXXX`
#     (JSON permite; o modulo json nao valida pareamento), e surrogate solto E
#     inencodavel em utf-8. Com `utf-8:surrogatepass`, "a\ud800b" sai como os
#     bytes `61 ed a0 80 62` — UTF-8 INVALIDO gravado no artefato; com utf-8
#     estrito o mesmo dado estoura UnicodeEncodeError e cai no fallback limpo.
#     Segundo motivo, independente e mais amplo: o handler do PYTHONIOENCODING
#     vale tambem para o DECODE do stdin, nao so para o encode do stdout. Logo
#     `utf-8:replace` produziria exatamente a corrupcao silenciosa que o ML-0A
#     mediu e reprovou (byte indefinido virando U+FFFD sem erro). Aceitar
#     handler seria o gate homologando o que a Wave 0 decidiu recusar.
#     Consequencia medida no consumidor, em
#     vault/notes/handler-de-erro-em-pythonioencoding-reintroduz-byte-invalido-
#     e-os-3-serve-divergem-2026-09-02.md: o mesmo artefato invalido faz Go e
#     Node responderem `active:true` com U+FFFD e o Python responder
#     `active:false` — o banner some em UM dos 3 runtimes, sem crash nem log.
#     `:strict` e aceito por ser explicitamente o default (nao muda nada); a
#     caixa dele e minuscula de proposito, porque o proprio Python recusa
#     `STRICT` (`codecs.lookup_error('STRICT')` -> unknown error handler name).
#     Zero falso positivo: os 37 gates da arvore usam `utf-8` puro.
#   - valor que nao seja alias de utf_8 (ex.: PYTHONIOENCODING=cp1252). E a
#     regressao mais barata de escrever e a que um `grep -q PYTHONIOENCODING`
#     ingenuo deixaria passar. Falsificada por execucao.
#   - declaracao apenas em linha de COMENTARIO (`^\s*#`) ou dentro de corpo de
#     heredoc: mencao morta, nao tem efeito no ambiente. E a "metade positiva"
#     do achado de 2026-09-01 — assert_count que nao exclui comentario.
#   - declaracao APOS a primeira invocacao de python3 no arquivo: sintaticamente
#     presente, semanticamente inutil para aquela invocacao. E a checagem de
#     ordem que torna a assercao semantica em vez de textual.
#   - assignment SEM `export` (`PYTHONIOENCODING=utf-8` solto): nao chega ao
#     ambiente do filho. RECUSADO de proposito.
#   - forma de prefixo por invocacao (`PYTHONIOENCODING=utf-8 python3 ...`):
#     semanticamente VALIDA, mas recusada no alvo 1 por decisao, nao por
#     descuido — asseverar "toda invocacao tem prefixo" exigiria parsear pipeline
#     de bash, e nenhum dos 37 gates usa essa forma hoje (zero falso positivo na
#     arvore atual). Quem quiser usa-la num gate novo tem de adicionar tambem o
#     `export`. No ALVO 2 e o inverso: la a forma de prefixo e a UNICA aceita,
#     porque o literal e um heredoc de script gerado onde `export` vazaria para
#     o resto do hook do usuario.
#
# COBERTURA PARCIAL, declarada: a assercao e estatica sobre o texto-fonte. Ela
# prova que a declaracao existe, e exportada, tem valor utf_8 e precede a
# primeira invocacao. NAO prova por observacao de runtime que o python3 daquele
# gate enxergou utf-8 — isso exigiria executar os 38 gates com um python3
# instrumentado, e dois deles (check-gates-falsify ~3m, check-barrier, que faz
# git) tornam isso inviavel no caminho de `make parity`. O mecanismo foi provado
# por execucao uma vez, com stub de python3, e esta no relatorio do ML-2A.
# Anotado como `partial=` em docs/cli-parity.md, nunca `gate=` puro.
# ------------------------------------------------------------------------------
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate): forca UTF-8 no stdio de todo
# python3 deste gate. Este gate se aplica A SI MESMO — ele proprio e um
# scripts/check-*.sh que invoca python3, entao a linha abaixo e asseverada
# pela sua propria varredura (ha uma checagem explicita de auto-inclusao na
# populacao, para que remover esta linha faca o gate se nomear).
export PYTHONIOENCODING=utf-8

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

SELF_BASENAME=$(basename "${BASH_SOURCE[0]}")

# Allowlist do ALVO 1 — excecao UNICA e NOMEADA, nunca skip silencioso.
#
# scripts/check-roadmap-barrier-contract.sh: o PR #238 de lourivalgarciajunior
# esta ABERTO sobre exatamente este sitio de codigo — o heredoc que escreve o
# $CORPUS_LINES_FILE cujo sha vira o CORPUS_HASH. Sob cp1252 o gate estoura
# (UnicodeEncodeError '✅' na linha 523), mas forcar a codificacao aqui
# MATARIA O CRASH SEM tornar o CORPUS_HASH independente do SO — o defeito de
# fundo e o hash depender da codificacao, e e esse que o #238 corrige. Aplicar
# o remedio parcial agora seria tomar trabalho ja feito por ele enquanto o
# seguramos por processo. Medido e registrado no ML-1B (42 de 43).
# Quando o #238 fechar: remover desta lista. Este gate AVISA sozinho quando
# isso acontecer — se o arquivo passar a ter a declaracao, ele reprova com
# "excecao obsoleta" em vez de aceitar os dois estados.
ALLOWLIST=(
  "scripts/check-roadmap-barrier-contract.sh"
)

# Alvo 2 — os 3 arquivos-fonte que embutem o literal attentionSignalScript.
ATTENTION_SOURCES=(
  "internal/generators/scaffold.go"
  "npm/src/generators/hooks.js"
  "pypi/trackfw/generators/init_gen.py"
)

python3 - "$SELF_BASENAME" "${#ALLOWLIST[@]}" "${ALLOWLIST[@]}" "${ATTENTION_SOURCES[@]}" <<'PYEOF'
import glob
import os
import re
import sys

argv = sys.argv[1:]
self_basename = argv[0]
n_allow = int(argv[1])
allowlist = argv[2:2 + n_allow]
attention_sources = argv[2 + n_allow:]

failures = []
notes = []

# Aliases do codec utf_8 do Python (encodings/aliases.py), mais o proprio nome.
# O `(?i:...)` cobre SO o valor: alias de codec e case-insensitive no Python,
# mas o NOME da variavel de ambiente e case-sensitive no shell. Nenhuma das tres
# regexes abaixo leva re.IGNORECASE global — ver a recusa de caixa no cabecalho.
UTF8_ALIASES = r"(?i:utf-8|utf8|utf_8|u8)"

# Handler de erro: so o default explicito. Minusculo de proposito — o Python
# recusa `STRICT`. Ver a recusa de handler no cabecalho (surrogate solto de
# json.load + handler valendo tambem para o decode do stdin).
UTF8_HANDLER = r"(?::strict)?"

# ALVO 1: assignment exportado, valor alias de utf_8, aspas e espacos tolerados,
# handler de erro opcional e restrito a `strict`.
DECL_RE = re.compile(
    r'^[ \t]*export[ \t]+PYTHONIOENCODING=(?P<q>["\']?)'
    + UTF8_ALIASES
    + UTF8_HANDLER
    + r'(?P=q)[ \t]*(?:#.*)?$',
)
# Qualquer assignment da variavel, com valor QUALQUER — usado so para
# diagnosticar "declarada, mas com valor errado" em vez de "ausente". Sem
# IGNORECASE: `pythonioencoding=` e outra variavel, e chama-la de "forma nao
# aceita" seria mais generoso do que ela merece — cai em "NAO declara".
ANY_ASSIGN_RE = re.compile(r'^[ \t]*(?:export[ \t]+)?PYTHONIOENCODING=')
PY3_RE = re.compile(r'\bpython3\b')
COMMENT_RE = re.compile(r'^[ \t]*#')

HEREDOC_RE = re.compile(r'<<-?[ \t]*(["\']?)([A-Za-z_][A-Za-z0-9_]*)\1')


def population_lines(path):
    """Predicado de POPULACAO (loose): exclui SO a linha inteiramente
    comentada, sem rastrear heredoc.

    Por que este predicado e diferente do de DECLARACAO (ML-2B, achado B1):
    HEREDOC_RE procura o delimitador na linha inteira, entao um comentario
    INLINE numa linha de codigo — `true  # exemplo: <<EOF` — liga o estado de
    heredoc e faz `code_lines` descartar TODO o resto do arquivo. Do lado da
    declaracao isso e conservador (menos linhas candidatas = mais dificil
    passar). Do lado da populacao e FAIL-OPEN: o arquivo inteiro sai da
    varredura e sua falta de declaracao deixa de ser vista. Medido: com o
    comentario inline mais a remocao do `export` de um gate, a arvore mutada
    passava com exit 0. A arvore ja tem cinco comentarios com `<<`, inertes so
    por serem de linha inteira — uma reflow de paragrafo os armaria.

    Trade-off assumido, na direcao segura: uma mencao MORTA a `python3` dentro
    de corpo de heredoc passa a colocar o arquivo na populacao e a exigir dele
    a declaracao. Isso e falso positivo, ruidoso e FECHADO — reprova pedindo
    uma linha inofensiva —, ao contrario do falso negativo que substitui. Hoje
    nao ocorre: as duas populacoes coincidem (38 = 38, delta vazio nas duas
    direcoes)."""
    with open(path, encoding="utf-8", errors="replace") as fh:
        return [
            (i, raw)
            for i, raw in enumerate(fh.read().splitlines(), start=1)
            if not COMMENT_RE.match(raw)
        ]


def code_lines(path):
    """Predicado de DECLARACAO (strict). Devolve [(lineno, texto)] das linhas de
    CODIGO do script: exclui linhas de comentario e todo corpo de heredoc. As
    duas exclusoes existem porque mencao morta (comentario, corpo de heredoc)
    nao tem efeito no ambiente — e a 'metade positiva' do achado de 2026-09-01.
    NAO use isto para decidir populacao: ver population_lines."""
    out = []
    delim = None
    with open(path, encoding="utf-8", errors="replace") as fh:
        for i, raw in enumerate(fh.read().splitlines(), start=1):
            if delim is not None:
                if raw.strip() == delim:
                    delim = None
                continue
            if COMMENT_RE.match(raw):
                continue
            out.append((i, raw))
            m = HEREDOC_RE.search(raw)
            if m:
                delim = m.group(2)
    return out


# ---------------------------------------------------------------- ALVO 1
gate_paths = sorted(glob.glob("scripts/check-*.sh"))

# GUARDA DE VACUIDADE (a): o glob tem de enumerar alguma coisa. Um glob que
# deixa de casar (pasta renomeada, cwd errado) e a forma mais comum de um gate
# virar decorativo.
if not gate_paths:
    failures.append(
        "ALVO 1 vacuo: o glob scripts/check-*.sh enumerou ZERO arquivos "
        "(cwd=%s). Recuso passar em silencio." % os.getcwd()
    )
else:
    invokers = []
    for path in gate_paths:
        # POPULACAO: predicado loose (ML-2B/B1).
        first_py3 = next(
            (n for n, t in population_lines(path) if PY3_RE.search(t)), None
        )
        if first_py3 is None:
            continue
        invokers.append(path)
        if path in allowlist:
            continue
        # DECLARACAO: predicado strict, com exclusao de heredoc.
        lines = code_lines(path)
        decl = next((n for n, t in lines if DECL_RE.match(t)), None)
        if decl is None:
            wrong = next((n for n, t in lines if ANY_ASSIGN_RE.match(t)), None)
            if wrong is not None:
                failures.append(
                    "%s: invoca python3 (linha %d) e atribui PYTHONIOENCODING na "
                    "linha %d, mas a forma nao e aceita — exige "
                    "`export PYTHONIOENCODING=<alias de utf_8>[:strict]`; "
                    "assignment sem export nao chega ao filho, valor nao-utf8 "
                    "nao corrige nada e handler de erro diferente de `strict` "
                    "reintroduz o defeito (encode de surrogate solto e decode "
                    "de byte invalido no stdin)."
                    % (path, first_py3, wrong)
                )
            else:
                failures.append(
                    "%s: invoca python3 (linha %d) e NAO declara "
                    "`export PYTHONIOENCODING=utf-8`." % (path, first_py3)
                )
        elif decl > first_py3:
            failures.append(
                "%s: declara PYTHONIOENCODING na linha %d, DEPOIS da primeira "
                "invocacao de python3 (linha %d) — a declaracao nao alcanca "
                "aquela invocacao." % (path, decl, first_py3)
            )

    # GUARDA DE VACUIDADE (b): a populacao de invocadores nao pode ser vazia.
    if not invokers:
        failures.append(
            "ALVO 1 vacuo: %d arquivos casaram scripts/check-*.sh, mas NENHUM "
            "foi classificado como invocador de python3. O predicado de "
            "populacao quebrou; recuso passar em silencio." % len(gate_paths)
        )

    # GUARDA DE VACUIDADE (c) / AUTO-APLICACAO: este gate e ele proprio um
    # scripts/check-*.sh que invoca python3. Se ele nao aparecer na propria
    # populacao, o predicado esta furado — e se aparecer, ele foi checado
    # pelas mesmas regras dos outros (remover o export daqui faz o gate se
    # nomear na lista de infratores).
    self_path = os.path.join("scripts", self_basename)
    if self_path not in invokers:
        failures.append(
            "ALVO 1: o proprio %s nao aparece na populacao de invocadores de "
            "python3 — o gate deixou de se aplicar a si mesmo." % self_path
        )

    # ALLOWLIST — tres asserçoes, nao uma.
    for allowed in allowlist:
        # (a) o caminho existe. Um rename faz a excecao proteger o nada,
        #     mesma classe de falha do `gate=` apontando caminho inexistente
        #     em check-parity-contract-coverage.sh.
        if not os.path.isfile(allowed):
            failures.append(
                "ALLOWLIST: %s nao existe no disco. A excecao esta protegendo "
                "um caminho morto — corrija o nome ou remova a entrada."
                % allowed
            )
            continue
        # (b) o arquivo continua SEM a declaracao. Se ganhou uma, a excecao
        #     ficou obsoleta e tem de ser removida — nao aceitar os dois estados.
        alines = code_lines(allowed)
        if any(DECL_RE.match(t) for _n, t in alines):
            failures.append(
                "ALLOWLIST OBSOLETA: %s agora declara PYTHONIOENCODING. A "
                "excecao (PR #238 aberto sobre o mesmo sitio) nao se aplica "
                "mais — remova a entrada de ALLOWLIST neste arquivo." % allowed
            )
        # (c) o arquivo esta de fato na populacao — se deixou de invocar
        #     python3, a excecao tambem ficou sem objeto. Pergunta de
        #     POPULACAO, logo predicado loose (ML-2B/B1): sob o strict, um
        #     comentario inline com `<<` neste arquivo dispararia "SEM OBJETO"
        #     sem motivo.
        if not any(PY3_RE.search(t) for _n, t in population_lines(allowed)):
            failures.append(
                "ALLOWLIST SEM OBJETO: %s nao invoca mais python3; a excecao "
                "nao tem mais razao de existir." % allowed
            )

    notes.append(
        "ALVO 1: %d scripts/check-*.sh enumerados, %d invocam python3, "
        "%d na allowlist, %d checados."
        % (len(gate_paths), len(invokers), len(allowlist),
           len(invokers) - len([a for a in allowlist if a in invokers]))
    )

# ---------------------------------------------------------------- ALVO 2
#
# Ancoragem: a invocacao do attentionSignalScript e identificada pela
# assinatura `python3 -c` + `json.load(sys.stdin)` na mesma linha. Isso a
# distingue do OUTRO python3 -c emitido por scaffold.go (o build-check de
# py_compile de projetos Python), que nao faz parte deste contrato.
ATTENTION_CALL_RE = re.compile(r'python3[ \t]+-c\b')
ATTENTION_SIG_RE = re.compile(r'json\.load\(sys\.stdin\)')
# Sem re.IGNORECASE, pelas mesmas duas razoes do alvo 1 — e aqui elas incidem
# sobre PRODUTO: `pythonioencoding=utf-8 python3 -c` no literal seta uma
# variavel que o Python nunca le (reversao completa do ML-1A na maquina de quem
# adota o trackfw), e handler que nao seja `strict` grava byte invalido no
# .trackfw-attention.json — que e exatamente o artefato lido pelos 3 `serve`.
PREFIX_RE = re.compile(
    r'PYTHONIOENCODING=(?P<q>["\']?)' + UTF8_ALIASES + UTF8_HANDLER
    + r'(?P=q)[ \t]+python3[ \t]+-c\b',
)
MIN_CALLS = 2  # o literal tem duas invocacoes: TOOL e MSG.

for src in attention_sources:
    if not os.path.isfile(src):
        failures.append(
            "ALVO 2: fonte esperada nao existe: %s. O literal pode ter mudado "
            "de arquivo — atualize ATTENTION_SOURCES neste gate." % src
        )
        continue
    total = 0
    prefixed = 0
    bad = []
    with open(src, encoding="utf-8", errors="replace") as fh:
        for i, raw in enumerate(fh.read().splitlines(), start=1):
            if not (ATTENTION_CALL_RE.search(raw) and ATTENTION_SIG_RE.search(raw)):
                continue
            total += 1
            if PREFIX_RE.search(raw):
                prefixed += 1
            else:
                bad.append(i)
    # GUARDA DE VACUIDADE do alvo 2: zero invocacoes casadas significa que a
    # ancora quebrou (literal renomeado, movido, reescrito) OU que o literal
    # sumiu. Nos dois casos o gate deixaria de medir o produto — reprova.
    if total < MIN_CALLS:
        failures.append(
            "ALVO 2 vacuo em %s: esperava >= %d invocacoes "
            "`python3 -c ... json.load(sys.stdin)` do attentionSignalScript, "
            "encontrei %d. A ancora quebrou ou o literal sumiu; recuso passar "
            "em silencio." % (src, MIN_CALLS, total)
        )
    elif prefixed != total:
        failures.append(
            "ALVO 2 em %s: %d de %d invocacoes do attentionSignalScript sem o "
            "prefixo PYTHONIOENCODING=utf-8 (linhas %s). "
            "check-attention-scripts-parity.sh NAO pega isto: ele compara os 3 "
            "CLIs entre si e fica verde se o prefixo sumir dos tres."
            % (src, total - prefixed, total, ", ".join(str(b) for b in bad))
        )
    else:
        notes.append("ALVO 2: %s — %d/%d invocacoes com prefixo." % (src, prefixed, total))

if not attention_sources:
    failures.append("ALVO 2 vacuo: nenhuma fonte de literal configurada.")

# ---------------------------------------------------------------- veredito
for n in notes:
    print("  " + n)

if failures:
    print("")
    print("check-output-encoding-declared: FAIL")
    for f in failures:
        print("  - " + f)
    sys.exit(1)

print("check-output-encoding-declared: OK")
PYEOF
