#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
check-job-annotations.py — AC4 do
ROADMAP-2026-09-11-o-ciclo-testa-onde-funciona-faltam-cp1252-windows-sem-privilegio-e-consumidor-novo.md

Gate: job com conclusão `success` e ≥1 anotação de nível `failure` (fora da
allowlist de jobs com continue-on-error intencional) → exit 1.

DECLARAÇÃO DE RECONCILIAÇÃO (ML-1D):
  Este gate afirma que job success com annotation_level="failure" (fora da
  allowlist) indica vazamento de ::error:: não intencional — como o #319,
  onde o self-test do ratchet vazava tokens ::error:: para stdout, gerando
  anotações de failure num job green.
  Falsificação: --self-test prova success+failure→exit 1 e success+warning→exit 0.

POLÍTICA DE ALLOWLIST:
  Alguns jobs emitem ::error:: intencionalmente em steps com continue-on-error: true
  — o RATCHET/árbitro final julga o veredito. Listar esses jobs na allowlist abaixo.
  Um job na allowlist com success+failure annotations = cobertura intencional de estado
  conhecido. Modificar a allowlist exige justificativa no commit message.

  Jobs na allowlist (2026-09-11):
    - windows-full-suites: steps de suite têm continue-on-error: true; emitem
      ::error:: para suite-load-failure / zero-test-failure. O ratchet final julga.
    - windows-defect-reproduction: mesmo padrão.
    - parity-falsify-shard: continue-on-error: true no nível de job.

LIMITAÇÃO DECLARADA:
  Este workflow é acionado via `on: workflow_run: [Quality], types: [completed]`.
  Workflows com trigger workflow_run APENAS executam a partir da branch default
  (main). Consequência: este gate não roda em PRs de feature branches — incluindo
  o PR que o adiciona. A limitação é de plataforma (GitHub Actions), não de design.
  Uma vez mergeado à main, o gate passa a rodar em todo push/PR subsequente.

Usage:
  # Self-test (run by `make parity-rest`):
  check-job-annotations.py --self-test

  # Normal check (called by workflow_run job via GitHub API):
  check-job-annotations.py \\
      --job-name <nome-do-job> \\
      --conclusion <success|failure|cancelled|skipped> \\
      --annotations-json <path/to/annotations.json>

  # Annotations JSON format (GitHub check-run annotations API):
  [
    {"annotation_level": "failure", "message": "...", "path": "...", ...},
    {"annotation_level": "warning", "message": "...", ...}
  ]

Exit codes:
  0 — sem violation (job não-success, ou success sem failure annotations, ou job na allowlist)
  1 — job success com ≥1 failure annotation e fora da allowlist
"""
from __future__ import annotations

import argparse
import contextlib
import io
import json
import sys
from pathlib import Path

# ---------------------------------------------------------------------------
# Allowlist de jobs onde ::error:: é intencional via continue-on-error
# (ver POLÍTICA DE ALLOWLIST acima)
# ---------------------------------------------------------------------------
ALLOWLISTED_JOBS: frozenset[str] = frozenset({
    "windows-full-suites",
    "windows-defect-reproduction",
    "parity-falsify-shard",
})

# ---------------------------------------------------------------------------
# Annotation helpers
# ---------------------------------------------------------------------------

_SINK = sys.stdout


@contextlib.contextmanager
def _capture():
    global _SINK
    buf = io.StringIO()
    old = _SINK
    _SINK = buf
    try:
        yield buf
    finally:
        _SINK = old


def _err(msg: str) -> None:
    print(f"::error::{msg}", file=_SINK, flush=True)


def _warn(msg: str) -> None:
    print(f"::warning::{msg}", file=_SINK, flush=True)


# ---------------------------------------------------------------------------
# Core gate logic
# ---------------------------------------------------------------------------

def check_job(job_name: str, conclusion: str, annotations: list[dict]) -> int:
    """Return 0 = OK, 1 = violation.

    Policy:
    - conclusion != "success" → pass through (job already failing or irrelevant)
    - conclusion == "success" AND job in allowlist → pass (intentional annotations)
    - conclusion == "success" AND ≥1 failure annotation AND NOT in allowlist → FAIL
    - warning-only annotations on a success job → pass
    """
    if conclusion != "success":
        return 0

    if job_name in ALLOWLISTED_JOBS:
        print(
            f"[check-job-annotations] {job_name}: success + allowlisted "
            f"(continue-on-error intencional) — pass",
            flush=True,
        )
        return 0

    failure_annotations = [a for a in annotations if a.get("annotation_level") == "failure"]
    warning_annotations = [a for a in annotations if a.get("annotation_level") == "warning"]

    if failure_annotations:
        _err(
            f"AC4: job '{job_name}' conclusion=success mas tem "
            f"{len(failure_annotations)} anotação(ões) de nível 'failure'. "
            f"Issue #319: job verde com ::error:: real indica defeito não detectado pelo gate. "
            f"Primeiro failure: {failure_annotations[0].get('message', '(sem mensagem)')}"
        )
        print(
            f"[check-job-annotations] {job_name}: FAIL — success + "
            f"{len(failure_annotations)} failure annotation(s), "
            f"{len(warning_annotations)} warning(s)",
            flush=True,
        )
        return 1

    print(
        f"[check-job-annotations] {job_name}: OK — success + "
        f"0 failure annotations, {len(warning_annotations)} warning(s)",
        flush=True,
    )
    return 0


# ---------------------------------------------------------------------------
# Self-test
# ---------------------------------------------------------------------------

def run_self_test() -> int:
    print("=== check-job-annotations --self-test ===")
    fail = 0

    # T1: success + failure annotation (NOT in allowlist) → exit 1
    with _capture() as ann:
        rc = check_job(
            job_name="go",
            conclusion="success",
            annotations=[{"annotation_level": "failure", "message": "::error::something"}],
        )
    if rc == 1 and "::error::" in ann.getvalue():
        print("SELF-TEST PASS: T1 — success+failure (non-allowlisted) → exit 1, ::error:: emitido")
    else:
        print(f"SELF-TEST FAIL: T1 — esperava rc=1 e ::error::, obteve rc={rc}, ann='{ann.getvalue()}'")
        fail = 1

    # T2: success + warning-only (NOT in allowlist) → exit 0
    with _capture() as ann:
        rc = check_job(
            job_name="node",
            conclusion="success",
            annotations=[{"annotation_level": "warning", "message": "something"}],
        )
    if rc == 0 and "::error::" not in ann.getvalue():
        print("SELF-TEST PASS: T2 — success+warning-only (non-allowlisted) → exit 0, sem ::error::")
    else:
        print(f"SELF-TEST FAIL: T2 — esperava rc=0 sem ::error::, obteve rc={rc}, ann='{ann.getvalue()}'")
        fail = 1

    # T3: success + failure annotation (IN allowlist) → exit 0
    with _capture() as ann:
        rc = check_job(
            job_name="windows-full-suites",
            conclusion="success",
            annotations=[{"annotation_level": "failure", "message": "suite-load-failure"}],
        )
    if rc == 0:
        print("SELF-TEST PASS: T3 — success+failure (allowlisted windows-full-suites) → exit 0")
    else:
        print(f"SELF-TEST FAIL: T3 — allowlisted job deve passar, obteve rc={rc}")
        fail = 1

    # T4: failure conclusion (any annotations) → exit 0 (job already failing)
    with _capture() as ann:
        rc = check_job(
            job_name="go",
            conclusion="failure",
            annotations=[{"annotation_level": "failure", "message": "real failure"}],
        )
    if rc == 0:
        print("SELF-TEST PASS: T4 — conclusion=failure → exit 0 (job já falhou por si só)")
    else:
        print(f"SELF-TEST FAIL: T4 — conclusion=failure deve passar (job já falhou), obteve rc={rc}")
        fail = 1

    # T5: success + no annotations → exit 0
    with _capture() as ann:
        rc = check_job(job_name="python", conclusion="success", annotations=[])
    if rc == 0:
        print("SELF-TEST PASS: T5 — success + sem anotações → exit 0")
    else:
        print(f"SELF-TEST FAIL: T5 — success sem anotações deve passar, obteve rc={rc}")
        fail = 1

    # T6: skipped conclusion → exit 0
    with _capture() as ann:
        rc = check_job(
            job_name="package-smoke",
            conclusion="skipped",
            annotations=[],
        )
    if rc == 0:
        print("SELF-TEST PASS: T6 — conclusion=skipped → exit 0")
    else:
        print(f"SELF-TEST FAIL: T6 — skipped deve passar, obteve rc={rc}")
        fail = 1

    # T7 (falsificação AC6): success + failure annotations (parity-falsify-shard, allowlisted) → exit 0
    with _capture() as ann:
        rc = check_job(
            job_name="parity-falsify-shard",
            conclusion="success",
            annotations=[
                {"annotation_level": "failure", "message": "shard failure"},
                {"annotation_level": "failure", "message": "another failure"},
            ],
        )
    if rc == 0:
        print("SELF-TEST PASS: T7 — parity-falsify-shard (allowlisted) + 2 failures → exit 0")
    else:
        print(f"SELF-TEST FAIL: T7 — parity-falsify-shard deve ser allowlisted, obteve rc={rc}")
        fail = 1

    print("")
    if fail == 0:
        print("Self-test summary: 7 PASS, 0 FAIL")
        return 0
    else:
        print(f"Self-test summary: {7 - fail} PASS, {fail} FAIL")
        return 1


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------

def main() -> int:
    parser = argparse.ArgumentParser(
        description="Gate: job success com failure annotation → exit 1"
    )
    parser.add_argument("--self-test", action="store_true", help="Rodar auto-teste")
    parser.add_argument("--job-name", help="Nome do job (como aparece no check-run)")
    parser.add_argument(
        "--conclusion",
        help="Conclusão do job (success|failure|cancelled|skipped|timed_out)",
    )
    parser.add_argument(
        "--annotations-json",
        help="Path para JSON com lista de anotações do check-run",
    )
    args = parser.parse_args()

    if args.self_test:
        return run_self_test()

    # Normal mode: require all three args
    if not all([args.job_name, args.conclusion, args.annotations_json]):
        parser.error(
            "Modo normal requer --job-name, --conclusion e --annotations-json. "
            "Use --self-test para o auto-teste."
        )

    path = Path(args.annotations_json)
    if not path.exists():
        _err(f"check-job-annotations: arquivo de anotações não encontrado: {path}")
        return 1

    with path.open(encoding="utf-8") as f:
        annotations = json.load(f)

    if not isinstance(annotations, list):
        _err(
            f"check-job-annotations: formato inválido em {path} — "
            "esperava lista de objetos"
        )
        return 1

    return check_job(
        job_name=args.job_name,
        conclusion=args.conclusion,
        annotations=annotations,
    )


if __name__ == "__main__":
    sys.exit(main())
