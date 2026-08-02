"""
status.py — Comando `trackfw status`.

Soma as duas visões que existiam nos três CLIs: o inventário agregado de
ADRs/REQs/Roadmaps (herdado da implementação Python original) e a visão
acionável com moldura (WIP, Blocked, Done, REQs bloqueadas por ADR não
aceito) que já existia em Go/Node. Layout consolidado nesta convergência
(ML-1C) — Go e Node ainda não tinham a seção "📊 Inventory" antes deste
ciclo; o texto da seção "⏳ REQs blocked by not-accepted ADRs" foi copiado
literalmente de internal/validator/validator.go (GetStatus).
"""

import os

from .. import config as _config
from .. import validator as _validator

# Seis estados de roadmap. Usado nos três pontos que antes enumeravam
# apenas 5 (faltava "analyzing") — bug corrigido nesta convergência.
_ROADMAP_STATES = ["backlog", "analyzing", "wip", "blocked", "done", "abandoned"]

_FRAME_TOP = "── trackfw status ──────────────────────"
_FRAME_BOTTOM = "────────────────────────────────────────"


def _list_files(path: str) -> list:
    """Retorna lista de arquivos (não-diretórios) em path. Retorna [] se não existir."""
    try:
        entries = []
        for name in sorted(os.listdir(path)):
            full = os.path.join(path, name)
            if not os.path.isdir(full):
                entries.append(name)
        return entries
    except OSError:
        return []


def _list_dirs(path: str) -> list:
    """Retorna lista de subdiretórios em path. Retorna [] se não existir."""
    try:
        return [
            name for name in sorted(os.listdir(path))
            if os.path.isdir(os.path.join(path, name))
        ]
    except OSError:
        return []


def _count_reqs_by_status(req_dir: str) -> dict:
    """
    Conta REQs e agrupa por Status (Open/Done/Closed/Other) lendo o frontmatter.
    "Done" e "Closed" são discriminados — REQ entregue não é a mesma coisa que
    REQ encerrada sem entrega (defeito corrigido nesta convergência).
    Retorna {"total": N, "open": X, "done": Y, "closed": Z, "other": W}.
    """
    files = _list_files(req_dir)
    counts = {"total": len(files), "open": 0, "done": 0, "closed": 0, "other": 0}
    for name in files:
        path = os.path.join(req_dir, name)
        try:
            with open(path, "r", encoding="utf-8") as f:
                content = f.read()
            fm = _validator.parse_frontmatter(content)
            status = fm.get("status", "").strip().lower()
            if status == "open":
                counts["open"] += 1
            elif status == "done":
                counts["done"] += 1
            elif status == "closed":
                counts["closed"] += 1
            else:
                counts["other"] += 1
        except OSError:
            counts["other"] += 1
    return counts


def _count_adrs(adr_dirs: list) -> int:
    """Conta total de ADRs em todos os adr_dirs."""
    total = 0
    for adr_dir in adr_dirs:
        total += len(_list_files(adr_dir))
    return total


def _roadmap_counts_flat(roadmap_dir: str) -> dict:
    """Conta roadmaps por estado no modo flat (6 estados)."""
    return {
        state: len(_list_files(os.path.join(roadmap_dir, state)))
        for state in _ROADMAP_STATES
    }


def _roadmap_counts_by_agent(roadmap_dir: str, agents: list) -> dict:
    """Retorna dict: agent → {state: count} (6 estados)."""
    result = {}
    for agent in agents:
        agent_dir = os.path.join(roadmap_dir, agent)
        result[agent] = {
            state: len(_list_files(os.path.join(agent_dir, state)))
            for state in _ROADMAP_STATES
        }
    return result


def _get_agents(cfg: dict) -> list:
    """Descobre os agentes: da config ou do filesystem."""
    agents = cfg.get("agents") or []
    if not agents:
        roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
        agents = _list_dirs(roadmap_dir)
    return agents


def _resolve(base: str, path: str) -> str:
    """Resolve path relativo a base se não for absoluto."""
    if os.path.isabs(path):
        return path
    return os.path.join(base, path)


def _blocked_reqs(cfg_local: dict) -> dict:
    """
    Mapa ordenado de REQ-basename → lista de ADR-basenames não aceitos (Draft ou
    Proposed) que a bloqueiam. Somente REQs com "Status: Open" no cabeçalho são
    incluídas. Espelha blockedREQs() de internal/validator/validator.go — mesma
    semântica, reaproveitando os helpers já existentes em trackfw/validator.py
    (_parse_blocked_adrs, _adr_draft_status_for_rule) em vez de duplicá-los.
    cfg_local deve conter req_dir/adr_dirs já resolvidos (absolutos) para esta
    chamada — resolve_req_files() não conhece cwd, apenas o dict de config.
    """
    result = {}
    for req_path in sorted(_validator.resolve_req_files(cfg_local)):
        try:
            with open(req_path, "r", encoding="utf-8") as f:
                content = f.read()
        except OSError:
            continue
        if "Status: Open" not in content:
            continue

        adr_names = _validator._parse_blocked_adrs(req_path)
        not_accepted = []
        for adr_basename in adr_names:
            notacc, ok = _validator._adr_draft_status_for_rule(adr_basename, cfg_local, None)
            if notacc:
                not_accepted.append(adr_basename)
        if not_accepted:
            result[os.path.basename(req_path)] = not_accepted
    return result


def _adr_status_label(adr_basename: str, adr_dirs: list) -> str:
    """Status bruto (ex.: 'Draft', 'Proposed') do ADR identificado por basename."""
    path = _validator._find_adr_file(adr_basename, adr_dirs)
    if not path:
        return ""
    try:
        with open(path, "r", encoding="utf-8") as f:
            return _validator._extract_adr_status(f.read())
    except OSError:
        return ""


def get_status(cwd: str = None) -> str:
    """
    Retorna string formatada com o status de governança do projeto: moldura +
    inventário agregado (ADRs/REQs/Roadmaps, 6 estados, REQs discriminadas em
    Open/Done/Closed) + visão acionável (WIP, Blocked, Done, REQs bloqueadas
    por ADR não aceito).
    """
    _config.reset()
    cfg = _config.load(cwd)

    base = cwd or os.getcwd()
    roadmap_dir = _resolve(base, cfg.get("roadmap_dir", "docs/roadmaps"))
    req_dir = _resolve(base, cfg.get("req_dir", "docs/req"))
    adr_dirs = [_resolve(base, d) for d in cfg.get("adr_dirs", ["docs/adr"])]
    namespacing = cfg.get("roadmap_namespacing", _config.NAMESPACING_FLAT)

    # cfg_local: mesma config, mas com paths já resolvidos para absolutos —
    # usado pelos helpers de trackfw/validator.py que não recebem cwd.
    cfg_local = dict(cfg)
    cfg_local["req_dir"] = req_dir
    cfg_local["roadmap_dir"] = roadmap_dir
    cfg_local["adr_dirs"] = adr_dirs

    adr_count = _count_adrs(adr_dirs)
    req_counts = _count_reqs_by_status(req_dir)

    lines = [_FRAME_TOP, ""]

    # ── 📊 Inventory ────────────────────────────────────────────────
    lines.append("📊 Inventory")
    lines.append(f"   {'ADRs':<12}{adr_count}")

    req_detail = f"{req_counts['open']} Open · {req_counts['done']} Done · {req_counts['closed']} Closed"
    if req_counts["other"] > 0:
        req_detail += f" · {req_counts['other']} Other"
    lines.append(f"   {'REQs':<12}{req_counts['total']}  ({req_detail})")

    if namespacing == _config.NAMESPACING_BY_AGENT:
        # _get_agents(cfg_local), não _get_agents(cfg): o fallback de _get_agents lista
        # os subdiretórios de roadmap_dir quando "agents" não está configurado, e o
        # cfg bruto carrega roadmap_dir ainda relativo — resolvido incorretamente
        # contra o cwd do processo, não contra o cwd= passado a get_status(). Bug
        # pré-existente descoberto durante o ML-2B (auditoria de by_agent); corrigido
        # aqui porque alimenta diretamente a seção "⚙ WIP by Agent" reescrita neste ML.
        agents = _get_agents(cfg_local)
        by_agent = _roadmap_counts_by_agent(roadmap_dir, agents)
        totals = {
            state: sum(by_agent[a][state] for a in agents)
            for state in _ROADMAP_STATES
        }
    else:
        totals = _roadmap_counts_flat(roadmap_dir)

    roadmap_total = sum(totals[state] for state in _ROADMAP_STATES)
    lines.append(f"   {'Roadmaps':<12}{roadmap_total}")
    lines.append(
        f"     backlog {totals['backlog']} · analyzing {totals['analyzing']} · wip {totals['wip']}"
    )
    lines.append(
        f"     blocked {totals['blocked']} · done {totals['done']} · abandoned {totals['abandoned']}"
    )

    if namespacing == _config.NAMESPACING_BY_AGENT:
        # ── ⚙ WIP by Agent ─────────────────────────────────────────
        # No modo by_agent, as seções flat (🔄 WIP / ❌ Blocked / ✅ Done) não se
        # aplicam e são omitidas — espelha GetStatus() em
        # internal/validator/validator.go (linhas 761-782) e getStatus() em
        # npm/src/validator/index.js (linhas 1371-1387). Lista arquivos reais de
        # cada agente (não os totais agregados usados no bloco Inventory acima).
        lines.append("")
        lines.append("⚙ WIP by Agent")
        for agent in agents:
            agent_wip = _list_files(os.path.join(roadmap_dir, agent, "wip"))
            if agent_wip:
                lines.append(f"  [{agent}] WIP ({len(agent_wip)})")
                for f in agent_wip:
                    lines.append(f"    {f}")
    else:
        # ── 🔄 WIP / ❌ Blocked / ✅ Done ───────────────────────────
        wip = _list_files(os.path.join(roadmap_dir, "wip"))
        blocked = _list_files(os.path.join(roadmap_dir, "blocked"))
        done = _list_files(os.path.join(roadmap_dir, "done"))

        lines.append("")
        lines.append(f"🔄 WIP ({len(wip)})")
        for f in wip:
            lines.append(f"   {f}")

        # ── ⚙ WIP by Squad (condicional) — espelha readWIPConfig()/bySquad de
        # internal/validator/validator.go e npm/src/validator/index.js.
        wip_cfg = _validator._wip_config_from(cfg)
        if wip_cfg.get("by_squad") and wip:
            by_squad = {}
            for f in wip:
                squad = _validator._parse_squad_from_frontmatter(os.path.join(roadmap_dir, "wip", f))
                if not squad:
                    squad = "(no squad)"
                by_squad[squad] = by_squad.get(squad, 0) + 1
            lines.append("")
            lines.append(f"⚙ WIP by Squad (limit: {wip_cfg['limit']} per squad)")
            for squad, count in by_squad.items():
                status_mark = "⚠" if count > wip_cfg["limit"] else "✓"
                noun = "roadmap" if count == 1 else "roadmaps"
                lines.append(f"   {(squad + ':'):<20} {count} {noun}  {status_mark}")

        lines.append("")
        lines.append(f"❌ Blocked ({len(blocked)})")
        for f in blocked:
            lines.append(f"   {f}")

        # ── ⚠ Stale WIP (condicional) — espelha validateStaleWIP() de Go/Node.
        stale_wip = _validator.validate_stale_wip(cfg_local)
        if stale_wip:
            lines.append("")
            lines.append(f"⚠  Stale WIP ({len(stale_wip)})")
            for w in stale_wip:
                message = w["message"] if isinstance(w, dict) else w
                if message.strip():
                    lines.append(f"   {message}")

        # ── ⏳ REQs blocked by not-accepted ADRs (condicional) ─────
        blocked_by_draft = _blocked_reqs(cfg_local)
        if blocked_by_draft:
            lines.append("")
            lines.append(f"⏳ REQs blocked by not-accepted ADRs ({len(blocked_by_draft)})")
            for req_file, adrs in blocked_by_draft.items():
                lines.append(f"   {req_file}")
                for adr in adrs:
                    status = _adr_status_label(adr, adr_dirs)
                    lines.append(f"     → {adr} ({status})")

        lines.append("")
        lines.append("✅ Done (last 5)")
        for f in done[-5:]:
            lines.append(f"   {f}")

    lines.append("")
    lines.append(_FRAME_BOTTOM)

    return "\n".join(lines) + "\n"


def register(subparsers):
    """Registra o subcomando 'status' no parser principal."""
    parser = subparsers.add_parser(
        "status",
        help="Exibe o status de governança do projeto",
    )
    parser.set_defaults(func=run)
    return parser


def run(args):
    """Executa o status e imprime o resultado."""
    print(get_status(), end="")
    return 0
