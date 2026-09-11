#!/usr/bin/env python3
"""
check-required-status-checks.py — verifica que os required_status_checks da
proteção da branch main concordam com a declaração local (D) e com os checks
que os workflows podem realmente emitir (W).

Três conjuntos:
  D = .github/required-status-checks.txt      (declaração — o contrato local)
  R = required_status_checks.contexts da API  (o que bloqueia merge no GitHub)
  W = check names derivados de .github/workflows/*.yml (o que o CI pode emitir)

Escopos de verificação (--scope):
  full (padrão) — D\\R, R\\W, D\\W.
                  Requer credencial de mantenedor (gh auth login, repo scope).
                  Falha fatal se R não for legível: 'não consegui procurar' é fatal.
                  Uso: localmente via 'make check-required-full', e na rotina de release.
  dw             — apenas D\\W (não consulta a API).
                  Não requer token. Declara explicitamente que D\\R e R\\W não foram
                  verificados e o motivo. Uso: CI (GITHUB_TOKEN não tem permissão de
                  administrador para ler /branches/main/protection).

Fundamento da divisão de escopo (medido 2026-09-11, run CI 34605163678, PR #317):
  gh api repos/kgsaran/trackfw/branches/main/protection
  → 404 "Resource not found" em CI (exit 2, GITHUB_TOKEN)
  → 200 com token pessoal de KG (scope 'repo')
  O GITHUB_TOKEN não tem permissão para ler a proteção da branch. Enquanto CI
  precisasse de R, o job ficaria permanentemente vermelho em todo PR — exatamente
  o ruído que o issue #275 descreve. Separar os escopos elimina o trade-off:
  CI verifica D\\W (sem token), o mantenedor verifica D/R/W localmente antes de
  cada release, onde a credencial existe por definição.

Três verificações (D\\R e R\\W dependem de R; D\\W não):
  D \\ R → job declarado bloqueante ausente dos required (defeito ML-4A)
  R \\ W → required aponta check que nenhum workflow emite (PR pendente para sempre)
  D \\ W → declared aponta check inexistente em qualquer workflow (declaração fantasma)

Limitação declarada de W:
  W é construído a partir de TODOS os jobs em TODOS os workflows sem filtrar por
  trigger (on:) ou por if: no nível de job. Consequência: um check required cujo
  job está num workflow que NUNCA dispara em pull_request (ex: release.yml) satisfaz
  R \\ W (R\\W = ∅) mas ainda deixa o PR pendente para sempre — o mesmo defeito que
  R\\W existe para pegar. Estado atual: os 10 checks required pertencem a jobs que
  disparam em pull_request; a limitação não afeta o gate hoje. Se um check for
  adicionado a required mas seu workflow não disparar em PR, o gate não o detectará —
  este risco está declarado aqui e no roadmap ML-4B.

Decisão 1 — onde mora a declaração:
  Arquivo próprio .github/required-status-checks.txt (um nome por linha).
  Razão: contrato específico deste repositório, não config do produto CLI trackfw.
  Separado do workflow YAML para tornar edições divergentes visíveis no diff sem
  exigir parsing de YAML na revisão; trivial de auditar e de testar sinteticamente.

Decisão 2 — permissões e escopos (medido 2026-09-11):

  FATO 1: 'administration: read' não é escopo válido de permissions: em workflow GitHub Actions.
    Confirmado: actionlint .github/workflows/quality.yml → linha 1069:
    "unknown permission scope 'administration'. all available permission scopes are
    'actions', 'artifact-metadata', 'attestations', 'checks', 'contents', ..."
    Declarar esse escopo faz o GitHub rejeitar o schema do workflow inteiro (0 jobs
    criados), não o step individual — o defeito pior possível para checks required.

  FATO 2: o endpoint retorna 401 para chamadas anônimas (medido 2026-09-11):
    curl -s -w '\\nHTTP %{http_code}\\n' https://api.github.com/repos/kgsaran/trackfw/branches/main/protection
    → {"message": "Requires authentication", "status": "401"}
    Controle: /branches/main anônimo → HTTP 200 (subrecurso protege mais que o recurso pai).

  FATO 3: com token pessoal de KG (scope 'repo') → HTTP 200. Com GITHUB_TOKEN em CI → 404.
    gh api repos/kgsaran/trackfw/branches/main/protection -i | head -1  → HTTP/2.0 200 OK
    CI run 34605163678 (PR #317): GITHUB_TOKEN → "Resource not found" (404, exit 2)

  Consequência: CI não pode verificar R. Solução: escopo dw para CI (sem token),
  verificação completa D/R/W localmente via 'make check-required-full' antes de cada release,
  onde o mantenedor tem credencial por definição.

Decisão 3 — direção da verificação:
  AMBAS as direções reprovam com exit 1 nomeando o(s) check(s) divergente(s).
  D\\R: o defeito de ontem (ML-4A) — job declarado mas não required.
  R\\W: check required que o CI nunca emitirá — PR fica pendente para sempre
        (vault/notes/matriz-em-job-required-por-nome-fica-pendente-para-sempre-
        2026-09-08.md).
  D\\W: declaração fantasma — job declarado que nenhum workflow define.

Códigos de saída:
  0 = todos os conjuntos verificados concordam
  1 = divergência encontrada (D\\R, R\\W, e/ou D\\W) — nomes listados
  2 = não foi possível executar a verificação (instrumento falhou)

Variáveis de ambiente para self-test (substituem fontes reais):
  SELFTEST_DECLARED           — lista D simulada (newline-separated)
  SELFTEST_REQUIRED           — lista R simulada (newline-separated)
  SELFTEST_REQUIRED_FAIL=1    — simula falha do gh api
  SELFTEST_WORKFLOW_CHECKS    — conjunto W simulado (newline-separated)
"""

import argparse
import itertools
import os
import subprocess
import sys
import tempfile
from pathlib import Path

try:
    import yaml
except ImportError:
    print("::error::PyYAML não encontrado — instale com: pip install PyYAML", file=sys.stderr)
    sys.exit(2)

ROOT = Path(os.environ.get("ROOT", "."))
WORKFLOWS_DIR = ROOT / ".github" / "workflows"
DECLARED_FILE = ROOT / ".github" / "required-status-checks.txt"
REPO = os.environ.get("REPO", "kgsaran/trackfw")
BRANCH = os.environ.get("BRANCH", "main")


# ── Helpers ──────────────────────────────────────────────────────────────────

def emit_error(msg: str) -> None:
    print(f"::error::{msg}", file=sys.stderr)


def emit_warn(msg: str) -> None:
    print(f"::warning::{msg}", file=sys.stderr)


def emit_ok(msg: str) -> None:
    print(f"check-required-status-checks: [OK] {msg}")


# ── Load declared (D) ─────────────────────────────────────────────────────────

def load_declared() -> list[str]:
    """Return the declared required checks from .github/required-status-checks.txt."""
    raw = os.environ.get("SELFTEST_DECLARED", "")
    if raw:
        items = [l.strip() for l in raw.splitlines() if l.strip() and not l.strip().startswith("#")]
        return items

    if not DECLARED_FILE.exists():
        emit_error(f"arquivo de declaração ausente: {DECLARED_FILE}")
        emit_error("Crie .github/required-status-checks.txt com um nome de check por linha.")
        sys.exit(2)

    lines = DECLARED_FILE.read_text(encoding="utf-8").splitlines()
    items = [l.strip() for l in lines if l.strip() and not l.strip().startswith("#")]

    if not items:
        emit_error(f"{DECLARED_FILE} está vazio — guarda de vacuidade disparada.")
        emit_error("Um arquivo vazio aprovaria qualquer configuração, o que seria falso.")
        sys.exit(2)

    return items


# ── Load live required (R) ────────────────────────────────────────────────────

def load_live_required() -> list[str]:
    """Return the live required_status_checks.contexts via gh api.

    Fatal (exit 2) if R is not readable — 'não consegui procurar' is fatal here
    because this function is called only in full scope, where the caller (local
    maintainer) is expected to have admin credentials.
    """
    raw = os.environ.get("SELFTEST_REQUIRED", "")
    if raw:
        return [l.strip() for l in raw.splitlines() if l.strip()]

    if os.environ.get("SELFTEST_REQUIRED_FAIL"):
        emit_error("SELFTEST: simulando falha ao ler proteção da branch.")
        emit_error("Causa: 'gh api' falhou — 'não consegui procurar' é fatal (exit 2).")
        sys.exit(2)

    # Chamada real
    try:
        result = subprocess.run(
            [
                "gh", "api",
                f"repos/{REPO}/branches/{BRANCH}/protection",
                "--jq", ".required_status_checks.contexts[]",
            ],
            capture_output=True,
            text=True,
        )
    except FileNotFoundError:
        emit_error("gh CLI não encontrado — instale github.com/cli/cli")
        sys.exit(2)

    if result.returncode != 0:
        emit_error(
            f"falha ao ler proteção da branch '{BRANCH}' em '{REPO}'."
        )
        emit_error(f"Saída raw do gh (stderr): {result.stderr.strip()!r}")
        emit_error(
            "Este gate (--scope full) requer credencial de mantenedor com 'repo' scope. "
            "Execute: gh auth login — e confirme com: "
            f"gh api repos/{REPO}/branches/{BRANCH}/protection --jq '.required_status_checks.contexts'"
        )
        emit_error(
            "Regra: 'não consegui procurar' é fatal — "
            "vault/notes/guarda-que-reporta-ausencia-precisa-distinguir-"
            "nao-achei-de-nao-consegui-procurar-2026-09-10.md"
        )
        sys.exit(2)

    return [l.strip() for l in result.stdout.splitlines() if l.strip()]


# ── Load workflow check names (W) ─────────────────────────────────────────────

def expand_matrix_names(job_id: str, matrix: dict) -> list[str]:
    """Expand a matrix job into the check names GitHub would report."""
    # Only list-valued keys produce matrix entries; include/exclude are special
    dims = {
        k: v for k, v in matrix.items()
        if isinstance(v, list) and k not in ("include", "exclude")
    }
    if not dims:
        return [job_id]

    dim_names = sorted(dims.keys())
    dim_values = [dims[k] for k in dim_names]

    names = []
    for combo in itertools.product(*dim_values):
        if len(combo) == 1:
            names.append(f"{job_id} ({combo[0]})")
        else:
            names.append(f"{job_id} ({', '.join(str(c) for c in combo)})")
    return names


def load_workflow_checks() -> set[str]:
    """Parse .github/workflows/*.yml and return all check names CI can emit (W)."""
    raw = os.environ.get("SELFTEST_WORKFLOW_CHECKS", "")
    if raw:
        return {l.strip() for l in raw.splitlines() if l.strip()}

    check_names: set[str] = set()

    if not WORKFLOWS_DIR.exists():
        emit_error(f"diretório de workflows ausente: {WORKFLOWS_DIR}")
        sys.exit(2)

    wf_files = sorted(set(WORKFLOWS_DIR.glob("*.yml")) | set(WORKFLOWS_DIR.glob("*.yaml")))
    if not wf_files:
        emit_error(f"nenhum arquivo .yml/.yaml encontrado em {WORKFLOWS_DIR}")
        sys.exit(2)

    for wf_path in wf_files:
        try:
            data = yaml.safe_load(wf_path.read_text(encoding="utf-8"))
        except yaml.YAMLError as exc:
            emit_error(f"falha ao parsear {wf_path.name}: {exc}")
            sys.exit(2)

        if not isinstance(data, dict) or "jobs" not in data:
            continue

        for job_id, job in data["jobs"].items():
            if not isinstance(job, dict):
                continue
            strategy = job.get("strategy") or {}
            matrix = strategy.get("matrix") or {}
            names = expand_matrix_names(str(job_id), matrix)
            check_names.update(names)

    return check_names


# ── Self-test ─────────────────────────────────────────────────────────────────

def self_test() -> None:
    """Run falsification arms in-process using env-var fixtures."""
    script = str(Path(__file__).resolve())
    pass_count = 0
    fail_count = 0

    def assert_arm(
        label: str,
        expected_exit: int,
        expected_in_output: str | None = None,
        extra_args: list[str] | None = None,
        **env_overrides: str,
    ) -> None:
        nonlocal pass_count, fail_count
        env = dict(os.environ)
        # Remove any real data; fixtures override
        for key in (
            "SELFTEST_DECLARED", "SELFTEST_REQUIRED",
            "SELFTEST_REQUIRED_FAIL", "SELFTEST_WORKFLOW_CHECKS",
        ):
            env.pop(key, None)
        env["ROOT"] = str(ROOT)
        env.update(env_overrides)

        cmd = [sys.executable, script] + (extra_args or [])
        result = subprocess.run(cmd, capture_output=True, text=True, env=env)

        ok = True
        combined = result.stdout + result.stderr

        if result.returncode != expected_exit:
            print(
                f"FAIL self-test: {label} — "
                f"esperava exit {expected_exit}, saiu {result.returncode}",
                file=sys.stderr,
            )
            if combined:
                print(f"  saída: {combined[:300]}", file=sys.stderr)
            ok = False

        if expected_in_output and expected_in_output not in combined:
            print(
                f"FAIL self-test: {label} — "
                f"esperava '{expected_in_output}' na saída, não encontrado",
                file=sys.stderr,
            )
            if combined:
                print(f"  saída: {combined[:300]}", file=sys.stderr)
            ok = False

        if ok:
            print(f"OK self-test: {label}")
            pass_count += 1
        else:
            fail_count += 1

    # ── Scope full: T1-T6 (comportamento original) ────────────────────────────

    # T1 — contra-braço obrigatório: D==R e todos em W → exit 0
    # Afirma: quando declared, required e workflow checks concordam, o gate passa.
    # Sem este braço, um gate que sempre reprova pareceria funcionar.
    assert_arm(
        "T1: D==R, todos em W → exit 0 (contra-braço obrigatório)",
        expected_exit=0,
        SELFTEST_DECLARED="go\nnode\nparity",
        SELFTEST_REQUIRED="go\nnode\nparity",
        SELFTEST_WORKFLOW_CHECKS="go\nnode\nparity",
    )

    # T2 — D\\R: job declarado bloqueante ausente dos required → exit 1 nomeando-o
    # Afirma: D\\R ≠ ∅ causa falha nomeando o check ausente (defeito ML-4A).
    assert_arm(
        "T2: 'parity' em D mas não em R → exit 1 nomeando 'parity'",
        expected_exit=1,
        expected_in_output="parity",
        SELFTEST_DECLARED="go\nnode\nparity",
        SELFTEST_REQUIRED="go\nnode",
        SELFTEST_WORKFLOW_CHECKS="go\nnode\nparity",
    )

    # T3 — R\\W: required aponta check que nenhum workflow emite → exit 1 nomeando-o
    # Afirma: R\\W ≠ ∅ causa falha nomeando o check fantasma (PR pendente para sempre).
    assert_arm(
        "T3: 'ghost-job' em R mas não em W → exit 1 nomeando 'ghost-job'",
        expected_exit=1,
        expected_in_output="ghost-job",
        SELFTEST_DECLARED="go",
        SELFTEST_REQUIRED="go\nghost-job",
        SELFTEST_WORKFLOW_CHECKS="go",
    )

    # T4 — falha na leitura → exit 2 com ::error:: e nunca silencioso
    # Afirma: quando o instrumento falha (SELFTEST_REQUIRED_FAIL simula gh api error),
    # o gate sai com exit 2 e emite ::error:: — "não consegui procurar" nunca passa.
    assert_arm(
        "T4: SELFTEST_REQUIRED_FAIL → exit 2 com ::error:: (nunca silencioso)",
        expected_exit=2,
        expected_in_output="::error::",
        SELFTEST_REQUIRED_FAIL="1",
        SELFTEST_DECLARED="go\nnode",
        SELFTEST_WORKFLOW_CHECKS="go\nnode",
    )

    # T5 — arquivo de declaração ausente → exit 2 (guarda de vacuidade: arquivo inexistente)
    # Afirma: quando .github/required-status-checks.txt não existe, o gate sai com
    # exit 2 — um arquivo ausente aprovaria qualquer configuração, o que seria falso.
    with tempfile.TemporaryDirectory() as td:
        assert_arm(
            "T5: arquivo de declaração ausente → exit 2 (guarda de vacuidade)",
            expected_exit=2,
            expected_in_output="::error::",
            ROOT=td,
            SELFTEST_REQUIRED="go\nnode",
            SELFTEST_WORKFLOW_CHECKS="go\nnode",
        )

    # T6 — arquivo de declaração vazio → exit 2 (guarda de vacuidade: arquivo vazio)
    # Afirma: quando .github/required-status-checks.txt existe mas está vazio, o gate
    # sai com exit 2 — um arquivo vazio aprovaria qualquer configuração, o que seria falso.
    with tempfile.TemporaryDirectory() as td:
        td_path = Path(td)
        (td_path / ".github").mkdir()
        (td_path / ".github" / "required-status-checks.txt").write_text("")
        assert_arm(
            "T6: arquivo de declaração vazio → exit 2 (guarda de vacuidade)",
            expected_exit=2,
            expected_in_output="::error::",
            ROOT=td,
            SELFTEST_REQUIRED="go\nnode",
            SELFTEST_WORKFLOW_CHECKS="go\nnode",
        )

    # ── Scope dw: T7-T10 ─────────────────────────────────────────────────────

    # T7 — --scope dw + SELFTEST_REQUIRED_FAIL=1 → exit 0
    # Afirma: em scope dw, falha no token (run 34605163678: GITHUB_TOKEN retorna 404)
    # não causa vermelho permanente em CI — o instrumento (R) não é invocado.
    # Reconciliação com medição: run 34605163678 provou que GITHUB_TOKEN não consegue
    # ler /branches/main/protection; este braço afirma que, com --scope dw, essa
    # limitação não impede o gate de passar — o escopo reduzido é a resposta à medição.
    assert_arm(
        "T7: --scope dw + token indisponível (SELFTEST_REQUIRED_FAIL) → exit 0",
        expected_exit=0,
        extra_args=["--scope", "dw"],
        SELFTEST_REQUIRED_FAIL="1",
        SELFTEST_DECLARED="go\nnode",
        SELFTEST_WORKFLOW_CHECKS="go\nnode",
    )

    # T8 — --scope dw + D\\R divergente → exit 0 (R não consultado em scope dw)
    # Afirma: D\\R não é verificado em scope dw — o flag restringe o escopo,
    # não é cosmético.
    assert_arm(
        "T8: --scope dw, D\\R divergente → exit 0 (R ignorado em scope dw)",
        expected_exit=0,
        extra_args=["--scope", "dw"],
        SELFTEST_DECLARED="go\nnode\nparity",
        SELFTEST_REQUIRED="go\nnode",    # parity ausente de R — mas R não é consultado
        SELFTEST_WORKFLOW_CHECKS="go\nnode\nparity",
    )

    # T9 — --scope dw + D\\W divergente → exit 1 (D\\W ainda funciona em scope dw)
    # Afirma: scope dw ainda verifica D\\W — o flag reduz, não desabilita todo o gate.
    assert_arm(
        "T9: --scope dw, D\\W divergente → exit 1 (D\\W funciona em scope dw)",
        expected_exit=1,
        extra_args=["--scope", "dw"],
        SELFTEST_DECLARED="go\nnode\nghost-declared",
        SELFTEST_REQUIRED="go\nnode",
        SELFTEST_WORKFLOW_CHECKS="go\nnode",    # ghost-declared ausente de W
    )

    # T10 — --scope dw + D==W → exit 0 e declara o que não verificou (contra-braço de T9)
    # Afirma: quando D e W concordam, scope dw passa — e a saída inclui a declaração
    # de que D\\R e R\\W não foram verificados. Sem este braço, T9 poderia ser satisfeito
    # por um gate que sempre reprova em scope dw.
    assert_arm(
        "T10: --scope dw, D==W → exit 0, declara R não verificado (contra-braço de T9)",
        expected_exit=0,
        expected_in_output="R não verificado",
        extra_args=["--scope", "dw"],
        SELFTEST_DECLARED="go\nnode",
        SELFTEST_REQUIRED="go\nnode",
        SELFTEST_WORKFLOW_CHECKS="go\nnode",
    )

    print("---")
    print(f"Self-test summary: {pass_count} PASS, {fail_count} FAIL")
    sys.exit(0 if fail_count == 0 else 1)


# ── Main ──────────────────────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(
        description="Verifica concordância entre required_status_checks, declaração e workflow checks."
    )
    parser.add_argument(
        "--self-test",
        action="store_true",
        help="Executa falsificação interna e encerra.",
    )
    parser.add_argument(
        "--scope",
        choices=["full", "dw"],
        default="full",
        help=(
            "full (padrão): verifica D\\R, R\\W e D\\W — requer credencial de mantenedor. "
            "dw: verifica apenas D\\W — sem chamada à API, adequado para CI."
        ),
    )
    args = parser.parse_args()

    if args.self_test:
        self_test()
        return  # unreachable, self_test calls sys.exit

    declared = load_declared()
    workflow_checks = load_workflow_checks()
    declared_set = set(declared)

    if args.scope == "dw":
        # D\W only — não consulta R
        phantom_in_declared = sorted(declared_set - workflow_checks)
        if phantom_in_declared:
            emit_error(
                "check(s) em .github/required-status-checks.txt que NENHUM workflow define:"
            )
            for name in phantom_in_declared:
                emit_error(f"  - '{name}'")
            emit_error(
                "Remova de .github/required-status-checks.txt ou adicione o job ao workflow."
            )
            emit_error(
                "FALHOU [scope=dw] — D\\W ≠ ∅. "
                "R não verificado (GITHUB_TOKEN sem permissão de administrador em CI): "
                "D\\R e R\\W não foram verificados nesta execução. "
                "Use 'make check-required-full' para verificação completa (requer credencial de mantenedor)."
            )
            sys.exit(1)

        emit_ok(
            f"[scope=dw] declared={len(declared_set)}, workflow_checks={len(workflow_checks)} — "
            "D\\\\W=∅. "
            "R não verificado (GITHUB_TOKEN sem permissão de administrador em CI): "
            "D\\\\R e R\\\\W não foram verificados nesta execução. "
            "Use 'make check-required-full' para verificação completa (requer credencial de mantenedor)."
        )
        return

    # Full scope: D/R/W
    live_required = load_live_required()
    required_set = set(live_required)

    fail = False

    # D \ R — job declarado bloqueante ausente dos required
    missing_from_required = sorted(declared_set - required_set)
    if missing_from_required:
        emit_error(
            "job(s) declarado(s) em .github/required-status-checks.txt "
            "AUSENTE(S) de required_status_checks:"
        )
        for name in missing_from_required:
            emit_error(f"  - '{name}'")
        emit_error(
            "Adicione ao required_status_checks em Settings > Branches > main "
            "(ou via gh api). Referência: ML-4A deste roadmap."
        )
        fail = True

    # R \ W — required aponta check que nenhum workflow pode emitir
    phantom_in_required = sorted(required_set - workflow_checks)
    if phantom_in_required:
        emit_error(
            "required_status_checks contém check(s) que NENHUM workflow emite:"
        )
        for name in phantom_in_required:
            emit_error(f"  - '{name}'")
        emit_error(
            "Um check required sem job correspondente fica pendente para sempre, "
            "bloqueando todos os PRs. "
            "Remova do required_status_checks ou adicione o job ao workflow. "
            "Referência: vault/notes/matriz-em-job-required-por-nome-fica-"
            "pendente-para-sempre-2026-09-08.md"
        )
        fail = True

    # D \ W — check declarado que nenhum workflow define (declaração fantasma)
    phantom_in_declared = sorted(declared_set - workflow_checks)
    if phantom_in_declared:
        emit_error(
            "check(s) em .github/required-status-checks.txt que NENHUM workflow define:"
        )
        for name in phantom_in_declared:
            emit_error(f"  - '{name}'")
        emit_error(
            "Remova de .github/required-status-checks.txt ou adicione o job ao workflow."
        )
        fail = True

    if fail:
        emit_error(
            "FALHOU [scope=full] — divergência entre declared (.github/required-status-checks.txt), "
            "required_status_checks (API) e workflow checks (.github/workflows/*.yml)."
        )
        sys.exit(1)

    emit_ok(
        f"[scope=full] declared={len(declared_set)}, required={len(required_set)}, "
        f"workflow_checks={len(workflow_checks)} — "
        "D\\\\R=∅, R\\\\W=∅, D\\\\W=∅ — todos os conjuntos concordam."
    )


if __name__ == "__main__":
    main()
