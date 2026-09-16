#!/usr/bin/env python3
"""check-workflow-yaml — verifica sintaxe YAML e referências needs: em workflows.

Por que existe: release.yml entrou em PR #352 com Python multilinha em coluna 1 que
quebrava o bloco `run: |` no YAML. `make quality` roda o --self-test do
check-required-status-checks.py (sobre fixtures), mas nunca parseia os workflows reais.
Um workflow sintaticamente inválido atravessou o ciclo local completo e só apareceu no CI.
Corrigido pelo Defeito 3 do PR #352 (ROADMAP-2026-09-12-v8-um-binario-muitos-canais).

Escopo: sintaxe YAML (yaml.safe_load) + validação de needs: (ML-3C).
Needs: cada ID referenciado em needs: deve existir como job no mesmo workflow.
Um needs: apontando para um job removido passa despercebido sem esta verificação.
NÃO valida schema de workflow completo nem expressões ${{ }}.

Guarda de vacuidade: falha se nenhum arquivo for encontrado — um validador
que não valida nada e sai 0 é o defeito que este projeto já pagou quatro vezes.
"""
import glob
import sys

try:
    import yaml
except ImportError:
    print("FAIL: PyYAML não instalado — pip install pyyaml", file=sys.stderr)
    sys.exit(1)


def check_needs(workflow_path: str, doc: object) -> list[str]:
    """Valida que todo ID em needs: existe como job no mesmo workflow.

    Retorna lista de erros (vazia = tudo ok).
    Reconciliação ML-3C: afirma que needs: referencia jobs existentes no mesmo arquivo.
    Falsificação: remover um job sem atualizar needs: → erro detectado aqui (negativo);
    um needs: correto → sem erros (positivo). Guarda de vacuidade: nenhum job com needs:
    → nenhum erro reportado (correto: workflow de single-job não tem needs:).
    """
    errors: list[str] = []
    if not isinstance(doc, dict):
        return errors
    jobs = doc.get("jobs", {})
    if not isinstance(jobs, dict):
        return errors
    job_ids = set(jobs.keys())
    for job_id, job in jobs.items():
        if not isinstance(job, dict):
            continue
        needs = job.get("needs")
        if needs is None:
            continue
        if isinstance(needs, str):
            refs = [needs]
        elif isinstance(needs, list):
            refs = needs
        else:
            continue
        for ref in refs:
            if ref not in job_ids:
                errors.append(
                    f"  job '{job_id}': needs: '{ref}' — job não existe no workflow"
                )
    return errors


files = sorted(
    glob.glob(".github/workflows/*.yml") + glob.glob(".github/workflows/*.yaml")
)

if not files:
    print(
        "FAIL: nenhum arquivo encontrado em .github/workflows/*.yml/.yaml"
        " — guarda de vacuidade",
        file=sys.stderr,
    )
    sys.exit(1)

ok = 0
fail = 0
for f in files:
    try:
        with open(f, encoding="utf-8") as fh:
            doc = yaml.safe_load(fh)
        needs_errors = check_needs(f, doc)
        if needs_errors:
            for err in needs_errors:
                print(f"FAIL: {f} — referência needs: inválida:\n{err}", file=sys.stderr)
            fail += 1
        else:
            print(f"ok: {f}")
            ok += 1
    except yaml.YAMLError as e:
        first_line = str(e).splitlines()[0] if str(e) else str(e)
        print(f"FAIL: {f} — {first_line}", file=sys.stderr)
        fail += 1

print(f"\ncheck-workflow-yaml: {ok} passed, {fail} failed")
sys.exit(0 if fail == 0 else 1)
