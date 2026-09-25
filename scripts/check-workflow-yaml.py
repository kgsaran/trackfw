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


GATE_WF = ".github/workflows/pr-closing-keyword.yml"
QUALITY_WF = ".github/workflows/quality.yml"
ANNOTATIONS_WF = ".github/workflows/check-annotations.yml"
GATE_JOB = "pr-closing-keyword"
GATE_WF_NAME = "PR Closing Keyword"
REQUIRED_TYPES = {"opened", "synchronize", "reopened", "edited"}


def _on(doc: object) -> dict:
    """`on:` é lido pelo YAML 1.1 como o booleano True — aceite as duas grafias."""
    if not isinstance(doc, dict):
        return {}
    block = doc.get("on", doc.get(True))
    return block if isinstance(block, dict) else {}


def check_pr_closing_keyword_trigger(docs: dict) -> list[str]:
    """ML-N3 (REQ-2026-09-05) — o gatilho do gate de palavra-chave de fechamento.

    Reconciliação, uma frase por afirmação:
      (1) `edited` em `pr-closing-keyword.yml` AFIRMA que o gate reavalia quando o
          corpo do PR muda sem commit novo — o AC1 desta REQ, que o default
          `opened · synchronize · reopened` não cobre.
      (2) a AUSÊNCIA de `edited` em `quality.yml` AFIRMA que essa reavaliação não
          passou a disparar as 13 suítes daquele arquivo (o custo que faria alguém
          desligar o gate); medido em 2026-09-25: 11 jobs sem `if:` + 2 com
          `if: always()`, nenhum com `if:` que exclua `edited`.
      (3) o gate existir em UM só workflow AFIRMA que há um único check context —
          pré-requisito do ML-N4, que decide `required_status_checks` por nome.
      (4) `GH_TOKEN` + `pull-requests: read` AFIRMAM que o caminho da API do gate
          (entregue no #416) está LIGADO — sem eles ele lê o payload imutável, que
          foi como o PR #293 mergeou verde com `**Não fecha #290**` no corpo.
      (5) o nome do workflow em `check-annotations.yml` AFIRMA que tirar o job do
          "Quality" não o tirou da verificação de anotações.

    Guarda de vacuidade: arquivo ausente é ERRO, nunca silêncio — um verificador
    que não acha o que verifica e sai 0 é o defeito que este projeto já pagou.
    """
    errors: list[str] = []

    gate = docs.get(GATE_WF)
    if gate is None:
        return [f"  {GATE_WF} — ausente: o gate de palavra-chave precisa de workflow próprio (ML-N3)"]

    if gate.get("name") != GATE_WF_NAME:
        errors.append(f"  {GATE_WF} — `name:` deve ser exatamente {GATE_WF_NAME!r} (check-annotations.yml o referencia por nome)")

    pr_on = _on(gate).get("pull_request")
    types = set(pr_on.get("types") or []) if isinstance(pr_on, dict) else set()
    missing = REQUIRED_TYPES - types
    if missing:
        errors.append(
            f"  {GATE_WF} — `on.pull_request.types` sem {sorted(missing)}; "
            "sem `edited` o gate nunca reavalia um corpo corrigido (AC1)"
        )

    jobs = gate.get("jobs") or {}
    job = jobs.get(GATE_JOB)
    if not isinstance(job, dict):
        errors.append(f"  {GATE_WF} — job id `{GATE_JOB}` ausente (o check context é o id)")
    else:
        if "name" in job:
            errors.append(
                f"  {GATE_WF} — job `{GATE_JOB}` NÃO pode ter `name:`: o nome do check "
                "passaria a ser esse, e required-status-checks.txt lista o id"
            )
        perms = job.get("permissions")
        if not isinstance(perms, dict) or perms.get("contents") != "read" \
                or perms.get("pull-requests") != "read":
            errors.append(
                f"  {GATE_WF} — job `{GATE_JOB}` precisa de `permissions: {{contents: read, "
                "pull-requests: read}`: permissions de job SUBSTITUI o do workflow, então "
                "omitir `contents` quebra o checkout e omitir `pull-requests` mata o `gh pr view`"
            )
        steps = job.get("steps") or []
        if not any((st.get("env") or {}).get("GH_TOKEN") for st in steps if isinstance(st, dict)):
            errors.append(
                f"  {GATE_WF} — nenhum step com `GH_TOKEN`: sem token o gate lê o payload "
                "imutável e um corpo corrigido por edição continua medido pelo texto antigo"
            )

    quality = docs.get(QUALITY_WF)
    if quality is None:
        errors.append(f"  {QUALITY_WF} — ausente (guarda de vacuidade)")
    else:
        if GATE_JOB in (quality.get("jobs") or {}):
            errors.append(
                f"  {QUALITY_WF} — job `{GATE_JOB}` ainda existe aqui: dois workflows emitindo "
                "o mesmo check dão dois contextos para um gate só"
            )
        q_pr = _on(quality).get("pull_request")
        q_types = set(q_pr.get("types") or []) if isinstance(q_pr, dict) else set()
        if "edited" in q_types:
            errors.append(
                f"  {QUALITY_WF} — `edited` no gatilho dispara TODOS os jobs deste arquivo "
                "a cada edição de descrição de PR; o gate que precisa de `edited` mora em "
                f"{GATE_WF}"
            )

    ann = docs.get(ANNOTATIONS_WF)
    if ann is None:
        errors.append(f"  {ANNOTATIONS_WF} — ausente (guarda de vacuidade)")
    else:
        wr = _on(ann).get("workflow_run")
        watched = (wr.get("workflows") or []) if isinstance(wr, dict) else []
        if GATE_WF_NAME not in watched:
            errors.append(
                f"  {ANNOTATIONS_WF} — `workflows:` não observa {GATE_WF_NAME!r}: tirar o job "
                "do 'Quality' sem isso perde a verificação de anotação em silêncio"
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
docs: dict = {}
for f in files:
    try:
        with open(f, encoding="utf-8") as fh:
            doc = yaml.safe_load(fh)
        docs[f.replace("\\", "/")] = doc if isinstance(doc, dict) else None
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

trigger_errors = check_pr_closing_keyword_trigger(docs)
if trigger_errors:
    for err in trigger_errors:
        print(f"FAIL: gatilho do gate pr-closing-keyword (ML-N3):\n{err}", file=sys.stderr)
    fail += 1
else:
    print("ok: gatilho do gate pr-closing-keyword (edited próprio, quality.yml intocado)")
    ok += 1

print(f"\ncheck-workflow-yaml: {ok} passed, {fail} failed")
sys.exit(0 if fail == 0 else 1)
