"""
validator.py — Validações de governança do trackfw.
Espelho Python de npm/src/validator/index.js (paridade de comportamento).
Stdlib apenas: os, pathlib, re, sys, datetime, subprocess.
"""

import glob as _glob
import json
import os
import re
import subprocess
import sys
from datetime import datetime, timezone

from . import config as _config
from .traceid import check_traceid
from trackfw.homedir import home_dir, expand_path

# _current_platform is seeded from sys.platform at import time. Tests override it
# via _set_platform_for_test to exercise the Windows guard on any host.
#
# Why the mode checks need it: on Windows the POSIX execute bit is not
# representable on NTFS, so "is this script executable?" has no truthful answer.
# os.access(path, os.X_OK) answers True for every existing file there — which
# means this runtime SILENTLY DIVERGED from Go and Node.js, that answer False for
# the same file. Same repo, three runtimes, two different `validate` outputs.
# Guarding explicitly makes all three agree, and says so in the code.
#
# Mirrors internal/validator/goos.go (Go, canonical) and
# npm/src/validator/index.js (Node.js).
_current_platform: str = sys.platform


def _set_platform_for_test(platform: str):
    """Override _current_platform for unit tests. Returns a restore callable."""
    global _current_platform
    prev = _current_platform
    _current_platform = platform

    def restore():
        global _current_platform
        _current_platform = prev

    return restore


STALE_WIP_DAYS = 7


# ---------------------------------------------------------------------------
# Invocação de git isolada de redirecionamento por ambiente
# ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1B.
# ADR: docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-
# estrita-entre-head-e-disco.md (Emenda 3).
#
# Achado do ML-3B, reproduzido por Zeus: _head_trackfw_yaml()/_is_git_worktree() invocavam
# subprocess.run(["git", ...], cwd=...) sem limpar o ambiente herdado do processo —
# GIT_DIR/GIT_WORK_TREE redirecionados para outro repositório git (sem trackfw.yaml versionado)
# faziam a resolução do HEAD falhar EM SILÊNCIO, e credentialGuardRuleSeverity caía só no disco:
# derrota o mecanismo M4 inteiro sem nenhum commit e sem sequer editar trackfw.yaml. Exposição
# NOVA para credential_guard_script_integrity e credential_guard_hook_resolvable — elas não
# dependiam de git antes do M4.
#
# TODA invocação de git deste módulo passa a ir por _git_run() abaixo — nunca chamar
# subprocess.run(["git", ...]) diretamente fora desta seção.
# ---------------------------------------------------------------------------

# GIT_ENV_PREFIX é o prefixo de TODA variável de ambiente que o git(1) reconhece como
# configuração (GIT_DIR, GIT_WORK_TREE, GIT_CONFIG_*, GIT_CEILING_DIRECTORIES, etc.).
#
# A tentativa inicial deste ML era uma lista fechada (GIT_DIR, GIT_WORK_TREE, GIT_COMMON_DIR,
# GIT_INDEX_FILE, GIT_OBJECT_DIRECTORY, GIT_ALTERNATE_OBJECT_DIRECTORIES,
# GIT_CEILING_DIRECTORIES, GIT_NAMESPACE) justificada por "variáveis que redirecionam ONDE o
# repositório é lido". Essa justificativa estava ERRADA: o vetor real não é redirecionamento, é
# fazer o `git` sair com status != 0 por QUALQUER motivo — toda chamada deste módulo trata falha
# do subprocesso como "sem âncora, silêncio" (_head_trackfw_yaml) ou "fallback para disco"
# (_credential_guard_rule_severity), então qualquer variável capaz de tornar o git fatal já é um
# bypass, redirecionando ou não. Provado com GIT_CONFIG_COUNT=abc (fora da lista fechada — injeta
# config arbitrária via GIT_CONFIG_COUNT/GIT_CONFIG_KEY_n/GIT_CONFIG_VALUE_n, e um valor
# malformado faz `git rev-parse --is-inside-work-tree` sair com "fatal: unable to parse
# command-line config", exit 128): a lista fechada não a cobria, e
# `credential_guard_mode_downgrade` silenciava por inteiro (_head_trackfw_yaml retornava
# ok=False), não só a severidade das outras duas regras.
#
# Por isso a abordagem correta é NEGATIVA por prefixo, não uma enumeração positiva: nenhuma
# invocação deste módulo (`rev-parse`, `show`, `log`, `symbolic-ref`) depende de qualquer GIT_*
# herdada do ambiente para funcionar corretamente — `-C cwd` já ancora explicitamente o
# repositório, então git redescobre a partir de cwd como se tivesse sido iniciado lá. Contextos
# legítimos que setam GIT_* (hooks do próprio git, `git submodule foreach`, worktrees vinculadas)
# continuam funcionando sem essas variáveis porque a descoberta normal a partir de `-C cwd` já
# resolve o mesmo repositório. Mesma abordagem e mesma justificativa dos irmãos Go/Node
# (internal/validator/validator_git_exec.go, npm/src/validator/git-exec.js) — manter em paridade.
GIT_ENV_PREFIX = "GIT_"


def _clean_git_env() -> dict:
    """Retorna uma cópia de os.environ sem nenhuma variável cujo nome comece com GIT_ENV_PREFIX —
    usado como env de toda invocação de git deste módulo."""
    return {k: v for k, v in os.environ.items() if not k.startswith(GIT_ENV_PREFIX)}


def _git_run(cwd: str, args: list, timeout: int = 5):
    """Executa `git -C cwd ...args` ancorado explicitamente em cwd via `-C` — nunca dependendo só
    do kwarg cwd do subprocess/descoberta implícita de repositório — e com toda variável GIT_*
    removida do ambiente herdado. Retorna subprocess.CompletedProcess; propaga exceções (timeout,
    git ausente) para o chamador, igual ao subprocess.run() cru que substituiu. cwd vazio/None cai
    para os.getcwd(), o mesmo comportamento que todo call site já assumia implicitamente antes
    deste ML.
    """
    root = cwd or os.getcwd()
    return subprocess.run(
        ["git", "-C", root] + list(args),
        capture_output=True, text=True, timeout=timeout,
        env=_clean_git_env(),
    )


# ---------------------------------------------------------------------------
# Helpers de field mapping e severidade (F2 + F3 — v2.4)
# ---------------------------------------------------------------------------

def _content_has_marker(content: str, markers: list) -> bool:
    """
    Retorna True se content contém qualquer marcador com valor não-vazio.
    Um marcador é considerado "sem valor" se a linha for exatamente
    "MARKER \n" ou "MARKER \r\n" (espaço + newline/CRLF) — P3: detecta
    campos vazios em arquivos CRLF além de arquivos LF.
    """
    for marker in markers:
        if marker in content and (marker + " \n") not in content and (marker + " \r\n") not in content:
            return True
    return False


# _RULE_DEFAULTS mapeia regras cujo default NÃO é 'error'.
_RULE_DEFAULTS = {
    "note_orphan": "warning",
    # ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A,
    # ADR-2026-08-12 Emenda 3: o script não carrega marcador de versão, então esta regra não
    # consegue distinguir drift legítimo (trackfw não atualizado ainda) de adulteração — fica
    # warning, nunca error. credential_guard_mode_downgrade fica deliberadamente ausente daqui:
    # cai no default "error" de _rule_severity.
    "credential_guard_script_integrity": "warning",
    # ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
    # desatualizados, ML-2B: mesmo raciocínio de credential_guard_script_integrity acima --
    # scripts/trackfw-git-branch-guard.sh também não carrega marcador de versão, então esta regra
    # não consegue distinguir drift legítimo de adulteração. git_branch_guard_hook_resolvable fica
    # deliberadamente ausente deste mapa (cai no default "error"), espelhando
    # credential_guard_hook_resolvable.
    "git_branch_guard_script_integrity": "warning",
    # ML-4A (REQ-2026-08-29, achado 1 do parecer hades-tf 2026-08-30): namespace oculto/ambíguo (nome
    # iniciado por ".") é sinal de baixo ruído por natureza, não um defeito de configuração como
    # agent_namespace_undeclared ("error"). Nunca "off" por default — silêncio total é o defeito que
    # esta REQ existe para fechar.
    "agent_namespace_hidden": "warning",
}


# ROADMAP-2026-08-12-ancorar-rules-no-head-para-as-regras-de-credential-guard, ML-1A.
# ADR: docs/adr/ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-
# estrita-entre-head-e-disco.md.
#
# As 3 regras abaixo resolvem severidade de forma DIFERENTE de todas as outras ~38: comparam HEAD
# contra disco e adotam a MAIS ESTRITA das duas, em vez de ler só o disco. Deliberado, não bug —
# sem isso, estas 3 regras podem ser desligadas pela mesma edição NÃO COMMITADA que elas deveriam
# denunciar (`rules: credential_guard_mode_downgrade: off` em trackfw.yaml, nunca commitado). Toda
# outra regra continua passando por _disk_rule_severity, byte-idêntico a antes deste ADR.
_CREDENTIAL_GUARD_ANCHORED_RULES = {
    "credential_guard_hook_resolvable",
    "credential_guard_script_integrity",
    "credential_guard_mode_downgrade",
}


def _credential_guard_severity_rank(s: str) -> int:
    """Ordena severidades da menos para a mais estrita, para a comparação "mais estrita vence" de
    _credential_guard_rule_severity. Qualquer valor fora de 'off'/'warning' só significa 'error' na
    prática — _apply_rule já trata qualquer valor não reconhecido como violation, então este
    ranking espelha esse mesmo fallback em vez de introduzir um contrato mais rígido.
    """
    if s == "off":
        return 0
    if s == "warning":
        return 1
    return 2


def _credential_guard_stricter_severity(a: str, b: str) -> str:
    """Retorna a mais estrita entre a e b ('error' > 'warning' > 'off')."""
    return a if _credential_guard_severity_rank(a) >= _credential_guard_severity_rank(b) else b


def _credential_guard_default_severity(name: str) -> str:
    """Mesmo fallback "_RULE_DEFAULTS > error" que _disk_rule_severity usa quando trackfw.yaml não
    tem rules: <name> — extraído para _credential_guard_rule_severity poder aplicá-lo igualmente ao
    lado HEAD (que não tem equivalente de _RULE_DEFAULTS próprio, já que
    config.parse_rules_from_content só devolve o que rules: em si contém).
    """
    return _RULE_DEFAULTS.get(name, "error")


def _credential_guard_rule_severity(name: str, cfg: dict, cwd: str = None) -> str:
    """Resolve a severidade de uma das 3 _CREDENTIAL_GUARD_ANCHORED_RULES como a MAIS ESTRITA entre
    HEAD e disco — direcional, não "ignora disco e usa só HEAD" (ver o parecer §2 e o ADR — o caso
    comum, HEAD sem menção à regra, precisa resolver para o default, ou seja o valor mais estrito
    possível, senão o disco venceria de volta silenciosamente sempre).

    Sem HEAD (não é git worktree, sem commits, ou trackfw.yaml não versionado no HEAD —
    _head_trackfw_yaml's 3 casos de "sem âncora"): cai no disco puro, igual a qualquer outra regra.
    ADR ponto de decisão 4: limite aceito, não um bypass acionável por adversário — nenhum desses 3
    casos é alcançável por uma edição não commitada de trackfw.yaml sozinha.
    """
    disk_severity = _disk_rule_severity(name, cfg)

    root = cwd or os.getcwd()
    head_content, ok = _head_trackfw_yaml(root)
    if not ok:
        return disk_severity

    head_rules = _config.parse_rules_from_content(head_content)
    head_severity = head_rules.get(name) or _credential_guard_default_severity(name)

    return _credential_guard_stricter_severity(head_severity, disk_severity)


def _rule_severity(name: str, cfg: dict, cwd: str = None) -> str:
    """Retorna severidade da regra: 'off' | 'warning' | 'error'.
    Prioridade: trackfw.yaml rules: > _RULE_DEFAULTS > 'error'.

    Para as 3 _CREDENTIAL_GUARD_ANCHORED_RULES, delega a _credential_guard_rule_severity acima —
    ver o comentário logo antes dessa constante para o porquê. Toda outra regra segue para
    _disk_rule_severity, textualmente idêntico ao corpo desta função antes do ADR-2026-08-12.
    """
    if name in _CREDENTIAL_GUARD_ANCHORED_RULES:
        return _credential_guard_rule_severity(name, cfg, cwd)
    return _disk_rule_severity(name, cfg)


def _disk_rule_severity(name: str, cfg: dict) -> str:
    """Resolução ordinária, só-disco, usada por toda regra exceto as 3
    _CREDENTIAL_GUARD_ANCHORED_RULES: trackfw.yaml rules: (CWD) > _RULE_DEFAULTS > 'error'.
    """
    rules = cfg.get("rules", {})
    if name in rules:
        return rules[name]
    if name in _RULE_DEFAULTS:
        return _RULE_DEFAULTS[name]
    return "error"


def _extract_file(msg: str) -> str:
    """Extrai o primeiro filename entre aspas duplas de uma mensagem. Retorna '' se ausente."""
    m = re.search(r'"([^"]+)"', msg)
    return m.group(1) if m else ""


def _enrich_items(items: list, rule_name: str) -> list:
    """
    Adiciona os campos 'rule' e 'file' a cada dict da lista, se ainda não presentes.
    Não modifica itens que já possuam esses campos.
    """
    result = []
    for item in items:
        if isinstance(item, dict):
            enriched = dict(item)
            if "rule" not in enriched:
                enriched["rule"] = rule_name
            if "file" not in enriched:
                enriched["file"] = _extract_file(enriched.get("message", ""))
            result.append(enriched)
        else:
            result.append(item)
    return result


def _apply_rule(rule_name: str, msgs: list, violations: list, warnings: list, cfg: dict, cwd: str = None):
    """
    Distribui msgs (lista de dicts) conforme a severidade configurada da regra.
    - 'off'     → descarta
    - 'warning' → adiciona a warnings
    - 'error'   → adiciona a violations (default)
    Enriquece cada item com 'rule' e 'file' antes de distribuir.

    cwd é repassado a _rule_severity só é consultado pelas 3 regras de credential-guard
    ancoradas no HEAD (ver _credential_guard_rule_severity) — toda outra regra o ignora.
    """
    if not msgs:
        return
    severity = _rule_severity(rule_name, cfg, cwd)
    if severity == "off":
        return
    enriched = _enrich_items(msgs, rule_name)
    if severity == "warning":
        warnings.extend(enriched)
    else:
        violations.extend(enriched)


# ---------------------------------------------------------------------------
# Utilitários internos
# ---------------------------------------------------------------------------

def list_dir(path: str) -> list:
    """
    Retorna lista de nomes de arquivo (não-diretórios) em path.
    Retorna [] se o diretório não existir ou ocorrer erro.
    """
    try:
        entries = []
        for name in os.listdir(path):
            try:
                full = os.path.join(path, name)
                if not os.path.isdir(full):
                    entries.append(name)
            except OSError:
                pass
        return entries
    except OSError:
        return []


def _try_list_dir(dir_path: str):
    """
    Tenta listar o diretório distinguindo "não existe" de outros erros.
    Retorna (entries: list, error: OSError|None).
    - error=None: sucesso, ou diretório ausente (ENOENT) — esperado para estados não usados.
    - error não-None: diretório EXISTE mas não pôde ser lido (ENOTDIR, EPERM…) — P2: reportar.
    """
    try:
        entries = []
        for name in os.listdir(dir_path):
            try:
                full = os.path.join(dir_path, name)
                if not os.path.isdir(full):
                    entries.append(name)
            except OSError:
                pass
        return entries, None
    except FileNotFoundError:
        return [], None  # diretório ausente — esperado
    except OSError as e:
        return [], e  # existe mas inacessível (ENOTDIR, EPERM…)


def _inspection_item(rule: str, target: str, err) -> dict:
    return {"type": "violation", "message": f'{rule}: could not inspect "{target}": {err}'}


def _list_dir_for_rule(rule: str, dir_path: str, messages: list) -> list:
    entries, err = _try_list_dir(dir_path)
    if err is not None:
        messages.append(_inspection_item(rule, dir_path, err))
    return entries


def _read_file_for_rule(rule: str, file_path: str, messages: list):
    try:
        with open(file_path, "r", encoding="utf-8") as f:
            return f.read()
    except OSError as e:
        messages.append(_inspection_item(rule, file_path, e))
        return None


def _walk_dir_md(dir_path: str) -> list:
    """Retorna basenames de todos .md recursivamente em dir_path."""
    return [os.path.basename(p) for p in _walk_dir_md_paths_for_rule("", dir_path, None)]


def _walk_dir_md_paths_for_rule(rule: str, dir_path: str, messages: list | None) -> list:
    """Retorna paths de todos .md recursivamente em dir_path e reporta falhas de walk."""
    result = []

    def onerror(err):
        if messages is not None and not isinstance(err, FileNotFoundError):
            messages.append(_inspection_item(rule, getattr(err, "filename", dir_path), err))

    for root, _, files in os.walk(dir_path, onerror=onerror):
        for name in files:
            if name.endswith(".md"):
                result.append(os.path.join(root, name))
    return result


def _is_subpath(path: str, parent: str) -> bool:
    """Retorna True se path está contido dentro do diretório parent (ambos absolutos)."""
    try:
        path_abs = os.path.abspath(path)
        parent_abs = os.path.abspath(parent)
        return os.path.commonpath([path_abs, parent_abs]) == parent_abs
    except ValueError:
        return False


def _find_adr_file(basename: str, adr_dirs: list) -> str:
    """Busca basename recursivamente em todos os adr_dirs. Retorna caminho completo ou ''."""
    for adr_dir in adr_dirs:
        expanded_dir = expand_path(adr_dir)
        try:
            for root, dirs, files in os.walk(expanded_dir):
                if basename in files:
                    return os.path.join(root, basename)
        except OSError:
            pass
    return ""


def _git_last_modified_time(file_path: str):
    """
    Retorna timestamp (float) do último commit que tocou o arquivo via git log.
    Retorna None se não for um repo git ou git não estiver disponível.
    """
    try:
        result = _git_run(".", ["log", "-1", "--format=%ct", "--", file_path])
        out = result.stdout.strip()
        if out:
            return float(out)
    except Exception:
        pass
    return None


_REF_DELIMITERS = ("\"", "'", "`")


def _strip_ref_delimiters(value: str) -> str:
    """
    Remove um delimitador (aspas duplas, simples ou backtick) de cada ponta,
    independentemente, sem exigir par casado — alinhado a Go (strings.Trim
    com cutset) e Node (regex de borda única). Uso contido ao caminho de
    extração de referência (_extract_ref_path); não afeta
    normalize_yaml_flat_value, que segue exigindo par casado para todo o
    resto do frontmatter (contrato do PR #104).
    """
    if value and value[0] in _REF_DELIMITERS:
        value = value[1:]
    if value and value[-1] in _REF_DELIMITERS:
        value = value[:-1]
    return value


def _extract_ref_path(content: str, field: str) -> str:
    """
    Extrai o caminho .md após 'field: valor' na mesma linha.
    Retorna '' se não encontrado ou não terminar em .md.
    """
    for line in content.split("\n"):
        trimmed = line.strip()
        if ":" not in trimmed:
            continue
        key, val = trimmed.split(":", 1)
        if key.strip().lower() == field.lower():
            val = val.strip()
            if not val or val in ("—", "-", "–"):
                return ""
            # Primeira "palavra" (antes de espaço)
            val = val.split()[0] if val.split() else ""
            val = _strip_ref_delimiters(val)
            if val.endswith(".md"):
                return val
    return ""


# resolve_agent_namespaces é re-exportado de trackfw.config (não definido aqui): trackfw.traceid é
# importado por este módulo (linha ~15) e também precisa do resolvedor canônico, então a
# implementação vive em config.py — o único módulo que nem validator nem traceid dependem de volta
# — para não introduzir um import cycle (validator → traceid → validator). Mantido acessível como
# `trackfw.validator.resolve_agent_namespaces` por compatibilidade com quem já importa daqui.
resolve_agent_namespaces = _config.resolve_agent_namespaces

# _AGENT_NAMESPACE_STATE_NAMES replica, só para esta regra, os 6 nomes de estado reservados de
# roadmap/REQ. Um diretório com um desses nomes no topo de roadmap_dir/req_dir é, na prática, resto
# de migração incompleta flat→by_agent (ex.: "wip" órfão) — não um agente. A união (decisão 1 do
# ADR) continua enumerando esses diretórios normalmente — nada fica invisível —, mas eles NÃO
# disparam validate_agent_namespace_undeclared: pedir para declarar "wip" como agente em agents:
# seria ruído confuso, não uma correção real (ML-0A, seção 3, item 3; recomendação adotada sem
# alteração). Esta exclusão vive só aqui, não no resolvedor — a colisão de nome não é
# "comprovadamente infraestrutura" como is_infra_dir_name, é uma inferência sobre o significado do
# nome, então só afeta a violação, nunca a união/enumeração.
_AGENT_NAMESPACE_STATE_NAMES = {"backlog", "analyzing", "wip", "blocked", "done", "abandoned"}


def _undeclared_namespaces_on_disk(cfg: dict, directory: str, declared: set) -> list:
    """Devolve, a partir do resolvedor canônico (que já filtra infra e não segue symlink), os nomes
    de namespace presentes em directory e ausentes de agents:, excluindo colisões com nome de estado
    reservado (_AGENT_NAMESPACE_STATE_NAMES) e nomes iniciados por "." (ML-4A: esses continuam na
    união — resolve_agent_namespaces não os filtra mais — mas não disparam a violação plena; ver
    _hidden_namespace_warnings para o aviso de baixo ruído que os substitui)."""
    out = []
    for name in resolve_agent_namespaces(cfg, directory):
        if name in declared or name in _AGENT_NAMESPACE_STATE_NAMES or _config.is_dot_prefixed_name(name):
            continue
        out.append(name)
    return out


def _dot_prefixed_undeclared_on_disk(cfg: dict, directory: str, declared: set) -> list:
    """Espelho de _undeclared_namespaces_on_disk para o caso ambíguo (nome iniciado por "."): mesmo
    resolvedor canônico, mesma exclusão de nomes já declarados, mas mantendo exatamente os nomes que
    _undeclared_namespaces_on_disk descarta por causa do ponto."""
    out = []
    for name in resolve_agent_namespaces(cfg, directory):
        if name in declared or not _config.is_dot_prefixed_name(name):
            continue
        out.append(name)
    return out


def validate_agent_namespace_undeclared(cfg: dict) -> list:
    """
    Regra "agent_namespace_undeclared" (ADR-2026-08-29, decisão 2 / REQ AC4, AC5, AC9): em modo
    by_agent, um namespace presente em disco (roadmap_dir e/ou req_dir — AC2 estende a união às duas
    árvores, e esta violação segue) e ausente de agents: é VIOLAÇÃO, não aviso — usa o mesmo default
    "error" de toda regra sem entrada em _RULE_DEFAULTS (_disk_rule_severity).

    A união já garante (Wave 1) que o namespace continua sendo ENUMERADO por todo consumidor mesmo
    com esta violação ativa — esta função só ADICIONA o sinal de configuração incompleta, nunca
    CONDICIONA a enumeração a ele (AC5-b).

    Deduplicação por namespace, não por árvore: o caso motivador (cmdb, "zeus" ausente de agents: e
    em disco em roadmap_dir E req_dir ao mesmo tempo) produziria duas violações quase-idênticas se o
    laço fosse por árvore — ruído no caso comum, não no caso raro. Uma violação por nome, nomeando
    todas as árvores onde ele foi encontrado.
    """
    if cfg.get("roadmap_namespacing") != _config.NAMESPACING_BY_AGENT:
        return []
    declared = set(cfg.get("agents") or [])

    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    req_dir = cfg.get("req_dir", "docs/req")
    roadmap_names = _undeclared_namespaces_on_disk(cfg, roadmap_dir, declared)
    req_names = _undeclared_namespaces_on_disk(cfg, req_dir, declared)

    in_roadmap = set(roadmap_names)
    in_req = set(req_names)

    seen = set()
    names = []
    for n in roadmap_names + req_names:
        if n in seen:
            continue
        seen.add(n)
        names.append(n)
    names.sort()

    msgs = []
    for name in names:
        trees = []
        if name in in_roadmap:
            trees.append("roadmap_dir")
        if name in in_req:
            trees.append("req_dir")
        msgs.append(
            'agent namespace "%s" exists in %s but is not declared in agents: — add it to trackfw.yaml'
            % (name, ", ".join(trees))
        )
    return msgs


def hidden_namespace_warnings(cfg: dict) -> list:
    """
    Regra "agent_namespace_hidden" — contraponto de baixo ruído de validate_agent_namespace_undeclared
    para nomes iniciados por "." (ML-4A, achado 1 do parecer hades-tf 2026-08-30). Um diretório
    oculto/ambíguo em disco (roadmap_dir e/ou req_dir), ausente de agents:, NÃO é filtrado da união
    (resolve_agent_namespaces mantém) e NÃO dispara a violação plena — mas também não é silêncio
    total: esta função emite um aviso nomeando explicitamente o diretório.
    """
    if cfg.get("roadmap_namespacing") != _config.NAMESPACING_BY_AGENT:
        return []
    declared = set(cfg.get("agents") or [])

    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    req_dir = cfg.get("req_dir", "docs/req")
    roadmap_names = _dot_prefixed_undeclared_on_disk(cfg, roadmap_dir, declared)
    req_names = _dot_prefixed_undeclared_on_disk(cfg, req_dir, declared)

    in_roadmap = set(roadmap_names)
    in_req = set(req_names)

    seen = set()
    names = []
    for n in roadmap_names + req_names:
        if n in seen:
            continue
        seen.add(n)
        names.append(n)
    names.sort()

    msgs = []
    for name in names:
        trees = []
        if name in in_roadmap:
            trees.append("roadmap_dir")
        if name in in_req:
            trees.append("req_dir")
        msgs.append(
            'dot-prefixed directory "%s" found in %s is treated as an agent namespace (fully '
            "enumerated, not declared in agents:) — declare it in trackfw.yaml if intentional, or "
            "remove it if it is leftover tooling"
            % (name, ", ".join(trees))
        )
    return msgs


def resolve_req_files(cfg: dict) -> list:
    """
    Retorna lista de paths completos de .md em req_dir,
    consciente de roadmap_namespacing: by_agent percorre req_dir/<agente>/<estado>/.
    """
    req_dir = cfg.get("req_dir", "docs/req")
    namespacing = cfg.get("roadmap_namespacing", "")
    if namespacing == "by_agent":
        states = ["backlog", "analyzing", "wip", "blocked", "done", "abandoned"]
        agents = resolve_agent_namespaces(cfg, req_dir)
        files = []
        for agent in agents:
            # ML-4A (achado 2, hades-tf 2026-08-30): agent vem do disco (nome de diretório sem
            # validação de formato), não de config — usa _list_md_files em vez de glob.glob para não
            # interpretar o nome como padrão (espelha internal/validator/validator.go's ListMDFiles;
            # glob.glob degradava graciosamente para "[" desbalanceado neste runtime, mas ainda
            # cross-casava "*" com todos os agentes, igual ao Go).
            for state in states:
                files.extend(_list_md_files(os.path.join(req_dir, agent, state)))
        return files
    # flat (comportamento anterior)
    return _glob.glob(os.path.join(req_dir, "*.md"))


def _list_md_files(directory: str) -> list:
    """
    Lista os arquivos .md diretamente dentro de directory (sem subdiretórios, sem glob) — substitui
    glob.glob(os.path.join(directory, "*.md")) em todo ponto onde um COMPONENTE do caminho vem de um
    nome de diretório lido do disco (ex.: um namespace de agente resolvido por
    resolve_agent_namespaces), em vez de vir de config ou de uma constante do código. Ver o
    comentário completo em ListMDFiles (internal/validator/validator.go) para a justificativa.
    """
    try:
        entries = os.listdir(directory)
    except OSError:
        return []
    files = [
        os.path.join(directory, name)
        for name in entries
        if name.endswith(".md") and not os.path.isdir(os.path.join(directory, name))
    ]
    files.sort()
    return files


def _resolve_state_dirs(cfg: dict, state: str) -> list:
    """
    Fonte única de resolução de caminho por estado (ex: 'wip', 'done') conforme o modo de
    namespacing. resolve_wip_dirs e resolve_done_dirs são wrappers finos sobre esta função.
    Duplicar a lógica aqui foi a causa raiz de defeitos anteriores (roadmap_dir divergente entre
    runtimes).
    flat     → [cfg["roadmap_dir"] + "/" + state]
    by_agent → [cfg["roadmap_dir"] + "/" + agent + "/" + state for agent in agents]
    """
    if cfg.get("roadmap_namespacing") == _config.NAMESPACING_BY_AGENT:
        roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
        agents = resolve_agent_namespaces(cfg, roadmap_dir)
        return [roadmap_dir + "/" + agent + "/" + state for agent in agents]

    return [cfg.get("roadmap_dir", "docs/roadmaps") + "/" + state]


def resolve_wip_dirs(cfg: dict) -> list:
    """
    Retorna lista de diretórios wip/ conforme o modo de namespacing.
    flat     → [cfg["roadmap_dir"] + "/wip"]
    by_agent → [cfg["roadmap_dir"] + "/" + agent + "/wip" for agent in agents]
    """
    return _resolve_state_dirs(cfg, "wip")


def resolve_done_dirs(cfg: dict) -> list:
    """
    Retorna lista de diretórios done/ conforme o modo de namespacing.
    flat     → [cfg["roadmap_dir"] + "/done"]
    by_agent → [cfg["roadmap_dir"] + "/" + agent + "/done" for agent in agents]
    """
    return _resolve_state_dirs(cfg, "done")


def normalize_yaml_flat_value(value: str) -> str:
    """Normaliza valor YAML flat removendo apenas delimitador externo pareado (aspas)."""
    if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
        return value[1:-1]
    return value


def parse_frontmatter(content: str) -> dict:
    """
    Extrai campos entre --- e --- do início do arquivo.
    Retorna dict com chaves em snake_case.
    """
    result = {}
    if not content.startswith("---"):
        return result
    lines = content.split("\n")
    in_block = False
    for i, line in enumerate(lines):
        stripped = line.strip()
        if i == 0 and stripped == "---":
            in_block = True
            continue
        if in_block:
            if stripped == "---":
                break
            colon_idx = stripped.find(":")
            if colon_idx >= 0:
                key = stripped[:colon_idx].strip().replace("-", "_")
                val = normalize_yaml_flat_value(stripped[colon_idx + 1:].strip())
                result[key] = val
    return result


def _parse_blocked_adrs(file_path: str) -> list:
    """
    Extrai basenames de ADRs da seção '## Blocked by ADRs' de um arquivo REQ.
    Espelha parseBlockedADRs do JS.
    """
    try:
        with open(file_path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return []

    lines = content.split("\n")
    adrs = []
    in_section = False
    for line in lines:
        if line == "## Blocked by ADRs":
            in_section = True
            continue
        if in_section:
            if line.startswith("## "):
                break
            if line.startswith("- "):
                item = line[2:].strip()
                parts = item.split()
                if parts and parts[0].endswith(".md"):
                    adrs.append(parts[0])
    return adrs


def _adr_is_draft(basename: str, cfg: dict) -> bool:
    return _adr_draft_status_for_rule(basename, cfg, None)[0]


def _extract_adr_status(content: str) -> str:
    """
    Extrai o status declarado de um ADR. Tenta primeiro o frontmatter (`status:`),
    a fonte estruturada e canônica emitida por todos os geradores (`adr new` e
    `NewADRDraft` escrevem `status:` e a linha de cabeçalho em sincronia). Cai para a
    linha de cabeçalho ('> Date: ... | Status: X') quando não há frontmatter, para
    cobrir ADRs legados sem bloco YAML (ex.: ADR-001). Retorna '' se nenhum for encontrado.
    """
    fm = parse_frontmatter(content)
    fm_status = fm.get("status", "").strip()
    if fm_status:
        return fm_status
    marker = "| Status: "
    for line in content.split("\n"):
        trimmed = line.strip()
        idx = trimmed.find(marker)
        if idx >= 0:
            rest = trimmed[idx + len(marker):]
            pipe_idx = rest.find(" |")
            if pipe_idx >= 0:
                rest = rest[:pipe_idx]
            return rest.strip()
    return ""


def _adr_not_accepted(content: str) -> bool:
    """
    Helper canônico único: verdadeiro se o status do ADR for 'Draft' ou 'Proposed'
    (comparação case-insensitive, espelhando strings.EqualFold do CLI Go). 'Aceito' é
    definido por exclusão — qualquer outro status (Accepted, Superseded, Deprecated,
    Rejected, ...) conta como aceito e não deve ser enumerado aqui.
    """
    return _extract_adr_status(content).strip().lower() in ("draft", "proposed")


def _adr_draft_status_for_rule(basename: str, cfg: dict, messages: list | None):
    """
    Verifica se <basename> está em status não aceito (Draft ou Proposed, via
    _adr_not_accepted) em algum dos adrDirs configurados.
    Busca recursivamente nas subpastas via _find_adr_file.
    """
    adr_dirs = [expand_path(d) for d in cfg.get("adr_dirs", ["docs/adr"])]
    p = _find_adr_file(basename, adr_dirs)
    if not p:
        return False, True
    try:
        with open(p, "r", encoding="utf-8") as f:
            return _adr_not_accepted(f.read()), True
    except OSError as e:
        if messages is not None:
            messages.append(_inspection_item("blocked_by_draft_adr", p, e))
        return False, False


def _wip_config_from(cfg: dict) -> dict:
    """
    Deriva {"limit": int, "by_squad": bool} a partir do dict de config já normalizado por
    _config.load() — nenhuma releitura de trackfw.yaml acontece aqui.
    """
    limit = cfg.get("wip_limit", 1)
    if not isinstance(limit, int) or limit <= 0:
        limit = 1
    return {"limit": limit, "by_squad": bool(cfg.get("wip_by_squad", False))}


def _parse_squad_from_frontmatter(file_path: str) -> str:
    """
    Extrai o valor do campo 'squad:' de um arquivo markdown.
    Retorna string vazia se ausente.
    """
    try:
        with open(file_path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return ""

    for line in content.split("\n"):
        trimmed = line.strip()
        if trimmed.startswith("squad:"):
            return normalize_yaml_flat_value(trimmed[len("squad:"):].strip())
    return ""


def _governance_mode_from(cfg: dict) -> dict:
    """
    Deriva {"mode": str, "lenient_until": datetime|None} a partir do dict de config já
    normalizado por _config.load() — nenhuma releitura de trackfw.yaml acontece aqui.
    cfg["governance_mode"] chega como o valor bruto do campo (string vazia se ausente);
    cfg["lenient_until"] chega como a data literal (ex.: "2026-08-02"), convertida aqui
    para datetime.
    """
    result = {"mode": "strict", "lenient_until": None}
    mode = cfg.get("governance_mode")
    if mode:
        result["mode"] = mode
    lenient_until = cfg.get("lenient_until")
    if lenient_until:
        try:
            result["lenient_until"] = datetime.fromisoformat(lenient_until)
        except ValueError:
            pass
    return result


_BASELINE_FILE = ".trackfw-baseline.json"


def _extract_messages(items: list) -> list:
    """Extrai campo 'message' de uma lista de dicts de violation/warning."""
    result = []
    for item in items:
        if isinstance(item, dict):
            result.append(item.get("message", str(item)))
        else:
            result.append(str(item))
    return result


def load_baseline() -> dict | None:
    """Lê .trackfw-baseline.json do CWD. Retorna None se não existir."""
    try:
        with open(_BASELINE_FILE, "r", encoding="utf-8") as f:
            return json.load(f)
    except FileNotFoundError:
        return None
    except (json.JSONDecodeError, OSError) as e:
        raise RuntimeError(f"Erro ao ler baseline: {e}") from e


def save_baseline(violations: list, warnings: list) -> None:
    """Salva violations e warnings como baseline em .trackfw-baseline.json.
    Aceita lista de dicts ou strings — normaliza para strings.
    """
    bf = {
        "created": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "violations": _extract_messages(violations),
        "warnings": _extract_messages(warnings),
    }
    with open(_BASELINE_FILE, "w", encoding="utf-8", newline="\n") as f:
        json.dump(bf, f, indent=2, ensure_ascii=False)


def _is_lenient(cwd: str = None) -> bool:
    """Retorna True se o projeto está em modo lenient e o prazo não expirou."""
    gm = _governance_mode_from(_config.load(cwd))
    if gm["mode"] != "lenient":
        return False
    if gm["lenient_until"] is None:
        return True
    # Comparação sem timezone
    now = datetime.now()
    lu = gm["lenient_until"]
    # Remove tzinfo se presente para comparação homogênea
    if lu.tzinfo is not None:
        now = datetime.now(timezone.utc)
    return now < lu


# ---------------------------------------------------------------------------
# Funções de validação públicas (assinatura: cfg como parâmetro)
# ---------------------------------------------------------------------------

def validate_wip_has_req(cfg: dict) -> list:
    """
    Roadmaps em wip/ sem marcador req no conteúdo → violation.
    Suporta modo by_agent via resolve_wip_dirs.
    Usa cfg["link_fields"]["req"] para os marcadores configuráveis.
    """
    wip_dirs = resolve_wip_dirs(cfg)
    req_markers = cfg.get("link_fields", {}).get("req", ["REQ:"])
    violations = []
    for wip_dir in wip_dirs:
        entries = _list_dir_for_rule("wip_has_req", wip_dir, violations)
        for name in entries:
            content = _read_file_for_rule("wip_has_req", os.path.join(wip_dir, name), violations)
            if content is None:
                continue
            if not _content_has_marker(content, req_markers):
                violations.append(
                    {"type": "violation", "message": f'roadmap "{name}" is in wip but has no linked REQ'}
                )
    return violations


def validate_reqs_have_adr(cfg: dict) -> list:
    """REQs em req_dir/ sem marcador adr no conteúdo → violation."""
    files = resolve_req_files(cfg)
    adr_markers = cfg.get("link_fields", {}).get("adr", ["ADR:"])
    violations = []
    for file_path in files:
        try:
            with open(file_path, "r", encoding="utf-8") as f:
                content = f.read()
            if not _content_has_marker(content, adr_markers):
                name = os.path.basename(file_path)
                violations.append(
                    {"type": "violation", "message": f'req "{name}" has no linked ADR'}
                )
        except OSError:
            pass
    return violations


def validate_blocked_has_req(cfg: dict) -> list:
    """Roadmaps em blocked/ sem marcador req → violation."""
    req_markers = cfg.get("link_fields", {}).get("req", ["REQ:"])
    violations = []
    for blocked_dir in _resolve_state_dirs(cfg, "blocked"):
        for name in _list_dir_for_rule("blocked_has_req", blocked_dir, violations):
            content = _read_file_for_rule("blocked_has_req", os.path.join(blocked_dir, name), violations)
            if content is None:
                continue
            if not _content_has_marker(content, req_markers):
                violations.append(
                    {"type": "violation", "message": f'roadmap "{name}" is in blocked but has no linked REQ'}
                )
    return violations


def validate_reqs_have_roadmap(cfg: dict) -> list:
    """REQs sem marcador roadmap → violation."""
    files = resolve_req_files(cfg)
    roadmap_markers = cfg.get("link_fields", {}).get("roadmap", ["Roadmap:"])
    violations = []
    for file_path in files:
        try:
            with open(file_path, "r", encoding="utf-8") as f:
                content = f.read()
            if not _content_has_marker(content, roadmap_markers):
                name = os.path.basename(file_path)
                violations.append(
                    {"type": "violation", "message": f'req "{name}" has no linked Roadmap'}
                )
        except OSError:
            pass
    return violations


def validate_adrs_are_referenced(cfg: dict, cwd: str = None) -> list:
    """ADRs em adr_dirs não referenciados em nenhuma REQ → violation (busca recursiva).
    Isenta arquivos localizados fora do diretório raiz (cwd).
    """
    abs_cwd = os.path.realpath(os.path.abspath(cwd or os.getcwd()))
    violations = []
    adrs = []
    adr_dirs = [expand_path(d) for d in cfg.get("adr_dirs", ["docs/adr"])]
    for adr_dir in adr_dirs:
        expanded_dir = expand_path(adr_dir)
        for file_path in _walk_dir_md_paths_for_rule("adr_orphan", expanded_dir, violations):
            real_path = os.path.realpath(file_path)
            # Isenta arquivos localizados fora do CWD (ex.: ADRs globais compartilhados ou symlinks externos)
            if not _is_subpath(real_path, abs_cwd):
                continue
            adrs.append(os.path.basename(file_path))

    req_files = resolve_req_files(cfg)
    combined = ""
    for file_path in req_files:
        content = _read_file_for_rule("adr_orphan", file_path, violations)
        if content is not None:
            combined += content

    for adr in adrs:
        if adr not in combined:
            violations.append(
                {"type": "violation", "message": f'adr "{adr}" is not referenced by any REQ'}
            )
    return violations


def validate_wip_has_acceptance_criteria(cfg: dict) -> list:
    """Roadmaps wip sem bloco de critérios de aceite → violation.
    Usa cfg["acceptance_markers"] para os marcadores configuráveis.
    """
    wip_dirs = resolve_wip_dirs(cfg)
    acceptance_markers = cfg.get("acceptance_markers", ["## Acceptance Criteria", "## Critérios de Aceite"])
    violations = []
    for wip_dir in wip_dirs:
        entries = _list_dir_for_rule("wip_acceptance", wip_dir, violations)
        for name in entries:
            content = _read_file_for_rule("wip_acceptance", os.path.join(wip_dir, name), violations)
            if content is None:
                continue
            if not _content_has_marker(content, acceptance_markers):
                violations.append(
                    {"type": "violation", "message": f'roadmap "{name}" is in wip but has no acceptance criteria block'}
                )
    return violations


def validate_wip_limit(cfg: dict) -> dict:
    """
    Verifica o WIP limit por agente, por squad ou global conforme trackfw.yaml.
    Retorna {"violations": [], "warnings": []}.
    """
    violations = []
    warnings = []

    if cfg.get("roadmap_namespacing") == _config.NAMESPACING_BY_AGENT:
        agents = resolve_agent_namespaces(cfg, cfg.get("roadmap_dir", "docs/roadmaps"))
        limit = cfg.get("wip_limit", 1)
        if limit <= 0:
            limit = 1
        for agent in agents:
            entries = list_dir(cfg["roadmap_dir"] + "/" + agent + "/wip")
            if len(entries) > limit:
                warnings.append({
                    "type": "warning",
                    "message": f'{len(entries)} roadmaps in wip/ for agent "{agent}" (limit: {limit}) — consider focusing'
                })
        return {"violations": violations, "warnings": warnings}

    # modo flat (global ou por squad)
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    wip_path = os.path.join(roadmap_dir, "wip")
    files = []
    try:
        files = [
            os.path.join(wip_path, f)
            for f in os.listdir(wip_path)
            if not os.path.isdir(os.path.join(wip_path, f))
        ]
    except OSError:
        return {"violations": violations, "warnings": warnings}

    wip_cfg = _wip_config_from(cfg)

    if not wip_cfg["by_squad"]:
        if len(files) > wip_cfg["limit"]:
            warnings.append({
                "type": "warning",
                "message": f'{len(files)} roadmaps in wip/ (limit: {wip_cfg["limit"]}) — consider focusing'
            })
        return {"violations": violations, "warnings": warnings}

    # por squad
    by_squad = {}
    for f in files:
        squad = _parse_squad_from_frontmatter(f)
        if not squad:
            squad = "(no squad)"
        by_squad.setdefault(squad, []).append(os.path.basename(f))

    for squad, items in by_squad.items():
        if len(items) > wip_cfg["limit"]:
            warnings.append({
                "type": "warning",
                "message": f'squad "{squad}" has {len(items)} roadmaps in wip/ (limit: {wip_cfg["limit"]})'
            })

    return {"violations": violations, "warnings": warnings}


def _roadmap_log_identity(cfg: dict, file_path: str) -> str:
    basename = os.path.basename(file_path)
    if cfg.get("roadmap_namespacing") != _config.NAMESPACING_BY_AGENT:
        return basename

    state_dir = os.path.dirname(file_path)
    agent_dir = os.path.dirname(state_dir)
    agent = os.path.basename(agent_dir)
    if agent:
        return f"{agent}/{basename}"
    return basename


def _parse_transition_log_line(line: str):
    fields = line.split()
    if len(fields) < 5:
        return None

    try:
        timestamp = datetime.strptime(f"{fields[0]} {fields[1]}", "%Y-%m-%d %H:%M").timestamp()
    except ValueError:
        return None

    arrow_idx = -1
    for idx in range(3, len(fields)):
        if fields[idx] in ("→", "->"):
            arrow_idx = idx
            break

    if arrow_idx < 0 or arrow_idx + 1 >= len(fields):
        return None

    return {
        "timestamp": timestamp,
        "name": fields[2],
        "to_state": fields[arrow_idx + 1],
    }


def _latest_wip_transition_time(cfg: dict, file_path: str):
    log_path = os.path.join(cfg.get("roadmap_dir", "docs/roadmaps"), ".trackfw-log")
    expected_name = _roadmap_log_identity(cfg, file_path)
    latest = None
    diagnostics = []

    try:
        with open(log_path, "r", encoding="utf-8") as f:
            for line in f:
                stripped = line.strip()
                if not stripped:
                    continue
                parsed = _parse_transition_log_line(stripped)
                if not parsed:
                    diagnostics.append({
                        "type": "warning",
                        "message": f'stale_wip: invalid support line in "{log_path}": "{stripped}"'
                    })
                    continue
                if parsed["name"] != expected_name or parsed["to_state"] != "wip":
                    continue
                if latest is None or parsed["timestamp"] > latest:
                    latest = parsed["timestamp"]
    except FileNotFoundError:
        return None, []
    except OSError as e:
        return None, [_inspection_item("stale_wip", log_path, e)]

    return latest, diagnostics


def validate_stale_wip(cfg: dict, days: int = None, now: float = None) -> list:
    """
    Arquivos em wip/ com idade desde a última transição para wip >= days dias → warning.
    Quando o log não existe ou não possui entrada parseável, usa mtime como fallback.
    Suporta modo by_agent via resolve_wip_dirs.
    """
    wip_dirs = resolve_wip_dirs(cfg)
    warnings = []
    threshold_days = days
    if threshold_days is None:
        try:
            threshold_days = int(cfg.get("stale_wip_days", STALE_WIP_DAYS))
        except (TypeError, ValueError):
            threshold_days = STALE_WIP_DAYS
    if threshold_days <= 0:
        threshold_days = STALE_WIP_DAYS

    now_ts = now if now is not None else datetime.now().timestamp()

    for wip_dir in wip_dirs:
        md_files = [
            os.path.join(wip_dir, f)
            for f in _list_dir_for_rule("stale_wip", wip_dir, warnings)
            if f.endswith(".md")
        ]

        for file_path in md_files:
            try:
                stat = os.stat(file_path)
                log_time, diagnostics = _latest_wip_transition_time(cfg, file_path)
                warnings.extend(diagnostics)
                ref_time = log_time if log_time is not None else stat.st_mtime
                age_seconds = now_ts - ref_time
                age_days = int(age_seconds / (60 * 60 * 24))
                if age_days >= threshold_days:
                    last_modified = datetime.fromtimestamp(ref_time).strftime("%Y-%m-%d")
                    basename = os.path.basename(file_path)
                    warnings.append({
                        "type": "warning",
                        "message": f"roadmap/wip/{basename} has been in WIP for {age_days} days (last modified {last_modified})"
                    })
            except OSError as e:
                warnings.append(_inspection_item("stale_wip", file_path, e))

    return warnings


def validate_reqs_not_blocked_by_draft_adrs(cfg: dict) -> list:
    """REQs Open com ADRs não aceitos (Draft ou Proposed, via _adr_not_accepted) na
    seção '## Blocked by ADRs' → violation. A regra deixou de ser cega a Proposed
    (ADR-2026-08-01), mas o **nome da regra permanece** blocked_by_draft_adr — é
    chave pública de config.
    """
    files = resolve_req_files(cfg)
    violations = []
    for file_path in files:
        name = os.path.basename(file_path)
        content = _read_file_for_rule("blocked_by_draft_adr", file_path, violations)
        if content is None:
            continue

        if "Status: Open" not in content:
            continue

        blocked_adrs = _parse_blocked_adrs(file_path)
        for adr_basename in blocked_adrs:
            if _adr_draft_status_for_rule(adr_basename, cfg, violations)[0]:
                # ML-1D (2026-08-01): reconciliação de paridade — "Draft" saiu porque a
                # regra cobre Proposed também; texto agora byte-idêntico ao Go/Node
                # ("is blocked by not-accepted ADR:").
                violations.append({
                    "type": "violation",
                    "message": f"REQ {name} is blocked by not-accepted ADR: {adr_basename}"
                })
    return violations


def validate_frontmatter_presence(cfg: dict) -> list:
    """Verifica presença de frontmatter em ADRs e REQs (busca recursiva em adr_dirs)."""
    violations = []
    adr_dirs = [expand_path(d) for d in cfg.get("adr_dirs", ["docs/adr"])]

    for adr_dir in adr_dirs:
        files = [f for f in _walk_dir_md(adr_dir) if f.endswith(".md")]
        for f in files:
            full_path = _find_adr_file(f, adr_dirs)
            if not full_path:
                continue
            try:
                with open(full_path, "r", encoding="utf-8") as fh:
                    content = fh.read()
                if not content.startswith("---"):
                    violations.append({
                        "type": "violation",
                        "message": f'adr "{f}" has no frontmatter block'
                    })
            except OSError:
                pass

    req_files = [p for p in resolve_req_files(cfg) if p.endswith(".md")]
    for file_path in req_files:
        try:
            with open(file_path, "r", encoding="utf-8") as fh:
                content = fh.read()
            if not content.startswith("---"):
                f = os.path.basename(file_path)
                violations.append({
                    "type": "violation",
                    "message": f'req "{f}" has no frontmatter block'
                })
        except OSError:
            pass

    return violations


def validate_ref_targets_exist(cfg: dict) -> list:
    """Verifica se arquivos referenciados em REQ:, ADR:, Roadmap: existem. Retorna warnings."""
    warnings = []

    # Roadmaps em wip e blocked: verificar REQ:
    dirs = resolve_wip_dirs(cfg) + _resolve_state_dirs(cfg, "blocked")
    for wip_dir in dirs:
        for name in _list_dir_for_rule("ref_targets_exist", wip_dir, warnings):
            content = _read_file_for_rule("ref_targets_exist", os.path.join(wip_dir, name), warnings)
            if content is None:
                continue
            ref = _extract_ref_path(content, "REQ")
            if ref and not _reference_exists(ref):
                warnings.append({
                    "type": "warning",
                    "message": f'roadmap "{name}" links to REQ "{ref}" which does not exist'
                })

    # REQs: verificar ADR: e Roadmap:
    for file_path in resolve_req_files(cfg):
        content = _read_file_for_rule("ref_targets_exist", file_path, warnings)
        if content is None:
            continue
        name = os.path.basename(file_path)
        adr_ref = _extract_ref_path(content, "ADR")
        if adr_ref and not _reference_exists(adr_ref):
            warnings.append({
                "type": "warning",
                "message": f'req "{name}" links to ADR "{adr_ref}" which does not exist'
            })
        roadmap_ref = _extract_ref_path(content, "Roadmap")
        if roadmap_ref and not _reference_exists(roadmap_ref):
            warnings.append({
                "type": "warning",
                "message": f'req "{name}" links to Roadmap "{roadmap_ref}" which does not exist'
            })

    return warnings


def _normalize_ref_separator(ref: str) -> str:
    """
    Normaliza um valor já extraído de um campo (roadmap:, req:, adr:) para o separador
    portável (/) antes de resolvê-lo no filesystem local. Um valor gravado no Windows antes do
    fix de escrita (ou por qualquer runtime que ainda não normalize) chega aqui com "\\"
    literal, que em POSIX não é separador — é caractere de nome de arquivo — e faz
    os.path.exists falhar numa referência que na verdade existe
    (docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md). NÃO aplicar ao
    buffer inteiro do arquivo — só ao valor já extraído do campo.
    """
    return ref.replace("\\", "/")


def _reference_exists(ref: str) -> bool:
    return os.path.exists(expand_path(_normalize_ref_separator(ref)))


def validate_req_roadmap_lifecycle(cfg: dict) -> list:
    """Sinaliza REQ Open cujo roadmap canônico referenciado já está em done/."""
    warnings = []
    for file_path in resolve_req_files(cfg):
        try:
            with open(file_path, "r", encoding="utf-8") as f:
                content = f.read()
            if not _req_status_is_open(content):
                continue
            ref = _extract_ref_path(content, "Roadmap")
            if not ref:
                continue
            expanded_ref = expand_path(_normalize_ref_separator(ref))
            if not os.path.isfile(expanded_ref):
                continue
            if os.path.basename(os.path.dirname(expanded_ref)) == "done":
                warnings.append({
                    "type": "warning",
                    "message": f'req "{os.path.basename(file_path)}" is Open but linked Roadmap "{ref}" is in done/'
                })
        except OSError:
            pass
    return warnings


def _req_status_is_open(content: str) -> bool:
    for line in content.split("\n"):
        trimmed = line.strip()
        if ":" in trimmed:
            key, val = trimmed.split(":", 1)
            if key.strip().lower() == "status":
                return normalize_yaml_flat_value(val.strip()).lower() == "open"
        marker = "| Status: "
        idx = trimmed.find(marker)
        if idx >= 0:
            rest = trimmed[idx + len(marker):]
            pipe_idx = rest.find(" |")
            if pipe_idx >= 0:
                rest = rest[:pipe_idx]
            return rest.strip().lower() == "open"
    return False


def _req_status_is_done(content: str) -> bool:
    for line in content.split("\n"):
        trimmed = line.strip()
        if ":" in trimmed:
            key, val = trimmed.split(":", 1)
            if key.strip().lower() == "status":
                return normalize_yaml_flat_value(val.strip()).lower() == "done"
        marker = "| Status: "
        idx = trimmed.find(marker)
        if idx >= 0:
            rest = trimmed[idx + len(marker):]
            pipe_idx = rest.find(" |")
            if pipe_idx >= 0:
                rest = rest[:pipe_idx]
            return rest.strip().lower() == "done"
    return False


def validate_adr_accepted_when_req_done(cfg: dict) -> list:
    """
    REQ com Status: Done referenciando (campo 'ADR:') um ADR ainda não aceito
    (Draft ou Proposed, via _adr_not_accepted) -> violation. 'Aceito' é definido por
    exclusão: Superseded/Deprecated/Rejected (e qualquer status != Draft/Proposed)
    não disparam a regra — REQ Done apoiada em ADR posteriormente substituído é
    histórico legítimo. REQ que não está Done nunca dispara esta regra.
    """
    violations = []
    adr_dirs = [expand_path(d) for d in cfg.get("adr_dirs", ["docs/adr"])]
    for file_path in resolve_req_files(cfg):
        req_name = os.path.basename(file_path)
        content = _read_file_for_rule("adr_accepted_when_req_done", file_path, violations)
        if content is None:
            continue
        if not _req_status_is_done(content):
            continue
        adr_ref = _extract_ref_path(content, "ADR")
        if not adr_ref:
            continue
        adr_basename = os.path.basename(adr_ref)
        adr_path = _find_adr_file(adr_basename, adr_dirs)
        if not adr_path:
            continue
        adr_content = _read_file_for_rule("adr_accepted_when_req_done", adr_path, violations)
        if adr_content is None:
            continue
        if _adr_not_accepted(adr_content):
            status = _extract_adr_status(adr_content) or "unknown"
            # ML-1D (2026-08-01): reconciliação de paridade — texto agora byte-idêntico
            # ao Go/Node: aspas em torno dos dois basenames + sufixo "(status: X)".
            violations.append({
                "type": "violation",
                "message": f'REQ "{req_name}" is Done but linked ADR "{adr_basename}" is not accepted (status: {status})'
            })
    return violations


_FOLDER_TO_STATUS = {
    "wip":       ["WIP", "wip", "In Progress"],
    "backlog":   ["Backlog", "backlog"],
    "analyzing": ["Analyzing", "analyzing"],
    "blocked":   ["Blocked", "blocked"],
    "done":      ["Done", "done"],
    "abandoned": ["Abandoned", "abandoned"],
}


def validate_folder_status_coherence(cfg: dict) -> list:
    """
    Verifica que o campo status: no frontmatter bate com a pasta onde o arquivo está.
    Divergência → warning.
    """
    warnings = []
    states = ["wip", "backlog", "analyzing", "blocked", "done", "abandoned"]
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")

    dirs = []
    if cfg.get("roadmap_namespacing") == _config.NAMESPACING_BY_AGENT:
        agents = resolve_agent_namespaces(cfg, roadmap_dir)
        for agent in agents:
            for state in states:
                dirs.append((os.path.join(roadmap_dir, agent, state), state))
    else:
        for state in states:
            dirs.append((os.path.join(roadmap_dir, state), state))

    for dir_path, state in dirs:
        # P2: distinguir "diretório ausente" (esperado) de outros erros (reportar).
        entries, read_error = _try_list_dir(dir_path)
        if read_error is not None:
            warnings.append({
                "type": "warning",
                "message": f'folder_status: could not read directory "{dir_path}": {read_error}'
            })
            continue
        for name in entries:
            if not name.endswith(".md"):
                continue
            try:
                with open(os.path.join(dir_path, name), "r", encoding="utf-8") as f:
                    content = f.read()
                fm = parse_frontmatter(content)
                declared = fm.get("status", "")
                if not declared:
                    continue
                expected = _FOLDER_TO_STATUS.get(state, [])
                if not any(e.lower() == declared.lower() for e in expected):
                    warnings.append({
                        "type": "warning",
                        "message": f'roadmap "{name}": folder is "{state}" but status declares "{declared}"'
                    })
            except OSError:
                pass

    return warnings


def validate_filename_uniqueness(cfg: dict) -> list:
    """Detecta o mesmo filename de roadmap em dois ou mais estados. Duplicata → violation."""
    states = ["wip", "backlog", "analyzing", "blocked", "done", "abandoned"]
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    seen = {}  # filename → [states]

    list_errors = []
    if cfg.get("roadmap_namespacing") == _config.NAMESPACING_BY_AGENT:
        agents = resolve_agent_namespaces(cfg, roadmap_dir)
        for agent in agents:
            for state in states:
                dir_path = os.path.join(roadmap_dir, agent, state)
                entries, read_error = _try_list_dir(dir_path)
                if read_error is not None:
                    list_errors.append({
                        "type": "violation",
                        "message": f'filename_uniqueness: could not read directory "{dir_path}": {read_error}'
                    })
                    continue
                for name in entries:
                    key = agent + "/" + name
                    seen.setdefault(key, []).append(state)
    else:
        for state in states:
            dir_path = os.path.join(roadmap_dir, state)
            entries, read_error = _try_list_dir(dir_path)
            if read_error is not None:
                list_errors.append({
                    "type": "violation",
                    "message": f'filename_uniqueness: could not read directory "{dir_path}": {read_error}'
                })
                continue
            for name in entries:
                seen.setdefault(name, []).append(state)

    violations = list(list_errors)
    # P3: ordenar os nomes e os estados para saída determinística.
    for name in sorted(seen.keys()):
        state_list = seen[name]
        if len(state_list) > 1:
            sorted_states = sorted(state_list)
            violations.append({
                "type": "violation",
                "message": f'roadmap "{name}" appears in multiple states: {sorted_states}'
            })
    return violations


def normalize_branch_slug(value: str) -> str:
    """Normaliza um slug de branch para comparação (lowercase, runs de não-alfanumérico → '-',
    sem '-' nas pontas). Espelha internal/validator/validator.go normalizeBranchSlug /
    NormalizeBranchSlug. Reutilizada por validate_branch_has_wip_roadmap e pelo comando
    `trackfw branch new` — nunca duplicar esta lógica."""
    return re.sub(r"[^a-z0-9]+", "-", value.lower()).strip("-")


def branch_slug_matches_roadmap(branch_slug: str, wip_dirs: list, done_dirs: list):
    """Verifica se branch_slug (já normalizado via normalize_branch_slug) casa com o nome de
    algum roadmap .md encontrado em wip_dirs ou done_dirs. Espelha
    internal/validator/validator.go BranchSlugMatchesRoadmap. Reutilizada por
    validate_branch_has_wip_roadmap e pelo comando `trackfw branch new` — nunca duplicar esta
    lógica.

    Retorna (matched: bool, candidates: list) — candidates lista todos os roadmaps .md
    encontrados em wip_dirs+done_dirs (para diagnóstico/mensagem de orientação quando matched é
    False).
    """
    matched = False
    candidates = []
    for search_dir in wip_dirs + done_dirs:
        if os.path.isdir(search_dir):
            for f in os.listdir(search_dir):
                if f.endswith('.md'):
                    candidates.append(f)
                    if branch_slug in normalize_branch_slug(f):
                        matched = True
    return matched, candidates


def branch_governance_orientation(branch: str) -> str:
    """Mensagem de orientação impressa quando uma branch feat/fix/refactor não tem nenhum
    roadmap em wip/ nem em done/ (candidates vazio). Espelha
    internal/validator/validator.go BranchGovernanceOrientation — byte-idêntica. Compartilhada
    por validate_branch_has_wip_roadmap e `trackfw branch new` — nunca duplicar esta string."""
    return (
        f'branch "{branch}" is a feat/fix/refactor branch but no roadmap is in wip/ nor done/ — '
        f'create governance artifacts first:\n'
        f'  trackfw req new "title"\n'
        f'  trackfw roadmap new "title"\n'
        f'  trackfw roadmap move <name> wip'
    )


def branch_no_matching_roadmap_message(branch: str, candidates: list) -> str:
    """Mensagem de orientação impressa quando existem roadmaps em wip/ ou done/ mas nenhum casa
    com o slug da branch. Espelha internal/validator/validator.go
    BranchNoMatchingRoadmapMessage — byte-idêntica. Compartilhada por
    validate_branch_has_wip_roadmap e `trackfw branch new` — nunca duplicar esta string. Não
    muta candidates."""
    # P3: sort for deterministic output regardless of filesystem ordering.
    sorted_candidates = sorted(candidates)
    display = sorted_candidates[:3]
    suffix = f", e mais {len(sorted_candidates) - 3}" if len(sorted_candidates) > 3 else ""
    return (
        f'branch "{branch}" has no matching roadmap in wip/ nor done/ '
        f'(found: {", ".join(display)}{suffix}) — include the branch slug in the roadmap filename '
        f'or set TRACKFW_BRANCH explicitly in CI'
    )


def validate_branch_has_wip_roadmap(cfg: dict) -> list:
    """Verifica que branch feat/fix/refactor tem ao menos um roadmap em wip/ antes de trabalhar."""
    # Derive the working directory from roadmap_dir so tests using tmp dirs get
    # an isolated git context (a tmp dir outside the repo returns non-zero).
    roadmap_dir = cfg.get("roadmap_dir", "docs/roadmaps")
    git_cwd = os.path.dirname(os.path.abspath(roadmap_dir)) if roadmap_dir else None
    branch = os.environ.get("TRACKFW_BRANCH") or ""
    if not branch and git_cwd and _is_git_worktree(git_cwd):
        try:
            result = _git_run(git_cwd, ['symbolic-ref', '--short', 'HEAD'])
            branch = result.stdout.strip() if result.returncode == 0 else ""
        except Exception:
            branch = ""
        if not branch:
            branch = (
                os.environ.get("GITHUB_HEAD_REF")
                or os.environ.get("CI_COMMIT_REF_NAME")
                or os.environ.get("GITHUB_REF_NAME")
                or ""
            )

    if not branch:
        return []

    if not (branch.startswith('feat/') or branch.startswith('fix/') or branch.startswith('refactor/')):
        return []

    wip_dirs = resolve_wip_dirs(cfg)
    done_dirs = resolve_done_dirs(cfg)
    branch_slug = normalize_branch_slug(branch.split("/", 1)[1])
    matched, candidates = branch_slug_matches_roadmap(branch_slug, wip_dirs, done_dirs)
    if matched:
        return []

    if not candidates:
        return [branch_governance_orientation(branch)]
    return [branch_no_matching_roadmap_message(branch, candidates)]


def _is_git_worktree(cwd: str) -> bool:
    """Retorna True se cwd pertence a um worktree git."""
    try:
        result = _git_run(cwd, ['rev-parse', '--is-inside-work-tree'])
        return result.returncode == 0 and result.stdout.strip() == "true"
    except Exception:
        return False


def validate_note_orphan(cfg: dict, cwd: str = None) -> list:
    """
    Detecta notas em vault/notes/ não referenciadas pelo index.md.
    index.md não conta como nota órfã. Projeto sem vault/ retorna [].
    Aceita link markdown `[texto](arquivo.md)` ou wikilink `[[nome]]`.
    """
    base = cwd or os.getcwd()
    vault_dir = os.path.join(base, "vault", "notes")
    if not os.path.isdir(vault_dir):
        return []

    index_path = os.path.join(vault_dir, "index.md")
    index_content = ""
    if os.path.exists(index_path):
        with open(index_path, "r", encoding="utf-8") as f:
            index_content = f.read()

    msgs = []
    try:
        entries = os.listdir(vault_dir)
    except OSError:
        return []

    for filename in sorted(entries):
        if not filename.endswith(".md") or filename == "index.md":
            continue
        name_without_ext = re.sub(r"\.md$", "", filename)
        referenced = (
            f"({filename})" in index_content
            or f"[[{name_without_ext}]]" in index_content
            or f"[[{filename}]]" in index_content
        )
        if not referenced:
            msgs.append({
                "type": "warning",
                "message": f'note "{filename}" is not referenced in vault/notes/index.md',
                "rule": "note_orphan",
                "file": filename,
            })
    return msgs


# CREDENTIAL_GUARD_SCRIPT_MARKER é o nome do script que a regra
# credential_guard_hook_resolvable procura dentro dos comandos de hook de projeto.
_CREDENTIAL_GUARD_SCRIPT_MARKER = "trackfw-credential-guard.sh"

# _GIT_BRANCH_GUARD_SCRIPT_MARKER é o nome do script que a regra git_branch_guard_hook_resolvable
# procura dentro dos comandos de hook de projeto (ROADMAP-2026-08-15-trackfw-validate-deve-
# detectar-scripts-de-hook-ausentes-ou-desatualizados, ML-2B — port do Go
# internal/validator/validator_credential_guard.go, gitBranchGuardScriptMarker). Mesmo padrão de
# _CREDENTIAL_GUARD_SCRIPT_MARKER — só o nome do arquivo muda.
_GIT_BRANCH_GUARD_SCRIPT_MARKER = "trackfw-git-branch-guard.sh"

# _CREDENTIAL_GUARD_HOOK_FILES é a lista fechada dos arquivos de hook de PROJETO que o trackfw
# gera hoje e que podem conter uma entrada de credential-guard OU git-branch-guard
# (ROADMAP-2026-08-12-mitigacao-do-fail-open-do-credential-guard, ML-1A; generalizada para os 2
# guards em ROADMAP-2026-08-15-..., ML-2B). Hooks de escopo GLOBAL (~/.trackfw/..., trackfw update
# harness) ficam fora — caso distinto, fora do repositório do usuário, e a checagem de dedup
# global_credential_guard_installed_*() já os pula de propósito nas entradas de projeto.
# Cada tupla é (rel_path, cli, requires_command_type, requires_var_or_shell_prefix).
# requires_command_type (ROADMAP-2026-08-17 ML-4B, port de
# credentialGuardHookFile.requiresCommandType, internal/validator/validator_credential_guard.go)
# é True para todo CLI cujo escritor sempre emite um campo irmão "type":"command"
# (Claude/Codex/Gemini, GitHub Copilot CLI, Kiro) -- um comando casado SEM esse irmão é uma
# entrada estruturalmente malformada que o CLI nunca executa em silêncio (achado da barreira
# hades-tf ML-4A), não meramente "ausente". False só para Cursor, cujo schema ({"command": ...})
# nunca carrega um campo "type".
# requires_var_or_shell_prefix (ROADMAP-2026-08-21 ML-1B, port de
# credentialGuardHookFile.requiresVarOrShellPrefix): True para Claude/Codex/Gemini -- o
# ADR-2026-08-11 exige que estes CLIs usem $VAR/... ou "$(git ...)/..." porque seus hooks rodam
# a partir do cwd do agente, não necessariamente a raiz do projeto. Um caminho relativo puro
# ("scripts/...") falha em silêncio a partir de qualquer subdiretório (causa raiz da
# REQ-2026-08-17). False para Cursor/Copilot/Kiro, onde o caminho relativo puro É a forma
# correta -- acusá-los seria falso-positivo (o risco dominante desta REQ).
_CREDENTIAL_GUARD_HOOK_FILES = [
    (".claude/settings.json", "Claude Code", True, True),
    (".codex/hooks.json", "Codex CLI", True, True),
    (".gemini/settings.json", "Gemini CLI", True, True),
    (".cursor/hooks.json", "Cursor", False, False),
    (".github/hooks/trackfw-attention.json", "GitHub Copilot CLI", True, False),
    (".kiro/hooks/trackfw-attention.json", "Kiro", True, False),
]


def _resolve_credential_guard_hook_path(raw: str, root: str):
    """Resolve o valor bruto de um comando de hook (string extraída do JSON) para um caminho de
    arquivo absoluto, usando exatamente as 3 formas de prefixo que o trackfw emite hoje
    (docs/cli-parity.md, "Mecanismo de resolução de caminho dos hooks de projeto, por CLI"):

    1. "$CLAUDE_PROJECT_DIR/…" / "$GEMINI_PROJECT_DIR/…" — placeholder de env var expandido em
       runtime pelo próprio CLI, substituído aqui pela raiz do projeto.
    2. '"$(git rev-parse --show-toplevel)/…"' — substituição de shell entre aspas literais
       (Codex). As aspas fazem parte do valor emitido e são removidas antes de resolver contra a
       raiz do projeto.
    3. Caminho relativo puro, sem prefixo nenhum (Cursor/Copilot/Kiro) — resolvido diretamente
       contra a raiz do projeto.

    Retorna None quando o valor não bate em nenhuma das 3 formas — o chamador NÃO deve tratar
    isso como violação.
    """
    claude_prefix = "$CLAUDE_PROJECT_DIR/"
    gemini_prefix = "$GEMINI_PROJECT_DIR/"
    codex_prefix = '"$(git rev-parse --show-toplevel)/'

    if raw.startswith(claude_prefix):
        return os.path.join(root, raw[len(claude_prefix):])
    if raw.startswith(gemini_prefix):
        return os.path.join(root, raw[len(gemini_prefix):])
    if raw.startswith(codex_prefix) and raw.endswith('"'):
        inner = raw[len(codex_prefix):-1]
        return os.path.join(root, inner)
    if not raw.startswith("$") and not raw.startswith('"') and not os.path.isabs(raw) and not raw.startswith("~/"):
        # Caminho relativo puro — Cursor (beforeShellExecution/preToolUse), GitHub Copilot CLI
        # (campo "bash"), Kiro (action.command).
        # ~/… é excluído: é classe 1 (tilde expande para $HOME — ancorado) mas o validator não
        # expande o til; retornar None silencia sem acusar.
        return os.path.join(root, raw)
    return None


# _HOOK_ANCHORAGE_CLASS_* -- classifica a semântica de ancoragem de um valor de comando de hook.
# Avaliada apenas para CLIs com requires_var_or_shell_prefix=True. Decisão: ADR-2026-08-22.
_HOOK_ANCHORAGE_CLASS_ANCHORED = 1
_HOOK_ANCHORAGE_CLASS_CWD_DEPENDENT = 2
_HOOK_ANCHORAGE_CLASS_UNDECIDABLE = 3


def _strip_outer_quotes_for_classify(raw: str) -> str:
    """Remove aspas duplas envolventes de raw, se presentes. Necessário porque "$PWD/scripts/…"
    entre aspas (achado D.3) deve receber o mesmo veredito que $PWD/… sem aspas: as aspas são
    sintaxe, não semântica de ancoragem."""
    if len(raw) >= 2 and raw[0] == '"' and raw[-1] == '"':
        return raw[1:-1]
    return raw


def _hook_value_was_quoted(raw: str) -> bool:
    """Reporta se raw tinha aspas duplas externas que _strip_outer_quotes_for_classify removeria.
    Usado para distinguir ~/… (tilde expande para $HOME — classe 1) de \"~/…\" (tilde NÃO expande
    dentro de aspas duplas — classe 2)."""
    return len(raw) >= 2 and raw[0] == '"' and raw[-1] == '"'


def _classify_hook_anchorage(raw_stripped: str, was_quoted: bool) -> int:
    """Retorna a classe de ancoragem de raw_stripped (com aspas externas já removidas).
    was_quoted indica se o valor original tinha aspas externas (obtido com _hook_value_was_quoted
    antes de chamar _strip_outer_quotes_for_classify). Ver _HOOK_ANCHORAGE_CLASS_* e ADR-2026-08-22.

    Classe 2 é um predicado, não uma lista de literais: o critério é 'expande a partir do cwd'.
    Classe 3 permanece em silêncio por escolha: $FOO/… pode estar correto no ambiente do usuário.
    """
    # Classe 1 -- ancora na raiz do projeto.
    if (
        raw_stripped.startswith("$CLAUDE_PROJECT_DIR/")
        or raw_stripped.startswith("$GEMINI_PROJECT_DIR/")
        or raw_stripped.startswith("$(git rev-parse --show-toplevel)/")
        or os.path.isabs(raw_stripped)
    ):
        return _HOOK_ANCHORAGE_CLASS_ANCHORED
    # ~/… sem aspas: tilde expande para $HOME em qualquer shell POSIX -- semanticamente ancorado.
    # "~/…" com aspas: tilde NÃO expande dentro de aspas duplas, logo a forma quebra -- classe 2.
    if raw_stripped.startswith("~/") and not was_quoted:
        return _HOOK_ANCHORAGE_CLASS_ANCHORED
    # Classe 2 -- expande a partir do cwd.
    # ${PWD}/… tem a mesma semântica de $PWD/… (PWD é mandado pelo POSIX, sempre o cwd).
    if (
        raw_stripped.startswith("$PWD/")
        or raw_stripped.startswith("${PWD}/")
        or raw_stripped.startswith("./")
        or raw_stripped.startswith("../")
        or (not raw_stripped.startswith("$") and not os.path.isabs(raw_stripped))
    ):
        return _HOOK_ANCHORAGE_CLASS_CWD_DEPENDENT
    # Classe 3 -- indecidível; silêncio declarado.
    return _HOOK_ANCHORAGE_CLASS_UNDECIDABLE


def _cwd_dependent_reason(raw_stripped: str) -> str:
    """Retorna o sufixo de mensagem específico por forma para violações de classe 2, iniciando em
    'with a …'. Formas contendo $PWD ou ${PWD} (em qualquer posição, inclusive em wrappers sh -c /
    env) recebem a mensagem do $PWD; demais recebem 'bare relative path'. Usa 'in' (não startsWith)
    para cobrir sh -c "$PWD/…" e env FOO=x $PWD/….
    A frase 'bare relative path' é preservada para não regredir os testes existentes e a UX da
    ROADMAP-2026-08-21 ML-1B."""
    if "$PWD" in raw_stripped or "${PWD}" in raw_stripped:
        return (
            "with a $PWD path — $PWD expands to the current working directory, not the project "
            "root; run `trackfw update` to fix it"
        )
    return (
        "with a bare relative path — this command only resolves from the project root and will "
        "silently fail when the agent's cwd is a subdirectory; run `trackfw update` to fix it"
    )


def _collect_commands_with_marker(value, marker: str, out: list):
    """Percorre recursivamente um valor JSON já decodificado e coleta todo valor-string que
    contém marker, independentemente do nome do campo que o contém -- junto com um sinal
    estrutural de que o objeto imediato que o contém também tem "type":"command" como campo irmão
    (ROADMAP-2026-08-17 ML-4B, port de guardCommandMatch, internal/validator/
    validator_credential_guard.go). Cada entrada de `out` é um dict {"raw": str,
    "type_is_command": bool}.

    Os 6 formatos de hook usam campos diferentes para o comando: "command" (Claude/Codex/
    Gemini/Cursor), "bash" (GitHub Copilot CLI), "action.command" (Kiro). Varrer por VALOR em vez
    de por caminho de chave evita acoplar esta regra à forma exata de cada schema. Todo schema
    aqui coloca "type" como IRMÃO do campo do comando dentro do MESMO objeto -- nunca aninhado
    mais fundo -- então basta ler "type" do dict que esta função já está visitando; não é
    necessária uma travessia schema-aware separada.

    Generalizada (ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
    desatualizados, ML-2B — port de collectCommandsWithMarker, internal/validator/
    validator_credential_guard.go) para aceitar qualquer marker — originalmente
    _collect_credential_guard_commands, hardcoded para _CREDENTIAL_GUARD_SCRIPT_MARKER; reusada
    agora também para _GIT_BRANCH_GUARD_SCRIPT_MARKER, sem duplicar a travessia recursiva.
    """
    if isinstance(value, str):
        # Match solto/top-level, fora de qualquer objeto que o contenha -- fallback defensivo, não
        # esperado disparar já que todo arquivo de hook lido aqui é raiz de um objeto JSON. Sem
        # objeto que o contenha, não há campo "type" irmão para ler.
        if marker in value:
            out.append({"raw": value, "type_is_command": False})
    elif isinstance(value, list):
        for item in value:
            _collect_commands_with_marker(item, marker, out)
    elif isinstance(value, dict):
        type_is_command = value.get("type") == "command"
        for val in value.values():
            if isinstance(val, str):
                if marker in val:
                    out.append({"raw": val, "type_is_command": type_is_command})
                continue
            _collect_commands_with_marker(val, marker, out)


def validate_guard_hook_resolvable(script_marker: str, cwd: str = None) -> list:
    """Implementação genérica compartilhada pelas regras "credential_guard_hook_resolvable" e
    "git_branch_guard_hook_resolvable": para cada arquivo de hook de PROJETO que existir, extrai
    os comandos que referenciam script_marker, resolve o caminho e verifica que o script existe e
    é executável.

    Generalizada a partir da antiga validate_credential_guard_hook_resolvable
    (ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
    ML-2B — port de validateGuardHookResolvable, internal/validator/validator_credential_guard.go)
    — a lógica de resolução de caminho por CLI é idêntica para os 2 scripts, só o marker e o texto
    da mensagem mudam.

    Riscos de regressão mapeados no roadmap (ver ML-1A/ML-2B):
    - A regra só avalia entradas que EXISTEM. Ausência de entrada de guard é estado legítimo
      (guard global instalado via `trackfw update harness`) — nunca é violação por si só.
    - Arquivo de hook ausente é pulado em silêncio.
    - Arquivo de hook presente mas com JSON inválido é pulado em silêncio — validar a forma do
      JSON não é escopo desta regra.
    """
    root = cwd or os.getcwd()
    msgs = []

    for rel_path, cli, requires_command_type, requires_var_or_shell_prefix in _CREDENTIAL_GUARD_HOOK_FILES:
        full_path = os.path.join(root, rel_path)
        try:
            with open(full_path, "r", encoding="utf-8") as f:
                content = f.read()
        except OSError:
            continue

        try:
            parsed = json.loads(content)
        except (json.JSONDecodeError, ValueError):
            continue

        commands = []
        _collect_commands_with_marker(parsed, script_marker, commands)

        seen = set()
        for m in commands:
            seen_key = (m["raw"], m["type_is_command"])
            if seen_key in seen:
                continue
            seen.add(seen_key)

            # ADR-2026-08-22: classificar por ancoragem ANTES de resolver. Aspas externas são
            # sintaxe, não semântica — removê-las antes garante que "$PWD/…" receba o mesmo
            # veredito que $PWD/… sem aspas (achado D.3).
            # was_quoted distingue ~/… (classe 1) de "~/…" (classe 2).
            was_quoted = _hook_value_was_quoted(m["raw"])
            raw_stripped = _strip_outer_quotes_for_classify(m["raw"])
            anchorage_class = _classify_hook_anchorage(raw_stripped, was_quoted)

            # Classe 2 (dependente do cwd) + CLI que exige ancoragem → acusar com mensagem
            # específica por forma. Cursor/Copilot/Kiro têm requires_var_or_shell_prefix=False
            # e nunca chegam aqui — falso-positivo eliminado por construção (AC3 da REQ).
            if requires_var_or_shell_prefix and anchorage_class == _HOOK_ANCHORAGE_CLASS_CWD_DEPENDENT:
                msgs.append({
                    "type": "violation",
                    "message": (
                        f'{rel_path} ({cli}) references {script_marker} '
                        f'{_cwd_dependent_reason(raw_stripped)}'
                    ),
                })
                continue

            # Classe 1 (ancorado) e classe 3 (indecidível) prosseguem para a resolução existente.
            resolved = _resolve_credential_guard_hook_path(m["raw"], root)
            if resolved is None:
                continue

            # ROADMAP-2026-08-17 ML-4B: a command that resolves to a real path but sits inside a
            # structurally malformed entry (missing/wrong "type" where this CLI's schema requires
            # it -- hades-tf ML-4A barrier finding) will NEVER be executed by the CLI, regardless
            # of whether the script itself exists and is executable. Reported instead of the
            # exists/executable checks below, which assume a structurally valid entry.
            if requires_command_type and not m["type_is_command"]:
                msgs.append({
                    "type": "violation",
                    "message": (
                        f'{rel_path} ({cli}) references {script_marker} resolved to '
                        f'"{resolved}", but the hook entry is missing "type":"command" (or has an '
                        f'invalid type) — {cli} will silently never execute it; run `trackfw '
                        f'update` to regenerate it'
                    ),
                })
                continue

            if not os.path.exists(resolved):
                msgs.append({
                    "type": "violation",
                    "message": (
                        f'{rel_path} ({cli}) references {script_marker} resolved to '
                        f'"{resolved}", but the script does not exist — run `trackfw update` to '
                        f'regenerate it'
                    ),
                })
            elif _current_platform != "win32" and not os.access(resolved, os.X_OK):
                msgs.append({
                    "type": "violation",
                    "message": (
                        f'{rel_path} ({cli}) references {script_marker} resolved to '
                        f'"{resolved}", but the script is not executable — run `trackfw update` to '
                        f'regenerate it'
                    ),
                })

    return msgs


def validate_credential_guard_hook_resolvable(cfg: dict, cwd: str = None) -> list:
    """Regra "credential_guard_hook_resolvable" — ver validate_guard_hook_resolvable para a
    implementação compartilhada. cfg é aceito por compatibilidade retroativa (não é consultado no
    corpo da regra, igual antes desta generalização)."""
    return validate_guard_hook_resolvable(_CREDENTIAL_GUARD_SCRIPT_MARKER, cwd)


def validate_git_branch_guard_hook_resolvable(cfg: dict, cwd: str = None) -> list:
    """Regra "git_branch_guard_hook_resolvable" — ver validate_guard_hook_resolvable para a
    implementação compartilhada."""
    return validate_guard_hook_resolvable(_GIT_BRANCH_GUARD_SCRIPT_MARKER, cwd)


# ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A.
# _CREDENTIAL_GUARD_SCRIPT_REFERENCE is a validator-local copy of the same template composed in
# generators/init_gen.py (_CREDENTIAL_GUARD_SH = _CG_HEADER + _CG_PROJECT_GUARD +
# _CG_DETECTION_CORE + _CG_PROJECT_TAIL). Kept as a literal copy -- same choice made for Go
# (internal/validator/validator_credential_guard_integrity_reference.go) and Node
# (npm/src/validator/index.js, CREDENTIAL_GUARD_SCRIPT_REFERENCE) -- for uniformity across the 3
# stacks, even though Python's _generate_credential_guard_script has no stdout side effect that
# would force this choice on its own (unlike Go/Node, whose generator functions print a success
# line on every call). Drift is caught by tests/test_credential_guard_integrity.py, which
# regenerates the real script via _generate_credential_guard_script() into a temp dir and asserts
# byte-equality against this constant. Raw string (r"""...""") to match the convention already
# used by _CG_HEADER/_CG_PROJECT_GUARD/_CG_DETECTION_CORE in generators/init_gen.py -- avoids
# Python interpreting the shell script's own backslash escapes (e.g. \. in the JWT regex).
_CREDENTIAL_GUARD_SCRIPT_REFERENCE = r"""#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

# Script is intentionally a no-op when executed outside the project root
[ -f "trackfw.yaml" ] || exit 0

JWT_PATTERN='eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'
AWS_KEY_PATTERN='AKIA[0-9A-Z]{16}'

MATCH=""
if printf '%s' "$INPUT" | grep -qE "$JWT_PATTERN"; then
  MATCH="JWT"
elif printf '%s' "$INPUT" | grep -qE "$AWS_KEY_PATTERN"; then
  MATCH="AWS access key"
fi

# The raw payload is JSON: any double quote inside the underlying tool_input.command is
# escaped as \" -- unescape those before scanning for redirect targets, or a quoted target
# like "$TMPFILE" is seen as starting with a literal backslash instead of a variable
# reference.
RAW=$(printf '%s' "$INPUT" | sed 's/\\"/"/g')

# Ignore matches that are only ever written to an ephemeral destination
# (mktemp-derived path or /dev/null). A match with no redirect at all
# (printed to stdout, e.g.) or redirected to a plain file path still
# alerts -- that is the incident this hook guards against.
is_ephemeral_target() {
  local target
  target=$(printf '%s' "$1" | tr -d "\"'" | sed -E 's/[},]+$//')
  case "$target" in
    /dev/null) return 0 ;;
    *mktemp*) return 0 ;;
  esac
  if printf '%s' "$target" | grep -qE '^\$\{?[A-Za-z_][A-Za-z0-9_]*\}?$'; then
    local varname pattern
    varname=$(printf '%s' "$target" | sed -E 's/^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$/\1/')
    pattern="*${varname}="'$(mktemp'"*"
    case "$RAW" in
      $pattern) return 0 ;;
    esac
  fi
  return 1
}

REDIRECTS=$(printf '%s' "$RAW" | grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+' || true)

# Second detection layer: only runs when the payload scan above found nothing -- keeps the common
# case (match already found) cheap and avoids reading files unnecessarily. Files above the size cap
# are skipped silently.
scan_file_for_pattern() {
  local path size
  path=$(printf '%s' "$1" | tr -d "\"'" | sed -E 's/[},]+$//')
  [ -n "$path" ] && [ -f "$path" ] || return 1
  size=$(wc -c < "$path" 2>/dev/null | tr -d '[:space:]')
  size=${size:-0}
  [ "$size" -lt 1048576 ] || return 1
  if grep -qE "$JWT_PATTERN" "$path" 2>/dev/null; then
    MATCH="JWT"
    return 0
  fi
  if grep -qE "$AWS_KEY_PATTERN" "$path" 2>/dev/null; then
    MATCH="AWS access key"
    return 0
  fi
  return 1
}

if [ -z "$MATCH" ] && [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      scan_file_for_pattern "$target" && break
    fi
  done <<< "$REDIRECTS"
fi

if [ -z "$MATCH" ]; then
  CMD_LINE=$(printf '%s' "$RAW" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  if [ -n "$CMD_LINE" ]; then
    set -- $CMD_LINE
    cmd_name="${1:-}"
    case "$cmd_name" in
      cat|head|tail|jq|grep)
        shift
        for token in "$@"; do
          scan_file_for_pattern "$token" && break
        done
        ;;
    esac
  fi
fi

[ -n "$MATCH" ] || exit 0

HAS_REDIRECT=0
EXEMPT=1
if [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    HAS_REDIRECT=1
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      EXEMPT=0
    fi
  done <<< "$REDIRECTS"
fi

if [ "$HAS_REDIRECT" -eq 1 ] && [ "$EXEMPT" -eq 1 ]; then
  exit 0
fi

DEFAULT_MODE="warn"
MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
esac

if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR=$(grep '^roadmap_dir:' trackfw.yaml 2>/dev/null | head -1 | sed 's/^roadmap_dir:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d '"' | tr -d "'" || true)
ROADMAP_DIR=${ROADMAP_DIR:-docs/roadmaps}

case "$ROADMAP_DIR" in
  /*|../*|*/../*|*/..|..) ROADMAP_DIR="docs/roadmaps" ;;
esac

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$MSG_ESC" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
"""


_CREDENTIAL_GUARD_MODE_LOOKBEHIND_LINES = 5


def _extract_credential_guard_mode(content: str):
    """Mirrors the shell script's own resolution of credential_guard.mode (`grep -A 5
    '^credential_guard:'`): the mode key is found on the matched line or within the 5 lines
    following it. Deliberately the SAME lightweight line-scan the shipped script itself uses --
    not a full YAML parser -- so this rule's notion of "what credential_guard.mode resolves to"
    matches what actually runs at hook time.

    Returns (mode: str, ok: bool). ok is False when no "credential_guard:" line exists at all, OR
    when it exists but no "mode:" key is found within the lookbehind window.
    """
    lines = content.split("\n")
    start = -1
    for i, line in enumerate(lines):
        if line.startswith("credential_guard:"):
            start = i
            break
    if start == -1:
        return "", False

    end = min(start + 1 + _CREDENTIAL_GUARD_MODE_LOOKBEHIND_LINES, len(lines))
    for line in lines[start:end]:
        trimmed = line.strip()
        if "mode:" not in trimmed:
            continue
        rest = trimmed[len("mode:"):] if trimmed.startswith("mode:") else trimmed
        rest = rest.strip()
        hash_idx = rest.find("#")
        if hash_idx >= 0:
            rest = rest[:hash_idx].strip()
        rest = rest.strip("\"'")
        return rest, True

    return "", False


def _head_trackfw_yaml(cwd: str):
    """Returns the content of trackfw.yaml as committed at HEAD, resolved relative to cwd (not
    necessarily the git toplevel -- `trackfw validate` can run from a subdirectory). ok is False
    whenever there is no usable anchor: not a git worktree, no commits yet, or trackfw.yaml not
    tracked at HEAD -- every one of these is a "no anchor, stay silent" case, never an error.
    """
    if not _is_git_worktree(cwd):
        return "", False
    try:
        verify = _git_run(cwd, ["rev-parse", "--verify", "HEAD"])
        if verify.returncode != 0:
            return "", False
    except Exception:
        return "", False

    try:
        show = _git_run(cwd, ["show", "HEAD:./trackfw.yaml"])
        if show.returncode != 0:
            return "", False
        return show.stdout, True
    except Exception:
        return "", False


def _credential_guard_mode_downgrade_message() -> str:
    return (
        "trackfw.yaml sets credential_guard.mode: block at the git HEAD commit, but the "
        "current file does not resolve to block — if this was intentional, commit the change; "
        "otherwise investigate before treating the credential guard as active"
    )


def validate_credential_guard_script_integrity(cwd: str = None) -> list:
    """Regra "credential_guard_script_integrity": compara scripts/trackfw-credential-guard.sh em
    disco contra o template que esta versão do trackfw geraria (âncora: o binário/pacote, não o
    git). Silenciosa quando o script não existe -- ausência é escopo de
    credential_guard_hook_resolvable, não desta regra. Severidade default "warning" (ver
    _RULE_DEFAULTS): o script não carrega marcador de versão, então esta regra não consegue
    distinguir drift legítimo de adulteração real -- a mensagem é causalmente neutra por isso
    (ADR-2026-08-12 Emenda 3).
    """
    root = cwd or os.getcwd()
    rel_path = "scripts/trackfw-credential-guard.sh"
    full_path = os.path.join(root, rel_path)
    try:
        with open(full_path, "r", encoding="utf-8") as f:
            content = f.read()
    except FileNotFoundError:
        return []
    except OSError:
        return []

    if content == _CREDENTIAL_GUARD_SCRIPT_REFERENCE:
        return []

    return [{
        "type": "warning",
        "message": (
            f"{rel_path} content diverges from the template this version of trackfw generates — "
            f"if you did not edit this file by hand, run `trackfw update` to regenerate it"
        ),
    }]




# ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-desatualizados,
# ML-2B: adds git-branch-guard coverage to the two existing credential-guard checks (existence/
# executability via validate_guard_hook_resolvable above, and content-drift integrity via
# validate_guard_script_integrity below), plus the GLOBAL-scope check that was missing for BOTH
# guards before this ML. Port of internal/validator/validator_git_branch_guard.go (Go, same ROADMAP
# ML-1A) -- see that file's doc comments for the full design rationale; this port follows it 1:1.

# _GIT_BRANCH_GUARD_SCRIPT_REFERENCE is a validator-local copy of the scripts/trackfw-git-branch-
# guard.sh template composed in generators/init_gen.py (_GIT_BRANCH_GUARD_SH). Kept as a literal
# copy -- same choice already made for _CREDENTIAL_GUARD_SCRIPT_REFERENCE above (uniformity across
# the 3 stacks; Go's version is additionally forced by an import-cycle constraint that Python does
# not share, but the existing Python pattern for credential-guard already chose a local literal
# copy regardless, so this port follows that same convention rather than importing from
# generators.init_gen directly).
#
# Unlike _CREDENTIAL_GUARD_SCRIPT_REFERENCE (project-scope only, distinct from the global-scope
# variant below), _GIT_BRANCH_GUARD_SCRIPT_REFERENCE is used VERBATIM for both project scope
# (_generate_git_branch_guard_script) and global scope (generate_global_git_branch_guard_script) --
# see generators/init_gen.py's doc comment on _GIT_BRANCH_GUARD_SH ("o conteúdo é idêntico entre
# escopo de projeto e escopo global"). So this single reference constant covers both
# git_branch_guard_script_integrity (project, scripts/trackfw-git-branch-guard.sh) and the global
# integrity check (~/.trackfw/scripts/trackfw-git-branch-guard.sh) -- no second reference constant
# needed, unlike credential-guard which requires a separate
# _CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE (different template, no project-guard block). Drift
# between this copy and the real generator is caught by
# test_git_branch_guard.py::TestGitBranchGuardScriptReference::test_reference_e_byte_identico_ao_gerador_real.
_GIT_BRANCH_GUARD_SCRIPT_REFERENCE = r"""#!/usr/bin/env bash
# trackfw git branch guard — bloqueia git commit/push/checkout -b/branch/worktree add -b
# brutos por subagente
#
# TRIPWIRE, NÃO FRONTEIRA DE SEGURANÇA: detecta o caso óbvio — comando git literal, sem
# indireção de shell — não é defesa contra um agente adversário competente. Evasões que
# exigem tokenizar como o bash (ex.: git${IFS}push, {git,push}, g""it push) permanecem
# abertas por decisão: ver docs/adr/ADR-2026-08-12-nao-ha-prevencao-contra-agente-induzido-
# com-escrita-irrestrita-a-resposta-e-deteccao-ancorada-no-git.md. O stripping de
# env/command abaixo reconhece as formas SEM argumentos antes de git (env git ...,
# command git ...) e o env seguido de uma sequência de atribuições CHAVE=valor
# (env FOO=bar git ..., env FOO=bar BAZ=qux git ...) — env com FLAGS (env -i git ...,
# env --ignore-environment git ...) e command com flags (command -p git ...) continuam
# evadindo; declarado, não fechado (ver AC5 do ML que adicionou esse stripping). A
# segmentação abaixo
# (quote_aware_split) evita falso-positivo em texto citado — não deve ser lida como imune a
# evasão por citação/tokenização do shell.
set -euo pipefail
set -f

# --- 0. Drena o stdin ANTES de qualquer saída antecipada (ML-1B, ROADMAP-2026-08-17-guard-
# global-cabeado-com-no-op-fora-de-projeto-e-integridade-independente-de-fiacao.md): sem isso,
# quem escreve o payload JSON no pipe recebe EPIPE quando o no-op abaixo sai com 0 antes de ler
# — reprodutível em 100% das chamadas fora de projeto trackfw, não é corrida de timing. Só drena
# se stdin não for um terminal interativo (-t 0): em invocação manual sem pipe, "cat" bloquearia
# esperando EOF (Ctrl-D). O valor lido é reaproveitado no passo 1 abaixo — nunca há uma segunda
# leitura.
_TRACKFW_STDIN=""
[ -t 0 ] || _TRACKFW_STDIN=$(cat 2>/dev/null || true)

# --- 0b. No-op fora de projeto trackfw (ADR-2026-08-17-guard-global-cabeado-com-no-op-fora-de-
# projeto-trackfw.md): sobe diretórios a partir do cwd FÍSICO (pwd -P, resolve symlink) até
# achar trackfw.yaml na raiz do projeto. Sem trackfw.yaml em nenhum ancestral, o guard não se
# aplica — fora de projeto trackfw não há trackfw ship como alternativa, e bloquear ali é custo
# sem contrapartida. Custo medido: só parameter expansion e test -f por nível, nenhum fork de
# processo; limitado pela profundidade do caminho.
_TRACKFW_ROOT_DIR=$(pwd -P)
_TRACKFW_FOUND=0
while :; do
  if [ -f "$_TRACKFW_ROOT_DIR/trackfw.yaml" ]; then
    _TRACKFW_FOUND=1
    break
  fi
  if [ "$_TRACKFW_ROOT_DIR" = "/" ]; then
    break
  fi
  _TRACKFW_ROOT_DIR="${_TRACKFW_ROOT_DIR%/*}"
  if [ -z "$_TRACKFW_ROOT_DIR" ]; then
    _TRACKFW_ROOT_DIR="/"
  fi
done
[ "$_TRACKFW_FOUND" -eq 1 ] || exit 0

# --- 1. Obter o comando git bruto ------------------------------------------------------------
if [ "$#" -gt 0 ]; then
  CMD_RAW="$*"
else
  INPUT="$_TRACKFW_STDIN"
  TRIMMED=$(printf '%s' "$INPUT" | sed -e 's/^[[:space:]]*//')
  case "$TRIMMED" in
    \{*)
      CMD_RAW=""
      if command -v jq >/dev/null 2>&1; then
        CMD_RAW=$(printf '%s' "$INPUT" | jq -r '.tool_input.command // .command // .tool_info.command_line // .hook_input.command // empty' 2>/dev/null || true)
      fi
      if [ -z "$CMD_RAW" ] || [ "$CMD_RAW" = "null" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"tool_info"[[:space:]]*:[[:space:]]*{[^}]*"command_line"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"hook_input"[[:space:]]*:[[:space:]]*{[^}]*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      if [ -z "$CMD_RAW" ]; then
        CMD_RAW=$(printf '%s' "$INPUT" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
      fi
      ;;
    *)
      CMD_RAW="$INPUT"
      ;;
  esac
fi

if [ -z "$CMD_RAW" ]; then
  CMD_RAW="${TRACKFW_GIT_COMMAND:-}"
fi

[ -n "$CMD_RAW" ] || exit 0

# --- 2. Pré-processamento anti-falso-positivo: neutraliza separadores reais (';', '&&',
# '||', '|', quebra de linha) que estão DENTRO de aspas ou de corpo de heredoc, para que
# conteúdo de mensagem (ex.: `-m "linha 1\nlinha 2"`) nunca seja fatiado em pseudo-segmentos
# e lido como comando -------------------------------------------------------------------
#
# strip_heredoc_bodies: remove o CORPO de blocos heredoc (<<DELIM ... DELIM), preservando a
# linha de abertura e a linha terminadora — cobre o padrão `git commit -F- <<'EOF' ... EOF`
# (heredoc não citado, fora do escopo de quote_aware_split abaixo). Heurística por linha, não
# sintaxe completa de shell: só remove o corpo quando encontra a linha terminadora
# correspondente. Se o heredoc nunca fecha (terminador ausente ou não localizado), devolve o
# texto ORIGINAL sem qualquer alteração — lado seguro: mais restritivo é preferível a esconder
# um comando real atrás de um heredoc mal-formado.
strip_heredoc_bodies() {
  printf '%s' "$1" | awk '
    BEGIN {
      dq = sprintf("%c", 34)
      sq = sprintf("%c", 39)
      in_heredoc = 0
      delim = ""
      ok = 1
    }
    {
      raw = raw $0 "\n"
      if (in_heredoc) {
        trimmed = $0
        sub(/^[ \t]+/, "", trimmed)
        sub(/[ \t]+$/, "", trimmed)
        if (trimmed == delim) {
          in_heredoc = 0
          out = out $0 "\n"
        }
        next
      }
      if (match($0, /<<-?[ \t]*[^ \t]+/)) {
        d = substr($0, RSTART, RLENGTH)
        sub(/^<<-?[ \t]*/, "", d)
        gsub(dq, "", d)
        gsub(sq, "", d)
        if (d != "") {
          delim = d
          in_heredoc = 1
        }
      }
      out = out $0 "\n"
    }
    END {
      if (in_heredoc) ok = 0
      if (ok) { printf "%s", out } else { printf "%s", raw }
    }
  '
}

# quote_aware_split: emite o texto com ';' isolado, '&&', '||' e '|' isolado convertidos em
# quebra de linha — EXCETO quando ocorrem dentro de uma string entre aspas simples ou duplas,
# caso em que são preservados como texto e uma quebra de linha real dentro das aspas vira
# espaço (nunca gera um novo pseudo-segmento). Substitui o antigo `sed` cego, que não
# distinguia texto citado de sintaxe de comando — a causa raiz do falso-positivo de linha de
# mensagem de commit iniciada por "git ...". Aspas não fechadas até o fim da entrada
# permanecem "abertas" até o fim — mesma semântica do shell real: uma aspa não fechada nunca
# deixa o texto seguinte executar como comando novo, só torna o restante parte da mesma
# string.
quote_aware_split() {
  printf '%s' "$1" | awk '
    BEGIN {
      dq = sprintf("%c", 34)
      sq = sprintf("%c", 39)
      bs = sprintf("%c", 92)
      nl = sprintf("%c", 10)
    }
    { s = (NR == 1) ? $0 : s nl $0 }
    END {
      n = length(s)
      q = ""
      out = ""
      i = 1
      while (i <= n) {
        c = substr(s, i, 1)
        if (q != "") {
          if (q == dq && c == bs && i < n) {
            nx = substr(s, i + 1, 1)
            out = out c (nx == nl ? " " : nx)
            i += 2
            continue
          }
          if (c == q) {
            q = ""
            out = out c
            i++
            continue
          }
          out = out (c == nl ? " " : c)
          i++
          continue
        }
        if (c == dq || c == sq) {
          q = c
          out = out c
          i++
          continue
        }
        if (substr(s, i, 2) == "&&" || substr(s, i, 2) == "||") {
          out = out nl
          i += 2
          continue
        }
        if (c == ";" || c == "|") {
          out = out nl
          i++
          continue
        }
        out = out c
        i++
      }
      printf "%s", out
    }
  '
}

# match_subcommand — casa contra "git (commit|push|checkout -b|switch -c)", segmento por
# segmento. Cada segmento é um comando real, obtido depois do pré-processamento acima
# (strip_heredoc_bodies + quote_aware_split), que converte ';', '&&', '||', '|' fora de aspas
# em quebra de linha e neutraliza os mesmos separadores quando aparecem dentro de
# aspas/heredoc. "git" só conta se for o PRIMEIRO token do segmento (por basename, então
# /usr/bin/git também casa) — nunca uma ocorrência solta em qualquer posição da string
# inteira. Isso evita: (a) o segundo comando de uma cadeia escapar da checagem, (b) um path
# absoluto para o git escapar por comparação de igualdade exata, e (c) texto de prosa —
# inclusive linha de mensagem de commit que COMEÇA com "git <sub>" (ex.: uma tabela
# documentando comandos bloqueados) — ser tratado como comando, porque esse texto agora nunca
# produz um novo segmento. `git switch -c/-C/--create` (forma alternativa a `checkout -b`
# para criar branch) é reconhecido varrendo TODOS os tokens após o subcomando, não só o
# primeiro — cobre `git switch --track -c feat/x` (flag antes de -c).
# checkout -b é reconhecido do mesmo jeito: varre TODOS os tokens até achar -b/-B/--orphan,
# não só o primeiro. Prefixos env e command antes de git são descartados antes da checagem do
# basename — cobre env git push/command git push sem exigir tokenizar como o bash.
match_subcommand() {
  normalized=$(strip_heredoc_bodies "$1")
  normalized=$(quote_aware_split "$normalized")
  while IFS= read -r seg; do
    seg_trimmed=$(printf '%s' "$seg" | sed -e 's/^[[:space:]]*//')
    [ -n "$seg_trimmed" ] || continue

    set -- $seg_trimmed
    first="$1"
    base="${first##*/}"
    while [ "$base" = "env" ] || [ "$base" = "command" ]; do
      is_env="$base"
      shift
      [ "$#" -gt 0 ] || break
      if [ "$is_env" = "env" ]; then
        while [ "$#" -gt 0 ]; do
          case "$1" in
            -*)
              break
              ;;
            *=*)
              shift
              ;;
            *)
              break
              ;;
          esac
        done
        [ "$#" -gt 0 ] || break
      fi
      first="$1"
      base="${first##*/}"
    done
    [ "$base" = "git" ] || continue
    shift

    sub=""
    while [ "$#" -gt 0 ]; do
      tok="$1"
      case "$tok" in
        -C|-c|--work-tree|--git-dir|--namespace)
          if [ "$#" -ge 2 ]; then shift 2; else shift; fi
          continue
          ;;
        -*)
          shift
          continue
          ;;
        *)
          sub="$tok"
          shift
          break
          ;;
      esac
    done

    case "$sub" in
      commit)
        echo "commit"
        return 0
        ;;
      push)
        echo "push"
        return 0
        ;;
      checkout)
        for tok2 in "$@"; do
          case "$tok2" in
            -b|-B|--orphan|--orphan=*)
              echo "checkout-b"
              return 0
              ;;
          esac
        done
        # git checkout -- <path> | git checkout . descarta alterações não commitadas do
        # caminho indicado, de forma irreversível, no worktree compartilhado — bloqueia
        # quando '--' aparece em qualquer posição (forma explícita de pathspec) ou quando
        # '.' aparece como token isolado. 'git checkout <branch>' sem nenhum dos dois
        # segue liberado por decisão (distinguir branch de caminho sem '--' é ambíguo, e
        # adivinhar produziria falso-positivo).
        checkout_path=0
        for tok2 in "$@"; do
          case "$tok2" in
            --|.)
              checkout_path=1
              ;;
          esac
        done
        if [ "$checkout_path" = "1" ]; then
          echo "checkout-path"
          return 0
        fi
        ;;
      switch)
        for tok2 in "$@"; do
          case "$tok2" in
            -c|-C|--create|--create=*|--force-create|--force-create=*)
              echo "switch-c"
              return 0
              ;;
          esac
        done
        ;;
      stash)
        # git stash: liberado só para leitura (list/show) — bloqueia a forma bare
        # (equivale a "push"), push, save, clear e drop. Decisão de KG: bloquear a
        # classe inteira, não só os literais medidos (ver REQ). Repositório com um único
        # worktree compartilhado entre subagentes paralelos — um stash de um agente
        # remove as alterações não commitadas de todos os outros.
        stash_sub="${1:-}"
        case "$stash_sub" in
          list|show)
            ;;
          *)
            echo "stash"
            return 0
            ;;
        esac
        ;;
      reset)
        # Só --hard bloqueia, em qualquer posição de token — --soft/--mixed (inclusive
        # sem flag, que é --mixed implícito) seguem liberados: --soft é o contorno
        # padrão para reempurrar trabalho staged via `trackfw ship -m "..."` (ainda falta commitar após --soft).
        for tok2 in "$@"; do
          case "$tok2" in
            --hard)
              echo "reset-hard"
              return 0
              ;;
          esac
        done
        ;;
      clean)
        # Bloqueia qualquer forma com force (-f, -fd, -fx, --force) ou -x/-X, EXCETO
        # quando -n/--dry-run também está presente (dry-run nunca apaga nada).
        clean_dry=0
        clean_force=0
        for tok2 in "$@"; do
          case "$tok2" in
            -n|--dry-run)
              clean_dry=1
              ;;
            -f*|--force|--force=*|-x|-X)
              clean_force=1
              ;;
          esac
        done
        if [ "$clean_dry" != "1" ] && [ "$clean_force" = "1" ]; then
          echo "clean-force"
          return 0
        fi
        ;;
      restore)
        # git restore --staged SOZINHO nunca toca o working tree (mexe só no
        # index), então segue liberado mesmo com path. Mas --worktree/-W (com ou
        # sem --staged junto) SEMPRE afeta o working tree — inclusive
        # "--staged --worktree", que restaura os dois — então bloqueia sempre que
        # --worktree/-W aparecer, e também no caso padrão (sem --staged em
        # nenhuma forma) com um argumento posicional (o path).
        restore_staged=0
        restore_worktree=0
        restore_positional=0
        for tok2 in "$@"; do
          case "$tok2" in
            --staged)
              restore_staged=1
              ;;
            --worktree|-W)
              restore_worktree=1
              ;;
            -*)
              ;;
            *)
              restore_positional=1
              ;;
          esac
        done
        if [ "$restore_positional" = "1" ]; then
          if [ "$restore_worktree" = "1" ] || [ "$restore_staged" != "1" ]; then
            echo "restore-path"
            return 0
          fi
        fi
        ;;
      branch)
        # git branch é majoritariamente leitura (sem args, -a, -r, -l, --list, -v/-vv,
        # --show-current, --contains, --no-contains, --merged, --no-merged, --sort=,
        # --format=, --points-at, -d/-D/--delete) — bloquear leitura seria pior que a
        # brecha. Só bloqueia: (a) -c/-C/-m/-M/--copy/--move (cria/renomeia branch,
        # qualquer posição de token) ou (b) um argumento posicional puro (nome da branch a
        # criar), a menos que -d/-D/--delete também esteja presente (delete tem
        # posicional legítimo — o nome a apagar). Flags de valor conhecidas (--contains,
        # --no-contains, --sort, --format, --points-at, --merged, --no-merged) têm seu
        # valor seguinte pulado quando vem em token separado, para não ser lido como
        # posicional de criação.
        branch_action=0
        has_delete=0
        saw_positional=0
        skip_next=0
        for tok2 in "$@"; do
          if [ "$skip_next" = "1" ]; then
            skip_next=0
            continue
          fi
          case "$tok2" in
            -c|-C|-m|-M|--copy|--copy=*|--move|--move=*)
              branch_action=1
              ;;
            -d|-D|--delete|--delete=*)
              has_delete=1
              ;;
            --contains|--no-contains|--sort|--format|--points-at|--merged|--no-merged)
              skip_next=1
              ;;
            -*)
              ;;
            *)
              saw_positional=1
              ;;
          esac
        done
        if [ "$has_delete" != "1" ]; then
          if [ "$branch_action" = "1" ] || [ "$saw_positional" = "1" ]; then
            echo "branch-create"
            return 0
          fi
        fi
        ;;
      worktree)
        if [ "${1:-}" = "add" ]; then
          shift
          for tok2 in "$@"; do
            case "$tok2" in
              -b|-B)
                echo "worktree-add-b"
                return 0
                ;;
            esac
          done
        elif [ "${1:-}" = "remove" ]; then
          # git worktree remove SEM -f/--force já recusa sozinho quando há alteração não
          # commitada no worktree indicado — só a forma com force é irreversível o bastante
          # para bloquear aqui.
          shift
          for tok2 in "$@"; do
            case "$tok2" in
              -f|--force)
                echo "worktree-remove-force"
                return 0
                ;;
            esac
          done
        fi
        ;;
      update-ref)
        # git update-ref reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o
        # objeto apontado nem exigir push — foi o mecanismo que tornou alcançável o exploit
        # descrito no ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md
        # (Emenda 1): forjar origin/<base> localmente para desviar o commit-alvo de trackfw
        # release tag. Sem forma de leitura equivalente a bloquear seletivamente — a
        # subcommand inteira é escrita — bloqueia sempre, sem exceção de token.
        echo "update-ref"
        return 0
        ;;
      rm)
        # git rm -f/--force apaga do working tree e do index de forma irreversível, mesma
        # classe de git clean -f/git reset --hard já bloqueados acima — sem exceção para
        # --cached (destrancar do index sem -f já segue liberado por não precisar de force).
        for tok2 in "$@"; do
          case "$tok2" in
            -f*|--force|--force=*)
              echo "rm-force"
              return 0
              ;;
          esac
        done
        ;;
    esac
  done <<EOF
$normalized
EOF
  return 1
}

SUBCOMMAND=$(match_subcommand "$CMD_RAW") || exit 0

case "$SUBCOMMAND" in
  checkout-b)
    REASON="trackfw: git checkout -b bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  switch-c)
    REASON="trackfw: git switch -c bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  branch-create)
    REASON="trackfw: git branch bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  worktree-add-b)
    REASON="trackfw: git worktree add -b bruto bloqueado. Use \`trackfw branch new <type>/<slug>\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  commit)
    REASON="trackfw: git commit bruto bloqueado. Use \`trackfw commit -m '<mensagem>'\`. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  push)
    REASON="trackfw: git push bruto bloqueado. Use \`trackfw push\` (para empurrar commits já criados), \`trackfw ship\` (para commit+push+PR em uma etapa) ou \`trackfw release tag\` (para publicar uma tag de release). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  stash)
    REASON="trackfw: git stash bruto bloqueado — worktree compartilhado entre subagentes, um stash remove as alterações não commitadas de todos os outros. \`git stash list\`/\`git stash show\` seguem liberados; para guardar trabalho em progresso, use uma branch própria via \`trackfw branch new\` e commit nela. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  reset-hard)
    REASON="trackfw: git reset --hard bruto bloqueado — descarta de forma irreversível as alterações não commitadas de todo o worktree compartilhado. \`git reset --soft\`/\`--mixed\` seguem liberados (ex.: \`git reset --soft HEAD~1\` é o caminho padrão; use \`trackfw ship -m "..."\` para commitar e empurrar). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  clean-force)
    REASON="trackfw: git clean -f/-x bruto bloqueado — apaga arquivos não rastreados do worktree compartilhado, de forma irreversível. \`git clean -n\`/\`--dry-run\` segue liberado para revisar antes o que seria apagado. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  restore-path)
    REASON="trackfw: git restore <path> bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. \`git restore --staged\` (não toca o working tree) segue liberado; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  checkout-path)
    REASON="trackfw: git checkout -- <path>/git checkout . bruto bloqueado — descarta de forma irreversível as alterações não commitadas do caminho indicado. \`git checkout <branch>\`/\`git switch <branch>\` seguem liberados; para descartar de fato, confirme antes com o usuário. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  update-ref)
    REASON="trackfw: git update-ref bruto bloqueado — reescreve um ref (inclusive refs/remotes/origin/*) sem tocar o objeto apontado nem exigir push, o que permite forjar o commit-alvo que \`trackfw release tag\` publicaria. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  worktree-remove-force)
    REASON="trackfw: git worktree remove -f/--force bruto bloqueado — remove um worktree e descarta de forma irreversível qualquer alteração não commitada nele. \`git worktree remove\` sem force segue liberado (recusa sozinho quando há algo não commitado). Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  rm-force)
    REASON="trackfw: git rm -f/--force bruto bloqueado — apaga arquivos do working tree e do index de forma irreversível, mesma classe de \`git clean -f\`/\`git reset --hard\` já bloqueados. Nada antes deste comando foi executado (comando composto é bloqueado por inteiro). Ver CLAUDE.md §1."
    ;;
  *)
    exit 0
    ;;
esac

printf '{"decision":"block","reason":"%s"}\n' "$REASON"
echo "$REASON" >&2
exit 2
"""


# _CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE is a validator-local copy of the GLOBAL-scope
# ~/.trackfw/scripts/trackfw-credential-guard.sh template composed in generators/init_gen.py
# (_GLOBAL_CREDENTIAL_GUARD_SH = _CG_HEADER + _CG_DETECTION_CORE + _CG_GLOBAL_TAIL). This is a
# DIFFERENT template than _CREDENTIAL_GUARD_SCRIPT_REFERENCE (the project-scope variant): the
# global variant omits the project-guard block ("no-op outside a trackfw.yaml project") and
# defaults credential_guard.mode to "block" instead of "warn". Comparing the global on-disk script
# against _CREDENTIAL_GUARD_SCRIPT_REFERENCE (the project template) would be a guaranteed false
# positive for every user with the global harness installed -- this repo included. Mirrors Go's
# credentialGuardGlobalScriptReference
# (internal/validator/validator_credential_guard_global_reference.go).
_CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE = r"""#!/usr/bin/env bash
# trackfw credential guard — PreToolUse/PostToolUse hook
set -euo pipefail

INPUT=$(cat)

JWT_PATTERN='eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+'
AWS_KEY_PATTERN='AKIA[0-9A-Z]{16}'

MATCH=""
if printf '%s' "$INPUT" | grep -qE "$JWT_PATTERN"; then
  MATCH="JWT"
elif printf '%s' "$INPUT" | grep -qE "$AWS_KEY_PATTERN"; then
  MATCH="AWS access key"
fi

# The raw payload is JSON: any double quote inside the underlying tool_input.command is
# escaped as \" -- unescape those before scanning for redirect targets, or a quoted target
# like "$TMPFILE" is seen as starting with a literal backslash instead of a variable
# reference.
RAW=$(printf '%s' "$INPUT" | sed 's/\\"/"/g')

# Ignore matches that are only ever written to an ephemeral destination
# (mktemp-derived path or /dev/null). A match with no redirect at all
# (printed to stdout, e.g.) or redirected to a plain file path still
# alerts -- that is the incident this hook guards against.
is_ephemeral_target() {
  local target
  target=$(printf '%s' "$1" | tr -d "\"'" | sed -E 's/[},]+$//')
  case "$target" in
    /dev/null) return 0 ;;
    *mktemp*) return 0 ;;
  esac
  if printf '%s' "$target" | grep -qE '^\$\{?[A-Za-z_][A-Za-z0-9_]*\}?$'; then
    local varname pattern
    varname=$(printf '%s' "$target" | sed -E 's/^\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?$/\1/')
    pattern="*${varname}="'$(mktemp'"*"
    case "$RAW" in
      $pattern) return 0 ;;
    esac
  fi
  return 1
}

REDIRECTS=$(printf '%s' "$RAW" | grep -oE '[0-9]?>>?[[:space:]]*[^[:space:]|&;,:]+' || true)

# Second detection layer: only runs when the payload scan above found nothing -- keeps the common
# case (match already found) cheap and avoids reading files unnecessarily. Files above the size cap
# are skipped silently.
scan_file_for_pattern() {
  local path size
  path=$(printf '%s' "$1" | tr -d "\"'" | sed -E 's/[},]+$//')
  [ -n "$path" ] && [ -f "$path" ] || return 1
  size=$(wc -c < "$path" 2>/dev/null | tr -d '[:space:]')
  size=${size:-0}
  [ "$size" -lt 1048576 ] || return 1
  if grep -qE "$JWT_PATTERN" "$path" 2>/dev/null; then
    MATCH="JWT"
    return 0
  fi
  if grep -qE "$AWS_KEY_PATTERN" "$path" 2>/dev/null; then
    MATCH="AWS access key"
    return 0
  fi
  return 1
}

if [ -z "$MATCH" ] && [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      scan_file_for_pattern "$target" && break
    fi
  done <<< "$REDIRECTS"
fi

if [ -z "$MATCH" ]; then
  CMD_LINE=$(printf '%s' "$RAW" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
  if [ -n "$CMD_LINE" ]; then
    set -- $CMD_LINE
    cmd_name="${1:-}"
    case "$cmd_name" in
      cat|head|tail|jq|grep)
        shift
        for token in "$@"; do
          scan_file_for_pattern "$token" && break
        done
        ;;
    esac
  fi
fi

[ -n "$MATCH" ] || exit 0

HAS_REDIRECT=0
EXEMPT=1
if [ -n "$REDIRECTS" ]; then
  while IFS= read -r line; do
    if [ -z "$line" ]; then
      continue
    fi
    HAS_REDIRECT=1
    target=$(printf '%s' "$line" | sed -E 's/^[0-9]?>>?[[:space:]]*//')
    if ! is_ephemeral_target "$target"; then
      EXEMPT=0
    fi
  done <<< "$REDIRECTS"
fi

if [ "$HAS_REDIRECT" -eq 1 ] && [ "$EXEMPT" -eq 1 ]; then
  exit 0
fi

DEFAULT_MODE="block"
MODE=$(grep -A 5 '^credential_guard:' trackfw.yaml 2>/dev/null | grep 'mode:' | head -1 | sed -E 's/^[[:space:]]*mode:[[:space:]]*//; s/[[:space:]]*#.*$//' | tr -d "\"'" || true)
case "$MODE" in
  warn|block) ;;
  *) MODE="$DEFAULT_MODE" ;;
esac

if [ "$MODE" = "block" ]; then
  echo "trackfw-credential-guard: blocked - possible $MATCH detected in tool payload." >&2
  exit 2
fi

echo "trackfw-credential-guard: warning - possible $MATCH detected in tool payload." >&2

ROADMAP_DIR="docs/roadmaps"
if [ ! -d "$ROADMAP_DIR" ]; then
  exit 0
fi

TIMESTAMP=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
MSG="Possible $MATCH detected in tool payload - review before materializing credentials in plain text."
MSG_ESC=$(echo "$MSG" | tr -d '\000-\037' | sed 's/\\/\\\\/g; s/"/\\"/g')

mkdir -p "$ROADMAP_DIR"
printf '{"tool":"credential-guard","message":"%s","level":"action_required","timestamp":"%s"}\n' \
  "$MSG_ESC" \
  "$TIMESTAMP" > "$ROADMAP_DIR/.trackfw-credential-guard.json"

exit 0
"""


def validate_guard_script_integrity(rel_path: str, reference_content: str, cwd: str = None) -> list:
    """Implementação genérica compartilhada por "credential_guard_script_integrity" e
    "git_branch_guard_script_integrity": compara rel_path em disco contra reference_content
    byte-a-byte (âncora: o binário/pacote, não o git). Silenciosa quando o script não existe --
    ausência é escopo de *_hook_resolvable, não desta regra.

    Port de validateGitBranchGuardScriptIntegrity/validateCredentialGuardScriptIntegrity
    (internal/validator/*.go). Severidade default "warning" para as duas regras (ver
    _RULE_DEFAULTS): nenhum dos dois scripts carrega marcador de versão, então esta regra não
    consegue distinguir drift legítimo de adulteração real -- a mensagem é causalmente neutra por
    isso (mesmo raciocínio de ADR-2026-08-12 Emenda 3, agora também aplicado a git-branch-guard).
    """
    root = cwd or os.getcwd()
    full_path = os.path.join(root, rel_path)
    try:
        with open(full_path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return []

    if content == reference_content:
        return []

    return [{
        "type": "warning",
        "message": (
            f"{rel_path} content diverges from the template this version of trackfw generates — "
            f"if you did not edit this file by hand, run `trackfw update` to regenerate it"
        ),
    }]


def validate_git_branch_guard_script_integrity(cwd: str = None) -> list:
    """Regra "git_branch_guard_script_integrity" -- ver validate_guard_script_integrity para a
    implementação compartilhada. Mirrors validate_credential_guard_script_integrity exactly -- same
    silent-on-absence contract."""
    return validate_guard_script_integrity(
        "scripts/trackfw-git-branch-guard.sh", _GIT_BRANCH_GUARD_SCRIPT_REFERENCE, cwd
    )


# _GLOBAL_GUARD_CONFIG_FILES associa um arquivo de hook/settings GLOBAL (por CLI, raiz em $HOME) ao
# CLI que o consome, para as checagens de escopo global abaixo. Distinto de
# _CREDENTIAL_GUARD_HOOK_FILES, cujo path é relativo à raiz do PROJETO, não a $HOME. Lista fechada
# dos 6 arquivos que `trackfw update harness` pode escrever uma entrada de guard -- contraparte
# global de _CREDENTIAL_GUARD_HOOK_FILES. Port de globalGuardConfigFiles
# (internal/validator/validator_git_branch_guard.go).
# requires_command_type (ROADMAP-2026-08-17 ML-4B) espelha _CREDENTIAL_GUARD_HOOK_FILES acima --
# True para todo CLI exceto Cursor.
_GLOBAL_GUARD_CONFIG_FILES = [
    (".claude/settings.json", "Claude Code", True),
    (".codex/hooks.json", "Codex CLI", True),
    (".gemini/settings.json", "Gemini CLI", True),
    (".cursor/hooks.json", "Cursor", False),
    (".copilot/settings.json", "GitHub Copilot CLI", True),
    (".kiro/hooks/trackfw-credential-guard.json", "Kiro", True),
]


def _global_guard_config_path(rel_path: str, cli: str, script_marker: str) -> str:
    """Resolve o caminho em disco (relativo a $HOME) que validate_guard_global_hook_resolvable
    deve ler para um par (entrada de _GLOBAL_GUARD_CONFIG_FILES, script_marker). Port de
    globalGuardConfigPath (internal/validator/validator_git_branch_guard.go) -- ver o doc comment
    daquela função para o racional completo (5 CLIs compartilham um único arquivo baseado em merge
    entre os dois guards; Kiro é a única exceção, com um arquivo dedicado por guard porque seu
    writer reescreve o documento inteiro por vez).

    ROADMAP-2026-08-17 ML-3B: antes desta função existir, _GLOBAL_GUARD_CONFIG_FILES sempre
    apontava o Kiro para trackfw-credential-guard.json para os DOIS guards, então
    git_branch_guard_hook_resolvable nunca inspecionava
    ~/.kiro/hooks/trackfw-git-branch-guard.json.
    """
    if cli == "Kiro" and script_marker == _GIT_BRANCH_GUARD_SCRIPT_MARKER:
        return ".kiro/hooks/trackfw-git-branch-guard.json"
    return rel_path


def validate_guard_global_hook_resolvable(script_marker: str, cwd: str = None) -> list:
    """Contraparte de escopo GLOBAL de validate_guard_hook_resolvable: para cada um dos 6
    _GLOBAL_GUARD_CONFIG_FILES que existir E referenciar script_marker, verifica que o script
    referenciado existe e é executável.

    Port de validateGuardGlobalHookResolvable (internal/validator/validator_git_branch_guard.go) --
    ver o doc comment daquela função para o racional completo (gap principal fechado pelo
    ROADMAP-2026-08-15... ML-1A/ML-2B: antes desta ML, nada em `trackfw validate` inspecionava
    estes 6 arquivos).

    Entradas globais são escritas pelos geradores (harness_credential_guard_target_*,
    generators/update.py) como caminhos absolutos já resolvidos (via
    global_credential_guard_script_path -- os.path.join(home, ".trackfw", "scripts", name)), nunca
    um placeholder como $CLAUDE_PROJECT_DIR -- então, ao contrário de
    _resolve_credential_guard_hook_path (escopo de projeto), nenhum prefixo precisa ser removido
    aqui: qualquer comando casado que NÃO for já um caminho absoluto não é uma forma que o trackfw
    emite e é pulado (nunca tratado como violação -- mesmo contrato "não é nosso wiring, não é
    nosso problema" do ramo ok=False de _resolve_credential_guard_hook_path).

    Fail-open: $HOME não resolvível, arquivo ilegível ou JSON inválido pulam esse arquivo em
    silêncio -- mesmo contrato que validate_guard_hook_resolvable já tem para arquivos de projeto.
    """
    home = home_dir()
    if not home or home == "~":
        return []

    msgs = []
    for base_rel_path, cli, requires_command_type in _GLOBAL_GUARD_CONFIG_FILES:
        rel_path = _global_guard_config_path(base_rel_path, cli, script_marker)
        full_path = os.path.join(home, rel_path)
        try:
            with open(full_path, "r", encoding="utf-8") as f:
                content = f.read()
        except OSError:
            continue

        try:
            parsed = json.loads(content)
        except (json.JSONDecodeError, ValueError):
            continue

        commands = []
        _collect_commands_with_marker(parsed, script_marker, commands)

        seen = set()
        for m in commands:
            seen_key = (m["raw"], m["type_is_command"])
            if seen_key in seen:
                continue
            seen.add(seen_key)

            if not os.path.isabs(m["raw"]):
                continue

            # ROADMAP-2026-08-17 ML-4B: reproduced by hades-tf (ML-4A barrier) -- a global config
            # entry with the correct absolute command but missing/wrong "type" (hand-edited, an
            # older trackfw version, another tool's merge) is silently never executed by the CLI,
            # even though the script itself exists and is fine. Reported instead of the
            # exists/executable checks below, which assume the entry is structurally valid.
            if requires_command_type and not m["type_is_command"]:
                msgs.append({
                    "type": "violation",
                    "message": (
                        f'~/{rel_path} ({cli}, global scope) references {script_marker} resolved '
                        f'to "{m["raw"]}", but the hook entry is missing "type":"command" (or has '
                        f'an invalid type) — {cli} will silently never execute it; run `trackfw '
                        f'update harness` to regenerate it'
                    ),
                })
                continue

            if not os.path.exists(m["raw"]):
                msgs.append({
                    "type": "violation",
                    "message": (
                        f'~/{rel_path} ({cli}, global scope) references {script_marker} resolved '
                        f'to "{m["raw"]}", but the script does not exist — run `trackfw update harness` '
                        f'to regenerate it'
                    ),
                })
            elif _current_platform != "win32" and not os.access(m["raw"], os.X_OK):
                msgs.append({
                    "type": "violation",
                    "message": (
                        f'~/{rel_path} ({cli}, global scope) references {script_marker} resolved '
                        f'to "{m["raw"]}", but the script is not executable — run `trackfw update '
                        f'harness` to regenerate it'
                    ),
                })

    return msgs


def validate_guard_global_script_integrity(script_file_name: str, reference_content: str, cwd: str = None) -> list:
    """Contraparte de escopo GLOBAL de validate_guard_script_integrity: verifica que o conteúdo de
    ~/.trackfw/scripts/<script_file_name> (o local fixo em que os geradores globais escrevem) bate
    byte-a-byte com reference_content.

    Port de validateGuardGlobalScriptIntegrity (internal/validator/validator_git_branch_guard.go).

    ROADMAP-2026-08-17 (guard global cabeado com no-op / integridade independente de fiação),
    ML-3A: dispara pela EXISTÊNCIA do artefato, não por nenhuma entrada em
    _GLOBAL_GUARD_CONFIG_FILES referenciar script_marker -- a condição antiga fazia um script que o
    trackfw escreveu (via `trackfw update harness`) mas que nenhum config ainda apontava poder
    apodrecer indefinidamente com `validate` verde. "Se o trackfw escreveu o script, o trackfw
    verifica o script" -- existência é a única precondição; fiação é irrelevante para saber se o
    artefato em si divergiu.

    Fail-open na ausência: um script que o trackfw nunca escreveu (usuário nunca rodou `update
    harness`) não é erro.

    Avaliação única por script (não uma vez por config referenciando-o): garante no máximo 1
    mensagem, independente de quantos (ou nenhum) configs referenciam o mesmo caminho em disco --
    é o que evita a dupla emissão agora que o git-branch-guard tem fiação global (Wave 2) E
    artefato global simultaneamente.

    script_file_name é o mesmo valor de _CREDENTIAL_GUARD_SCRIPT_MARKER/_GIT_BRANCH_GUARD_SCRIPT_MARKER
    nos dois call sites abaixo -- ambos já são o nome literal do arquivo do script, mesma
    equivalência que o port Go/Node também reaproveita.
    """
    home = home_dir()
    if not home or home == "~":
        return []

    script_path = os.path.join(home, ".trackfw", "scripts", script_file_name)
    try:
        with open(script_path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        # Não instalado não é violação -- mesmo contrato de todo outro check
        # *_script_integrity (projeto e global) neste arquivo.
        return []

    if content == reference_content:
        return []

    return [{
        "type": "warning",
        "message": (
            f'{script_path} (global scope) content diverges from the template this version of '
            f'trackfw generates — if you did not edit this file by hand, run `trackfw update '
            f'harness` to regenerate it'
        ),
    }]


# validate_credential_guard_global_hook_resolvable / validate_credential_guard_global_script_integrity /
# validate_git_branch_guard_global_hook_resolvable / validate_git_branch_guard_global_script_integrity
# are the 4 thin wrappers wired in validate_unfiltered -- each folds its messages into the SAME rule
# name as its project-scope counterpart (credential_guard_hook_resolvable,
# credential_guard_script_integrity, git_branch_guard_hook_resolvable,
# git_branch_guard_script_integrity respectively), so no new "rules:" entries in trackfw.yaml are
# needed. Port of the 4 equivalent Go wrappers (internal/validator/validator_git_branch_guard.go).

def validate_credential_guard_global_hook_resolvable(cwd: str = None) -> list:
    return validate_guard_global_hook_resolvable(_CREDENTIAL_GUARD_SCRIPT_MARKER, cwd)


def validate_credential_guard_global_script_integrity(cwd: str = None) -> list:
    return validate_guard_global_script_integrity(
        _CREDENTIAL_GUARD_SCRIPT_MARKER, _CREDENTIAL_GUARD_GLOBAL_SCRIPT_REFERENCE, cwd
    )


def validate_git_branch_guard_global_hook_resolvable(cwd: str = None) -> list:
    return validate_guard_global_hook_resolvable(_GIT_BRANCH_GUARD_SCRIPT_MARKER, cwd)


def validate_git_branch_guard_global_script_integrity(cwd: str = None) -> list:
    return validate_guard_global_script_integrity(
        _GIT_BRANCH_GUARD_SCRIPT_MARKER, _GIT_BRANCH_GUARD_SCRIPT_REFERENCE, cwd
    )


def validate_credential_guard_mode_downgrade(cwd: str = None) -> list:
    """Regra "credential_guard_mode_downgrade": dispara apenas quando credential_guard.mode era
    explicitamente "block" no HEAD do git e o trackfw.yaml atual em disco não resolve mais para
    "block" (warn explícito, valor não reconhecido, ou chave/arquivo ausente -- todos os quais o
    próprio script resolveria como "warn", o DEFAULT_MODE da variante de projeto).

    Silenciosa sempre que HEAD não é "block": isso é "sem âncora para detectar downgrade", não
    "nada errado". A ausência da chave em DISCO NUNCA é tratada como silêncio -- é exatamente a
    via que esta regra existe para cobrir.
    """
    root = cwd or os.getcwd()
    head_content, ok = _head_trackfw_yaml(root)
    if not ok:
        return []

    head_mode, _ = _extract_credential_guard_mode(head_content)
    if head_mode != "block":
        return []

    disk_path = os.path.join(root, "trackfw.yaml")
    try:
        with open(disk_path, "r", encoding="utf-8") as f:
            disk_content = f.read()
    except FileNotFoundError:
        # trackfw.yaml deletado inteiramente enquanto HEAD tinha mode: block -- é o downgrade.
        return [{"type": "violation", "message": _credential_guard_mode_downgrade_message()}]
    except OSError:
        return [{"type": "violation", "message": _credential_guard_mode_downgrade_message()}]

    disk_mode, _ = _extract_credential_guard_mode(disk_content)
    if disk_mode == "block":
        return []

    return [{"type": "violation", "message": _credential_guard_mode_downgrade_message()}]



def validate_adr_dirs_exist(cfg: dict) -> dict:
    """
    Verifica se todos os diretórios configurados em adr_dirs existem.
    - Se strict_ci_paths for True → gera violation em violations.
    - Se strict_ci_paths for False (default) → gera Warning em warnings.
    """
    violations = []
    warnings = []
    strict_ci = cfg.get("strict_ci_paths", False)
    adr_dirs = [expand_path(d) for d in cfg.get("adr_dirs", ["docs/adr"])]
    for adr_dir in adr_dirs:
        if not os.path.exists(adr_dir) or not os.path.isdir(adr_dir):
            msg = f'adr_dir "{adr_dir}" does not exist'
            item = {
                "type": "violation" if strict_ci else "warning",
                "message": msg,
                "rule": "adr_dir_exists",
                "file": adr_dir,
            }
            if strict_ci:
                violations.append(item)
            else:
                warnings.append(item)
    return {"violations": violations, "warnings": warnings}


# ---------------------------------------------------------------------------
# thirdparty_artifact_has_provenance (ADR-2026-08-15 D2) — ML-3A
# ---------------------------------------------------------------------------

_THIRDPARTY_ORIGIN = "thirdparty"


def validate_thirdparty_artifact_has_provenance(cwd: str = None) -> list:
    """Regra "thirdparty_artifact_has_provenance" (ADR-2026-08-15 D2) — a detecção real, ancorada
    em git, por trás do guardrail TRACKFW_ORCHESTRATOR_SESSION (que D2 é explícito não ser um
    controle de segurança). NUNCA faz fetch de rede (D6) — lê só
    .trackfw/integrations-manifest.json, .trackfw/thirdparty-provenance.json e
    .trackfw/thirdparty-quarantine/<checksum>.json, todos já em disco (e, por convenção deste
    projeto, versionados no repositório).

    Duas ramificações, ambas fatais (error — a regra está deliberadamente ausente de
    _RULE_DEFAULTS):
      1. um artifact do manifest carrega um claim com origin == "thirdparty" mas
         thirdparty-provenance.json não tem entrada chaveada por aquele destino;
      2. existe entrada de proveniência, mas seu installed_sha256 não pode ser reconciliado com o
         que de fato está em disco no destino declarado.

    Ramificação 2 (ADR-2026-08-15 D2-bis, ML-3B) — checksum_sha256 é sha256 dos bytes BRUTOS (D6),
    mas o arquivo instalado é sempre normalize_third_party_content(raw), que não é a função
    identidade em geral, então comparar checksum_sha256 direto contra sha256(arquivo instalado)
    produz falso-positivo em toda instalação legítima cujo conteúdo bruto não fosse já exatamente
    strip+newline único. A resolução ML-3A usava o registro de quarentena como ponte entre os dois
    domínios — correta, mas tornava um artefato de ESTÁGIO (.trackfw/thirdparty-quarantine/,
    destinado a ser podado) dependência obrigatória de um gate PERMANENTE. D2-bis resolve isso com
    um segundo campo na entrada de proveniência, installed_sha256 = sha256(bytes NORMALIZADOS),
    calculado no momento do install pelo mesmo código que grava o arquivo
    (pypi/trackfw/commands/thirdparty.py). checksum_sha256 permanece intocado, é a âncora de
    aprovação D8c. A ramificação 2 agora compara sha256(arquivo instalado) diretamente contra
    entry["installed_sha256"] — dois domínios já normalizados, sem ponte via quarentena. A
    ausência da quarentena deixou de ser erro desta regra."""
    from . import thirdparty as _thirdparty

    root = cwd or os.getcwd()
    manifest_path = os.path.join(root, ".trackfw", "integrations-manifest.json")
    try:
        with open(manifest_path, "r", encoding="utf-8") as fh:
            manifest = json.load(fh)
    except FileNotFoundError:
        return []
    except OSError as error:
        raise RuntimeError(f"thirdparty_artifact_has_provenance: read {manifest_path}: {error}") from error

    destinations = []
    for destination, artifact in (manifest.get("artifacts") or {}).items():
        claims = artifact.get("claims") or []
        if any(claim.get("origin") == _THIRDPARTY_ORIGIN for claim in claims):
            destinations.append(destination)
    if not destinations:
        return []
    destinations.sort()

    try:
        prov = _thirdparty.load_provenance(root)
    except Exception as error:  # noqa: BLE001 - mirrors Go's single wrapped error
        raise RuntimeError(f"thirdparty_artifact_has_provenance: {error}") from error

    msgs = []
    for destination in destinations:
        # Provenance keys are NOT the manifest's absolute destination —
        # verified empirically against the real install command
        # (pypi/trackfw/commands/thirdparty.py): verify_approval/
        # upsert_provenance_entry are called with the project-root-relative
        # (or "~/"-prefixed, global-scope) destination string BEFORE
        # IntegrationManager._resolve() joins it against root to produce the
        # absolute manifest key. Every claim reached here came from the
        # PROJECT manifest, so its scope is always "project" (a
        # global-scope claim lives in the home manifest instead, which this
        # rule intentionally never reads).
        #
        # os.path.relpath inverts _resolve()'s os.path.join(root, relative) for PATH
        # SEMANTICS — but NOT for STRING-KEY matching, which is how this value is actually used
        # two lines below. os.path.relpath returns a path using the native OS separator, while
        # the provenance JSON key is always written with "/" (a key inside a versioned artifact
        # is portable data, not a filesystem path — mirrors
        # internal/validator/validator_thirdparty_provenance.go and the same fix there). On a
        # platform whose native separator is "\", the two would never be byte-equal even though
        # they name the same destination. Normalize before the dict lookup
        # (docs/seguranca/2026-09-01-modelo-de-ameaca-do-separador-em-artefato.md).
        provenance_key = _normalize_ref_separator(os.path.relpath(destination, root))
        entry = (prov.get("entries") or {}).get(provenance_key)
        if not entry:
            msgs.append({
                "type": "violation",
                "message": (
                    f'thirdparty_artifact_has_provenance: "{destination}" is claimed as a third-party artifact but '
                    "has no entry in .trackfw/thirdparty-provenance.json — obtain a favorable hades-tf review and "
                    "record an approved provenance entry for this destination before this can pass validate "
                    "(D2 branch i)"
                ),
            })
            continue

        installed_sha256 = entry.get("installed_sha256", "")
        try:
            with open(destination, "rb") as fh:
                installed = fh.read()
        except OSError as error:
            msgs.append({
                "type": "violation",
                "message": (
                    f'thirdparty_artifact_has_provenance: "{destination}" is claimed as a third-party artifact with '
                    f"an approved provenance entry, but the destination file could not be read ({error})"
                ),
            })
            continue

        if _thirdparty.checksum(installed) != installed_sha256:
            msgs.append({
                "type": "violation",
                "message": (
                    f'thirdparty_artifact_has_provenance: "{destination}" — installed content does not match '
                    f"installed_sha256 {installed_sha256} recorded in .trackfw/thirdparty-provenance.json — the "
                    "artifact was modified after approval or installed outside the fetch/install flow "
                    "(D2 branch ii, D2-bis)"
                ),
            })

    return msgs


# ---------------------------------------------------------------------------
# validate() — ponto de entrada principal
# ---------------------------------------------------------------------------

def validate_unfiltered(cwd: str = None) -> dict:
    """
    Executa todas as validações sem filtro de baseline.
    Retorna {"violations": [...], "warnings": [...]} onde cada item é um dict com "message".
    Usa _apply_rule para distribuir resultados conforme severidade configurada (F3 — v2.4).
    """
    _config.reset()
    cfg = _config.load(cwd)

    violations = []
    warnings = []

    # Verificação de existência de adr_dirs (Warning por padrão, Erro se strict_ci_paths: true)
    dir_check = validate_adr_dirs_exist(cfg)
    violations.extend(dir_check["violations"])
    warnings.extend(dir_check["warnings"])

    # Regras com severidade configurável via cfg["rules"]
    _apply_rule("wip_has_req",          validate_wip_has_req(cfg),                    violations, warnings, cfg)
    _apply_rule("adr_orphan",           validate_adrs_are_referenced(cfg, cwd),       violations, warnings, cfg)
    _apply_rule("wip_acceptance",       validate_wip_has_acceptance_criteria(cfg),    violations, warnings, cfg)
    _apply_rule("blocked_by_draft_adr", validate_reqs_not_blocked_by_draft_adrs(cfg), violations, warnings, cfg)
    _apply_rule("adr_accepted_when_req_done", validate_adr_accepted_when_req_done(cfg), violations, warnings, cfg)
    _apply_rule("filename_uniqueness",  validate_filename_uniqueness(cfg),            violations, warnings, cfg)
    _apply_rule("branch_has_wip_roadmap", validate_branch_has_wip_roadmap(cfg),      violations, warnings, cfg)
    _apply_rule("ref_targets_exist",    validate_ref_targets_exist(cfg),              violations, warnings, cfg)
    warnings += _enrich_items(validate_req_roadmap_lifecycle(cfg), "req_roadmap_lifecycle")
    _apply_rule("folder_status",        validate_folder_status_coherence(cfg),        violations, warnings, cfg)
    _apply_rule("stale_wip",            validate_stale_wip(cfg),                      violations, warnings, cfg)
    _apply_rule("note_orphan",          validate_note_orphan(cfg, cwd),               violations, warnings, cfg)

    # ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
    # desatualizados, ML-2B: soma as mensagens de escopo GLOBAL sob a MESMA regra -- ver os 4
    # wrappers de escopo global definidos acima (validate_credential_guard_global_hook_resolvable
    # etc.), port de internal/validator/validator_git_branch_guard.go.
    _apply_rule(
        "credential_guard_hook_resolvable",
        validate_credential_guard_hook_resolvable(cfg, cwd) + validate_credential_guard_global_hook_resolvable(cwd),
        violations, warnings, cfg, cwd,
    )

    # ROADMAP-2026-08-12-deteccao-de-adulteracao-do-credential-guard-regra-de-validate, ML-1A:
    # detecta adulteração do credential-guard, âncora por alvo (ADR-2026-08-12 Emenda 1).
    _apply_rule(
        "credential_guard_script_integrity",
        validate_credential_guard_script_integrity(cwd) + validate_credential_guard_global_script_integrity(cwd),
        violations, warnings, cfg, cwd,
    )
    _apply_rule("credential_guard_mode_downgrade", validate_credential_guard_mode_downgrade(cwd), violations, warnings, cfg, cwd)

    # ROADMAP-2026-08-15-trackfw-validate-deve-detectar-scripts-de-hook-ausentes-ou-
    # desatualizados, ML-2B: mesma cobertura acima (existência/executabilidade + integridade,
    # projeto e global), generalizada para trackfw-git-branch-guard.sh.
    _apply_rule(
        "git_branch_guard_hook_resolvable",
        validate_git_branch_guard_hook_resolvable(cfg, cwd) + validate_git_branch_guard_global_hook_resolvable(cwd),
        violations, warnings, cfg, cwd,
    )
    _apply_rule(
        "git_branch_guard_script_integrity",
        validate_git_branch_guard_script_integrity(cwd) + validate_git_branch_guard_global_script_integrity(cwd),
        violations, warnings, cfg, cwd,
    )

    # ADR-2026-08-15-gate-de-duas-fases-..., ML-3A (D2): detecção ancorada em git por trás do
    # guardrail TRACKFW_ORCHESTRATOR_SESSION.
    _apply_rule(
        "thirdparty_artifact_has_provenance",
        validate_thirdparty_artifact_has_provenance(cwd),
        violations, warnings, cfg, cwd,
    )

    # Regras com severidade configurável (req_has_adr, blocked_has_req, req_has_roadmap)
    _apply_rule("req_has_adr",     validate_reqs_have_adr(cfg),     violations, warnings, cfg)
    _apply_rule("blocked_has_req", validate_blocked_has_req(cfg),   violations, warnings, cfg)
    _apply_rule("req_has_roadmap", validate_reqs_have_roadmap(cfg), violations, warnings, cfg)
    violations += _enrich_items(validate_frontmatter_presence(cfg),    "frontmatter_presence")

    # wip_limit: violations e warnings já separados internamente
    wip_limit_result = validate_wip_limit(cfg)
    _apply_rule("wip_limit", wip_limit_result["violations"], violations, warnings, cfg)
    warnings += _enrich_items(wip_limit_result["warnings"], "wip_limit")

    # Verificação bidirecional de req_id (desativada se trace_id_field não configurado)
    violations += _enrich_items(check_traceid(cfg), "traceid")

    # ML-2A (REQ-2026-08-29): namespace de agente em disco e não declarado em agents: — violação, não
    # aviso (ver comentário em validate_agent_namespace_undeclared).
    _apply_rule("agent_namespace_undeclared", validate_agent_namespace_undeclared(cfg), violations, warnings, cfg)

    # ML-4A (achado 1, hades-tf 2026-08-30): contraponto de baixo ruído para nomes ocultos/ambíguos
    # (iniciados por ".") — aviso, nunca silêncio total, nunca erro (_RULE_DEFAULTS abaixo).
    _apply_rule("agent_namespace_hidden", hidden_namespace_warnings(cfg), violations, warnings, cfg)

    return {"violations": violations, "warnings": warnings}


def validate(cwd: str = None) -> dict:
    """Executa validações, filtra pelo baseline (ratchet) e aplica modo lenient.

    ADR-2026-08-12-severidade-das-regras-de-credential-guard-resolvida-pela-mais-estrita-entre-
    head-e-disco: carve-out do baseline — violations/warnings de uma das 3
    _CREDENTIAL_GUARD_ANCHORED_RULES NUNCA são toleradas via .trackfw-baseline.json, não importa o
    que o arquivo contenha para elas. Mecanismo DIFERENTE do HEAD-vs-disco em
    _credential_guard_rule_severity: .trackfw-baseline.json é .gitignore'd DE PROPÓSITO ("baseline
    local de violations toleradas (nao versionado)"), então não há HEAD desse arquivo para
    comparar — "exigir commit" simplesmente não se aplica a um arquivo que o projeto decidiu nunca
    versionar. A única forma de fechar esse canal é excluir estas 3 regras da elegibilidade de
    ratchet, por nome, independente do conteúdo da mensagem — daí a checagem por item.get("rule")
    abaixo (populada por _enrich_items em _apply_rule).
    """
    result = validate_unfiltered(cwd)
    violations = result.get("violations", [])
    warnings = result.get("warnings", [])

    # Ratchet: filtrar violations e warnings que já estavam no baseline
    baseline = load_baseline()
    if baseline is not None:
        baseline_set = set(baseline.get("violations", []))
        net_new = [
            v for v in violations
            if _extract_messages([v])[0] not in baseline_set
            or (isinstance(v, dict) and v.get("rule") in _CREDENTIAL_GUARD_ANCHORED_RULES)
        ]
        violations = net_new
        baseline_warn_set = set(baseline.get("warnings", []))
        warnings = [
            w for w in warnings
            if _extract_messages([w])[0] not in baseline_warn_set
            or (isinstance(w, dict) and w.get("rule") in _CREDENTIAL_GUARD_ANCHORED_RULES)
        ]

    # Modo lenient: mover violations para warnings
    if _is_lenient(cwd):
        warnings = warnings + violations
        violations = []

    return {"violations": violations, "warnings": warnings}


# ---------------------------------------------------------------------------
# Aliases e exportações para compatibilidade com o CLI
# ---------------------------------------------------------------------------

validate_single_wip = validate_wip_limit
