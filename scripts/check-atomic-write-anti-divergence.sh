#!/usr/bin/env bash
# Gate: as três cópias replicadas de `_atomic_write` do CLI Python (ROADMAP-
# 2026-09-01-escrita-atomica-do-cli-python-funciona-no-windows, ML-1B) não
# divergem entre si no trecho de segurança — o fallback condicional de
# os.fchmod (Unix-only) para os.chmod(path) (Windows).
#
# Por que este gate existe (docs/seguranca/2026-09-01-modelo-de-ameaca-da-
# escrita-atomica-no-windows.md, seção 4.3, veredito do ML-0A): a triplicação
# NÃO é extraída para um helper compartilhado — quarantine.py:34-37 documenta
# que isso é deliberado, para manter o pacote `thirdparty` (que processa
# conteúdo de terceiro, superfície de maior desconfiança do projeto)
# independente de trackfw.integrations. Sem extração, o modo de falha mais
# provável é "corrigir duas de três e esquecer a terceira" — alguém ajusta o
# fallback em identity/__init__.py e integrations/manager.py, não percebe que
# thirdparty/quarantine.py ficou para trás (ou vice-versa), e nada mais no
# projeto pega isso: os três arquivos são independentes por desenho, então
# nenhum import/teste cruzado detectaria a divergência. Este gate é o que
# sustenta a decisão de não extrair — sem ele, a decisão não é defensável.
#
# O QUE ESTE GATE COMPARA, E POR QUÊ (a pergunta que qualquer alteração
# futura a este script precisa responder de novo):
#
# Compara o CORPO NORMALIZADO do bloco de segurança — do `fchmod = getattr(os,
# "fchmod", None)` até o `os.chmod(temporary, mode)` do fallback, inclusive —
# nas três cópias, exigindo IGUALDADE TEXTUAL exata após dedent. Não compara
# o arquivo inteiro, nem a função inteira: as linhas de escrita
# (`stream.write(content)` vs `stream.write(data)`) e o parâmetro de tipo
# (`str` vs `Path`) diferem legitimamente entre os três call sites e não têm
# nenhuma relação com a garantia de TOCTOU — comparar o arquivo inteiro
# produziria falsos positivos a cada refatoração inofensiva de nome de
# variável fora do trecho sensível. O bloco escolhido é exatamente o trecho
# que decide se a garantia do POSIX (fchmod no descritor, sem janela) se
# mantém ou se degrada silenciosamente para chmod no caminho (com janela) —
# é o único trecho onde uma divergência TEM significado de segurança.
#
# Por que "corpo NORMALIZADO" e não três strings fixas (o padrão de
# scripts/check-ref-separator-portability.sh, com um `assert_has` por
# runtime): as três cópias não têm a mesma indentação absoluta —
# `integrations/manager.py` define `_atomic_write` como @staticmethod dentro
# de uma classe (bloco a 12 espaços), enquanto `identity/__init__.py` e
# `thirdparty/quarantine.py` são funções de módulo (bloco a 8 espaços). Uma
# comparação por string fixa exigiria duas constantes diferentes (uma por
# nível de indentação) só para tolerar esse deslocamento incidental — e não
# provaria "as três são iguais entre si", provaria "cada uma bate com uma
# cópia congelada dentro do próprio gate", que é uma propriedade mais fraca:
# se as três derivassem de forma consistente para um texto melhor (ex.:
# reescrever o comentário para citar a versão exata do CPython), a
# comparação-contra-golden-fixo reprovaria as três por igual — falso
# positivo — e exigiria editar o gate toda vez, viciando quem o audita a
# ignorar reprovações. `textwrap.dedent` remove o deslocamento uniforme de
# indentação (a estrutura relativa if/else é preservada — dedent calcula o
# prefixo comum a TODAS as linhas do bloco, não zera cada linha
# individualmente) e a comparação passa a ser as três contra as OUTRAS DUAS,
# que é a propriedade que a REQ pede ("gate falsificado nas duas direções:
# com as três iguais passa; divergindo uma, reprova") — nem frouxa demais
# (não ignora nenhuma palavra do comentário nem a ordem if/else) nem rígida
# demais (não reprova por reformatação de indentação que não muda
# comportamento nenhum).
#
# Duas guardas de vacuidade distintas (docs/cli-parity.md, "Quatro
# propriedades exigidas de todo gate novo de paridade"):
#   1. Arquivo ausente/movido: cada um dos três caminhos é checado por
#      existência ANTES da extração, nomeando o arquivo que falta — nunca
#      "0 blocos encontrados, gate passa em silêncio".
#   2. Extração falhou (âncora `fchmod = getattr(os, "fchmod", None)` ou
#      `os.chmod(temporary, mode)` não encontrada em algum dos três, porque
#      alguém reescreveu o fallback de forma que a âncora não bate mais):
#      reprova nomeando o arquivo, nunca compara silenciosamente 2 de 3.
#   3. Contagem: exige exatamente 3 blocos extraídos com sucesso antes de
#      comparar — pega alguém removendo um dos três `_atomic_write` do
#      escopo do gate sem que a extração dos outros dois esconda isso.
#
# ROOT (arg 1, default ".") é usado tanto pela guarda de existência quanto
# pela extração real — MESMO cwd e MESMOS caminhos das duas etapas (achado
# repetido duas vezes nesta sessão, conforme o roadmap: uma guarda que olha
# um cwd/caminho e a varredura real que olha outro produz "gate passa" sem
# medir nada).
set -euo pipefail

ROOT="${1:-.}"

FILES=(
  "pypi/trackfw/identity/__init__.py"
  "pypi/trackfw/thirdparty/quarantine.py"
  "pypi/trackfw/integrations/manager.py"
)

fail=0

# Guarda de vacuidade 1 — existência, nomeando o arquivo ausente.
for f in "${FILES[@]}"; do
  if [[ ! -f "$ROOT/$f" ]]; then
    echo "check-atomic-write-anti-divergence: arquivo ausente: $f (ROOT=$ROOT)" >&2
    fail=1
  fi
done

if [[ "$fail" -ne 0 ]]; then
  echo "check-atomic-write-anti-divergence: FALHOU — não é possível comparar com arquivo(s) ausente(s)" >&2
  exit 1
fi

python3 - "$ROOT" "${FILES[@]}" <<'PYEOF'
import re
import sys
import textwrap

root = sys.argv[1]
paths = sys.argv[2:]

# Âncoras exatas do trecho de segurança (guarda de capacidade -> fallback
# condicional -> comentário de justificativa -> chmod no caminho). ^ com
# re.M ancora no início de linha para capturar a indentação original da
# primeira linha também — sem isso, o dedent seguinte vê a primeira linha
# com indentação 0 (o ponto em que o regex bateu, não o início da linha) e
# calcula um prefixo comum errado, quebrando a normalização para o bloco de
# integrations/manager.py (staticmethod, um nível a mais de indentação).
PATTERN = re.compile(
    r'(?m)^([ \t]*fchmod = getattr\(os, "fchmod", None\).*?os\.chmod\(temporary, mode\))',
    re.S,
)

blocks = {}
fail = False
for rel in paths:
    full = root.rstrip("/") + "/" + rel
    with open(full, "r", encoding="utf-8") as fh:
        content = fh.read()
    match = PATTERN.search(content)
    if match is None:
        print(
            f"check-atomic-write-anti-divergence: extração falhou em {rel} — "
            f"âncora 'fchmod = getattr(os, \"fchmod\", None)' .. 'os.chmod(temporary, mode)' "
            f"não encontrada (fallback removido, reescrito além do reconhecível, ou função "
            f"renomeada)",
            file=sys.stderr,
        )
        fail = True
        continue
    blocks[rel] = textwrap.dedent(match.group(1)).strip()

# Guarda de vacuidade 3 — exatamente 3 blocos extraídos, não 0, não 2.
if len(blocks) != 3:
    print(
        f"check-atomic-write-anti-divergence: vacuidade — esperava extrair 3 blocos, "
        f"extraiu {len(blocks)}",
        file=sys.stderr,
    )
    fail = True

if fail:
    sys.exit(1)

items = list(blocks.items())
baseline_path, baseline_text = items[0]
diverged = [rel for rel, text in items[1:] if text != baseline_text]

if diverged:
    print(
        "check-atomic-write-anti-divergence: DIVERGÊNCIA no bloco de fallback "
        "os.fchmod -> os.chmod entre as três cópias de _atomic_write:",
        file=sys.stderr,
    )
    print(f"  referência: {baseline_path}", file=sys.stderr)
    for rel in diverged:
        print(f"  diverge: {rel}", file=sys.stderr)
    sys.exit(1)

print(
    f"check-atomic-write-anti-divergence: OK — {len(blocks)} cópias "
    f"({', '.join(rel for rel, _ in items)}) com bloco de fallback idêntico após normalização"
)
PYEOF
