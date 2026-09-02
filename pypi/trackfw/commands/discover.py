"""
discover.py — Subcomando CLI 'discover'.
Escaneia o repositorio e detecta a estrutura de governança.
Espelha npm/src/commands/discover.js e internal/discover/discover.go em Python puro.
"""

import os
import shutil
import sys
import stat
import datetime
import subprocess

from trackfw.forge.resolve import resolve_from_repo as _forge_resolve_from_repo


# ---------------------------------------------------------------------------
# helpers de filesystem
# ---------------------------------------------------------------------------

def _is_dir(path: str) -> bool:
    try:
        return os.path.isdir(path)
    except OSError:
        return False


def _is_file(path: str) -> bool:
    try:
        return os.path.isfile(path)
    except OSError:
        return False


def _count_md(directory: str) -> int:
    """Conta recursivamente arquivos .md num diretório."""
    n = 0
    try:
        for root, _, files in os.walk(directory):
            for f in files:
                if f.endswith(".md"):
                    n += 1
    except OSError:
        pass
    return n


def _list_subdirs(directory: str) -> list[str]:
    """Lista subdiretórios imediatos de um diretório."""
    try:
        return [
            e for e in os.listdir(directory)
            if os.path.isdir(os.path.join(directory, e))
        ]
    except OSError:
        return []


def _has_file_with_substring(path: str, sub: str) -> bool:
    """Lê path e retorna True se seu conteúdo contém sub. Retorna False silenciosamente se o
    arquivo não existir ou não puder ser lido (best-effort)."""
    try:
        with open(path, "r", encoding="utf-8") as f:
            content = f.read()
    except OSError:
        return False
    return sub in content


def _has_go_test_file(root_dir: str) -> bool:
    """Percorre root_dir recursivamente procurando qualquer arquivo *_test.go."""
    try:
        for _, _, files in os.walk(root_dir):
            for f in files:
                if f.endswith("_test.go"):
                    return True
    except OSError:
        pass
    return False


def detect_test_framework(root_dir: str) -> str:
    """Heurística best-effort para sugerir um framework de teste com base em arquivos de
    configuração presentes na raiz do projeto. Nunca retorna erro — retorna "" quando nenhum
    arquivo-gatilho é encontrado. Ordem de precedência: jest, vitest, pytest, go test. Espelha
    detectTestFramework (Go) e detectTestFramework (Node).
    """
    if _is_file(os.path.join(root_dir, "jest.config.js")) or _is_file(os.path.join(root_dir, "jest.config.ts")):
        return "jest"
    if _is_file(os.path.join(root_dir, "vitest.config.js")) or _is_file(os.path.join(root_dir, "vitest.config.ts")):
        return "vitest"
    if _is_file(os.path.join(root_dir, "pytest.ini")):
        return "pytest"
    if _has_file_with_substring(os.path.join(root_dir, "pyproject.toml"), "[tool.pytest"):
        return "pytest"
    if _has_file_with_substring(os.path.join(root_dir, "setup.cfg"), "[tool:pytest]"):
        return "pytest"
    if _is_file(os.path.join(root_dir, "go.mod")) and _has_go_test_file(root_dir):
        return "go test"
    return ""


# ---------------------------------------------------------------------------
# scan
# ---------------------------------------------------------------------------

def scan(root_dir: str) -> dict:
    """
    Escaneia root_dir e retorna dict com estrutura de governança detectada.
    Espelha Scan() do Go e scan() do JS.
    """
    r = {
        "adr_dirs": [],
        "req_dir": "",
        "roadmap_dir": "",
        "roadmap_namespacing": "flat",
        "agents": [],
        "adr_count": 0,
        "req_count": 0,
        "roadmap_count": 0,
        "has_trackfw_yaml": False,
        "has_trackfw_log": False,
        "governance_score": 0,
        "hook_framework": "none",
        "ci_system": "none",
        "forge": "",
        # suggested_test_framework é uma sugestão best-effort (nunca erro, "" quando nada
        # bate) baseada em arquivos de configuração presentes na raiz do projeto. É apenas
        # impressa como sugestão pelo comando `discover` — nunca escrita automaticamente em
        # trackfw.yaml (a convenção de agent_conventions deve ser sempre declarada pelo
        # time, não inferida).
        "suggested_test_framework": "",
    }

    # 1. trackfw.yaml
    r["has_trackfw_yaml"] = _is_file(os.path.join(root_dir, "trackfw.yaml"))

    # 2. REQ dir — candidatos em ordem de preferência
    for candidate in ["docs/req", "docs/requisições", "docs/requirements", "docs/reqs"]:
        full = os.path.join(root_dir, candidate)
        if _is_dir(full):
            r["req_dir"] = candidate
            r["req_count"] = _count_md(full)
            break

    # 3. ADR dirs — docs/adr e suas subdirs
    adr_root = os.path.join(root_dir, "docs", "adr")
    if _is_dir(adr_root):
        sub_dirs = _list_subdirs(adr_root)
        if sub_dirs:
            for sub in sub_dirs:
                rel = "docs/adr/" + sub
                r["adr_dirs"].append(rel)
                r["adr_count"] += _count_md(os.path.join(root_dir, rel))
        else:
            r["adr_dirs"] = ["docs/adr"]
            r["adr_count"] = _count_md(adr_root)

    # 4. Roadmap dir e namespacing
    roadmap_root = os.path.join(root_dir, "docs", "roadmaps")
    if _is_dir(roadmap_root):
        r["roadmap_dir"] = "docs/roadmaps"

        agent_dirs = _list_subdirs(roadmap_root)
        by_agent = False
        agents = []
        for sub in agent_dirs:
            wip_dir = os.path.join(roadmap_root, sub, "wip")
            analyzing_dir = os.path.join(roadmap_root, sub, "analyzing")
            backlog_dir = os.path.join(roadmap_root, sub, "backlog")
            done_dir = os.path.join(roadmap_root, sub, "done")
            abandoned_dir = os.path.join(roadmap_root, sub, "abandoned")
            blocked_dir = os.path.join(roadmap_root, sub, "blocked")
            if any(_is_dir(d) for d in [wip_dir, analyzing_dir, backlog_dir, done_dir, abandoned_dir, blocked_dir]):
                by_agent = True
                agents.append(sub)

        if by_agent:
            r["roadmap_namespacing"] = "by_agent"
            r["agents"] = agents
            for agent in agents:
                for state in ["backlog", "analyzing", "wip", "blocked", "done", "abandoned"]:
                    r["roadmap_count"] += _count_md(os.path.join(roadmap_root, agent, state))
        else:
            r["roadmap_namespacing"] = "flat"
            for state in ["backlog", "analyzing", "wip", "blocked", "done", "abandoned"]:
                r["roadmap_count"] += _count_md(os.path.join(roadmap_root, state))

        r["has_trackfw_log"] = _is_file(os.path.join(roadmap_root, ".trackfw-log"))

    # 5. Hook framework
    if _is_file(os.path.join(root_dir, "lefthook.yml")) or _is_file(os.path.join(root_dir, ".lefthook.yml")):
        r["hook_framework"] = "lefthook"
    elif _is_dir(os.path.join(root_dir, ".husky")):
        r["hook_framework"] = "husky"
    elif _is_file(os.path.join(root_dir, ".pre-commit-config.yaml")):
        r["hook_framework"] = "pre-commit"
    else:
        r["hook_framework"] = "none"

    # 6. CI system
    if _is_dir(os.path.join(root_dir, ".github", "workflows")):
        r["ci_system"] = "github-actions"
    elif _is_file(os.path.join(root_dir, ".gitlab-ci.yml")):
        r["ci_system"] = "gitlab"
    else:
        r["ci_system"] = "none"

    # 6b. Suggested test framework — best-effort heuristic, never an error.
    r["suggested_test_framework"] = detect_test_framework(root_dir)

    # 7. Forge detection — reuse forge/resolve.py (no duplicate parse).
    # _git_remote_url returns "" when repo dir is not a git repo or on any error.
    # CI detection is filesystem-based and always works.
    try:
        res = _forge_resolve_from_repo(repo_dir=root_dir)
        if res.source != "none":
            r["forge"] = res.forge
    except Exception:
        pass  # forge detection is best-effort; never block scan on it

    # 8. Score
    r["governance_score"] = _calc_score(r)

    return r


def _calc_score(r: dict) -> int:
    score = 0
    if r["adr_count"] > 0:
        score += 20
    if r["req_count"] > 0:
        score += 20
    if r["roadmap_count"] > 0:
        score += 20
    if r["has_trackfw_yaml"]:
        score += 20
    if r["has_trackfw_log"]:
        score += 20
    return score


# ---------------------------------------------------------------------------
# generate_yaml
# ---------------------------------------------------------------------------

def generate_yaml(result: dict) -> str:
    """Gera conteúdo do trackfw.yaml calibrado para o resultado do scan."""
    lines = [
        "# trackfw configuration — gerado por trackfw discover",
        "# governance_mode: lenient permite validação não-bloqueante durante onboarding",
        "",
        "governance_mode: lenient",
        "",
    ]

    if result["adr_dirs"]:
        lines.append("adr_dirs:")
        for d in result["adr_dirs"]:
            lines.append(f"  - {d}")
    else:
        lines.append("adr_dirs:")
        lines.append("  - docs/adr")

    req_dir = result["req_dir"] or "docs/req"
    lines.append(f"req_dir: {req_dir}")

    roadmap_dir = result["roadmap_dir"] or "docs/roadmaps"
    lines.append(f"roadmap_dir: {roadmap_dir}")

    lines.append(f"roadmap_namespacing: {result['roadmap_namespacing']}")

    if result["agents"]:
        lines.append("agents:")
        for a in result["agents"]:
            lines.append(f"  - {a}")

    lines.append(f"hooks: {result['hook_framework']}")
    lines.append(f"ci: {result['ci_system']}")

    if result.get("forge"):
        lines.append(f"forge: {result['forge']}")

    lines.append("")

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# generate_bootstrap_log
# ---------------------------------------------------------------------------

def generate_bootstrap_log(result: dict, root_dir: str) -> str:
    """
    Gera entradas retroativas para o .trackfw-log baseadas no mtime
    dos arquivos em done/.
    """
    roadmap_root = os.path.join(root_dir, result["roadmap_dir"])
    lines = []

    def append_entries(directory: str, agent: str):
        if not _is_dir(directory):
            return
        try:
            entries = sorted(os.listdir(directory))
        except OSError:
            return
        for fname in entries:
            if not fname.endswith(".md"):
                continue
            fpath = os.path.join(directory, fname)
            try:
                mtime = os.stat(fpath).st_mtime
            except OSError:
                continue
            ts = datetime.datetime.fromtimestamp(mtime).strftime("%Y-%m-%d %H:%M")
            basename = f"{agent}/{fname}" if agent else fname
            lines.append(f"{ts}  {basename:<50}  backlog -> done")

    if result["roadmap_namespacing"] == "by_agent":
        for agent in result["agents"]:
            append_entries(os.path.join(roadmap_root, agent, "done"), agent)
    else:
        append_entries(os.path.join(roadmap_root, "done"), "")

    return "\n".join(lines) + ("\n" if lines else "")


# ---------------------------------------------------------------------------
# install_gates
# ---------------------------------------------------------------------------

def install_gates(result: dict, root_dir: str) -> None:
    """Instala artefatos de governança: validate script, hook entry, CI workflow."""
    _write_validate_script(root_dir)
    _install_hook(result["hook_framework"], root_dir)
    if result["ci_system"] == "github-actions":
        _write_ci_workflow(root_dir)


def _write_validate_script(root_dir: str) -> None:
    # Delegates to the single canonical generator shared with `trackfw init`
    # and `trackfw update`'s `validate-script` target — see
    # trackfw/generators/init_gen.py:generate_validate_script (ML-6H).
    from trackfw.generators.init_gen import generate_validate_script
    generate_validate_script(root_dir)


def _install_hook(framework: str, root_dir: str) -> None:
    hook_entry = "\npre-commit:\n  commands:\n    trackfw-validate:\n      run: scripts/trackfw-validate.sh\n"
    husky_entry = "\nscripts/trackfw-validate.sh\n"

    if framework == "lefthook":
        cfg_path = os.path.join(root_dir, "lefthook.yml")
        if not _is_file(cfg_path):
            cfg_path = os.path.join(root_dir, ".lefthook.yml")
        with open(cfg_path, "r", encoding="utf-8") as f:
            content = f.read()
        if "trackfw" in content:
            return  # idempotente
        with open(cfg_path, "a", encoding="utf-8", newline="\n") as f:
            f.write(hook_entry)
    elif framework == "husky":
        husky_hook = os.path.join(root_dir, ".husky", "pre-commit")
        with open(husky_hook, "a", encoding="utf-8", newline="\n") as f:
            f.write(husky_entry)
    else:
        pkg_json = os.path.join(root_dir, "package.json")
        if os.path.isfile(pkg_json):
            _install_husky(root_dir)
        elif shutil.which("node"):
            _install_husky_npx(root_dir)
        else:
            _install_lefthook(root_dir)


def _install_husky(root_dir: str) -> None:
    """Instala husky via npm e configura o pre-commit hook."""
    try:
        subprocess.run(
            ["npm", "install", "--save-dev", "husky"],
            cwd=root_dir,
            check=False,
        )
        subprocess.run(
            ["npx", "husky", "init"],
            cwd=root_dir,
            check=False,
        )
        husky_dir = os.path.join(root_dir, ".husky")
        os.makedirs(husky_dir, exist_ok=True)
        hook_file = os.path.join(husky_dir, "pre-commit")
        with open(hook_file, "a", encoding="utf-8", newline="\n") as f:
            f.write("\nscripts/trackfw-validate.sh\n")
    except subprocess.CalledProcessError as exc:
        print(f"Aviso: falha ao instalar husky: {exc}")


def _install_husky_npx(root_dir: str) -> None:
    """Instala husky via npx sem precisar de package.json."""
    print("ℹ node detected — using husky via npx (no package.json required)")
    try:
        subprocess.run(["npx", "husky", "init"], cwd=root_dir, check=False)
        print("✓ husky initialized via npx")
    except Exception:
        print("⚠ npx husky init failed — install manually: npx husky init")

    husky_dir = os.path.join(root_dir, ".husky")
    os.makedirs(husky_dir, exist_ok=True)
    hook_path = os.path.join(husky_dir, "pre-commit")
    with open(hook_path, "a", encoding="utf-8", newline="\n") as f:
        f.write("\nscripts/trackfw-validate.sh\n")
    os.chmod(hook_path, 0o755)
    print("✓ trackfw entry added to .husky/pre-commit")


def _install_lefthook(root_dir: str) -> None:
    """Cria lefthook.yml na raiz e tenta executar lefthook install."""
    lefthook_yml = os.path.join(root_dir, "lefthook.yml")

    # idempotência: se já existe e menciona trackfw, não sobrescrever
    if os.path.isfile(lefthook_yml):
        with open(lefthook_yml, "r", encoding="utf-8") as f:
            if "trackfw" in f.read():
                return

    content = (
        "pre-commit:\n"
        "  commands:\n"
        "    trackfw-validate:\n"
        "      run: scripts/trackfw-validate.sh\n"
    )
    with open(lefthook_yml, "w", encoding="utf-8", newline="\n") as f:
        f.write(content)

    try:
        subprocess.run(
            ["lefthook", "install"],
            cwd=root_dir,
            check=False,
        )
    except FileNotFoundError:
        print("Aviso: lefthook não encontrado no PATH — lefthook.yml criado, mas 'lefthook install' foi ignorado")


# DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH is the canonical relative path of the second,
# independent CI workflow trackfw writes — the one `trackfw discover --init` generates
# via install_gates, distinct from GITHUB_ACTIONS_WORKFLOW_PATH (trackfw-gate.yml,
# written by init/update, trackfw.generators.init_gen). Both files can coexist in the
# same project (ADR-2026-08-28). Exported so scaffold_doctor.py can compare against it
# by path.
DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH = ".github/workflows/trackfw-validate.yml"


def build_discover_github_actions_workflow_content() -> str:
    """Returns the content trackfw writes to DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH.

    Mirrors generators.BuildDiscoverGitHubActionsWorkflowContent in
    internal/generators/scaffold_doctor.go (Go, canonical source of truth) and
    npm/src/commands/discover.js's buildDiscoverGitHubActionsWorkflowContent
    byte-for-byte for the same version.

    NOT version-independent (ADR-2026-08-28, REQ-2026-08-28 AC6/AC7): the
    `go install .../cmd/trackfw@vX.Y.Z` step pins the second install mechanism
    (`go install ...@latest`) to trackfw.__version__ — never a literal. Scaffold
    doctor calls this to compare disk content against the current template
    (AC10/AC11).

    Job id is `governance-go-install` (ML-1A, ROADMAP-2026-09-01) — was
    `governance`, colliding with build_github_actions_workflow_content's job
    id (trackfw-gate.yml, generators/init_gen.py). See that function's doc
    comment for the full rationale; named after the install mechanism
    (`go install`), not the workflow file.
    """
    from trackfw import __version__
    return (
        "name: trackfw validate\n"
        "on: [push, pull_request]\n"
        "jobs:\n"
        "  governance-go-install:\n"
        "    runs-on: ubuntu-latest\n"
        "    steps:\n"
        "      - uses: actions/checkout@v4\n"
        "      - uses: actions/setup-go@v5\n"
        "        with:\n"
        '          go-version: "1.22"\n'
        f"      - run: go install github.com/kgsaran/trackfw/cmd/trackfw@v{__version__}\n"
        "      - run: trackfw validate\n"
    )


def _write_ci_workflow(root_dir: str) -> None:
    workflows_dir = os.path.join(root_dir, ".github", "workflows")
    os.makedirs(workflows_dir, exist_ok=True)
    dest = os.path.join(workflows_dir, "trackfw-validate.yml")
    # os.path.islink is checked BEFORE _is_file (os.path.isfile, follows
    # symlinks): a DANGLING symlink at dest resolves to "does not exist"
    # under os.path.isfile, so the idempotency guard below would not fire,
    # and open(dest, "w", newline="\n") would then follow the link and CREATE the
    # workflow template at whatever path outside the project the symlink
    # points to. A symlink here — live or dangling — is treated as "already
    # present" so this function never writes through it; it refuses loudly
    # instead of silently creating a file somewhere the caller never asked
    # for.
    if os.path.islink(dest):
        print(
            f"aviso: {DISCOVER_GITHUB_ACTIONS_WORKFLOW_PATH} é um symlink; "
            "trackfw discover não escreve através de symlinks — arquivo não foi tocado",
            file=sys.stderr,
        )
        return
    if _is_file(dest):
        return  # idempotente
    content = build_discover_github_actions_workflow_content()
    with open(dest, "w", encoding="utf-8", newline="\n") as f:
        f.write(content)


# ---------------------------------------------------------------------------
# handler do comando
# ---------------------------------------------------------------------------

def _cmd_discover(args):
    cwd = os.getcwd()
    print(f"trackfw discover — scanning {cwd}\n")

    r = scan(cwd)

    # ADRs
    if r["adr_count"] > 0:
        dirs_str = ", ".join(r["adr_dirs"])
        print(f"ADRs encontrados:      {r['adr_count']:<4}  ({dirs_str})")
    else:
        print("Aviso: nenhum ADR encontrado")

    # REQs
    if r["req_count"] > 0:
        print(f"REQs encontrados:      {r['req_count']:<4}  ({r['req_dir']})")
    else:
        print("Aviso: nenhum REQ encontrado")

    # Roadmaps
    if r["roadmap_count"] > 0:
        mode = "by_agent mode" if r["roadmap_namespacing"] == "by_agent" else r["roadmap_namespacing"]
        print(f"Roadmaps encontrados:  {r['roadmap_count']:<4}  ({r['roadmap_dir']} — {mode})")
    else:
        print("Aviso: nenhum roadmap encontrado")

    # Agents
    if r["agents"]:
        print(f"Agentes detectados: {', '.join(r['agents'])}")

    # trackfw.yaml
    if not r["has_trackfw_yaml"]:
        print("Aviso: trackfw.yaml nao encontrado — execute com --init para gerar")
    else:
        print("trackfw.yaml encontrado")

    # .trackfw-log
    if not r["has_trackfw_log"]:
        print("Aviso: .trackfw-log nao encontrado — execute com --bootstrap-log para criar historico retroativo")
    else:
        print(".trackfw-log encontrado")

    # Hooks
    if r["hook_framework"] != "none":
        print(f"Hooks: {r['hook_framework']}")
    else:
        print("Aviso: nenhum hook framework detectado")

    # CI
    if r["ci_system"] != "none":
        print(f"CI: {r['ci_system']}")
    else:
        print("Aviso: nenhum sistema de CI detectado")

    # suggested test framework — printed only, never written to trackfw.yaml
    # automatically (agent_conventions must always be declared by the team).
    if r["suggested_test_framework"]:
        print(f"Suggested test framework: {r['suggested_test_framework']} (add to trackfw.yaml as agent_conventions: if correct)")

    print(f"\nGovernance Score: {r['governance_score']}/100")

    if getattr(args, "init", False):
        yaml_path = os.path.join(cwd, "trackfw.yaml")
        if _is_file(yaml_path):
            print("\nAviso: trackfw.yaml ja existe — remova-o primeiro se quiser regenerar")
            return
        yaml_content = generate_yaml(r)
        with open(yaml_path, "w", encoding="utf-8", newline="\n") as f:
            f.write(yaml_content)
        print("\ntrackfw.yaml gerado")
        try:
            install_gates(r, cwd)
            print("Gates de governança instalados")
        except Exception as e:
            print(f"Aviso: instalacao parcial dos gates: {e}")
        try:
            from trackfw.generators.init_gen import inject_rules_detected
            inject_rules_detected(cwd)
            print("Regras trackfw injetadas nos arquivos de agentes")
        except Exception as e:
            print(f"Aviso: injecao parcial de regras de agentes: {e}")
        try:
            from trackfw.generators.init_gen import _generate_attention_scripts
            _generate_attention_scripts(cwd)
        except Exception as e:
            print(f"  ⚠ attention scripts: {e}")
        try:
            from trackfw.generators.init_gen import _generate_credential_guard_script
            _generate_credential_guard_script(cwd)
        except Exception as e:
            print(f"  ⚠ credential guard script: {e}")
        try:
            from trackfw.generators.init_gen import _generate_git_branch_guard_script
            _generate_git_branch_guard_script(cwd)
        except Exception as e:
            print(f"  ⚠ git branch guard script: {e}")
        try:
            from trackfw.generators.hooks import inject_hooks_detected
            inject_hooks_detected(cwd)
            print('  ✓ agent hooks atualizados')
        except Exception as e:
            print(f'  ⚠ agent hooks: {e}')
        try:
            from trackfw.generators.init_gen import generate_claude_commands
            generate_claude_commands(cwd)
        except Exception as e:
            print(f"Aviso: instalacao parcial dos slash commands: {e}")
        try:
            from trackfw.generators.init_gen import print_architect_next_steps
            print_architect_next_steps(cwd)
        except Exception:
            pass

    if getattr(args, "bootstrap_log", False):
        if not r["roadmap_dir"]:
            print("Aviso: nenhum diretório de roadmap detectado — impossível criar bootstrap log", file=sys.stderr)
            return
        log_path = os.path.join(cwd, r["roadmap_dir"], ".trackfw-log")
        os.makedirs(os.path.dirname(log_path), exist_ok=True)

        # ler entradas já existentes para dedup (chave: linha trimada)
        existing: set[str] = set()
        if _is_file(log_path):
            try:
                with open(log_path, encoding="utf-8") as f:
                    for line in f:
                        stripped = line.rstrip("\n")
                        if stripped.strip():
                            existing.add(stripped.strip())
            except OSError:
                pass

        log_content = generate_bootstrap_log(r, cwd)
        written = 0
        with open(log_path, "a", encoding="utf-8", newline="\n") as f:
            for line in log_content.splitlines():
                if not line.strip():
                    continue
                if line.strip() in existing:
                    continue
                f.write(line + "\n")
                written += 1

        print(f"Bootstrap log gravado em {log_path} ({written} novas entradas)")


# ---------------------------------------------------------------------------
# registro no argparse
# ---------------------------------------------------------------------------

def register(subparsers):
    """Registra o comando 'discover' no argparse."""
    p = subparsers.add_parser(
        "discover",
        help="Escaneia o repositório e detecta a estrutura de governança",
    )
    p.add_argument(
        "--init",
        action="store_true",
        default=False,
        help="Gera trackfw.yaml calibrado para este projeto",
    )
    p.add_argument(
        "--bootstrap-log",
        dest="bootstrap_log",
        action="store_true",
        default=False,
        help="Cria .trackfw-log retroativo a partir dos arquivos em done/",
    )
    p.set_defaults(func=_cmd_discover)
