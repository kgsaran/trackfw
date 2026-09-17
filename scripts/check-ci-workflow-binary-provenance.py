#!/usr/bin/env python3
"""
check-ci-workflow-binary-provenance.py — AC5 gate (ML-1D corretivo do ML-1C,
ML-1C corretivo do ML-1B, REQ-2026-09-17-gerador-aplica-o-template-de-consumidor-
ao-proprio-produtor-e-o-doctor-prescreve-desfazer-a-correcao)

Afirma: todo workflow commitado deste repositório que execute "trackfw validate"
compila o binário do trackfw do código do próprio PR — prova positiva de um passo
"go build .../cmd/trackfw" no MESMO JOB que executa "trackfw validate".

ML-1D (isolamento por job): a prova positiva é exigida no bloco do job que contém
"trackfw validate". O "go build" em outro job não satisfaz o requisito.

ML-1C (lógica invertida): reprova por AUSÊNCIA de prova positiva, não por
reconhecimento de mecanismo. Qualquer mecanismo de obtenção de binário publicado
(npm, pip, brew, install.sh | sh, download-artifact, imagem pré-construída, ou
outro mecanismo ainda desconhecido) reprova por não ter o "go build" exigido.

Escopo: workflows commitados em .github/workflows/*.yml e .github/workflows/*.yaml
que executam "trackfw validate".

Residual declarado — fora do escopo deste gate:
  (1) Um workflow que rode "trackfw doctor" ou "trackfw status" com binário publicado
      está fora do escopo (não executa "trackfw validate"). Decisão explícita.
  (2) GitLab CI: buildGitLabCIWorkflowContent (scaffold.go:2021) emite "install.sh | sh"
      sem braço de produtor. O caminho que o builder escreve é ".gitlab-ci-trackfw.yml"
      (constante GitLabCIWorkflowPath, scaffold.go:1947). Em 2026-09-17, "git ls-files
      '.gitlab-ci*'" retorna vazio — o arquivo não está commitado. O gate não cobre
      .github/workflows/-externo; cobertura de template é responsabilidade do
      check-ci-workflow-pin-parity.sh ou de gate dedicado se o arquivo vier a ser
      commitado.

Exceções legítimas:
  Para adicionar uma exceção nomeada, inclua o basename em ALLOWED_EXCEPTIONS com
  o motivo. NÃO alargue o padrão de prova positiva.
  Hoje: nenhuma exceção — todos os workflows deste repositório que executam
  "trackfw validate" compilam do fonte.

Enumeração: git ls-files — apenas arquivos commitados; GitHub Actions aceita .yml e
.yaml; ambas as extensões são enumeradas. Arquivos com outras extensões em
.github/workflows/ geram aviso.

SKIP_COUNTER_ARMS=1: pula a seção de contra-braço (para chamadas externas que
não querem reexecutar os fixtures; o Makefile não passa esta variável).
"""

import io
import os
import re
import shutil
import subprocess
import sys
import tempfile
import textwrap

try:
    import yaml
    HAVE_YAML = True
except ImportError:
    HAVE_YAML = False

SCRIPT_NAME = os.path.basename(__file__)

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

# Pattern: a "run:" step line that compiles trackfw from source.
# Requires whitespace before the path component to avoid matching spurious
# occurrences of "cmd/trackfw" in comments or other contexts.
# Examples matched:
#   go build -o /usr/local/bin/trackfw ./cmd/trackfw
#   go build -o /tmp/x ./cmd/trackfw
#   go build ./cmd/trackfw
BUILD_RE = re.compile(
    r'go\s+build\b.*\s(?:\./|[\w./_-]+/)cmd/trackfw(?:\s|$)'
)

VALIDATE_STR = 'trackfw validate'

# basename → reason; empty today.
ALLOWED_EXCEPTIONS: dict = {}


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def strip_bash_comments(text: str) -> str:
    """
    Strip lines whose first non-whitespace character is '#' (bash comments
    inside "run:" blocks). This prevents a line like "# go build ./cmd/trackfw"
    from satisfying the proof requirement.
    YAML-level comments are already stripped by the YAML parser and never
    appear in the parsed string values.
    """
    out = []
    for line in text.splitlines():
        stripped = line.lstrip()
        if stripped and stripped[0] == '#':
            continue
        out.append(line)
    return '\n'.join(out)


def _collect_run_texts(steps: list) -> list:
    """Return all non-empty run: values from a steps list."""
    runs = []
    for step in steps:
        if not isinstance(step, dict):
            continue
        run = step.get('run')
        if run and isinstance(run, str):
            runs.append(run)
    return runs


def job_runs_validate(steps: list) -> bool:
    """Return True if any step in the job runs 'trackfw validate'."""
    for run in _collect_run_texts(steps):
        if VALIDATE_STR in strip_bash_comments(run):
            return True
    return False


def job_has_source_build(steps: list) -> bool:
    """
    Return True if the job has a step that compiles trackfw from source
    (matches BUILD_RE after comment stripping).
    """
    for run in _collect_run_texts(steps):
        if BUILD_RE.search(strip_bash_comments(run)):
            return True
    return False


# ---------------------------------------------------------------------------
# File scanning
# ---------------------------------------------------------------------------

def scan_workflow_file(filepath: str, relpath: str) -> tuple:
    """
    Parse one workflow file with PyYAML and check job-level provenance.

    Returns:
      (failures: list[str], counted: bool)
      failures: error messages (empty = file OK).
      counted:  True if the file contains at least one job with 'trackfw validate'
                (i.e., this file was "checked" for the vacuity guard).
    """
    try:
        with open(filepath, 'r', encoding='utf-8') as fh:
            data = yaml.safe_load(fh)
    except Exception as exc:
        # Parse errors are non-fatal warnings; don't count the file.
        print(
            f"{SCRIPT_NAME}: AVISO — parse error em {relpath}: {exc}",
            file=sys.stderr,
        )
        return [], False

    if not data or not isinstance(data, dict):
        return [], False

    jobs = data.get('jobs') or {}
    if not isinstance(jobs, dict):
        return [], False

    failures = []
    counted = False

    for job_id, job in jobs.items():
        if not job or not isinstance(job, dict):
            continue
        steps = job.get('steps') or []
        if not job_runs_validate(steps):
            continue

        # This job runs 'trackfw validate' — it is in scope.
        counted = True
        bn = os.path.basename(relpath)

        if bn in ALLOWED_EXCEPTIONS:
            reason = ALLOWED_EXCEPTIONS[bn]
            print(f"OK excecao nomeada: {relpath} (job: {job_id}) — {reason}")
            continue

        if not job_has_source_build(steps):
            failures.append(
                f"{SCRIPT_NAME}: FALHA — {relpath} (job: {job_id}) executa "
                f"'trackfw validate' sem compilar o trackfw do codigo do PR "
                f"(nenhum passo 'go build .../cmd/trackfw' encontrado no job)"
            )

    return failures, counted


# ---------------------------------------------------------------------------
# Directory scanning
# ---------------------------------------------------------------------------

def enumerate_workflows(root: str) -> list:
    """Return relative paths to committed .yml/.yaml workflow files."""
    paths = []
    for glob in ('.github/workflows/*.yml', '.github/workflows/*.yaml'):
        try:
            result = subprocess.run(
                ['git', 'ls-files', glob],
                capture_output=True, text=True, cwd=root,
            )
            for line in result.stdout.splitlines():
                line = line.strip()
                if line:
                    paths.append(line)
        except Exception:
            pass
    return paths


def scan_dir(root: str) -> tuple:
    """
    Scan root for workflow files.

    Returns:
      (main_fail: bool, checked: int)
      Prints failure messages to stderr; OK messages to stdout.
    """
    workflows = enumerate_workflows(root)

    # Vacuity: detect unexpected extensions in .github/workflows/
    try:
        all_result = subprocess.run(
            ['git', 'ls-files', '.github/workflows/'],
            capture_output=True, text=True, cwd=root,
        )
        all_files = [l.strip() for l in all_result.stdout.splitlines() if l.strip()]
    except Exception:
        all_files = []

    total_all = len(all_files)
    total_enum = len(workflows)
    if total_all > total_enum:
        skipped = total_all - total_enum
        print(
            f"{SCRIPT_NAME}: AVISO — {skipped} arquivo(s) em .github/workflows/ "
            f"com extensao diferente de .yml/.yaml ignorados na enumeracao",
            file=sys.stderr,
        )

    main_fail = False
    main_checked = 0

    for wf in workflows:
        full = os.path.join(root, wf)
        failures, counted = scan_workflow_file(full, wf)
        if counted:
            main_checked += 1
        for msg in failures:
            print(msg, file=sys.stderr)
            main_fail = True

    # Vacuity guard: the gate must have measured something.
    if main_checked == 0:
        print(
            f"{SCRIPT_NAME}: FALHA (vacuidade) — nenhum workflow com "
            f"'trackfw validate' encontrado em .github/workflows/; o gate nao mediu nada",
            file=sys.stderr,
        )
        main_fail = True

    return main_fail, main_checked


# ---------------------------------------------------------------------------
# Counter-arm infrastructure
# ---------------------------------------------------------------------------

def _write_fixture(tmpdir: str, name: str, filename: str, content: str) -> str:
    """
    Create a throwaway git repo at tmpdir/name with one workflow file.
    Returns the repo root.
    """
    d = os.path.join(tmpdir, name)
    wf_dir = os.path.join(d, '.github', 'workflows')
    os.makedirs(wf_dir, exist_ok=True)
    fpath = os.path.join(wf_dir, filename)
    with open(fpath, 'w', encoding='utf-8') as fh:
        fh.write(textwrap.dedent(content))
    subprocess.run(['git', 'init', '-q'], cwd=d, capture_output=True)
    subprocess.run(['git', 'add', '-A'], cwd=d, capture_output=True)
    return d


def _capture_scan(fixture_dir: str) -> tuple:
    """
    Run scan_dir(fixture_dir) and capture its stderr output.
    Returns (fail: bool, stderr_text: str).
    """
    buf = io.StringIO()
    old_stderr = sys.stderr
    sys.stderr = buf
    try:
        fail, _ = scan_dir(fixture_dir)
    finally:
        sys.stderr = old_stderr
    return fail, buf.getvalue()


def _assert_fails(fixture_dir: str, filename: str, label: str, counter_fail: list) -> None:
    """Verify that the fixture fails AND the output names the file."""
    fail, output = _capture_scan(fixture_dir)
    if not fail:
        print(
            f"{SCRIPT_NAME}: FALHA (contra-braco {label}) — deveria retornar RC!=0 mas retornou RC=0",
            file=sys.stderr,
        )
        counter_fail.append(label)
        return
    if filename not in output:
        print(
            f"{SCRIPT_NAME}: FALHA (contra-braco {label}) — output nao nomeia o arquivo '{filename}'",
            file=sys.stderr,
        )
        print(f"  output capturado: {output}", file=sys.stderr)
        counter_fail.append(label)
        return
    print(f"OK contra-braco: {filename} reprova nomeando o arquivo ({label})")


def _assert_fails_with_job(
    fixture_dir: str, filename: str, job_id: str, label: str, counter_fail: list
) -> None:
    """Verify that the fixture fails AND the output names BOTH the file AND the job."""
    fail, output = _capture_scan(fixture_dir)
    if not fail:
        print(
            f"{SCRIPT_NAME}: FALHA (contra-braco {label}) — deveria retornar RC!=0 mas retornou RC=0",
            file=sys.stderr,
        )
        counter_fail.append(label)
        return
    missing = []
    if filename not in output:
        missing.append(f"arquivo '{filename}'")
    if job_id not in output:
        missing.append(f"job '{job_id}'")
    if missing:
        print(
            f"{SCRIPT_NAME}: FALHA (contra-braco {label}) — output nao nomeia {' e '.join(missing)}",
            file=sys.stderr,
        )
        print(f"  output capturado: {output}", file=sys.stderr)
        counter_fail.append(label)
        return
    print(f"OK contra-braco: {filename} (job: {job_id}) reprova nomeando arquivo e job ({label})")


def _assert_passes(fixture_dir: str, label: str, counter_fail: list) -> None:
    """Verify that the fixture passes (RC=0)."""
    fail, _ = _capture_scan(fixture_dir)
    if fail:
        print(
            f"{SCRIPT_NAME}: FALHA (contra-braco {label}) — fixture deveria aprovar (RC=0) mas falhou",
            file=sys.stderr,
        )
        counter_fail.append(label)
        return
    print(f"OK contra-braco: {label} aprovado (gate nao e uniformemente vermelho)")


# ---------------------------------------------------------------------------
# Counter-arms: 8 fixtures total
#   5 bad (original ML-1C): npm, pip, brew, artifact, .yaml-extension
#   1 good (original ML-1C): single-job source build
#   1 bad  (ML-1D new):      split — go build in a DIFFERENT job
#   1 good (ML-1D new):      multi-job legit — go build in the SAME job that validates
# ---------------------------------------------------------------------------

def run_counter_arms() -> bool:
    """Run all 8 counter-arms. Returns True if all pass."""
    tmpdir = tempfile.mkdtemp(prefix='trackfw-binary-provenance.')
    try:
        counter_fail: list = []

        # --- bad-1: npm install -g trackfw
        # Reconciliacao (ML-1C): o gate invertido reprova npm porque nenhum passo
        # "go build .../cmd/trackfw" existe no job que valida — npm e binario publicado.
        d = _write_fixture(tmpdir, 'bad-npm', 'w-npm.yml', """\
            name: w-npm
            on: [pull_request]
            jobs:
              job:
                runs-on: ubuntu-latest
                steps:
                  - uses: actions/checkout@v4
                  - run: npm install -g trackfw
                  - run: trackfw validate
            """)
        _assert_fails(d, 'w-npm.yml', 'bad-npm', counter_fail)

        # --- bad-2: pip install trackfw
        # Reconciliacao (ML-1C): o gate invertido reprova pip porque nenhum passo
        # "go build .../cmd/trackfw" existe no job que valida — pip e binario publicado.
        d = _write_fixture(tmpdir, 'bad-pip', 'w-pip.yml', """\
            name: w-pip
            on: [pull_request]
            jobs:
              job:
                runs-on: ubuntu-latest
                steps:
                  - uses: actions/checkout@v4
                  - run: pip install trackfw
                  - run: trackfw validate
            """)
        _assert_fails(d, 'w-pip.yml', 'bad-pip', counter_fail)

        # --- bad-3: brew install
        # Reconciliacao (ML-1C): o gate invertido reprova brew porque nenhum passo
        # "go build .../cmd/trackfw" existe no job que valida — brew e binario publicado.
        d = _write_fixture(tmpdir, 'bad-brew', 'w-brew.yml', """\
            name: w-brew
            on: [pull_request]
            jobs:
              job:
                runs-on: ubuntu-latest
                steps:
                  - uses: actions/checkout@v4
                  - run: brew install kgsaran/tap/trackfw
                  - run: trackfw validate
            """)
        _assert_fails(d, 'w-brew.yml', 'bad-brew', counter_fail)

        # --- bad-4: actions/download-artifact@v4
        # Reconciliacao (ML-1C): o gate invertido reprova download-artifact porque nenhum
        # passo "go build .../cmd/trackfw" existe no job que valida — artefato
        # pre-construido e binario publicado.
        d = _write_fixture(tmpdir, 'bad-artifact', 'w-artifact.yml', """\
            name: w-artifact
            on: [pull_request]
            jobs:
              job:
                runs-on: ubuntu-latest
                steps:
                  - uses: actions/checkout@v4
                  - uses: actions/download-artifact@v4
                    with:
                      name: trackfw-binary
                  - run: chmod +x trackfw && sudo mv trackfw /usr/local/bin/
                  - run: trackfw validate
            """)
        _assert_fails(d, 'w-artifact.yml', 'bad-artifact', counter_fail)

        # --- bad-5: extensao .yaml com install.sh | sh
        # Reconciliacao (ML-1C): a enumeracao cobre *.yaml; este fixture prova que um
        # arquivo .yaml com binario publicado e detectado — o ML-1B ignorava .yaml.
        d = _write_fixture(tmpdir, 'bad-yaml', 'w-yaml.yaml', """\
            name: w-yaml
            on: [pull_request]
            jobs:
              job:
                runs-on: ubuntu-latest
                steps:
                  - uses: actions/checkout@v4
                  - run: |
                      curl -sSfL https://github.com/kgsaran/trackfw/releases/latest/download/install.sh | sh
                  - run: trackfw validate
            """)
        _assert_fails(d, 'w-yaml.yaml', 'bad-yaml', counter_fail)

        # --- good-1: single-job, compila do fonte
        # Reconciliacao (ML-1C): o gate invertido nao e uniformemente vermelho — um workflow
        # que compila do fonte e verificado e nao sinalizado.
        d = _write_fixture(tmpdir, 'good-source', 'w-source.yml', """\
            name: w-source
            on: [pull_request]
            jobs:
              governance-go-install:
                runs-on: ubuntu-latest
                steps:
                  - uses: actions/checkout@v4
                  - uses: actions/setup-go@v7
                    with:
                      go-version-file: go.mod
                  - run: go build -o /usr/local/bin/trackfw ./cmd/trackfw
                  - run: trackfw validate
            """)
        _assert_passes(d, 'good-source', counter_fail)

        # --- bad-6: split — go build em job DIFERENTE do que valida
        # Reconciliacao (ML-1D): o gate agora isola o job; um "go build" em job separado
        # nao satisfaz a prova positiva exigida no job que executa "trackfw validate".
        # O output deve nomear o arquivo E o job "governance".
        d = _write_fixture(tmpdir, 'bad-split', 'w-split.yml', """\
            name: split
            on: [pull_request]
            jobs:
              build-something-else:
                runs-on: ubuntu-latest
                steps:
                  - uses: actions/checkout@v4
                  - run: go build -o /tmp/x ./cmd/trackfw
              governance:
                runs-on: ubuntu-latest
                steps:
                  - uses: actions/checkout@v4
                  - run: npm install -g trackfw
                  - run: trackfw validate
            """)
        _assert_fails_with_job(d, 'w-split.yml', 'governance', 'bad-split', counter_fail)

        # --- good-2: multi-job legit — go build no MESMO job que valida
        # Reconciliacao (ML-1D): a correcao nao e uniformemente vermelha — um workflow
        # multi-job em que o job que valida tem "go build" no proprio bloco aprova,
        # mesmo que outro job nao tenha nenhum build.
        d = _write_fixture(tmpdir, 'good-multijob', 'w-multijob.yml', """\
            name: multi-job-legit
            on: [pull_request]
            jobs:
              build-release:
                runs-on: ubuntu-latest
                steps:
                  - uses: actions/checkout@v4
                  - run: npm install something-unrelated
              governance:
                runs-on: ubuntu-latest
                steps:
                  - uses: actions/checkout@v4
                  - uses: actions/setup-go@v7
                    with:
                      go-version-file: go.mod
                  - run: go build -o /usr/local/bin/trackfw ./cmd/trackfw
                  - run: trackfw validate
            """)
        _assert_passes(d, 'good-multijob (multi-job, prova no job que valida)', counter_fail)

        return len(counter_fail) == 0

    finally:
        shutil.rmtree(tmpdir, ignore_errors=True)


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main() -> None:
    root = sys.argv[1] if len(sys.argv) > 1 else '.'
    skip_counter_arms = os.environ.get('SKIP_COUNTER_ARMS', '0') == '1'

    if not HAVE_YAML:
        print(
            f"{SCRIPT_NAME}: ERRO — PyYAML nao encontrado. Instale com: pip install pyyaml",
            file=sys.stderr,
        )
        sys.exit(2)

    main_fail, main_checked = scan_dir(root)

    if skip_counter_arms:
        sys.exit(1 if main_fail else 0)

    counter_ok = run_counter_arms()

    if main_fail or not counter_ok:
        print(f"{SCRIPT_NAME}: FALHOU", file=sys.stderr)
        sys.exit(1)

    print(
        f"{SCRIPT_NAME}: OK — {main_checked} workflow(s) com 'trackfw validate' "
        f"verificados, todos compilam do fonte do PR (isolamento por job)"
    )


if __name__ == '__main__':
    main()
