---
status: wip
date: 2026-09-03
squad: hefesto-tf
req: "docs/req/REQ-2026-09-03-a-raiz-do-gitattributes-nao-declara-eol-para-os-fontes-e-217-arquivos-go-chegam-em-crlf-no-windows.md"
---

# Roadmap: Declarar `eol=lf` para os fontes na raiz do `.gitattributes`

> Criado em: 2026-09-03 | Status: wip

## Context

REQ: docs/req/REQ-2026-09-03-a-raiz-do-gitattributes-nao-declara-eol-para-os-fontes-e-217-arquivos-go-chegam-em-crlf-no-windows.md

## Diagnóstico

Medido em clone local com `core.autocrlf=true` — o default do runner Windows:

```
arquivos .go com CRLF no checkout:  217
total de arquivos .go:              217
```

O PR #254 do reporter (mergeado) removeu o impedimento — os 3 testes passaram de **igualdade** para
**contenção** — mas **não acrescentou a linha**. Verificado na `main`: a raiz tem só
`.trackfw-log merge=union`.

**Maior redução de contagem de Windows ainda disponível, e custa uma linha.**

## Acceptance Criteria

- [x] `eol=lf` declarado para os fontes dos 3 stacks, enumerados **por medição**
- [x] Falsificação: com a regra 0 arquivos em CRLF; sem ela, 217
- [x] 🔴 Os 3 testes de contenção do #254 continuam passando
- [x] 🔴 A metade de **produto** do CRLF permanece **visível**
- [x] `merge=union` intacto; sem ruído de renormalização
- [x] `make quality` verde

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 1 — A regra
> Dependências: nenhuma. PR #254 já mergeado.

### ML-1A — `eol=lf` na raiz, enumerado por medição
**Status:** ✅ Concluído
**Agente:** `hefesto-tf`
**Files affected:** `.gitattributes` (raiz)

**Ações:**
1. Clonar com `-c core.autocrlf=true` em `$(mktemp -d)` e **listar** o que chega em CRLF. A lista é
   a fonte do escopo — não presumir por extensão.
2. Declarar `eol=lf` para os fontes e para artefatos que testes leem **como texto**.

🔴 **A linha que separa este ML de esconder um defeito:** o parser de frontmatter é **cego a CRLF** e
emitiu frontmatter duplicado em `TestRenderOpenCodeAgent`. Declarar `eol` sobre os **assets que o
produto processa** esconderia isso. O ML-1C mediu e **removeu** o pin que já tinha feito, porque
fixar só o lado esperado **cura zero** e apaga a evidência. **Enumere o que fica de fora, e por quê.**

🔴 **Os 3 testes de contenção do #254 são o controle mais importante.** Eles comparam a raiz com uma
constante de **produto** (`GITATTRIBUTES_BLOCK`, nos 3 geradores). A regra nova entra **fora** do
bloco, e a contenção tem de continuar satisfeita — nos **3** runtimes, porque o Python passa por
universal newlines e medir num só engana.

**Critérios de aceite:**
- [x] Clone `autocrlf=true`: **0** arquivos de fonte em CRLF (antes: 217 `.go`)
- [x] 🔴 Falsificação: removendo a regra, volta a 217
- [x] 🔴 `go test ./internal/generators/ -run GitAttributes`, `npm test`, `pytest` — os 3 testes de
      contenção **passam**
- [x] 🔴 Enumeração escrita do que ficou **de fora** por ser entrada de produto
- [x] `git check-attr merge -- .trackfw-log` → `union`
- [x] `git ls-files --eol` sem ruído de renormalização
- [x] `make quality` verde e `trackfw validate` exit 0


**Evidência de aceite — auditoria do arquiteto (2026-09-03), verificada por mim:**

```
clone --local -c core.autocrlf=true, COM a regra   -> .go em CRLF: 0   (antes: 217/217)
3 testes de contencao do #254: Go ok · Python 5 passed · Node 5 pass / 0 fail
git check-attr merge -- .trackfw-log             -> union (2 caminhos)
git ls-files --eol | grep -c i/crlf              -> 0     (sem renormalizacao)
make quality QUALITY_EXIT=0, zero FAIL · validate exit 0
```

🔴 **A medição do agente é MAIOR que a minha.** Eu medi 217 `.go`; ele mediu **1.559 de 1.591**
arquivos versionados chegando em CRLF — 217/217 `.go`, 156/166 `.js`, 153/153 `.py`. E **nenhum blob
tem CRLF no índice** (1583 `i/lf`, 0 `i/crlf`), então `eol` afeta só o **checkout**: não renormaliza
nada.

🔴 **Ele nomeou a classe do defeito, que eu não tinha articulado:**
`arquivo de checkout` × `constante compilada no binário`. Uma raw string em Go, um template literal
em JS ou um bloco de aspas triplas em Python **carrega o `\r` do checkout para dentro do valor** e
diverge da constante, que está sempre em LF.

**E a consequência define o escopo:** comparação entre **dois arquivos de checkout** é **simétrica**
em CRLF e **não quebra** — por isso o escopo é **fonte**, não asset. Sem essa distinção eu teria
pedido `*.md` também, e escondido o defeito do parser.

🔴 **`text=auto` em vez de `text`, por um motivo que eu não previa:**
`npm/src/integrations/doctor.js` e `npm/src/validator/index.js` contêm **bytes NUL literais** como
separador de chave (`join('\0')`), então o git os classifica como binários. `text` atropelaria a
detecção; `text=auto` a preserva. **É a mesma propriedade que faz o `grep` sem `-a` pular esse
arquivo** — um defeito, dois sintomas, e o segundo já produziu 2 premissas falsas neste repositório.

**A linha do ML-1C foi mantida, com enumeração escrita.** Nada de `*.md`, porque o parser é cego a
CRLF. Ficaram de fora, cada um com contagem e motivo: `*/integrations/assets/**` (90), os goldens
(4), o corpus da barreira (144), `*.md` em geral (919), `*.ps1` (CRLF é o nativo correto no Windows)
e `Formula/trackfw.rb` (Homebrew, nunca sob `autocrlf`).

**Nota de processo:** o agente caiu por erro `529` de servidor **ao entregar o relatório**, tendo
escrito o `.gitattributes` antes — decisão correta, e a quarta queda de servidor da sessão sem perda
de trabalho. Toda a evidência acima foi produzida por mim, não herdada do relatório.

**Comandos de validação:** `git clone --local -c core.autocrlf=true`, `make quality`

## Verificação que só o CI fecha

A contagem de Windows na `main`. **É a mesma medição da Wave 2 da REQ de desmascaramento** — as duas
fecham no mesmo run, e por isso este ML entra **antes** de disparar a medição.

## Barreira final

Arquiteto. **Sem `hades-tf`** — configuração de repositório, sem superfície de ataque.
