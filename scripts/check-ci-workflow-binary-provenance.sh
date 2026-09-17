#!/usr/bin/env bash
# check-ci-workflow-binary-provenance.sh — thin wrapper (ML-1D)
#
# A implementação completa foi migrada para Python em
# check-ci-workflow-binary-provenance.py (ML-1D: isolamento por job).
# Este wrapper mantém o ponto de entrada original para que o Makefile
# (target parity-rest) e qualquer chamada externa continuem funcionando
# sem alteração.
#
# SKIP_COUNTER_ARMS=1 e o argumento ROOT são repassados ao script Python.

# Codificacao de saida: forca UTF-8 no stdio do python3 invocado abaixo.
# Sem isto, sob console cp1252 (Windows) ou locale não-UTF-8, o python3
# pode falhar ao imprimir caracteres não-ASCII no output do gate.
export PYTHONIOENCODING=utf-8

exec python3 "$(dirname "$0")/check-ci-workflow-binary-provenance.py" "$@"
