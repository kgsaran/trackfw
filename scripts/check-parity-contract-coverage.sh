#!/usr/bin/env bash
# check-parity-contract-coverage.sh — meta-checker de cobertura de contrato
# em docs/cli-parity.md (REQ-2026-08-18-contrato-pinado-no-cli-parity-sem-
# gate-nomeado-e-contrato-nao-aplicado.md, ML-1B).
#
# Formato da anotação (ADR-2026-08-20-anotacao-de-cobertura-de-contrato-no-
# cli-parity.md, Emenda 1) — um comentário HTML na primeira linha não-vazia
# depois do cabeçalho de cada seção `##`/`###`/`####`:
#
#   <!-- trackfw-contract: gate=<caminhos> -->
#   <!-- trackfw-contract: gate=<caminhos> partial=<o que fica de fora> -->
#   <!-- trackfw-contract: gap reason=<motivo> -->
#   <!-- trackfw-contract: none reason=<motivo> -->
#
# 🔴 Bloqueante (ROADMAP ML-3A, 2026-08-20): a triagem da Wave 2 fechou —
# 177/177 seções anotadas (gate= 72 · partial 51 · gap 42 · none 12 · sem
# anotação 0 · inválidas 0). Seção SEM anotação nenhuma agora REPROVA — era
# modo relatório só enquanto a triagem estava em curso (ver histórico no
# roadmap). Anotação PRESENTE e INVÁLIDA reprova pelos 6 casos de sempre
# (Emenda 1 definiu 1-5; Emenda 2 acrescentou o 6º), mais o 7º caso que o
# ML-3A acrescenta — SEÇÃO SEM ANOTAÇÃO:
#   1. `gate=` sem caminho nomeado (vazio)
#   2. `gate=` nomeando caminho que não existe no disco
#   3. `gap`/`none` SEM a chave `reason=` (chave ausente, não vazia — caso
#      distinto do 6: aqui a chave nem foi escrita)
#   4. chave desconhecida na anotação (ex.: `reson=` por erro de digitação)
#   5. anotação malformada — prefixo `trackfw-contract:` sem estado reconhecido
#      (nenhum de `gate=`/`gap`/`none`)
#   6. QUALQUER chave presente com valor vazio (regra GERAL — ADR Emenda 2:
#      toda chave presente exige valor não-vazio; chave sem valor é erro,
#      nunca "não se aplica" — para "não se aplica", omite-se a chave). Não
#      é um `if` por chave: é um laço sobre todo par presente na anotação,
#      então cobre chave nova sem precisar de emendar o ADR de novo. Este
#      caso SUBSUME o caso 1 (`gate=` vazio é uma instância dele) e também
#      pega `partial=`/`reason=` vazios, que antes da Emenda 2 passavam.
#   7. SEÇÃO SEM ANOTAÇÃO — a triagem fechou (177/177), então a partir de
#      agora ausência de anotação não é "ainda não triado", é regressão: uma
#      seção nova entrou sem passar pela triagem. Reprova.
#
# O relatório (contagem por estado + lista de `gap`) é o entregável desta
# REQ, não decoração: é o número que se quer acompanhar cair.
set -euo pipefail

# Codificacao de saida (ML-1B, ROADMAP-2026-09-02-saida-nao-ascii-declara-
# codificacao-em-script-gerado-e-em-gate): forca UTF-8 no stdio de todo
# python3 deste gate. Sob console cp1252 (Windows) o Python herda a codepage
# e um print() de caractere fora do cp1252 estoura UnicodeEncodeError -- o
# gate reprova por um motivo alheio ao que ele mede. Declarado aqui, e nao no
# Makefile, para valer tambem na invocacao direta pelo workflow de CI, na
# invocacao manual de um gate isolado e na invocacao de um gate por outro.
# Trade-off assumido: num console genuinamente cp1252 a saida vira mojibake
# em vez de crashar -- acento ilegivel com exit code correto vale mais que
# uma reprovacao falsa.
export PYTHONIOENCODING=utf-8

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DOC="${1:-$ROOT_DIR/docs/cli-parity.md}"

if [[ ! -f "$DOC" ]]; then
  echo "check-parity-contract-coverage: documento não encontrado: $DOC" >&2
  exit 1
fi

python3 - "$DOC" "$ROOT_DIR" <<'PYEOF'
import sys
import re
import os

DOC_PATH = sys.argv[1]
ROOT = sys.argv[2]

KNOWN_KEYS = ["gate=", "partial=", "reason="]
ANNOT_RE = re.compile(r'^<!--\s*trackfw-contract:\s*(.*?)\s*-->\s*$')
HEADER_RE = re.compile(r'^(#{2,4})\s+(.*?)\s*$')
FENCE_RE = re.compile(r'^\s*```')

with open(DOC_PATH, encoding="utf-8") as f:
    lines = f.read().splitlines()

# Pass 1: mark which lines sit inside a fenced code block, so headers and
# annotations inside ```...``` fences (templates, examples) are never
# mistaken for real document structure.
in_fence = False
fenced = [False] * len(lines)
for i, line in enumerate(lines):
    if FENCE_RE.match(line):
        fenced[i] = True  # the fence delimiter line itself doesn't count as content
        in_fence = not in_fence
        continue
    fenced[i] = in_fence

# Pass 2: collect real headers (level 2-4, outside fences).
headers = []  # (line_index, level, title)
for i, line in enumerate(lines):
    if fenced[i]:
        continue
    m = HEADER_RE.match(line)
    if m:
        headers.append((i, len(m.group(1)), m.group(2)))


def find_keys(s):
    """Positions of known key prefixes, only when preceded by start-of-string
    or whitespace — avoids matching '=' signs buried mid-word inside free
    text."""
    positions = []
    i = 0
    n = len(s)
    while i < n:
        if i == 0 or s[i - 1].isspace():
            matched = None
            for k in KNOWN_KEYS:
                if s.startswith(k, i):
                    matched = k
                    break
            if matched:
                positions.append((i, matched))
                i += len(matched)
                continue
        i += 1
    return positions


def extract_kv(body, positions):
    kv = {}
    for idx, (pos, key) in enumerate(positions):
        start = pos + len(key)
        end = positions[idx + 1][0] if idx + 1 < len(positions) else len(body)
        kv[key[:-1]] = body[start:end].strip()
    return kv


KNOWN_KEY_NAMES = [k[:-1] for k in KNOWN_KEYS]  # ["gate", "partial", "reason"]


def levenshtein(a, b):
    """Distância de edição clássica (inserção/remoção/substituição, sem
    transposição) — suficiente para o caso que queremos pegar (um caractere
    trocado/faltando/sobrando), não para todo typo imaginável."""
    if a == b:
        return 0
    prev = list(range(len(b) + 1))
    for i, ca in enumerate(a, 1):
        curr = [i] + [0] * len(b)
        for j, cb in enumerate(b, 1):
            curr[j] = min(
                prev[j] + 1,       # remoção
                curr[j - 1] + 1,   # inserção
                prev[j - 1] + (ca != cb),  # substituição (0 se igual)
            )
        prev = curr
    return prev[-1]


def find_key_like_tokens(s):
    """Posições de tokens alfabéticos minúsculos imediatamente seguidos de
    '=', só quando precedidos por início de string ou espaço — mesma regra
    de fronteira usada por find_keys(), mas sem se limitar às 3 chaves
    conhecidas: aqui queremos candidatos a CHAVE DIGITADA ERRADA, não texto
    livre com '=' no meio (ex.: 'LANG=pt_BR', '--flag=valor' nunca batem
    aqui: 'LANG' tem maiúsculas, e o 'f' de '--flag' é precedido por '-',
    não por espaço)."""
    tokens = []
    i, n = 0, len(s)
    while i < n:
        if i == 0 or s[i - 1].isspace():
            j = i
            while j < n and s[j].isalpha() and s[j].islower():
                j += 1
            if j > i and j < n and s[j] == "=":
                tokens.append((i, s[i:j]))
        i += 1
    return tokens


def find_unknown_key_typos(body, positions):
    """Segunda passada, independente de find_keys()/extract_kv(): varre TODO
    o corpo (inclusive dentro do valor de reason=/partial=, onde uma chave
    desconhecida digitada depois de uma chave real seria hoje engolida em
    silêncio pelo fatiamento de extract_kv — a lacuna que este laço fecha).

    Critério heurístico (decisão de desenho, ver Nota de parsing do ADR):
    um token é tratado como CHAVE DESCONHECIDA só quando está a distância de
    edição <= 1 de gate/partial/reason (uma inserção, remoção ou
    substituição de caractere) — ex.: 'reson=' (falta o 'a' de 'reason').

    Trade-off explícito, assumido deliberadamente para o lado do
    falso-negativo:
      - ACEITA indevidamente (falso-negativo): typos de 2+ caracteres ou
        transposições (ex.: 'raeson=', 'gatee=' com uma letra a mais E uma
        trocada) escapam. Também escapa qualquer palavra de texto livre que
        contenha '=' mas não esteja perto de nenhuma das 3 chaves (ex.:
        'modo=debug', 'status=ok').
      - REJEITA indevidamente (falso-positivo), caso raro e aceito: uma
        palavra de texto livre que por coincidência fique a 1 edição de uma
        chave conhecida e apareça imediatamente seguida de '=' — ex.:
        'reason=convertido à rate=10' seria sinalizado, porque 'rate' está a
        1 edição de 'gate'. Julgamento: esse padrão ('palavra=valor' solto
        dentro de uma frase) é raro em português corrido; quando acontecer,
        quem anota reformula o motivo (ex.: 'à taxa de 10') — custo baixo e
        localizado, contra o custo de deixar 'reson=' (o typo real que
        motivou esta regra) voltar a passar em silêncio.
      - Distância 1 (não 2+) foi escolhida por ser o ponto onde o
        falso-positivo já fica raro o bastante para não atrapalhar o texto
        livre; distância 2 pegaria mais typos reais mas também mais palavras
        comuns por acidente (ex.: 'data='  a 2 de 'gate=').
    """
    known_starts = {pos for pos, _ in positions}
    typos = []
    for start, token in find_key_like_tokens(body):
        if start in known_starts or token in KNOWN_KEY_NAMES:
            continue
        for name in KNOWN_KEY_NAMES:
            if levenshtein(token, name) <= 1:
                typos.append(token)
                break
    return typos


def parse_annotation(raw):
    """Parser por prefixo de chave conhecido — reason=/partial= são texto
    livre e podem conter '=' e ',' (Nota de parsing, ADR Emenda 1)."""
    content = raw.strip()
    if content.startswith("gate="):
        state = "gate"
        body = content
    elif content == "gap" or content.startswith("gap "):
        state = "gap"
        body = content[len("gap"):].strip()
    elif content == "none" or content.startswith("none "):
        state = "none"
        body = content[len("none"):].strip()
    else:
        return {"state": "malformed", "kv": {}, "parse_errors": [
            "prefixo trackfw-contract sem estado reconhecido (esperado gate=/gap/none): '%s'" % content
        ]}

    positions = find_keys(body)
    kv = extract_kv(body, positions)

    leading = body[:positions[0][0]] if positions else body
    parse_errors = []
    if leading.strip():
        parse_errors.append("chave desconhecida na anotação: '%s'" % leading.strip())

    # Chave desconhecida em QUALQUER posição, não só antes da primeira chave
    # reconhecida (o `leading` acima só cobre esse caso). Sem isto, uma chave
    # digitada errada DEPOIS de uma chave real é engolida em silêncio dentro
    # do valor da chave anterior por extract_kv() — exatamente o cenário que
    # a Nota de parsing do ADR (Emenda 1) descreve para 'reson='. Ver
    # find_unknown_key_typos() para o critério heurístico e o trade-off.
    for token in sorted(set(find_unknown_key_typos(body, positions))):
        parse_errors.append(
            "chave desconhecida na anotação: '%s=' (não é gate=/partial=/reason=)" % token
        )

    # ADR Emenda 2 — regra GERAL, não caso a caso: toda chave PRESENTE exige
    # valor não-vazio. Isto cobre gate=/partial=/reason= hoje e qualquer
    # chave nova amanhã, sem precisar de um `if` por chave nem de emendar o
    # ADR de novo. Chave ausente é assunto separado (checado por estado,
    # abaixo, para gap/none exigindo reason=).
    for key in sorted(kv):
        if not kv[key].strip():
            parse_errors.append("%s= presente com valor vazio" % key)

    return {"state": state, "kv": kv, "parse_errors": parse_errors}


# Pass 3: for each header, the annotation is the first non-blank line right
# after it, up to (not including) the next real header or EOF. If that first
# non-blank line isn't a trackfw-contract comment, the section is unannotated
# — counted, not failed (Wave 2 triage hasn't closed yet).
sections = []
for idx, (line_no, level, title) in enumerate(headers):
    end = headers[idx + 1][0] if idx + 1 < len(headers) else len(lines)
    annotation_raw = None
    for j in range(line_no + 1, end):
        if fenced[j]:
            continue
        if lines[j].strip() == "":
            continue
        m = ANNOT_RE.match(lines[j])
        if m:
            annotation_raw = m.group(1)
        break  # first non-blank line decides, annotated or not
    sections.append({
        "line": line_no + 1,
        "level": level,
        "title": title,
        "raw": annotation_raw,
    })

violations = []
counts = {"gate_full": 0, "gate_partial": 0, "gap": 0, "none": 0, "unannotated": 0, "invalid": 0}
gap_list = []
unannotated_list = []

for sec in sections:
    heading = "%s %s (linha %d)" % ("#" * sec["level"], sec["title"], sec["line"])

    if sec["raw"] is None:
        counts["unannotated"] += 1
        unannotated_list.append(heading)
        violations.append("%s: seção sem anotação trackfw-contract" % heading)
        continue

    parsed = parse_annotation(sec["raw"])
    state = parsed["state"]
    kv = parsed["kv"]
    errs = list(parsed["parse_errors"])

    if state == "malformed":
        counts["invalid"] += 1
        violations.append("%s: %s" % (heading, "; ".join(errs)))
        continue

    if errs:
        counts["invalid"] += 1
        violations.append("%s: %s" % (heading, "; ".join(errs)))
        continue

    if state == "gate":
        # kv["gate"] é garantidamente não-vazio aqui: o laço geral acima já
        # reprovou (e já deu `continue`, via `errs`) qualquer chave presente
        # com valor vazio, `gate=` incluso.
        gate_value = kv["gate"].strip()

        missing = []
        for path in [p.strip() for p in gate_value.split(",") if p.strip()]:
            if not os.path.isfile(os.path.join(ROOT, path)):
                missing.append(path)
        if missing:
            counts["invalid"] += 1
            violations.append(
                "%s: gate nomeado não existe no disco: %s" % (heading, ", ".join(missing))
            )
            continue

        partial_value = kv.get("partial", "").strip()
        if partial_value:
            counts["gate_partial"] += 1
            gap_list.append((heading, "partial: " + partial_value))
        else:
            counts["gate_full"] += 1
        continue

    if state in ("gap", "none"):
        # Chave AUSENTE (não escrita) é caso distinto de chave PRESENTE-vazia
        # (já reprovado acima pelo laço geral): aqui a chave nem existe.
        if "reason" not in kv:
            counts["invalid"] += 1
            violations.append("%s: %s sem reason= (motivo obrigatório)" % (heading, state))
            continue
        reason_value = kv["reason"].strip()
        counts[state] += 1
        if state == "gap":
            gap_list.append((heading, reason_value))
        continue

total = len(sections)
by_level = {2: 0, 3: 0, 4: 0}
for sec in sections:
    by_level[sec["level"]] += 1

naive_by_level = {2: 0, 3: 0, 4: 0}
for i, line in enumerate(lines):
    m = HEADER_RE.match(line)
    if m:
        naive_by_level[len(m.group(1))] += 1
naive_total = sum(naive_by_level.values())

print("== check-parity-contract-coverage: relatório ==")
print("documento: %s" % os.path.relpath(DOC_PATH, ROOT))
print("total de seções reais (##/###/####), fora de fences: %d  (## %d · ### %d · #### %d)" % (
    total, by_level[2], by_level[3], by_level[4]
))
if naive_total != total:
    print(
        "  nota: grep ingênuo (sem consciência de ``` fences) conta %d (## %d · ### %d · #### %d) "
        "— a diferença de %d é cabeçalho de TEMPLATE literal (ex.: `req new`/`adr new`/`roadmap "
        "new`) dentro de bloco de código, não seção real do documento." % (
            naive_total, naive_by_level[2], naive_by_level[3], naive_by_level[4],
            naive_total - total,
        )
    )
print("  gate= (cobertura plena):  %d" % counts["gate_full"])
print("  gate= com partial=:       %d" % counts["gate_partial"])
print("  gap (contrato SEM gate):  %d" % counts["gap"])
print("  none (não-contrato):      %d" % counts["none"])
print("  sem anotação:             %d" % counts["unannotated"])
print("  anotação inválida:        %d" % counts["invalid"])
print("")
print("-- lista de gap/partial (produto mais valioso da REQ) --")
if gap_list:
    for heading, reason in gap_list:
        print("  - %s -- %s" % (heading, reason))
else:
    print("  (nenhum)")
print("")
print("-- seções sem anotação (reprovam — bloqueante desde o ML-3A) --")
if unannotated_list:
    for heading in unannotated_list:
        print("  - %s" % heading)
else:
    print("  (nenhuma)")

if violations:
    print("")
    print("== VIOLAÇÕES (reprovam: anotação inválida ou seção sem anotação) ==", file=sys.stderr)
    for v in violations:
        print("FAIL: %s" % v, file=sys.stderr)
    sys.exit(1)

print("")
print("OK — nenhuma anotação inválida e nenhuma seção sem anotação")
sys.exit(0)
PYEOF
