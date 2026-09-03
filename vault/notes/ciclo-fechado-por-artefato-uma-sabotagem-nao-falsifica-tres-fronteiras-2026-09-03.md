---
title: Ciclo fechado gerador→verificador: uma sabotagem não falsifica três artefatos, e o status do ADR não é falsificável pela chave do frontmatter
tags: [qa, ciclo-fechado, falsificacao, gate, adr, note, req, paridade]
date: 2026-09-03
related: [[resolvedor-de-req-era-if-else-e-a-uniao-colide-com-o-namespace-vindo-do-disco-2026-09-03]], [[armadilhas-ao-escrever-cenario-em-check-gates-falsify-2026-08-12]], [[promise-flutuante-em-action-do-cli-node-e-invisivel-na-fronteira-2026-09-02]], [[serve-validator-index-detectado-como-binario-grep-silencioso-2026-08-29]]
---

## 1. O reflexo errado: uma sabotagem para falsificar o gate inteiro

Ao montar o ciclo fechado dos três artefatos (`req new`, `adr new`, `note new` → `validate`), o
reflexo é provar a falsificação com **uma** sabotagem — a do resolvedor de REQ, que é o defeito que
motivou a REQ. **Medido: ela falsifica só um terço do gate.**

```
SABOTAGEM A — caso canônico <agente>/*.md removido dos 3 resolvedores
  reprovam:  9 asserções, todas em by_agent (req_has_adr, status-literal, adr_orphan-clears)
  intactas:  as 6 de note_orphan e as 6 de adr_orphan-names, em todos os runtimes
```

A razão é estrutural, e vale para qualquer gate multi-artefato:

- **`adr_orphan` fica MAIS forte sob a sabotagem de REQ.** A regra dispara quando o ADR existe e
  *nenhuma REQ o referencia*. Sumindo com as REQs, o ADR fica mais órfão, não menos — a asserção
  passa com mais folga sobre a árvore quebrada.
- **`note_orphan` nunca toca `req_dir`.** `validateNoteOrphan` lê `vault/notes` e `index.md` e nada
  mais; é insensível por construção.

**Regra de bolso:** a sabotagem tem de atingir a **fronteira daquele artefato**. Três braços, três
seams — verificador para REQ, gerador para NOTE, gerador para ADR. Um gate com uma sabotagem só e
três braços tem dois braços que passam nas duas árvores, ou seja, que não medem nada.

## 2. O status do ADR NÃO é falsificável pela chave do frontmatter — há fallback pelo cabeçalho

Tentativa natural de reproduzir a classe "gerador emite uma chave, verificador lê outra": trocar
`status:` por `state:` no frontmatter emitido por `adr new`, nos 3 runtimes.

```
SABOTAGEM C — 'status:' -> 'state:' no frontmatter dos 3 geradores de ADR
  resultado: EXIT=0, 36/36 asserções verdes   <- NÃO discrimina
```

O verificador prefere o frontmatter mas **cai de volta na linha de cabeçalho**
`> Date: <data> | Status: <status>`, que o mesmo gerador também escreve. A redundância do template
mascara a perda da chave canônica. Consequência prática: **uma regressão que derrube só o campo
machine-readable do ADR passa silenciosa pelo `validate`** — a asserção honesta é sobre o
**vocabulário**, não sobre a chave:

```
SABOTAGEM C' — 'Proposed' -> 'Rascunho' (vocabulário em português)
  reprovam: as 6 asserções de adr/*/status-literal-read-back, nos 3 CLIs
```

## 3. O ponto de decisão do status no Python é o argparse, não o default da função

`generate_adr(title, status='Proposed', ...)` **não** é o ponto de decisão: `commands/adr.py` sempre
passa `status=args.status`. Sabotar o default da função não muda nada — a sabotagem só discrimina no
`add_argument("--status", default=...)`.

E há uma segunda camada: **o argparse valida o default de string contra `choices` mesmo quando a
flag não é passada.** Trocar só o `default` faz o comando abortar por erro de argumento — o gate
reprova pelo motivo errado, e quem auditar conclui que o cenário é bom quando ele está medindo
outra coisa. `default` e `choices` entram na sabotagem juntos.

> Nota de paridade lateral: `--status` e `--dir` do `adr new` são **Python-only**; Go e Node não têm
> equivalente (`docs/cli-parity.md` já registra isso como exceção sem gate cross-CLI).

## 4. `adr_orphan` e `note_orphan` são `warning` — o gate tem de reprovar sozinho

Ambas têm severidade default `warning` (`ruleDefaults`, `internal/validator/validator.go:101`), e
warning **não move o exit code do `validate`**. Um cenário escrito com `assert_fails_with` sobre
`trackfw validate` passaria sempre, provando nada — a armadilha nº2 de
[[armadilhas-ao-escrever-cenario-em-check-gates-falsify-2026-08-12]], que aqui aparece do lado do
*consumidor* do gate. Por isso `check-artifact-closed-cycle.sh` parseia o JSON, decide, e emite o
próprio diagnóstico (`closed cycle broken: <artefato>/<runtime>/<layout>/<asserção>`) — e é **esse
literal**, não o do `validate`, que os Cenários 183/184/185 casam.

## 5. `validate --json` do Go não é JSON puro — `json.load` quebra com "Extra data"

Quando há violação, o Go imprime o objeto JSON **e depois** a linha `N violation(s) found`. Node e
Python imprimem JSON multi-linha, sem cauda. Um parser com `json.load(sys.stdin)` funciona nos
fixtures de 0 violações (é por isso que `check-artifact-parity.sh` nunca esbarrou nisso) e quebra em
qualquer fixture que viole algo. O idioma portável é `raw_decode` a partir da primeira `{`.

## 6. `trackfw init` do Python semeia um ADR que Go e Node não semeiam

Projeto novo: Python cria `docs/adr/ADR-001-inicio-do-projeto.md`; Go e Node não. Logo, o Python sai
do `init` com **1 warning `adr_orphan` a mais** que os outros dois. Divergência pré-existente, não
tocada aqui — mas ela **proíbe qualquer métrica de ciclo fechado baseada em contagem** (de ADRs, de
warnings ou de violações). A métrica tem de ser **por basename do artefato gerado**, que é imune.

## 7. Ao restaurar sabotagem, conferir `npm/src/validator/index.js` com `grep -a`

`grep -c 'SABOTADO' internal/validator/validator.go npm/src/validator/index.js pypi/trackfw/validator.py`
devolveu **duas** linhas, não três: o `index.js` é classificado como binário e é pulado **em
silêncio** ([[serve-validator-index-detectado-como-binario-grep-silencioso-2026-08-29]]). Numa
verificação de restauração isso é pior do que numa varredura: a ausência da linha parece "arquivo
limpo". Confirme com `grep -a` ou com `cmp` contra o backup.

## Relacionado

- `scripts/check-artifact-closed-cycle.sh` — o gate
- `scripts/check-gates-falsify.sh` — Cenários 183 (REQ/verificador), 184 (NOTE/gerador),
  185 (ADR/gerador)
- `docs/cli-parity.md` — seção "Ciclo fechado gerador → verificador"
