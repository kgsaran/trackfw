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
        fused.append({
            "start": ranges[0][0],
            "end": ranges[-1][1],
            "ranges": ranges,
            "labels": labels,
            "n_lines": n_lines,
        })

    return fused, edges


def pack_lpt(fused, n_chunks):
    """Longest-processing-time-first: maior bloco primeiro, sempre no chunk
    mais vazio no momento. Garante que o maior bloco fundido nunca fica
    sozinho num chunk artificialmente pequeno."""
    buckets = [{"items": [], "weight": 0} for _ in range(n_chunks)]
    for f in sorted(fused, key=lambda x: -x["n_lines"]):
        buckets.sort(key=lambda b: b["weight"])
        buckets[0]["items"].append(f)
        buckets[0]["weight"] += f["n_lines"]
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

    manifest = []
    label_lines = []
    for i, b in enumerate(buckets):
        chunk_lines = list(prelude_lines)
        # preserva a ordem original dentro do chunk (não é requisito de
        # corretude — os blocos já não têm dependência entre si por
        # construção — mas facilita leitura de log/diagnóstico).
        ordered_items = sorted(b["items"], key=lambda x: x["start"])
        for f in ordered_items:
            for s, e in f["ranges"]:
                chunk_lines.extend(lines[s:e])
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

        manifest.append({
            "chunk": i,
            "path": chunk_path,
            "n_fused_blocks": len(b["items"]),
            "n_lines": b["weight"],
            "labels": all_labels,
        })

    # Relatório em stdout, consumido pelo driver e por humano.
    print(f"prelude_end_line={prelude_end}")
    print(f"total_segments={len(segments)}")
    print(f"support_segments={len(support_segments)} support_lines={sum(s['end']-s['start'] for s in support_segments)}")
    print(f"assertion_segments={len(assertion_segments)}")
    print(f"fused_units={len(fused)}")
    print(f"cross_segment_edges={len(edges)}")
    largest = max(fused, key=lambda f: f["n_lines"])
    print(f"largest_fused_unit_lines={largest['n_lines']} labels={largest['labels'][0]}..{largest['labels'][-1]} count={len(largest['labels'])}")
    for m in manifest:
        print(f"chunk={m['chunk']} path={m['path']} fused_blocks={m['n_fused_blocks']} lines={m['n_lines']} n_labels={len(m['labels'])}")
    for l in label_lines:
        print(l)


if __name__ == "__main__":
    main()
