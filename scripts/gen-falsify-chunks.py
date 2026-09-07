#!/usr/bin/env python3
"""Gerador de fronteiras + empacotador para paralelizar scripts/check-gates-falsify.sh
(ML-2D, ROADMAP-2026-09-06-perfil-e-aceleracao-do-check-gates-falsify).

O ML-2C içou as 40 funções auxiliares para o preâmbulo (linhas 1-996) — a partir
daí, qualquer fronteira "# Cenário N — ..." é um ponto de corte SINTATICAMENTE
válido (nenhum trecho usa função definida depois de si). Mas sintaticamente
válido não é o mesmo que SEMANTICAMENTE independente: alguns cenários leem
variável ($T27_GO_BIN, $GBG_ORIGINAL_PWD, ...) atribuída por um cenário anterior
(reuso de binário Go já compilado, principalmente). Este gerador:

  1. Descobre as fronteiras de cenário em runtime (nunca por número de linha
     memorizado — o arquivo já provou 3x que isso quebra: caminho relativo
     velho, heredoc mal detectado, contagem de linha defasada, ver ML-2A/ML-2C).
  2. Calcula o fechamento de dependência entre cenários (quem lê variável
     atribuída por outro) e FUNDE cenários dependentes num único bloco
     indivisível — sem isso, um split ingênuo quebra sob `set -u` (a mesma
     afirmação estática "zero dependência entre cenários" que o ML-2A precisou
     reverter, agora medida, não presumida).
  3. Empacota os blocos fundidos em N chunks por peso (linhas de código, proxy
     de tempo — não temos telemetria por cenário) usando LPT (longest processing
     time first), e materializa N scripts bash: preâmbulo (idêntico, byte a
     byte) + os blocos atribuídos, na ordem original.

Uso:
    python3 gen-falsify-chunks.py <script-fonte> <dir-de-saida> <n-chunks>

Cada chunk materializado é executável standalone (herda `set -euo pipefail` e
todo o preâmbulo do script original). O driver (run-gates-falsify-parallel.sh)
é quem invoca cada chunk como processo separado e agrega os exit codes.
"""
import re
import sys
import os

HDR_PAT = re.compile(r'^# Cen[aá]rio[s]?\s+([0-9][0-9a-zA-Z/–\-]*)\s+(--|—)')
ASSIGN_PAT = re.compile(r'^\s*(?:local\s+|export\s+|declare\s+)?([A-Za-z_][A-Za-z0-9_]*)\+?=')
REF_PAT = re.compile(r'\$\{?([A-Za-z_][A-Za-z0-9_]*)')
IGNORE_VARS = {str(d) for d in range(10)} | {"@", "*", "#", "?", "$", "!", "-", "_"}


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


def build_segments(lines):
    boundaries = find_boundaries(lines)
    prelude_end = boundaries[0][0]
    n = len(lines)
    segments = []
    for idx, (start, label) in enumerate(boundaries):
        end = boundaries[idx + 1][0] if idx + 1 < len(boundaries) else n
        segments.append({"start": start, "end": end, "label": label})
    return prelude_end, segments


def fuse_dependent_segments(lines, prelude_end, segments):
    """Retorna lista de blocos fundidos [(start,end,labels)], união de
    índices de segmento sempre que um segmento referencia variável atribuída
    por outro segmento (não pelo preâmbulo)."""
    prelude_assigns, _ = scan_region(lines, 0, prelude_end)

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
        start = segments[members[0]]["start"]
        end = segments[members[-1]]["end"]
        labels = [segments[m]["label"] for m in members]
        fused.append({"start": start, "end": end, "labels": labels, "n_lines": end - start})

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


def main():
    if len(sys.argv) != 4:
        raise SystemExit(f"uso: {sys.argv[0]} <script-fonte> <dir-de-saida> <n-chunks>")
    src_path, out_dir, n_chunks = sys.argv[1], sys.argv[2], int(sys.argv[3])
    if n_chunks < 1:
        raise SystemExit("n-chunks precisa ser >= 1")

    text = open(src_path, encoding='utf-8').read()
    lines = text.split('\n')

    prelude_end, segments = build_segments(lines)
    fused, edges = fuse_dependent_segments(lines, prelude_end, segments)

    n_chunks = min(n_chunks, len(fused))
    buckets = pack_lpt(fused, n_chunks)

    os.makedirs(out_dir, exist_ok=True)
    prelude_lines = lines[:prelude_end]

    manifest = []
    for i, b in enumerate(buckets):
        chunk_lines = list(prelude_lines)
        # preserva a ordem original dentro do chunk (não é requisito de
        # corretude — os blocos já não têm dependência entre si por
        # construção — mas facilita leitura de log/diagnóstico).
        for f in sorted(b["items"], key=lambda x: x["start"]):
            chunk_lines.extend(lines[f["start"]:f["end"]])
        chunk_path = os.path.join(out_dir, f"chunk_{i}.sh")
        with open(chunk_path, 'w', encoding='utf-8') as fh:
            fh.write('\n'.join(chunk_lines))
        os.chmod(chunk_path, 0o755)
        all_labels = [lbl for f in b["items"] for lbl in f["labels"]]
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
    print(f"fused_units={len(fused)}")
    print(f"cross_segment_edges={len(edges)}")
    largest = max(fused, key=lambda f: f["n_lines"])
    print(f"largest_fused_unit_lines={largest['n_lines']} labels={largest['labels'][0]}..{largest['labels'][-1]} count={len(largest['labels'])}")
    for m in manifest:
        print(f"chunk={m['chunk']} path={m['path']} fused_blocks={m['n_fused_blocks']} lines={m['n_lines']} n_labels={len(m['labels'])}")


if __name__ == "__main__":
    main()
