---
title: MSYS/Git-Bash só converte caminho POSIX quando é o TOKEN INTEIRO — embutido numa string maior, não converte
tags: [windows, msys, git-bash, python, path, gotcha]
date: 2026-09-07
related: [[isolacao-de-home-no-windows-o-defeito-e-divergencia-de-canal-nao-vacuidade-2026-09-03]]
---

## O sintoma

`scripts/check-update-parity.sh`, Cenário 6 (fixture, não o teste que o Cenário 17 do
`check-gates-falsify.sh` pretende provar), roda `install_agent_global go ...` — o binário Go imprime
"install complete" (sucesso) — e a linha seguinte, um `python3 -c "...json.load(open('$manifest'))..."`
que lê o manifest recém-escrito, reprova com `FileNotFoundError`, mesmo caminho, mesmo processo, sem
nenhuma escrita concorrente.

## A causa — medida, não suposta

Git-Bash/MSYS converte automaticamente um caminho estilo POSIX (`/tmp/...`) para o caminho Windows
real **quando esse caminho é o valor INTEIRO** de um argv token ou de uma env var — mas **não**
quando o mesmo literal está **embutido dentro de uma string maior** (ex.: um script `python -c
"...open('/tmp/x')..."` — o argv que o `python -c` recebe é o SCRIPT inteiro, não o caminho isolado).

Medido em VM Windows 10 ARM64, sem o shim de PATH deste ML e sem nenhum código do trackfw envolvido
(python.exe nativo direto):

```bash
touch /tmp/probe-file
/c/.../python.exe -c "print(open('/tmp/probe-file').read())"                      # FileNotFoundError
/c/.../python.exe -c "import sys; print(open(sys.argv[1]).read())" /tmp/probe-file # "hello" — OK
```

A diferença entre as duas chamadas é **só** se `/tmp/probe-file` chega como token isolado (convertido)
ou embutido em uma string maior (não convertido, tratado pelo Win32 como caminho drive-relativo ao
drive corrente do processo — aqui, inexistente sob `C:\tmp\...`, confirmado por `ls /c/tmp` →
"No such file or directory").

No cenário real: `HOME="$home_dir" "$GO_BIN" ...` passa `$home_dir` como valor INTEIRO de env var →
convertido → o Go escreve no lugar certo (confirmado: `find /tmp/trackfw-update-parity.XXX -name
integrations-manifest.json` encontra o arquivo pelo caminho bash-resolvido). O `python3 -c
"...open('$manifest')..."` seguinte embute o mesmo literal DENTRO do script → não convertido →
`FileNotFoundError`.

## Por que não é do escopo deste ML (interpretador/binário)

Reproduzido **idêntico com e sem** o shim de PATH do ML-2A anterior (`FALSIFY_PY_SHIM_DIR`) e com
python.exe chamado por caminho absoluto direto, sem `python3` bare envolvido. É comportamento do
runtime MSYS na fronteira bash→processo nativo, não uma questão de qual `python3`/binário é
escolhido. Categoria "separador de caminho" — deliberadamente fora do escopo de
`ROADMAP-2026-09-07-gates-rodam-no-windows-resolucao-de-interpretador-e-binario` (ML-2A).

## Efeito prático

`check-gates-falsify.sh` é fail-fast (`assert_fails_with` faz `exit 1` sob `set -euo pipefail`) —
em QUALQUER plataforma, não só Windows. No Windows, esse é o primeiro cenário (17) cuja fixture
interna toca esse padrão (`python3 -c "...open('$path')..."` com o caminho por interpolação de bash
dentro do script, não como argv separado), e o gate aborta ali. Scripts que já usam este padrão:
`scripts/check-update-parity.sh:560,617,685` (as três ocorrências de
`art_key=$(python3 -c "import json; d=json.load(open('$manifest'))...")`).

## Se algum dia isso vai ser corrigido

Não é decisão deste ML. Se corrigido, o conserto correto é passar o caminho como **argv separado**
(`python3 -c "import json,sys; d=json.load(open(sys.argv[1])); ..." "$manifest"`), não reescrever a
string manualmente — isso vale para qualquer gate do repo que interpole um caminho DENTRO de um
script `-c`, não só os 3 sítios medidos aqui.
