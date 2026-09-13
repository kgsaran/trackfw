#!/usr/bin/env python3
"""check-workflow-yaml — verifica sintaxe YAML de todos os .github/workflows/*.yml e *.yaml.

Por que existe: release.yml entrou em PR #352 com Python multilinha em coluna 1 que
quebrava o bloco `run: |` no YAML. `make quality` roda o --self-test do
check-required-status-checks.py (sobre fixtures), mas nunca parseia os workflows reais.
Um workflow sintaticamente inválido atravessou o ciclo local completo e só apareceu no CI.
Corrigido pelo Defeito 3 do PR #352 (ROADMAP-2026-09-12-v8-um-binario-muitos-canais).

Escopo: sintaxe YAML apenas (yaml.safe_load). NÃO valida schema de workflow,
expressões ${{ }} nem dependências entre jobs.

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
            yaml.safe_load(fh)
        print(f"ok: {f}")
        ok += 1
    except yaml.YAMLError as e:
        first_line = str(e).splitlines()[0] if str(e) else str(e)
        print(f"FAIL: {f} — {first_line}", file=sys.stderr)
        fail += 1

print(f"\ncheck-workflow-yaml: {ok} passed, {fail} failed")
sys.exit(0 if fail == 0 else 1)
