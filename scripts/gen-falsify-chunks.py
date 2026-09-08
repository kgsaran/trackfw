#!/usr/bin/env python3
"""Gerador de fronteiras + empacotador para paralelizar scripts/check-gates-falsify.sh
(ML-2D, ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify).

O ML-2C içou as 40 funções auxiliares "óbvias" para o preâmbulo (linhas
1-996) — a partir daí, qualquer fronteira "# Cenário N — ..." é um ponto de
corte SINTATICAMENTE válido para a maioria dos casos. Mas a auditoria do
ML-2D (reprovado) mediu dois mecanismos DIFERENTES de perda de cobertura, e
só um se resolve com fronteiras melhores:

  1. Blocos de SUPORTE disfarçados de cenário: um comentário "# Cenário N —
     ..." pode preceder um trecho que não contém NENHUMA asserção própria —
     só definições de função consumidas por cenários distantes (medido:
     linhas 913-1160, rotuladas "Cenário 166", definem 5 funções cujo único
     uso real fica entre as linhas 9015 e 10927 — quase 8000 linhas depois).
     Sem tratamento especial, esse bloco vira um segmento normal e pode cair
     num chunk diferente do dos seus chamadores: a função inexiste ali sob
     `set -euo pipefail`, e TUDO que roda depois nesse chunk morre em
     silêncio (reproduzido: chunk_0 define, chunk_6 chama, chunk_6 aborta na
     primeira chamada). Isso explica tanto os 77 rótulos perdidos (cauda do
     chunk que abortou) quanto o único FAIL sobrevivente do ML-2D
     (`serve-chain-canonical-link/node/edge-baseline`, `exit=2` — reproduzido
     isoladamente: `bash -c "$(declare -f fn); fn ..."` com `fn` indefinida
     sai com exit 2, não 127).

     Correção: um segmento sem asserção própria E cujo conteúdo de topo é só
     definição de função (comentários/linhas em branco à parte) é
     içado para o PREÂMBULO ESTENDIDO — igual à função já era, definição de
     função não tem ordem de execução, então duplicá-la em todo chunk é
     seguro e não custa paralelismo.

     🔴 A hipótese anterior do ML-2D ("42 cabeçalhos usam grafia diferente de
     '# Cenário N — ...'") NÃO se confirmou: o padrão já aceita acento
     opcional, plural e os dois travessões, e o gap real entre "137
     cabeçalhos" e "95 que casam o padrão estrito" era, na maior parte,
     PROSA (comentários que MENCIONAM um cenário sem SER seu cabeçalho —
     ex.: "# Cenários 14/16/17/20/21..."). Afrouxar ainda mais o padrão
     pioraria: casaria mais prosa e cortaria o preâmbulo mais cedo.

  2. Blocos de suporte com EFEITO COLATERAL ordenado (ex.: um `cd` que muda
     o cwd do processo até um cenário posterior restaurá-lo) — esses NÃO
     podem ser içados para o preâmbulo (rodar primeiro quebraria todo
     cenário anterior). Mas já são cobertos pelo mecanismo de fusão por
     VARIÁVEL abaixo, contanto que a variável que marca a fronteira do
     efeito (ex.: a variável usada para restaurar o estado) seja
     referenciada no cenário que fecha o bloco — medido: `GBG_FIXTURE_DIR`/
     `GBG_ORIGINAL_PWD` (linhas 5958-6473) já fundem corretamente pela
     referência a `GBG_ORIGINAL_PWD` no Cenário 64.

Passos deste gerador:

  1. Descobre as fronteiras de cenário em runtime (nunca por número de linha
     memorizado — o arquivo já provou 3x que isso quebra, ver ML-2A/ML-2C).
  2. Classifica cada segmento: SUPORTE (zero asserção própria E só definição
     de função) vira preâmbulo estendido; os demais são segmentos de
     asserção, sujeitos ao passo 3.
  3. Calcula o fechamento de dependência por VARIÁVEL entre segmentos de
     asserção (quem lê variável atribuída por outro) e funde os dependentes
     num único bloco indivisível — mesma lógica do ML-2A/ML-2D original.
  4. Empacota os blocos fundidos em N chunks por peso (LPT) e materializa N
     scripts bash: preâmbulo estendido (idêntico, byte a byte, em todo
     chunk) + os blocos atribuídos, na ordem original.
  5. Emite, por chunk, os rótulos de asserção esperados (literais e, para
     `assert_* "prefixo/$var"`, um glob `prefixo/*`) extraídos do PRÓPRIO
     texto do chunk — não uma lista congelada. O driver usa isso para a
     guarda de conjunto (nunca mascarar chunk que morreu no meio).

Uso:
    python3 gen-falsify-chunks.py <script-fonte> <dir-de-saida> <n-chunks>

Cada chunk materializado é executável standalone (herda `set -euo pipefail` e
todo o preâmbulo estendido). O driver (run-gates-falsify-parallel.sh) invoca
cada chunk como processo separado, agrega os exit codes e confere a guarda de
rótulos.
"""
import re
import sys
import os
import json

HDR_PAT = re.compile(r'^# Cen[aá]rio[s]?\s+([0-9][0-9a-zA-Z/–\-]*)\s+(--|—)')
ASSIGN_PAT = re.compile(r'^\s*(?:local\s+|export\s+|declare\s+)?([A-Za-z_][A-Za-z0-9_]*)\+?=')
REF_PAT = re.compile(r'\$\{?([A-Za-z_][A-Za-z0-9_]*)')
IGNORE_VARS = {str(d) for d in range(10)} | {"@", "*", "#", "?", "$", "!", "-", "_"}

ASSERT_FNS = (
    "fails_with", "succeeds", "lacks_pattern", "output_contains",
    "output_lacks", "would_now_fail", "guard_exit", "writer_no_epipe",
)
ASSERT_CALL_PAT = re.compile(
    r'\bassert_(' + "|".join(ASSERT_FNS) + r')\s+"([^"]*)"'
)
ECHO_SIGNAL_PAT = re.compile(r'echo\s+"(OK|FAIL)\s')
FUNC_DEF_PAT = re.compile(r'^([A-Za-z_][A-Za-z0-9_]*)\s*\(\)\s*\{\s*$')
HEREDOC_START_PAT = re.compile(r"<<-?\s*'?\"?([A-Za-z_][A-Za-z0-9_]*)'?\"?\s*$")


def find_boundaries(lines):
    boundaries = []
    for i, l in enumerate(lines):
        m = HDR_PAT.match(l)
        if m:
            boundaries.append((i, m.group(1)))
    if not boundaries:
        raise SystemExit("gen-falsify-chunks: nenhuma fronteira '# Cenário N — ...' encontrada")
    return boundaries


def scan_region(lines, start, end):
    assigns, refs = set(), set()
    for i in range(start, end):
        line = lines[i]
        if line.strip().startswith('#'):
            continue
        for m in ASSIGN_PAT.finditer(line):
            assigns.add(m.group(1))
        for m in REF_PAT.finditer(line):
            v = m.group(1)
            if v not in IGNORE_VARS:
                refs.add(v)
    return assigns, refs


def segment_has_test_signal(lines, start, end):
    """True se o segmento contém pelo menos uma chamada assert_* ou um echo
    literal de OK/FAIL fora de comentário — ou seja, produz cobertura
    própria. Usa busca por substring (não âncora em início de linha):
    muitas chamadas do arquivo real vêm depois de `cd ... &&` ou dentro de
    subshell `( ... )`, não na coluna 0."""
    for i in range(start, end):
        line = lines[i]
        if line.strip().startswith('#'):
            continue
        if ASSERT_CALL_PAT.search(line) or ECHO_SIGNAL_PAT.search(line):
            return True
    return False


def segment_is_pure_function_defs(lines, start, end):
    """True se todo statement de topo do segmento (fora de heredoc, fora de
    corpo de função) é uma definição de função — só comentário/linha em
    branco entre elas. Segmento vazio (só comentário) também conta como
    "puro": não tem efeito colateral de topo, é seguro içar (não muda nada).

    Heredoc-aware e fecha função por `}` solitário na coluna 0 — mesmo
    critério que o ML-2C já validou como seguro para este arquivo (docstring
    do módulo tem os detalhes de por que naive brace-count quebra aqui)."""
    i = start
    in_heredoc = False
    heredoc_marker = None
    depth = 0
    while i < end:
        line = lines[i]
        if in_heredoc:
            if line.rstrip('\n') == heredoc_marker:
                in_heredoc = False
            i += 1
            continue
        if depth == 0:
            stripped = line.strip()
            if stripped == '' or stripped.startswith('#'):
                i += 1
                continue
            if not FUNC_DEF_PAT.match(line):
                return False
            depth = 1
            i += 1
            continue
        else:
            hd = HEREDOC_START_PAT.search(line)
            if hd:
                in_heredoc = True
                heredoc_marker = hd.group(1)
                i += 1
                continue
            if line.rstrip() == '}':
                depth -= 1
            i += 1
            continue
    if depth != 0:
        raise SystemExit(
            f"gen-falsify-chunks: segmento [{start}:{end}] termina com função "
            f"aberta (depth={depth}) -- provavelmente heredoc mal detectado; "
            "recuse-se a classificar em vez de arriscar iça-lo errado"
        )
    return True


def build_segments(lines):
    boundaries = find_boundaries(lines)
    prelude_end = boundaries[0][0]
    n = len(lines)
    segments = []
    for idx, (start, label) in enumerate(boundaries):
        end = boundaries[idx + 1][0] if idx + 1 < len(boundaries) else n
        segments.append({"start": start, "end": end, "label": label})
    return prelude_end, segments


def split_support_segments(lines, segments):
    """Separa segmentos de SUPORTE (içáveis para o preâmbulo estendido, sem
    perda de ordem porque não têm efeito colateral de topo) dos segmentos de
    ASSERÇÃO (cobertura própria, sujeitos a fusão por variável + LPT)."""
    support, assertion = [], []
    for s in segments:
        has_signal = segment_has_test_signal(lines, s["start"], s["end"])
        if not has_signal and segment_is_pure_function_defs(lines, s["start"], s["end"]):
            support.append(s)
        else:
            assertion.append(s)
    return support, assertion


def fuse_dependent_segments(lines, prelude_assigns, segments):
    """Retorna lista de blocos fundidos [(start,end,labels)], união de
    índices de segmento sempre que um segmento referencia variável atribuída
    por outro segmento (não pelo preâmbulo, real ou estendido)."""
    seg_assigns = []
    seg_refs = []
    for s in segments:
        a, r = scan_region(lines, s["start"], s["end"])
        seg_assigns.append(a)
        seg_refs.append(r)

    var_owner = {}
    for idx, a in enumerate(seg_assigns):
        for v in a:
            if v in prelude_assigns:
                continue
            if v not in var_owner:
                var_owner[v] = idx

    parent = list(range(len(segments)))

    def uf_find(x):
        while parent[x] != x:
            parent[x] = parent[parent[x]]
            x = parent[x]
        return x

    def uf_union(a, b):
        ra, rb = uf_find(a), uf_find(b)
        if ra != rb:
            parent[max(ra, rb)] = min(ra, rb)

    edges = []
    for idx, refs in enumerate(seg_refs):
        for v in refs:
            if v in seg_assigns[idx] or v in prelude_assigns:
                continue
            owner = var_owner.get(v)
            if owner is not None and owner != idx:
                edges.append((idx, owner, v))
                a, b = min(idx, owner), max(idx, owner)
                for k in range(a, b):
                    uf_union(k, k + 1)

    groups = {}
    for i in range(len(segments)):
        r = uf_find(i)
        groups.setdefault(r, []).append(i)

    fused = []
    for r, members in sorted(groups.items()):
        members.sort()
        # Ranges INDIVIDUAIS dos segmentos-membro, não um [start,end) único
        # cobrindo o intervalo inteiro: entre dois segmentos de asserção
        # fundidos por variável pode existir um segmento de SUPORTE (já
        # içado para o preâmbulo estendido, ver split_support_segments) —
        # um range único contaria essas linhas duas vezes (uma no preâmbulo,
        # outra aqui). Guarda de completude em main() denuncia regressão
        # nisso.
        ranges = [(segments[m]["start"], segments[m]["end"]) for m in members]
        labels = [segments[m]["label"] for m in members]
        n_lines = sum(e - s for s, e in ranges)
        # ML-2H: chave de PESO = rótulo de asserção real (contrato já usado
        # pela guarda de conjunto), não o número de cenário acima (`labels`,
        # que renumera com inserção -- já provado hostil a memória
        # posicional 3x neste arquivo). Literais e prefixos glob entram na
        # mesma chave-espaço: um cenário `for x in ...; assert_* "pfx/$x"`
        # calibra pelo prefixo, do mesmo jeito que a guarda de conjunto já
        # casa por glob.
        weight_keys = set()
        for s, e in ranges:
            lit, glb = extract_expected_labels(lines, s, e)
            weight_keys.update(lit)
            weight_keys.update(glb)
        fused.append({
            "start": ranges[0][0],
            "end": ranges[-1][1],
            "ranges": ranges,
            "labels": labels,
            "n_lines": n_lines,
            "weight_keys": sorted(weight_keys),
        })

    return fused, edges


def pack_lpt(fused, n_chunks):
    """Longest-processing-time-first: maior bloco primeiro (por PESO, ver
    `f["weight"]" -- ML-2H trocou a fonte de linha para tempo medido, sem
    mudar o algoritmo LPT em si), sempre no chunk mais vazio no momento.
    Garante que o maior bloco fundido nunca fica sozinho num chunk
    artificialmente pequeno."""
    buckets = [{"items": [], "weight": 0.0} for _ in range(n_chunks)]
    for f in sorted(fused, key=lambda x: -x["weight"]):
        buckets.sort(key=lambda b: b["weight"])
        buckets[0]["items"].append(f)
        buckets[0]["weight"] += f["weight"]
    return buckets


def extract_expected_labels(lines, start, end):
    """Extrai, do próprio texto-fonte do intervalo [start,end), os rótulos
    que os assert_* deste trecho PODEM emitir. Rótulo literal (sem `$`) vira
    exigência exata; rótulo com `$var` vira prefixo glob (o valor real só se
    resolve em runtime, ex.: loop `for path_name in ...`) -- ver
    ROADMAP-2026-09-06 ML-2D, "derive do próprio fonte, nunca lista
    congelada"."""
    literals, globs = [], []
    for i in range(start, end):
        line = lines[i]
        if line.strip().startswith('#'):
            continue
        for m in ASSERT_CALL_PAT.finditer(line):
            fn, label = m.group(1), m.group(2)
            # assert_would_now_fail nunca ecoa o rótulo cru -- sempre
            # "$label/non-vacuity" (PROOF ou FAIL, ver definição do helper
            # em scripts/check-gates-falsify.sh). Sem este ajuste, o rótulo
            # literal extraído daqui nunca bate com o que o chunk realmente
            # emite -- falso positivo medido no ML-2D (git-branch-guard-
            # global-script-integrity, credential-guard-script-integrity).
            if fn == "would_now_fail" and '$' not in label:
                label = f"{label}/non-vacuity"
            if '$' in label:
                prefix = label.split('$', 1)[0]
                globs.append(prefix)
            else:
                literals.append(label)
    return literals, globs


# ML-2H (ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify...):
# empacotar por número de LINHA não prevê TEMPO -- medido no CI (run
# 34277875332): desequilíbrio de 5,5x entre o shard mais lento e o previsto
# como mais lento, que foi na verdade o mais rápido. A variância mora em
# EXECUÇÃO (cenários que sobem múltiplos runtimes / processos), não em
# COMPILAÇÃO -- ML-1A já mediu compilação = 8,8% do tempo total (81,1s de
# 921,4s, 93 builds) espalhado por 26 sítios de `build_go_or_fail`, sinal
# fraco demais para produzir 5,5x. Por isso o peso vem de CALIBRAÇÃO (tempo
# medido, ver gen-falsify-scenario-weights.py), não de heurística estática
# do fonte.
DEFAULT_WEIGHTS_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                                     "falsify-scenario-weights.json")


def load_weights():
    """Lê o arquivo de pesos versionado (path fixo por padrão, sobrescrevível
    por TRACKFW_FALSIFY_WEIGHTS -- mesma convenção de pin dos outros
    consumidores de scripts/, ver check-parity-call-site-pins.sh). Retorna
    (dict rótulo->segundos, path, existe_bool). Arquivo ausente NÃO é erro:
    é o modo legado (peso por linha), mas é sinalizado em stderr para nunca
    degradar em silêncio -- ver assign_weights()."""
    path = os.environ.get("TRACKFW_FALSIFY_WEIGHTS", DEFAULT_WEIGHTS_PATH)
    if not os.path.isfile(path):
        return {}, None, path, False
    with open(path, encoding="utf-8") as fh:
        payload = json.load(fh)
    weights = payload.get("weights", {})
    fallback_unlabeled = payload.get("_fallback_weight_for_unlabeled")
    if fallback_unlabeled is not None:
        fallback_unlabeled = float(fallback_unlabeled)
    return {k: float(v) for k, v in weights.items()}, fallback_unlabeled, path, True


def assign_weights(fused, weights, fallback_unlabeled, weights_exist):
    """Atribui `f['weight']`/`f['weight_source']` a cada bloco fundido.

    - Sem arquivo de pesos: peso = n_lines (modo legado, idêntico ao
      comportamento pré-ML-2H). Uma nota em stderr, uma vez, não por bloco.
    - Com arquivo: peso = soma dos pesos calibrados de cada `weight_keys`
      (rótulo de asserção ou prefixo glob) do bloco. Rótulo NOVO (sem
      entrada no arquivo) usa o peso PESSIMISTA (máximo já calibrado) --
      nunca 0, nunca silencioso: uma linha nomeada em stderr por rótulo
      ausente. Bloco sem NENHUM `weight_keys` (não deveria ocorrer -- todo
      segmento de asserção sobrevive a split_support_segments por ter sinal
      de teste -- mas se ocorrer, cai para n_lines desse bloco específico e
      avisa, em vez de presumir peso 0 e sub-alocar silenciosamente)."""
    if not weights_exist:
        sys.stderr.write(
            f"gen-falsify-chunks: nenhum arquivo de pesos em '{DEFAULT_WEIGHTS_PATH}' "
            "(ou TRACKFW_FALSIFY_WEIGHTS) -- empacotando por numero de linha (modo "
            "legado, pre-ML-2H). Rode gen-falsify-scenario-weights.py para calibrar.\n"
        )
        for f in fused:
            f["weight"] = float(f["n_lines"])
            f["weight_source"] = "lines"
        return

    pessimistic = max(weights.values()) if weights else 0.0
    for f in fused:
        if not f["weight_keys"]:
            # ML-2H, correção pós-auditoria: peso SEGUNDOS aqui, nunca linha
            # -- um bloco sem rótulo extraível ainda tem duração medida (ver
            # `unlabeled_durations` em gen-falsify-scenario-weights.py); usar
            # n_lines misturava unidade linha dentro de um pacote calibrado
            # em segundos (achado: 46% da massa de empacotamento era linha
            # disfarçada de segundo). Sem `_fallback_weight_for_unlabeled`
            # no arquivo (calibração antiga, ou nenhum bloco sem rótulo
            # ocorreu na calibração) cai para o pessimista GERAL -- ainda em
            # segundos, nunca linha.
            unlabeled_weight = fallback_unlabeled if fallback_unlabeled is not None else pessimistic
            sys.stderr.write(
                f"gen-falsify-chunks: AVISO bloco iniciando na linha {f['start']} "
                f"nao produziu nenhum rotulo de asserção extraivel -- usando peso "
                f"pessimista para bloco sem rotulo ({unlabeled_weight:.4f}s), nao "
                "linha e nao silenciando.\n"
            )
            f["weight"] = unlabeled_weight
            f["weight_source"] = "time_fallback_no_keys"
            continue
        total = 0.0
        missing = []
        for k in f["weight_keys"]:
            if k in weights:
                total += weights[k]
            else:
                total += pessimistic
                missing.append(k)
        if missing:
            for k in missing:
                sys.stderr.write(
                    f"gen-falsify-chunks: AVISO rotulo '{k}' (bloco linha {f['start']}) "
                    f"sem peso calibrado -- usando peso pessimista {pessimistic:.4f}s "
                    "(maximo ja calibrado no arquivo). Cenario novo/renomeado: rode "
                    "gen-falsify-scenario-weights.py para recalibrar.\n"
                )
            f["weight_source"] = "time_pessimistic_fallback"
        else:
            f["weight_source"] = "time"
        f["weight"] = total


def main():
    if len(sys.argv) != 4:
        raise SystemExit(f"uso: {sys.argv[0]} <script-fonte> <dir-de-saida> <n-chunks>")
    src_path, out_dir, n_chunks = sys.argv[1], sys.argv[2], int(sys.argv[3])
    if n_chunks < 1:
        raise SystemExit("n-chunks precisa ser >= 1")

    text = open(src_path, encoding='utf-8').read()
    lines = text.split('\n')

    prelude_end, segments = build_segments(lines)
    support_segments, assertion_segments = split_support_segments(lines, segments)

    prelude_assigns, _ = scan_region(lines, 0, prelude_end)
    for s in support_segments:
        a, _ = scan_region(lines, s["start"], s["end"])
        prelude_assigns |= a

    fused, edges = fuse_dependent_segments(lines, prelude_assigns, assertion_segments)

    weights, fallback_unlabeled, weights_path, weights_exist = load_weights()
    assign_weights(fused, weights, fallback_unlabeled, weights_exist)

    n_chunks = min(n_chunks, len(fused))
    buckets = pack_lpt(fused, n_chunks)

    # Guarda de completude estrutural: preâmbulo (real) + todo segmento de
    # suporte + todo segmento de asserção têm de somar o arquivo inteiro,
    # sem lacuna e sem sobreposição -- por construção os segmentos já
    # particionam [prelude_end, n), então isto é uma checagem barata contra
    # regressão futura no particionamento, não uma prova nova.
    accounted = prelude_end + sum(s["end"] - s["start"] for s in support_segments)
    accounted += sum(f["n_lines"] for f in fused)
    if accounted != len(lines):
        raise SystemExit(
            f"gen-falsify-chunks: guarda de completude falhou -- "
            f"contabilizado={accounted} linhas, arquivo tem {len(lines)}. "
            "Particionamento perdeu ou duplicou conteudo; recusando gerar chunks."
        )

    os.makedirs(out_dir, exist_ok=True)
    prelude_lines = list(lines[:prelude_end])
    for s in sorted(support_segments, key=lambda x: x["start"]):
        prelude_lines.extend(lines[s["start"]:s["end"]])

    # ML-2H: marca de tempo por BLOCO FUNDIDO, opt-in via FALSIFY_TIMING_FILE
    # (best-effort -- nunca aborta o chunk se a variável não estiver setada
    # ou o diretório não existir; calibração é sempre opcional). Emitida
    # ANTES do sentinela CHUNK_COMPLETE (que continua sendo a ÚLTIMA linha,
    # ver comentário abaixo) e nunca começando com OK/FAIL/PROOF -- não
    # interfere no colhedor de rótulos (`grep -oE '^(OK|FAIL|PROOF)...'` em
    # run-gates-falsify-shard.sh) nem no sentinela. $EPOCHREALTIME é
    # variável nativa do bash (>=5.0, disponível em ubuntu-latest e no bash
    # via homebrew usado localmente) -- delta calculado depois, fora do
    # bash, por gen-falsify-scenario-weights.py (bash não faz ponto
    # flutuante nativo).
    TIMING_HELPER = (
        "__falsify_timing_mark() {\n"
        '  [[ -n "${FALSIFY_TIMING_FILE:-}" ]] || return 0\n'
        '  printf \'FALSIFY_TIMING phase=%s block=%s labels=%s ts=%s\\n\' '
        '"$1" "$2" "$3" "$EPOCHREALTIME" >> "$FALSIFY_TIMING_FILE" 2>/dev/null || true\n'
        "}\n"
    )

    manifest = []
    label_lines = []
    for i, b in enumerate(buckets):
        chunk_lines = list(prelude_lines)
        chunk_lines.append(TIMING_HELPER.rstrip("\n"))
        # preserva a ordem original dentro do chunk (não é requisito de
        # corretude — os blocos já não têm dependência entre si por
        # construção — mas facilita leitura de log/diagnóstico).
        ordered_items = sorted(b["items"], key=lambda x: x["start"])
        for f in ordered_items:
            block_id = f"{i}-{f['start']}"
            label_csv = ",".join(k.replace('"', '\\"') for k in f["weight_keys"]) or "none"
            chunk_lines.append(f'__falsify_timing_mark start "{block_id}" "{label_csv}"')
            for s, e in f["ranges"]:
                chunk_lines.extend(lines[s:e])
            chunk_lines.append(f'__falsify_timing_mark end "{block_id}" "{label_csv}"')
        chunk_path = os.path.join(out_dir, f"chunk_{i}.sh")
        with open(chunk_path, 'w', encoding='utf-8') as fh:
            fh.write('\n'.join(chunk_lines))
            # ML-2B (ROADMAP-2026-09-07-gates-rodam-no-windows...): fecha
            # cada chunk com a MESMA checagem de $FALSIFY_ENUM_TALLY que
            # o preâmbulo estendido já traz do check-gates-falsify.sh -- sem
            # isto, só o chunk que por acaso herda o TRECHO FINAL do arquivo
            # de origem (a última fatia de linhas, que carrega o fechamento
            # do script real) chegaria a essa checagem; os outros N-1 chunks
            # nunca converteriam tally>0 em exit != 0 -- a guarda 1 (nunca
            # torna o gate verde) do modo de enumeração ficaria furada em
            # todo chunk que não fosse esse. Medido por falsificação: sem
            # este bloco, um chunk sabotado (não o que carrega a cauda do
            # arquivo) reportava FAIL no stderr e ainda assim saía com
            # exit 0.
            #
            # Lê $FALSIFY_ENUM_TALLY (arquivo em $WORK, não variável) pelo
            # mesmo motivo do preâmbulo: o incremento pode ter acontecido
            # dentro de subshell/pipeline de algum cenário deste chunk, e só
            # escrita em arquivo atravessa essa fronteira -- ver nota na
            # definição de falsify_fail_point/falsify_count_failure.
            #
            # Silencioso (sem nenhuma linha nova em stdout/stderr) quando
            # desligado OU quando ligado sem nenhuma reprovação -- para não
            # perturbar o sentinela CHUNK_COMPLETE (run-gates-falsify-
            # parallel.sh exige que ele seja a ÚLTIMA linha do log). Só emite
            # e sai != 0 ANTES do sentinela quando há reprovação real -- o
            # driver então relata "chunk não chegou ao sentinela" (mensagem
            # pré-existente, pensada para crash) mas o exit code agregado já
            # é != 0 de qualquer forma, então a guarda 1 se sustenta pelo
            # aggregate check do driver mesmo quando este texto de
            # diagnóstico é impreciso para este caso novo.
            fh.write(
                '\nif [[ "${TRACKFW_FALSIFY_ENUMERATE:-0}" == "1" ]]; then\n'
                '  falsify_enum_n=$(wc -l < "$FALSIFY_ENUM_TALLY" 2>/dev/null || echo 0)\n'
                '  falsify_enum_n=${falsify_enum_n//[[:space:]]/}\n'
                '  if [[ "${falsify_enum_n:-0}" -gt 0 ]]; then\n'
                '    echo "[falsify/enumerate] '
                f'$falsify_enum_n cenário(s) reprovaram no chunk {i} '
                '(enumerados acima, cada um prefixado FAIL) -- exit 1" >&2\n'
                '    exit 1\n'
                '  fi\n'
                'fi\n'
            )
            fh.write(f'echo "CHUNK_COMPLETE {i}"\n')
        os.chmod(chunk_path, 0o755)
        all_labels = [lbl for f in b["items"] for lbl in f["labels"]]

        chunk_literals, chunk_globs = [], []
        for f in ordered_items:
            for s, e in f["ranges"]:
                lit, glb = extract_expected_labels(lines, s, e)
                chunk_literals.extend(lit)
                chunk_globs.extend(glb)
        for lit in chunk_literals:
            label_lines.append(f"chunk={i} label={lit}")
        for glb in sorted(set(chunk_globs)):
            label_lines.append(f"chunk={i} label_glob={glb}")

        chunk_n_lines = sum(f["n_lines"] for f in b["items"])
        manifest.append({
            "chunk": i,
            "path": chunk_path,
            "n_fused_blocks": len(b["items"]),
            "n_lines": chunk_n_lines,
            "weight": b["weight"],
            "labels": all_labels,
        })

    # Relatório em stdout, consumido pelo driver e por humano.
    print(f"prelude_end_line={prelude_end}")
    print(f"total_segments={len(segments)}")
    print(f"support_segments={len(support_segments)} support_lines={sum(s['end']-s['start'] for s in support_segments)}")
    print(f"assertion_segments={len(assertion_segments)}")
    print(f"fused_units={len(fused)}")
    print(f"cross_segment_edges={len(edges)}")
    weight_sources = sorted({f["weight_source"] for f in fused})
    print(f"weight_source={','.join(weight_sources)} weights_path={weights_path} weights_file_found={weights_exist}")
    largest_lines = max(fused, key=lambda f: f["n_lines"])
    print(f"largest_fused_unit_lines={largest_lines['n_lines']} labels={largest_lines['labels'][0]}..{largest_lines['labels'][-1]} count={len(largest_lines['labels'])}")
    largest_weight = max(fused, key=lambda f: f["weight"])
    print(f"largest_fused_unit_weight={largest_weight['weight']:.4f} labels={largest_weight['labels'][0]}..{largest_weight['labels'][-1]} count={len(largest_weight['labels'])}")
    for m in manifest:
        print(f"chunk={m['chunk']} path={m['path']} fused_blocks={m['n_fused_blocks']} lines={m['n_lines']} weight={m['weight']:.4f} n_labels={len(m['labels'])}")
    for l in label_lines:
        print(l)


if __name__ == "__main__":
    main()
