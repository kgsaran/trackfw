#!/usr/bin/env python3
"""Gerador/atualizador do arquivo de pesos calibrados para
gen-falsify-chunks.py (ML-2H, ROADMAP-2026-09-06-perfil-e-aceleracao-do-
check-gates-falsify-sem-perder-cobertura.md).

Contexto: o empacotador (`pack_lpt`) precisava de um peso por BLOCO FUNDIDO
que previsse TEMPO, não número de linha -- medido no CI (run 34277875332),
peso por linha produziu 5,5x de desequilíbrio porque a variância mora em
EXECUÇÃO (cenários que sobem múltiplos processos/runtimes), não em
compilação (ML-1A: compilação = 8,8% do tempo total, espalhada por 26
sítios -- sinal fraco demais para prever o gargalo real).

Fonte dos pesos: `gen-falsify-chunks.py` (>= ML-2H) instrumenta cada bloco
fundido com marcas `FALSIFY_TIMING phase=start|end block=<id>
labels=<csv> ts=$EPOCHREALTIME` quando a variável de ambiente
`FALSIFY_TIMING_FILE` está setada -- opt-in, custo zero quando desligada
(a marca é `[[ -n "${FALSIFY_TIMING_FILE:-}" ]] || return 0`, sempre
presente no chunk gerado, nunca precisa reinstrumentar nada). Este script lê
esse arquivo de marcas e escreve `scripts/falsify-scenario-weights.json`.

Método de atribuição (declarado, aproximado -- um bloco pode emitir vários
rótulos, e o tempo de parede do bloco não se divide exatamente por rótulo):
peso(rótulo) = média, entre todas as ocorrências do rótulo no arquivo de
marcas, de (duração do bloco que o contém / número de rótulos desse
bloco). Rótulo que aparece em mais de um bloco fundido ao longo de runs
diferentes (só pode ocorrer entre CALIBRAÇÕES distintas, nunca dentro da
mesma -- um rótulo pertence a exatamente um bloco fundido por construção do
gerador) tem os valores agregados por média.

Manutenção da fonte (como não envelhece em silêncio): qualquer execução dos
shards com `FALSIFY_TIMING_FILE` setada produz uma amostra nova -- inclusive
o job de CI, se um dia setar a variável (não faz isso hoje; ver handoff do
ML-2H). Cenário/rótulo NOVO não aparece neste arquivo até a PRÓXIMA
calibração rodar; `gen-falsify-chunks.py` nunca trata isso como peso zero --
usa o peso PESSIMISTA (máximo já calibrado) e avisa em stderr, nomeando o
rótulo, toda vez que empacota. Refazer a calibração é sempre: rodar os N
shards com a variável setada, um por vez ou concorrentes SEM overlap de
arquivo (uma FALSIFY_TIMING_FILE por shard, concatenados depois -- ver
receita no roadmap), e reinvocar este script sobre o arquivo concatenado.

Uso:
    python3 gen-falsify-scenario-weights.py <arquivo-de-marcas> <weights-out.json>

Pode ser chamado várias vezes com arquivos de marcas diferentes concatenados
manualmente (`cat timing-*.log > timing-full.log`) -- não faz merge com um
arquivo de pesos anterior; cada chamada é uma calibração nova e completa.
"""
import sys
import re
import json
import collections
import datetime

MARK = re.compile(
    r'^FALSIFY_TIMING phase=(start|end) block=(\S+) labels=(\S+) ts=([0-9.]+)$'
)


def parse_marks(path):
    """Casa marcas start/end pelo mesmo block_id. Marca sem par (arquivo
    truncado por um run interrompido) é DESCARTADA e contada, nunca
    interpretada como duração 0 -- duração 0 sub-alocaria o bloco real.

    Bloco SEM rótulo extraível (`labels=none`, ver `assign_weights` em
    gen-falsify-chunks.py -- o caso "AVISO bloco ... nao produziu nenhum
    rotulo") tem sua duração real medida do MESMO jeito -- não é
    descartado. A duração desses blocos alimenta `unlabeled_durations`,
    separado de `durations_by_label`, para virar
    `_fallback_weight_for_unlabeled` -- em SEGUNDOS, não linhas. Sem isso,
    `assign_weights` cairia de volta para peso por LINHA nesse ramo mesmo
    com arquivo de pesos presente -- unidade errada dentro de um pacote
    calibrado em segundos (achado da auditoria desta entrega: 46% da massa
    de empacotamento era contagem de linha disfarçada de segundo)."""
    starts = {}
    durations_by_label = collections.defaultdict(list)
    unlabeled_durations = []
    unmatched = 0
    total_pairs = 0
    with open(path, encoding='utf-8') as fh:
        for raw in fh:
            m = MARK.match(raw.strip())
            if not m:
                continue
            phase, block_id, labels_csv, ts_s = m.groups()
            ts = float(ts_s)
            labels = [l for l in labels_csv.split(',') if l and l != 'none']
            if phase == 'start':
                starts[block_id] = (ts, labels)
                continue
            if block_id not in starts:
                unmatched += 1
                continue
            start_ts, start_labels = starts.pop(block_id)
            duration = ts - start_ts
            if duration < 0:
                unmatched += 1
                continue
            labels = labels or start_labels
            if not labels:
                unlabeled_durations.append(duration)
                total_pairs += 1
                continue
            total_pairs += 1
            per_label = duration / len(labels)
            for l in labels:
                durations_by_label[l].append(per_label)
    unmatched += len(starts)  # starts sem end correspondente
    return durations_by_label, unlabeled_durations, total_pairs, unmatched


def main():
    if len(sys.argv) != 3:
        raise SystemExit(f"uso: {sys.argv[0]} <arquivo-de-marcas> <weights-out.json>")
    timing_path, out_path = sys.argv[1], sys.argv[2]

    durations_by_label, unlabeled_durations, total_pairs, unmatched = parse_marks(timing_path)
    if not durations_by_label and not unlabeled_durations:
        raise SystemExit(
            f"gen-falsify-scenario-weights: nenhuma marca FALSIFY_TIMING "
            f"casada (start+end) em '{timing_path}' -- nada para calibrar. "
            "Rode os shards com FALSIFY_TIMING_FILE setada primeiro."
        )

    weights = {
        label: round(sum(vals) / len(vals), 4)
        for label, vals in durations_by_label.items()
    }

    # Blocos SEM rotulo extraivel (ver docstring de parse_marks) tem peso
    # pessimista proprio, em SEGUNDOS -- nunca peso por linha misturado num
    # arquivo calibrado em segundos (achado da auditoria desta entrega: 46%
    # da massa de empacotamento era linha disfarcada de segundo porque este
    # ramo nao existia e assign_weights caia para n_lines). Pessimista =
    # MAXIMO observado entre os blocos sem rotulo, mesma politica de
    # "nunca subalocar" que os rotulos individuais ja seguem.
    fallback_unlabeled = round(max(unlabeled_durations), 4) if unlabeled_durations else None

    payload = {
        "_method": (
            "peso(rotulo) = media, entre ocorrencias, de "
            "(duracao_do_bloco_fundido / n_rotulos_do_bloco); duracao = "
            "FALSIFY_TIMING end.ts - start.ts (EPOCHREALTIME) por bloco -- "
            "ver docstring de gen-falsify-scenario-weights.py. "
            "_fallback_weight_for_unlabeled = MAXIMO medido entre blocos sem "
            "rotulo extraivel (labels=none) -- usado por assign_weights no "
            "ramo 'bloco sem weight_keys', em segundos, nunca linha."
        ),
        "_n_labels": len(weights),
        "_n_blocks_pareados": total_pairs,
        "_n_blocos_sem_rotulo": len(unlabeled_durations),
        "_n_marcas_sem_par": unmatched,
        "_fallback_weight_for_unlabeled": fallback_unlabeled,
        "_calibrated_at": datetime.datetime.now(datetime.timezone.utc).strftime(
            "%Y-%m-%dT%H:%M:%SZ"
        ),
        "_calibrated_from": timing_path,
        "weights": weights,
    }
    with open(out_path, 'w', encoding='utf-8') as fh:
        json.dump(payload, fh, indent=2, sort_keys=True, ensure_ascii=False)
        fh.write('\n')

    print(
        f"gen-falsify-scenario-weights: {len(weights)} rotulos calibrados "
        f"de {total_pairs} blocos ({len(unlabeled_durations)} sem rotulo, "
        f"fallback={fallback_unlabeled}, {unmatched} marca(s) sem par "
        f"descartada(s)) -- escrito em {out_path}"
    )


if __name__ == "__main__":
    main()
