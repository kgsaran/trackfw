"""
commands/metrics.py — Subcomando `trackfw metrics`.
Lê .trackfw-log e calcula throughput, cycle time e WIP age.
Espelho Python de npm/src/commands/metrics.js.
"""

import os
import re
import sys
from datetime import datetime, timedelta

# Regex que faz match do formato do .trackfw-log
# 2026-06-12 14:30  ROADMAP-2026-06-12-auth.md                  backlog → wip
LINE_RE = re.compile(
    r"^(\d{4}-\d{2}-\d{2} \d{2}:\d{2})\s{2,}(\S+)\s{2,}(\S+)\s+→\s+(\S+)"
)

LOG_PATH = os.path.join("docs", "roadmaps", ".trackfw-log")


def _parse_log(file_path):
    """
    Lê o arquivo .trackfw-log e retorna lista de transições.
    Retorna [] se o arquivo não existe.
    """
    if not os.path.exists(file_path):
        return []
    transitions = []
    try:
        with open(file_path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return []
    for line in content.split("\n"):
        line = line.strip()
        if not line:
            continue
        m = LINE_RE.match(line)
        if not m:
            continue
        ts_str, basename, from_state, to_state = m.group(1), m.group(2), m.group(3), m.group(4)
        try:
            timestamp = datetime.strptime(ts_str, "%Y-%m-%d %H:%M")
        except ValueError:
            continue
        transitions.append({
            "timestamp": timestamp,
            "basename": basename.strip(),
            "from": from_state.strip(),
            "to": to_state.strip(),
        })
    return transitions


def _filter(transitions, since_dt):
    """Retorna transições com timestamp >= since_dt."""
    return [t for t in transitions if t["timestamp"] >= since_dt]


def _calculate(transitions):
    """
    Computa cycle time médio, throughput e WIP age.
    Retorna dict {cycle_time_mean_s, throughput, wip_entries}.
    """
    by_name = {}
    for tr in transitions:
        name = tr["basename"]
        by_name.setdefault(name, []).append(tr)

    # Cycle time: da entrada em backlog ou wip até done
    cycle_times = []
    for entries in by_name.values():
        start_ts = None
        done_ts = None
        for e in entries:
            if e["to"] in ("backlog", "wip") and start_ts is None:
                start_ts = e["timestamp"]
            if e["to"] == "done":
                done_ts = e["timestamp"]
        if start_ts is not None and done_ts is not None:
            delta = (done_ts - start_ts).total_seconds()
            cycle_times.append(delta)

    cycle_time_mean_s = 0.0
    if cycle_times:
        cycle_time_mean_s = sum(cycle_times) / len(cycle_times)

    # Throughput: roadmaps done por semana
    done_count = 0
    timestamps = [tr["timestamp"] for tr in transitions]
    for tr in transitions:
        if tr["to"] == "done":
            done_count += 1

    throughput = 0.0
    if done_count > 0 and timestamps:
        min_ts = min(timestamps)
        max_ts = max(timestamps)
        delta_s = (max_ts - min_ts).total_seconds()
        weeks = delta_s / (7 * 24 * 3600)
        if weeks < 1:
            weeks = 1
        throughput = done_count / weeks

    # WIP age: basenames em wip sem done ou abandoned posterior
    now = datetime.now()
    wip_entries = []
    for basename, entries in by_name.items():
        wip_ts = None
        concluded = False
        for e in entries:
            if e["to"] == "wip":
                wip_ts = e["timestamp"]
            if e["to"] in ("done", "abandoned"):
                concluded = True
        if wip_ts is not None and not concluded:
            age_s = (now - wip_ts).total_seconds()
            wip_entries.append({"basename": basename, "age_s": age_s})

    return {
        "cycle_time_mean_s": cycle_time_mean_s,
        "throughput": throughput,
        "wip_entries": wip_entries,
    }


def _format_duration(seconds):
    """Formata segundos em string legível (days/hours)."""
    total_hours = int(seconds / 3600)
    days = total_hours // 24
    hours = total_hours % 24
    if days > 0:
        return f"{days} days {hours} hours"
    return f"{hours} hours"


def _export_csv(metrics, transitions, file_path):
    """Grava transições e métricas em um arquivo CSV."""
    rows = ["basename,from,to,timestamp"]
    for tr in transitions:
        ts = tr["timestamp"].strftime("%Y-%m-%d %H:%M")
        rows.append(f"{tr['basename']},{tr['from']},{tr['to']},{ts}")
    rows.append("")
    rows.append("metric,value")
    cycle_hours = metrics["cycle_time_mean_s"] / 3600
    rows.append(f"cycle_time_mean_hours,{cycle_hours:.2f}")
    rows.append(f"throughput_per_week,{metrics['throughput']:.2f}")
    rows.append(f"wip_count,{len(metrics['wip_entries'])}")
    with open(file_path, "w", encoding="utf-8") as f:
        f.write("\n".join(rows) + "\n")


def _print_metrics(metrics):
    """Imprime as métricas em formato tabela ASCII."""
    print("── trackfw metrics ──────────────────────")

    if metrics["cycle_time_mean_s"] > 0:
        print(f"  Cycle Time Mean   : {_format_duration(metrics['cycle_time_mean_s'])}")
    else:
        print("  Cycle Time Mean   : n/a (no completed cycles)")

    if metrics["throughput"] > 0:
        print(f"  Throughput        : {metrics['throughput']:.2f} roadmaps/week")
    else:
        print("  Throughput        : n/a (no completed roadmaps)")

    wip = metrics["wip_entries"]
    if not wip:
        print("  WIP Age           : no items in progress")
    else:
        print(f"  WIP Age ({len(wip)} items) :")
        for w in wip:
            print(f"    - {w['basename']}: {_format_duration(w['age_s'])}")

    print("─────────────────────────────────────────")


def _parse_days(s):
    """Converte '7d', '30d', '90d' em número de dias (int)."""
    s = s.strip()
    if not s or s[-1] != "d":
        raise ValueError(f"formato inválido: '{s}' (use: 7d, 30d, 90d)")
    try:
        n = int(s[:-1])
    except ValueError:
        raise ValueError(f"número inválido em '{s}'")
    if n <= 0:
        raise ValueError(f"número deve ser positivo: '{s}'")
    return n


def register(subparsers):
    """Adiciona subcomando `metrics` ao parser principal."""
    parser = subparsers.add_parser(
        "metrics",
        help="Show delivery metrics from .trackfw-log",
    )
    parser.add_argument(
        "--days",
        metavar="N",
        default=None,
        help="Filter to last N days (e.g. --days 30)",
    )
    parser.add_argument(
        "--since",
        metavar="PERIOD",
        default=None,
        help="Filter period in format Nd (e.g. --since 30d)",
    )
    parser.add_argument(
        "--export",
        metavar="FILE",
        default=None,
        help="Export metrics to CSV file",
    )
    parser.set_defaults(func=_cmd_metrics)


def _cmd_metrics(args):
    transitions = _parse_log(LOG_PATH)

    if not transitions:
        print("No log found")
        return

    # Suporte a --days N e --since Nd
    since_dt = None
    if getattr(args, "days", None):
        try:
            n = int(args.days)
            since_dt = datetime.now() - timedelta(days=n)
        except ValueError:
            print(f"invalid --days value: {args.days}", file=sys.stderr)
            sys.exit(1)
    elif getattr(args, "since", None):
        try:
            n = _parse_days(args.since)
            since_dt = datetime.now() - timedelta(days=n)
        except ValueError as e:
            print(f"invalid --since format: {e}", file=sys.stderr)
            sys.exit(1)

    if since_dt is not None:
        transitions = _filter(transitions, since_dt)
        if not transitions:
            print("No log found")
            return

    metrics = _calculate(transitions)
    _print_metrics(metrics)

    if getattr(args, "export", None):
        _export_csv(metrics, transitions, args.export)
        print(f"exported to {args.export}")
