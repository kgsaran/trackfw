"""
serve/api_metrics.py — Metrics API: lead time, cycle time, burndown.
Reutiliza _parse_log de commands/metrics.py — sem reimplementação.
Espelho Python de internal/serve/api_metrics.go e npm/src/serve/api_metrics.js.
"""

import os
from datetime import datetime, timedelta

from trackfw.commands.metrics import _parse_log

STATES = ["wip", "backlog", "blocked", "done", "abandoned"]
LOG_PATH = os.path.join("docs", "roadmaps", ".trackfw-log")


def _state_distribution(cfg):
    """
    Conta arquivos .md por estado varrendo os diretórios de roadmap.
    """
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    namespacing = cfg.get("roadmap_namespacing", "flat")
    distribution = {s: 0 for s in STATES}

    def _count_dir(dir_path, state):
        if not os.path.isdir(dir_path):
            return 0
        try:
            return sum(
                1 for f in os.listdir(dir_path)
                if f.endswith(".md") and not os.path.isdir(os.path.join(dir_path, f))
            )
        except OSError:
            return 0

    if namespacing == "by_agent":
        agents = cfg.get("agents") or []
        if not agents:
            try:
                agents = [
                    e for e in os.listdir(roadmap_dir)
                    if os.path.isdir(os.path.join(roadmap_dir, e))
                ]
            except OSError:
                agents = []
        for agent in agents:
            for state in STATES:
                distribution[state] += _count_dir(os.path.join(roadmap_dir, agent, state), state)
    else:
        for state in STATES:
            distribution[state] += _count_dir(os.path.join(roadmap_dir, state), state)

    return distribution


def _calc_lead_time(transitions):
    """
    Lead time: do primeiro movimento (qualquer → backlog) até done.
    Retorna média em dias.
    """
    by_name = {}
    for tr in transitions:
        by_name.setdefault(tr["basename"], []).append(tr)

    lead_times = []
    for entries in by_name.values():
        start_ts = None
        done_ts = None
        for e in sorted(entries, key=lambda x: x["timestamp"]):
            if start_ts is None:
                start_ts = e["timestamp"]
            if e["to"] == "done":
                done_ts = e["timestamp"]
        if start_ts is not None and done_ts is not None:
            delta_days = (done_ts - start_ts).total_seconds() / 86400.0
            lead_times.append(delta_days)

    if not lead_times:
        return 0.0
    return sum(lead_times) / len(lead_times)


def _calc_cycle_time(transitions):
    """
    Cycle time: da entrada em wip até done.
    Retorna média em dias.
    """
    by_name = {}
    for tr in transitions:
        by_name.setdefault(tr["basename"], []).append(tr)

    cycle_times = []
    for entries in by_name.values():
        wip_ts = None
        done_ts = None
        for e in sorted(entries, key=lambda x: x["timestamp"]):
            if e["to"] == "wip" and wip_ts is None:
                wip_ts = e["timestamp"]
            if e["to"] == "done":
                done_ts = e["timestamp"]
        if wip_ts is not None and done_ts is not None:
            delta_days = (done_ts - wip_ts).total_seconds() / 86400.0
            cycle_times.append(delta_days)

    if not cycle_times:
        return 0.0
    return sum(cycle_times) / len(cycle_times)


def _calc_abandonment_rate(transitions):
    """
    Taxa de abandono: count(abandoned) / count(abandoned + done).
    """
    by_name = {}
    for tr in transitions:
        by_name.setdefault(tr["basename"], []).append(tr)

    done_count = 0
    abandoned_count = 0
    for entries in by_name.values():
        final_states = {e["to"] for e in entries}
        if "abandoned" in final_states:
            abandoned_count += 1
        elif "done" in final_states:
            done_count += 1

    total = done_count + abandoned_count
    if total == 0:
        return 0.0
    return abandoned_count / total


def _calc_burndown(transitions):
    """
    Burndown semanal: para cada semana, quantos roadmaps estão abertos e fechados.
    Retorna lista de { date, open, closed }.
    """
    if not transitions:
        return []

    timestamps = [tr["timestamp"] for tr in transitions]
    min_ts = min(timestamps)
    max_ts = max(timestamps)

    # Normalizar para início da semana (segunda-feira)
    start = min_ts - timedelta(days=min_ts.weekday())
    start = start.replace(hour=0, minute=0, second=0, microsecond=0)
    end = max_ts + timedelta(days=7)

    # Construir eventos por basename
    by_name = {}
    for tr in transitions:
        by_name.setdefault(tr["basename"], []).append(tr)

    burndown = []
    current = start
    while current <= end:
        week_end = current + timedelta(days=7)
        open_count = 0
        closed_count = 0

        for entries in by_name.values():
            # Pegar o estado final até o fim da semana
            state_at_week = None
            for e in sorted(entries, key=lambda x: x["timestamp"]):
                if e["timestamp"] <= week_end:
                    state_at_week = e["to"]
            if state_at_week is None:
                pass
            elif state_at_week in ("done", "abandoned"):
                closed_count += 1
            else:
                open_count += 1

        burndown.append({
            "date": current.strftime("%Y-%m-%d"),
            "open": open_count,
            "closed": closed_count,
        })
        current = week_end

    return burndown


def get_metrics(cfg):
    """
    Calcula e retorna métricas de entrega a partir do .trackfw-log.
    """
    log_path = os.path.join(cfg.get("roadmap_dir", "docs/roadmaps"), ".trackfw-log")
    # Fallback para path padrão se não existir
    if not os.path.exists(log_path):
        log_path = LOG_PATH

    transitions = _parse_log(log_path)

    lead_time = _calc_lead_time(transitions)
    cycle_time = _calc_cycle_time(transitions)
    abandonment_rate = _calc_abandonment_rate(transitions)
    state_distribution = _state_distribution(cfg)
    burndown = _calc_burndown(transitions)

    return {
        "lead_time_avg_days": round(lead_time, 2),
        "cycle_time_avg_days": round(cycle_time, 2),
        "abandonment_rate": round(abandonment_rate, 4),
        "state_distribution": state_distribution,
        "burndown": burndown,
    }
