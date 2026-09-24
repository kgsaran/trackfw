---
status: Done
date: 2026-09-24
author: "trackfw_architect"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-24-caminho-posix-interpolado-dentro-do-codigo-python-nao-e-convertido-pelo-msys-e-o-open-morre-no-windows.md"
---

# REQ: caminho POSIX interpolado dentro do código Python não é convertido pelo MSYS, e o `open()` morre no Windows

> Date: 2026-09-24 | Status: Done
| Linear Issue:
| Jira Issue:

Origem: **#363** e o **PR #417** (consumidor externo, com medição e censo próprio), mergeado como
`dc95ff34`. Aquele PR corrigiu **5 sítios de 1 arquivo** e **declarou os restantes** em vez de
escondê-los — esta REQ existe para que os declarados não se percam na fila.

## Motivation

O Git Bash converte caminho POSIX → caminho do Windows em **`argv`**, e **não** dentro de uma
string de código. O Python nativo do Windows lê `/tmp` como `C:\tmp`, e o `open()` morre num
diretório que o `mkdir` criou uma linha acima.

Reprodução mínima do reportante, **com controle** — é o que separa as duas formas:

```bash
d=$(mktemp -d "${TMPDIR:-/tmp}/repro.XXXXXX"); mkdir -p "$d/sub"
python3 -c "open('$d/sub/f.json','w')"                              # caminho NO CÓDIGO
python3 - "$d/sub/g.json" <<<"import sys; open(sys.argv[1],'w')"    # caminho em ARGV
python3 -c "open('$(cygpath -m "$d")/sub/h.json','w')"              # controle
```

| forma | resultado |
|---|---|
| interpolado no código | `FileNotFoundError: '/tmp/repro.nb38FM/sub/f.json'` |
| em `argv` | escreve |
| `cygpath -m` no código (controle) | escreve |

```
python3 -c "import os; print(os.path.abspath('/tmp'))"   ->   C:\tmp
```

**Efeito medido:** `check-validate-rule-pins.sh` passava os 5 pins e **morria ao montar a fixture**;
`check-doctor-parity.sh` falhava com `'\tmp\trackfw-doctor-parity.cPqznX\l\project\...'`. Nenhum CI
viu, porque os dois rodam em `parity-other-gates` / `ubuntu-latest`, onde `/tmp` é `/tmp` para os
dois lados.

### Causa distinta do CRLF — e a separação está medida, não presumida

Este projeto exige a medição escrita antes de separar. Ela existe: a REQ do CRLF (`#414`) trata do
`\r` que o `python3` acrescenta ao **stdout**; aqui o stdout é irrelevante e o que quebra é a
**tradução de caminho** na entrada. Corrigir uma **não** fecha a outra — que é o teste de separação
da casa. O próprio reportante separou assim, no #417.

### Medição inicial do arquiteto (aproximada — a Wave 0 refuta ou confirma)

🔴 **Não herde o número do #417.** Ele relata *"15 sítios, 5 corrigidos, 10 restantes"*; minha
varredura, por outro critério, acha **7 candidatos** e um deles já está correto:

```
$ grep -rn '\$' scripts/*.sh | grep -E "(open|Path|os\.path\.join|listdir|makedirs|read_text|write_text)\("
scripts/check-gates-falsify.sh:6493        open('$ROOT_DIR/npm/package.json')
scripts/check-serve-api-file-security.sh:87,92
scripts/check-update-parity.sh:354,379,408
scripts/check-thirdparty-parity.sh:167     ← já usa sys.argv[1]: correto
                                           (+1 falso positivo: literal Go em :4594)
```

Os dois métodos discordam, e **nenhum dos dois é o veredito**. A enumeração real, por critério
aplicável por terceiro, é o primeiro entregável.

## Acceptance Criteria

- [x] **Enumeração real**, classificada em **(a)** interpola caminho no código e escreve/lê arquivo
      → ML-0A: **6 sítios (a)** em 5 blocos, 3 arquivos; reconciliação escrita (o #417 era largo no predicado e estreito na unidade)
      → defeito · **(b)** passa por `argv` ou recebe caminho já convertido → correto · **(c)** o `$`
      não é caminho → fora. Com o comando que produziu a lista, e a **reconciliação escrita** com os
      dois números divergentes de partida (15/10 do #417 vs 7 do arquiteto)
- [x] Todo sítio **(a)** corrigido **por `argv`**, que é o padrão que o próprio repositório já
      → os 6 por `sys.argv` — precedente `check-thirdparty-parity.sh:167`; **0** caminhos interpolados restantes
      pratica (`check-thirdparty-parity.sh:167`, `check-doctor-parity.sh` em `_normalize_version_in_file`)
- [x] 🔴 **Gate que impede a reintrodução**, falsificável, com guarda de não-vacuidade, e com as
      → `check-interpolated-path-in-python.sh` + Cenário 200 (9 braços); dois pisos de não-vacuidade; poupa os 2 não-flag por razões independentes
      formas **não cobertas declaradas** no cabeçalho
- [x] Falsificação nas duas direções, **exercitada no Windows** — é a plataforma onde o defeito vive,
      → VM Windows nos **dois braços**: rc=1 com os 6 antes, rc=0 depois; o rótulo `direction-b-shim-absent` **voltou**
      e é por isso que nenhum CI viu
- [x] `make quality` e **CI** verdes
      → `make quality` RC=0 · 1275 `^OK ` · 0 `: FALHA`; `trackfw barrier` passa nas waves 0, 1 e 2; CI verde no PR #422 (`8a9dba76`)

## Negative scope — o que esta REQ NÃO faz

- **Não** reabre a #363 por conta própria: o reportante mediu que ela **só fecha quando o ramo
  Windows roda inteiro**, o que depende também do cenário do bit de execução em NTFS. Fechar aquela
  issue não é entregável daqui.
- **Não** migra os gates de `python3` para Go. Outra decisão, outro custo, desnecessária para fechar
  esta causa.
- **Não** trata o `\r` de stdout (REQ do CRLF, `#414`, Done) nem a apuração do censo
  (REQ-2026-09-23). Mecanismos distintos, separados com medição.

## Linked ADR
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-24-caminho-posix-interpolado-dentro-do-codigo-python-nao-e-convertido-pelo-msys-e-o-open-morre-no-windows.md`
